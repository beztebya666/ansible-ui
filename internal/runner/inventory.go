package runner

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/nikiv/ansible-ui/internal/model"
)

// ListInventory resolves an inventory's hosts/groups by running
// `ansible-inventory --list` in the project working dir — exactly how a run
// would resolve it, so static, file and dynamic/plugin inventories all work.
// Inline content (static inventories) is materialised to a transient file first.
func ListInventory(spec model.InventoryListSpec) model.InventoryListResult {
	dir := spec.Dir
	if dir == "" {
		dir = os.TempDir()
	}
	invArg := spec.InventoryArg

	// Materialise static/inline content into the project dir so ansible discovers
	// any neighbouring group_vars/host_vars, then clean it up.
	if spec.InlineContent != "" && spec.Filename != "" {
		p := filepath.Join(dir, filepath.FromSlash(spec.Filename))
		if err := os.WriteFile(p, []byte(spec.InlineContent), 0o644); err != nil {
			return model.InventoryListResult{Error: "write inventory: " + err.Error()}
		}
		defer os.Remove(p)
		invArg = spec.Filename
	}
	if invArg == "" {
		return model.InventoryListResult{Error: "no inventory to resolve"}
	}

	cmd := exec.Command("ansible-inventory", "-i", invArg, "--list")
	cmd.Dir = dir
	env := append(os.Environ(), "HOME=/tmp", "ANSIBLE_LOCALHOST_WARNING=False",
		"ANSIBLE_INVENTORY_UNPARSED_WARNING=False")
	if _, err := os.Stat(filepath.Join(dir, "ansible.cfg")); err == nil {
		env = append(env, "ANSIBLE_CONFIG="+filepath.Join(dir, "ansible.cfg"))
	}
	for k, v := range spec.Env {
		env = append(env, k+"="+v)
	}
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return model.InventoryListResult{Error: msg}
	}
	return model.InventoryListResult{JSON: stdout.String()}
}
