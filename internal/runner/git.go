package runner

import (
	"bytes"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/nikiv/ansible-ui/internal/model"
)

// reposRoot is where repositories are checked out (shared /data volume).
func reposRoot() string {
	if v := os.Getenv("DATA_DIR"); v != "" {
		return filepath.Join(v, "repos")
	}
	return "/data/repos"
}

// Sync clones or updates a repository into <DATA_DIR>/repos/<repoId> and checks
// out the requested branch at its latest commit.
func Sync(spec model.GitSyncSpec) model.GitSyncResult {
	dest := filepath.Join(reposRoot(), spec.RepoID)
	branch := spec.Branch
	if branch == "" {
		branch = "main"
	}

	env := append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ASKPASS=/bin/true",
	)

	// SSH key auth.
	var cleanup []string
	defer func() {
		for _, f := range cleanup {
			_ = os.Remove(f)
		}
	}()
	if spec.SSHPrivateKey != "" {
		if sshCmd, files := sshGitCommand(spec.SSHPrivateKey, spec.SSHCertificate); sshCmd != "" {
			cleanup = append(cleanup, files...)
			env = append(env, "GIT_SSH_COMMAND="+sshCmd)
		}
	}

	// HTTP(S) basic auth: inject credentials for the operation, then scrub the
	// remote URL so the password is never persisted in .git/config.
	cloneURL := spec.GitURL
	cleanURL := spec.GitURL
	if spec.Password != "" && (strings.HasPrefix(cloneURL, "http://") || strings.HasPrefix(cloneURL, "https://")) {
		if u, err := url.Parse(cloneURL); err == nil {
			user := spec.Username
			if user == "" {
				user = "git"
			}
			u.User = url.UserPassword(user, spec.Password)
			cloneURL = u.String()
		}
	}

	git := func(dir string, args ...string) (string, error) {
		cmd := exec.Command("git", args...)
		if dir != "" {
			cmd.Dir = dir
		}
		cmd.Env = env
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		err := cmd.Run()
		return out.String(), err
	}

	var res model.GitSyncResult
	fail := func(out string, err error) model.GitSyncResult {
		res.Error = strings.TrimSpace(scrub(out, spec.Password))
		if res.Error == "" && err != nil {
			res.Error = err.Error()
		}
		return res
	}

	if _, err := os.Stat(filepath.Join(dest, ".git")); err == nil {
		// Existing checkout: force it to EXACTLY match origin/<branch>, recovering
		// from a dirty working tree (leftover/untracked files from a prior run) and
		// a force-pushed (rewritten-history) upstream.
		if out, err := git(dest, "remote", "set-url", "origin", cloneURL); err != nil {
			return fail(out, err)
		}
		// --force updates remote-tracking refs even when upstream history was rewritten.
		if out, err := git(dest, "fetch", "--prune", "--force", "origin"); err != nil {
			return fail(out, err)
		}
		// -f discards local modifications so the checkout never aborts on "untracked
		// working tree files would be overwritten".
		if out, err := git(dest, "checkout", "-f", "-B", branch, "origin/"+branch); err != nil {
			if out2, err2 := git(dest, "checkout", "-f", branch); err2 != nil {
				return fail(out+"\n"+out2, err2)
			}
		}
		if out, err := git(dest, "reset", "--hard", "origin/"+branch); err != nil {
			return fail(out, err)
		}
		// Drop any remaining untracked files/dirs so each run starts from a pristine,
		// reproducible tree (transient inventories, generated files, etc.).
		_, _ = git(dest, "clean", "-fd")
	} else {
		_ = os.MkdirAll(reposRoot(), 0o755)
		_ = os.RemoveAll(dest)
		if out, err := git("", "clone", "--branch", branch, cloneURL, dest); err != nil {
			// retry without -branch (repo default branch)
			if out2, err2 := git("", "clone", cloneURL, dest); err2 != nil {
				return fail(out+"\n"+out2, err2)
			}
		}
	}
	// Scrub any persisted credentials from the remote URL.
	_, _ = git(dest, "remote", "set-url", "origin", cleanURL)

	commit, _ := git(dest, "rev-parse", "HEAD")
	msg, _ := git(dest, "log", "-1", "--pretty=%s")
	res.Commit = strings.TrimSpace(commit)
	res.Ref = branch
	res.Message = strings.TrimSpace(msg)
	return res
}

// sshGitCommand writes the SSH private key (and optional signed certificate) to
// temp files and returns the GIT_SSH_COMMAND value plus the temp paths to clean
// up. An OpenSSH user certificate is presented via `-o CertificateFile` so a key
// signed by a trusted CA authenticates even when its bare public key isn't an
// installed authorized_key.
func sshGitCommand(sshKey, sshCert string) (sshCmd string, files []string) {
	kf, err := os.CreateTemp("", "aui-gitkey-*")
	if err != nil {
		return "", nil
	}
	key := sshKey
	if !strings.HasSuffix(key, "\n") {
		key += "\n"
	}
	_, _ = kf.WriteString(key)
	_ = kf.Chmod(0o600)
	_ = kf.Close()
	files = append(files, kf.Name())
	sshCmd = "ssh -i " + kf.Name()
	if strings.TrimSpace(sshCert) != "" {
		if cf, err := os.CreateTemp("", "aui-gitcert-*.pub"); err == nil {
			cert := sshCert
			if !strings.HasSuffix(cert, "\n") {
				cert += "\n"
			}
			_, _ = cf.WriteString(cert)
			_ = cf.Chmod(0o644)
			_ = cf.Close()
			files = append(files, cf.Name())
			sshCmd += " -o CertificateFile=" + cf.Name()
		}
	}
	sshCmd += " -o IdentitiesOnly=yes -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null"
	return sshCmd, files
}

// gitAuth prepares the environment (SSH key) and authenticated/clean clone URLs
// for a credentialed git operation. cleanup removes any temp key file.
func gitAuth(gitURL, sshKey, sshCert, username, password string) (env []string, cloneURL, cleanURL string, cleanup func()) {
	env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=/bin/true")
	var files []string
	if sshKey != "" {
		if sshCmd, kfiles := sshGitCommand(sshKey, sshCert); sshCmd != "" {
			files = append(files, kfiles...)
			env = append(env, "GIT_SSH_COMMAND="+sshCmd)
		}
	}
	cloneURL, cleanURL = gitURL, gitURL
	if password != "" && (strings.HasPrefix(cloneURL, "http://") || strings.HasPrefix(cloneURL, "https://")) {
		if u, err := url.Parse(cloneURL); err == nil {
			user := username
			if user == "" {
				user = "git"
			}
			u.User = url.UserPassword(user, password)
			cloneURL = u.String()
		}
	}
	cleanup = func() {
		for _, f := range files {
			_ = os.Remove(f)
		}
	}
	return env, cloneURL, cleanURL, cleanup
}

// Commit writes files into the repo checkout, commits them onto a NEW branch and
// pushes it — it deliberately never touches the (often protected) base branch, so
// the change lands as a branch the operator can open a PR/MR from.
func Commit(spec model.GitCommitSpec) model.GitCommitResult {
	dest := filepath.Join(reposRoot(), spec.RepoID)
	base := spec.BaseBranch
	if base == "" {
		base = "main"
	}
	var res model.GitCommitResult
	res.Branch = spec.NewBranch

	env, cloneURL, cleanURL, cleanup := gitAuth(spec.GitURL, spec.SSHPrivateKey, spec.SSHCertificate, spec.Username, spec.Password)
	defer cleanup()

	git := func(args ...string) (string, error) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dest
		cmd.Env = env
		var out bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &out
		err := cmd.Run() // run BEFORE reading out — argument order would read it empty
		return out.String(), err
	}
	fail := func(out string, err error) model.GitCommitResult {
		res.Error = strings.TrimSpace(scrub(out, spec.Password))
		if res.Error == "" && err != nil {
			res.Error = err.Error()
		}
		return res
	}

	if _, err := os.Stat(filepath.Join(dest, ".git")); err != nil {
		_ = os.MkdirAll(reposRoot(), 0o755)
		c := exec.Command("git", "clone", "--branch", base, cloneURL, dest)
		c.Env = env
		var out bytes.Buffer
		c.Stdout, c.Stderr = &out, &out
		if cerr := c.Run(); cerr != nil {
			return fail(out.String(), cerr)
		}
	}
	if out, err := git("remote", "set-url", "origin", cloneURL); err != nil {
		return fail(out, err)
	}
	_, _ = git("fetch", "--prune", "origin")
	// Start the new branch from the latest base; fall back to a fresh local branch.
	if _, err := git("checkout", "-B", spec.NewBranch, "origin/"+base); err != nil {
		if out2, err2 := git("checkout", "-B", spec.NewBranch); err2 != nil {
			_, _ = git("remote", "set-url", "origin", cleanURL)
			return fail(out2, err2)
		}
	}
	for _, f := range spec.Files {
		abs := filepath.Join(dest, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			res.Error = err.Error()
			return res
		}
		if err := os.WriteFile(abs, []byte(f.Content), 0o644); err != nil {
			res.Error = err.Error()
			return res
		}
		if out, err := git("add", "--", f.Path); err != nil {
			return fail(out, err)
		}
	}
	if _, err := git("diff", "--cached", "--quiet"); err == nil {
		_, _ = git("remote", "set-url", "origin", cleanURL)
		res.Error = "no changes to commit"
		return res
	}
	name := spec.AuthorName
	if name == "" {
		name = "ansible-ui"
	}
	email := spec.AuthorEmail
	if email == "" {
		email = "ansible-ui@local"
	}
	if out, err := git("-c", "user.name="+name, "-c", "user.email="+email, "commit", "-m", spec.Message); err != nil {
		_, _ = git("remote", "set-url", "origin", cleanURL)
		return fail(out, err)
	}
	if out, err := git("push", "origin", "HEAD:refs/heads/"+spec.NewBranch); err != nil {
		_, _ = git("remote", "set-url", "origin", cleanURL)
		return fail(out, err)
	}
	_, _ = git("remote", "set-url", "origin", cleanURL) // scrub credentials from .git/config
	commit, _ := git("rev-parse", "HEAD")
	res.Commit = strings.TrimSpace(commit)
	res.Pushed = true
	return res
}

// scrub removes a secret from text before returning it to a caller/log.
func scrub(s, secret string) string {
	if secret == "" {
		return s
	}
	return strings.ReplaceAll(s, secret, "***")
}
