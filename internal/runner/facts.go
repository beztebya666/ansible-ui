package runner

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/nikiv/ansible-ui/internal/model"
)

// GatherFacts runs `ansible <pattern> -m setup --tree <dir>` for an inventory and
// returns each host's gathered facts. Uses --tree so facts land as one JSON file
// per host (robust parsing, no stdout interleaving). Static inventories are
// materialised first; the pattern defaults to "all".
func GatherFacts(spec model.FactsGatherSpec) model.FactsGatherResult {
	dir := spec.Dir
	if dir == "" {
		dir = os.TempDir()
	}
	invArg := spec.InventoryArg
	if spec.InlineContent != "" && spec.Filename != "" {
		p := filepath.Join(dir, filepath.FromSlash(spec.Filename))
		if err := os.WriteFile(p, []byte(spec.InlineContent), 0o644); err != nil {
			return model.FactsGatherResult{Error: "write inventory: " + err.Error()}
		}
		defer os.Remove(p)
		invArg = spec.Filename
	}
	if invArg == "" {
		return model.FactsGatherResult{Error: "no inventory to gather from"}
	}
	pattern := strings.TrimSpace(spec.Pattern)
	if pattern == "" {
		pattern = "all"
	}

	tree, err := os.MkdirTemp("", "aui-facts-")
	if err != nil {
		return model.FactsGatherResult{Error: err.Error()}
	}
	defer os.RemoveAll(tree)

	cmd := exec.Command("ansible", pattern, "-i", invArg, "-m", "setup", "--tree", tree)
	cmd.Dir = dir
	env := append(os.Environ(), "HOME=/tmp", "ANSIBLE_HOST_KEY_CHECKING=False",
		"ANSIBLE_LOCALHOST_WARNING=False", "ANSIBLE_DEPRECATION_WARNINGS=False")
	if _, serr := os.Stat(filepath.Join(dir, "ansible.cfg")); serr == nil {
		env = append(env, "ANSIBLE_CONFIG="+filepath.Join(dir, "ansible.cfg"))
	}
	for k, v := range spec.Env {
		env = append(env, k+"="+v)
	}
	cmd.Env = env
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	// ansible exits non-zero if ANY host is unreachable; we still read whatever
	// facts landed in the tree, so a partial gather is usable.
	_ = cmd.Run()

	out := model.FactsGatherResult{Hosts: map[string]map[string]any{}}
	entries, _ := os.ReadDir(tree)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, rerr := os.ReadFile(filepath.Join(tree, e.Name()))
		if rerr != nil {
			continue
		}
		var doc struct {
			AnsibleFacts map[string]any `json:"ansible_facts"`
		}
		if json.Unmarshal(data, &doc) == nil && doc.AnsibleFacts != nil {
			out.Hosts[e.Name()] = doc.AnsibleFacts
		}
	}
	if len(out.Hosts) == 0 {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = "no facts gathered (hosts unreachable?)"
		}
		out.Error = msg
	}
	return out
}

// PingHosts checks reachability via `ansible <pattern> -m ping --tree` and
// returns host → reachable. A host is reachable when its tree file reports
// "ping": "pong"; unreachable hosts (or those with an error result) → false.
func PingHosts(spec model.PingSpec) model.PingResult {
	dir := spec.Dir
	if dir == "" {
		dir = os.TempDir()
	}
	invArg := spec.InventoryArg
	if spec.InlineContent != "" && spec.Filename != "" {
		p := filepath.Join(dir, filepath.FromSlash(spec.Filename))
		if err := os.WriteFile(p, []byte(spec.InlineContent), 0o644); err != nil {
			return model.PingResult{Error: "write inventory: " + err.Error()}
		}
		defer os.Remove(p)
		invArg = spec.Filename
	}
	if invArg == "" {
		return model.PingResult{Error: "no inventory to ping"}
	}
	pattern := strings.TrimSpace(spec.Pattern)
	if pattern == "" {
		pattern = "all"
	}
	tree, err := os.MkdirTemp("", "aui-ping-")
	if err != nil {
		return model.PingResult{Error: err.Error()}
	}
	defer os.RemoveAll(tree)

	cmd := exec.Command("ansible", pattern, "-i", invArg, "-m", "ping", "--tree", tree)
	cmd.Dir = dir
	env := append(os.Environ(), "HOME=/tmp", "ANSIBLE_HOST_KEY_CHECKING=False",
		"ANSIBLE_LOCALHOST_WARNING=False", "ANSIBLE_DEPRECATION_WARNINGS=False")
	if _, serr := os.Stat(filepath.Join(dir, "ansible.cfg")); serr == nil {
		env = append(env, "ANSIBLE_CONFIG="+filepath.Join(dir, "ansible.cfg"))
	}
	for k, v := range spec.Env {
		env = append(env, k+"="+v)
	}
	cmd.Env = env
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	_ = cmd.Run() // non-zero when any host is unreachable; we read the tree regardless

	out := model.PingResult{Reachable: map[string]bool{}}
	entries, _ := os.ReadDir(tree)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, rerr := os.ReadFile(filepath.Join(tree, e.Name()))
		if rerr != nil {
			continue
		}
		var doc struct {
			Ping        string `json:"ping"`
			Unreachable bool   `json:"unreachable"`
		}
		_ = json.Unmarshal(data, &doc)
		out.Reachable[e.Name()] = doc.Ping == "pong" && !doc.Unreachable
	}
	if len(out.Reachable) == 0 {
		out.Error = strings.TrimSpace(stderr.String())
	}
	return out
}
