// Package runner executes ansible-playbook inside a pseudo-terminal so that
// colour, progress and TTY-only behaviour are preserved byte-for-byte, then
// streams the raw output to callers.
package runner

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/nikiv/ansible-ui/internal/model"
)

// Process is a running command attached to a PTY.
type Process struct {
	cmd        *exec.Cmd
	pty        *os.File
	vaultFiles []string
	agentPID   string // ssh-agent holding inventory connection keys (killed on exit)
}

// startPTY launches an arbitrary command inside a PTY.
func startPTY(bin string, args []string, dir string, env []string, cols, rows int) (*Process, error) {
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.SysProcAttr = procAttr()
	if cols <= 0 {
		cols = 120
	}
	if rows <= 0 {
		rows = 34
	}
	f, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
	if err != nil {
		return nil, fmt.Errorf("start pty: %w", err)
	}
	return &Process{cmd: cmd, pty: f}, nil
}

// Start launches the execution for the given spec inside a PTY, dispatching on
// the app (Ansible by default; Terraform-family and scripts otherwise).
func Start(spec model.ExecSpec) (*Process, error) {
	switch {
	case spec.AppBin != "":
		return startCustom(spec)
	case spec.App == model.AppPulumi:
		return startPulumi(spec)
	case model.IsInfraApp(spec.App):
		return startInfra(spec)
	case model.IsScriptApp(spec.App):
		return startScript(spec)
	default:
		return startAnsible(spec)
	}
}

// startCustom runs a user-defined application: <bin> <appArgs…> <cliArgs…>
// [entrypoint] in a PTY, with extra-vars exported as environment variables.
func startCustom(spec model.ExecSpec) (*Process, error) {
	args := append([]string{}, spec.AppArgs...)
	args = append(args, spec.CliArgs...)
	if strings.TrimSpace(spec.Playbook) != "" {
		args = append(args, spec.Playbook)
	}
	env := buildEnv(spec)
	for k, v := range spec.ExtraVars {
		env = append(env, k+"="+fmt.Sprint(v))
	}
	return startPTY(spec.AppBin, args, spec.Dir, env, spec.Cols, spec.Rows)
}

// startAnsible launches ansible-playbook for the given spec inside a PTY.
func startAnsible(spec model.ExecSpec) (*Process, error) {
	bin := os.Getenv("ANSIBLE_PLAYBOOK_BIN")
	if bin == "" {
		bin = "ansible-playbook"
	}
	// Transient files cleaned up when the run ends.
	var vaultFiles []string

	// Extra-vars (which may include resolved secrets) are written to a transient
	// 0600 file and passed via "-e @file" — so values never touch the command
	// line, the run log, run.args or `ps`.
	var ev []string
	if len(spec.ExtraVars) > 0 {
		if j, err := json.Marshal(spec.ExtraVars); err == nil {
			if f, ferr := os.CreateTemp("", "aui-vars-*.json"); ferr == nil {
				_, _ = f.Write(j)
				_ = f.Close()
				_ = os.Chmod(f.Name(), 0o600)
				vaultFiles = append(vaultFiles, f.Name())
				ev = []string{"-e", "@" + f.Name()}
			}
		}
	}
	args := buildAnsibleArgs(spec, ev)

	// Vault passwords (from Key Store credentials) are each written to a transient
	// file and passed via --vault-password-file (multi-vault supported).
	for _, pw := range vaultPasswords(spec) {
		if f, err := os.CreateTemp("", "aui-vault-*"); err == nil {
			_, _ = f.WriteString(pw)
			_ = f.Close()
			vaultFiles = append(vaultFiles, f.Name())
			args = append([]string{"--vault-password-file", f.Name()}, args...)
		}
	}

	env := buildEnv(spec)

	// Inventory connection credentials: load the SSH key(s) into an ssh-agent for
	// this run so ansible authenticates to the target hosts. Multiple keys are all
	// added (ansible/ssh tries each); --user (in buildAnsibleArgs) sets the login.
	var agentPID string
	if len(spec.SSHPrivateKeys) > 0 {
		sock, pid, keyFiles := startSSHAgentWithKeys(spec.SSHPrivateKeys)
		vaultFiles = append(vaultFiles, keyFiles...)
		agentPID = pid
		if sock != "" {
			// Dynamic hosts won't be in known_hosts; don't let the prompt hang the run.
			env = append(env, "SSH_AUTH_SOCK="+sock, "ANSIBLE_HOST_KEY_CHECKING=False")
		}
	}

	p, err := startPTY(bin, args, spec.Dir, env, spec.Cols, spec.Rows)
	if err != nil {
		for _, vf := range vaultFiles {
			_ = os.Remove(vf)
		}
		killAgent(agentPID)
		return nil, err
	}
	p.vaultFiles = vaultFiles
	p.agentPID = agentPID
	return p, nil
}

// startSSHAgentWithKeys starts an ssh-agent and adds each (unencrypted) key to it,
// returning the auth-socket path, the agent PID, and the temp key-file paths (to
// clean up). Keys are written to 0600 temp files; nothing touches the command line.
func startSSHAgentWithKeys(keys []string) (sock, pid string, files []string) {
	out, err := exec.Command("ssh-agent", "-s").Output()
	if err != nil {
		return "", "", nil
	}
	for _, line := range strings.Split(string(out), "\n") {
		if v := strings.TrimPrefix(line, "SSH_AUTH_SOCK="); v != line {
			sock = strings.SplitN(v, ";", 2)[0]
		} else if v := strings.TrimPrefix(line, "SSH_AGENT_PID="); v != line {
			pid = strings.SplitN(v, ";", 2)[0]
		}
	}
	if sock == "" {
		killAgent(pid)
		return "", "", nil
	}
	for _, k := range keys {
		f, ferr := os.CreateTemp("", "aui-sshkey-*")
		if ferr != nil {
			continue
		}
		key := k
		if !strings.HasSuffix(key, "\n") {
			key += "\n"
		}
		_, _ = f.WriteString(key)
		_ = f.Close()
		_ = os.Chmod(f.Name(), 0o600)
		files = append(files, f.Name())
		add := exec.Command("ssh-add", f.Name())
		add.Env = append(os.Environ(), "SSH_AUTH_SOCK="+sock)
		_ = add.Run() // unencrypted keys load without input; encrypted ones are skipped
	}
	return sock, pid, files
}

// killAgent terminates an ssh-agent started for a run (portable across OSes).
func killAgent(pid string) {
	if pid == "" {
		return
	}
	if n, err := strconv.Atoi(pid); err == nil {
		if p, perr := os.FindProcess(n); perr == nil {
			_ = p.Kill()
		}
	}
}

// vaultPasswords collects the single + multi-vault passwords from the spec.
func vaultPasswords(spec model.ExecSpec) []string {
	var out []string
	if spec.VaultPassword != "" {
		out = append(out, spec.VaultPassword)
	}
	out = append(out, spec.VaultPasswords...)
	return out
}

// infraBin maps a Terraform-family app to its binary.
func infraBin(app string) string {
	switch app {
	case model.AppTofu:
		return "tofu"
	case model.AppTerragrunt:
		return "terragrunt"
	default:
		return "terraform"
	}
}

// startInfra runs a Terraform/OpenTofu/Terragrunt workflow (init + action) in a
// PTY. extra-vars become TF_VAR_* so values never touch the shell command line.
func startInfra(spec model.ExecSpec) (*Process, error) {
	bin := infraBin(spec.App)
	dir := spec.Dir
	if sub := strings.TrimSpace(spec.Playbook); sub != "" && sub != "." {
		dir = filepath.Join(spec.Dir, sub)
	}
	// Auto-approve only when requested; otherwise apply/destroy prompt in the live
	// PTY (the operator can type "yes"). plan never needs approval.
	approve := ""
	if spec.AutoApprove {
		approve = " -auto-approve"
	}
	extra := ""
	if len(spec.CliArgs) > 0 {
		extra = " " + strings.Join(spec.CliArgs, " ")
	}
	action := spec.Action
	switch action {
	case "apply":
		action = bin + " apply -input=false" + approve + extra
	case "destroy":
		action = bin + " destroy -input=false" + approve + extra
	default:
		action = bin + " plan -input=false" + extra
	}
	initCmd := bin + " init -input=false"
	// HTTP state backend (Terraform/OpenTofu): drop a backend override file and
	// pass the address via -backend-config so state lives at the configured URL.
	if be := strings.TrimSpace(spec.TfBackend); be != "" && (spec.App == model.AppTerraform || spec.App == model.AppTofu) {
		_ = os.WriteFile(filepath.Join(dir, "aui_backend_override.tf"),
			[]byte("terraform {\n  backend \"http\" {}\n}\n"), 0o644)
		initCmd += " -backend-config='address=" + be + "' -backend-config='lock_address=" + be +
			"' -backend-config='unlock_address=" + be + "'"
	}
	steps := initCmd
	// Select (or create) the workspace before the action.
	if ws := strings.TrimSpace(spec.Workspace); ws != "" {
		steps += " && (" + bin + " workspace select " + ws + " || " + bin + " workspace new " + ws + ")"
	}
	script := steps + " && " + action
	// Remove the transient backend override afterwards (preserving the exit code)
	// so it doesn't leak into other runs sharing the working dir.
	if strings.TrimSpace(spec.TfBackend) != "" && (spec.App == model.AppTerraform || spec.App == model.AppTofu) {
		script = "(" + script + "); rc=$?; rm -f aui_backend_override.tf; exit $rc"
	}

	env := buildEnv(spec)
	for k, v := range spec.ExtraVars {
		env = append(env, "TF_VAR_"+k+"="+fmt.Sprint(v))
	}
	return startPTY("sh", []string{"-c", script}, dir, env, spec.Cols, spec.Rows)
}

// startPulumi runs a Pulumi workflow (stack select/init + preview/up/destroy) in
// a PTY against a **local file backend** beside the program, so no Pulumi Cloud
// login is needed. extra-vars are exported as env vars; the program/config can
// read them. The default secrets provider uses PULUMI_CONFIG_PASSPHRASE.
func startPulumi(spec model.ExecSpec) (*Process, error) {
	dir := spec.Dir
	if sub := strings.TrimSpace(spec.Playbook); sub != "" && sub != "." {
		dir = filepath.Join(spec.Dir, sub)
	}
	stack := strings.TrimSpace(spec.Workspace)
	if stack == "" {
		stack = "dev"
	}
	extra := ""
	if len(spec.CliArgs) > 0 {
		extra = " " + strings.Join(spec.CliArgs, " ")
	}
	var action string
	switch spec.Action {
	case "apply", "up":
		action = "pulumi up --yes --skip-preview --non-interactive" + extra
	case "destroy":
		action = "pulumi destroy --yes --non-interactive" + extra
	default:
		action = "pulumi preview --non-interactive" + extra
	}
	// Use (or create) the stack, then run the action.
	script := "pulumi stack select --create " + stack + " && " + action

	env := buildEnv(spec)
	env = append(env,
		"PULUMI_BACKEND_URL=file://"+dir, // local state beside the program
		"PULUMI_CONFIG_PASSPHRASE=",      // empty passphrase for the default secrets provider
		"PULUMI_SKIP_UPDATE_CHECK=true",
	)
	for k, v := range spec.ExtraVars {
		env = append(env, k+"="+fmt.Sprint(v))
	}
	return startPTY("sh", []string{"-c", script}, dir, env, spec.Cols, spec.Rows)
}

// startScript runs a single Bash/Python/PowerShell script in a PTY. extra-vars
// are exported as environment variables for the script to read.
func startScript(spec model.ExecSpec) (*Process, error) {
	var bin string
	var args []string
	switch spec.App {
	case model.AppPython:
		bin = os.Getenv("PYTHON_BIN")
		if bin == "" {
			bin = "python3"
		}
		args = []string{spec.Playbook}
	case model.AppPowerShell:
		bin = "pwsh"
		args = []string{"-File", spec.Playbook}
	default: // bash
		bin = "bash"
		args = []string{spec.Playbook}
	}
	args = append(args, spec.CliArgs...)
	env := buildEnv(spec)
	for k, v := range spec.ExtraVars {
		env = append(env, k+"="+fmt.Sprint(v))
	}
	return startPTY(bin, args, spec.Dir, env, spec.Cols, spec.Rows)
}

// StartGalaxy installs a requirements file with ansible-galaxy in a PTY.
func StartGalaxy(reqFile string, spec model.ExecSpec) (*Process, error) {
	args := append([]string{"install", "-r", reqFile, "--force"}, spec.GalaxyArgs...)
	return startPTY("ansible-galaxy", args, spec.Dir, buildEnv(spec), spec.Cols, spec.Rows)
}

// GalaxyRequirements returns the requirements files present in dir (relative).
func GalaxyRequirements(dir string) []string {
	var out []string
	for _, f := range []string{"requirements.yml", "requirements.yaml", "roles/requirements.yml", "collections/requirements.yml"} {
		if st, err := os.Stat(filepath.Join(dir, f)); err == nil && !st.IsDir() {
			out = append(out, f)
		}
	}
	return out
}

// Read pulls raw terminal bytes; returns io.EOF when the process exits.
func (p *Process) Read(b []byte) (int, error) { return p.pty.Read(b) }

// Resize updates the PTY window size.
func (p *Process) Resize(cols, rows int) error {
	if cols <= 0 || rows <= 0 {
		return nil
	}
	return pty.Setsize(p.pty, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
}

// PID of the underlying process (0 if not started).
func (p *Process) PID() int {
	if p.cmd.Process == nil {
		return 0
	}
	return p.cmd.Process.Pid
}

// Cancel asks the process group to stop, escalating SIGINT → SIGTERM → SIGKILL.
func (p *Process) Cancel() {
	if p.cmd.Process == nil {
		return
	}
	_ = killGroup(p.cmd.Process, syscall.SIGINT)
	go func() {
		time.Sleep(4 * time.Second)
		_ = killGroup(p.cmd.Process, syscall.SIGTERM)
		time.Sleep(3 * time.Second)
		_ = killGroup(p.cmd.Process, syscall.SIGKILL)
	}()
}

// Wait reaps the process and returns its exit code.
func (p *Process) Wait() int {
	err := p.cmd.Wait()
	_ = p.pty.Close()
	for _, vf := range p.vaultFiles {
		_ = os.Remove(vf)
	}
	killAgent(p.agentPID)
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return -1
}

// BuildArgs returns a display/record-friendly command line for the spec's app
// (what the run effectively executes). The runner builds the real arg slices in
// the start* functions; this mirrors them for the UI and run history.
func BuildArgs(spec model.ExecSpec) []string {
	switch {
	case spec.App == model.AppPulumi:
		action := spec.Action
		switch action {
		case "apply", "up":
			action = "up"
		case "destroy":
			action = "destroy"
		default:
			action = "preview"
		}
		return []string{"pulumi", action}
	case model.IsInfraApp(spec.App):
		bin := infraBin(spec.App)
		action := spec.Action
		if action == "" {
			action = "plan"
		}
		return []string{bin, "init", "&&", bin, action}
	case spec.App == model.AppPython:
		return []string{"python3", spec.Playbook}
	case spec.App == model.AppPowerShell:
		return []string{"pwsh", "-File", spec.Playbook}
	case spec.App == model.AppBash:
		return []string{"bash", spec.Playbook}
	default:
		// Displayed/recorded command. The caller (api) passes a spec whose ExtraVars
		// already have secret values masked, so rendering them inline is safe and
		// lets the operator see exactly what was applied. The REAL run instead writes
		// a transient file and passes "-e @file" (see startAnsible).
		var ev []string
		if len(spec.ExtraVars) > 0 {
			if j, err := json.Marshal(spec.ExtraVars); err == nil {
				ev = []string{"-e", string(j)}
			}
		}
		return buildAnsibleArgs(spec, ev)
	}
}

// buildAnsibleArgs turns an ExecSpec into an ansible-playbook argument list. Args
// are passed as a slice (never a shell string), so values cannot inject commands.
// buildAnsibleArgs assembles ansible-playbook flags. extraVarsArg is the
// rendered "-e …" portion (the caller decides: a temp-file ref for real
// execution, a redacted placeholder for the displayed/recorded command) so
// secret extra-var VALUES never land on the command line, the log or run.args.
func buildAnsibleArgs(spec model.ExecSpec, extraVarsArg []string) []string {
	args := make([]string, 0, 16)
	if spec.Inventory != "" {
		args = append(args, "-i", spec.Inventory)
	}
	if spec.SSHUser != "" {
		args = append(args, "--user", spec.SSHUser) // inventory connection user
	}
	if spec.Limit != "" {
		args = append(args, "--limit", spec.Limit)
	}
	if spec.Tags != "" {
		args = append(args, "--tags", spec.Tags)
	}
	if spec.SkipTags != "" {
		args = append(args, "--skip-tags", spec.SkipTags)
	}
	if spec.Check {
		args = append(args, "--check")
	}
	if spec.Diff {
		args = append(args, "--diff")
	}
	if spec.Verbosity > 0 {
		v := spec.Verbosity
		if v > 4 {
			v = 4
		}
		args = append(args, "-"+strings.Repeat("v", v))
	}
	args = append(args, extraVarsArg...)
	args = append(args, spec.CliArgs...)
	args = append(args, spec.Playbook)
	return args
}

func buildEnv(spec model.ExecSpec) []string {
	env := map[string]string{
		"TERM":                        "xterm-256color",
		"ANSIBLE_FORCE_COLOR":         "1",
		"PY_COLORS":                   "1",
		"FORCE_COLOR":                 "1",
		"ANSIBLE_HOST_KEY_CHECKING":   "False",
		"ANSIBLE_RETRY_FILES_ENABLED": "0",
		"LANG":                        "C.UTF-8",
		"LC_ALL":                      "C.UTF-8",
	}
	for k, v := range spec.Env {
		env[k] = v
	}
	out := os.Environ()
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	return out
}
