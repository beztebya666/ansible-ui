package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"

	"github.com/nikiv/ansible-ui/internal/model"
	"github.com/nikiv/ansible-ui/internal/runnerclient"
)

type inventoryGroup struct {
	Name     string   `json:"name"`
	Hosts    []string `json:"hosts"`
	Children []string `json:"children,omitempty"`
}

type inventoryHostsResult struct {
	Groups      []inventoryGroup `json:"groups"`
	Hosts       []string         `json:"hosts"` // every distinct host, sorted
	Total       int              `json:"total"`
	Unsupported bool             `json:"unsupported,omitempty"`
	Message     string           `json:"message,omitempty"`
}

// invResolveParams returns the runner working dir + inventory arg for resolving
// an inventory's hosts (used by ping/monitoring). Static inventories return
// their content as inline+filename to materialise; cloud/url aren't supported.
func (s *Server) invResolveParams(ctx context.Context, inv *model.Inventory) (dir, invArg, inline, filename string, supported bool) {
	proj, err := s.store.GetProject(ctx, inv.ProjectID)
	if err != nil {
		return "", "", "", "", false
	}
	dir = s.projectDir(proj)
	switch inv.Type {
	case model.InventoryStatic:
		return dir, "", inv.Content, ".aui-inv-mon-" + inv.ID, true
	case model.InventoryFile:
		return dir, inv.Content, "", "", true
	case model.InventoryDynamic:
		_ = os.Chmod(filepath.Join(dir, filepath.FromSlash(inv.Content)), 0o755)
		return dir, inv.Content, "", "", true
	}
	return dir, "", "", "", false
}

// handleInventoryHosts resolves an inventory's groups + hosts via the runner's
// ansible-inventory, so the operator can preview exactly what a run would target.
func (s *Server) handleInventoryHosts(w http.ResponseWriter, r *http.Request) {
	inv, err := s.store.GetInventory(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	proj, err := s.store.GetProject(r.Context(), inv.ProjectID)
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	dir := s.projectDir(proj)

	spec := model.InventoryListSpec{Dir: dir}
	previewName := ".aui-inv-preview-" + inv.ID
	switch inv.Type {
	case model.InventoryStatic:
		spec.InlineContent = inv.Content
		spec.Filename = previewName
	case model.InventoryFile:
		spec.InventoryArg = inv.Content
	case model.InventoryDynamic:
		spec.InventoryArg = inv.Content
		// A dynamic inventory is an executable script; ensure it can be invoked.
		_ = os.Chmod(filepath.Join(dir, filepath.FromSlash(inv.Content)), 0o755)
	default: // cloud, url — need provider creds / a fetch; not previewed live yet.
		writeJSON(w, http.StatusOK, inventoryHostsResult{
			Unsupported: true,
			Message:     "Live preview isn't available for " + inv.Type + " inventories yet — they resolve at run time.",
		})
		return
	}

	res, err := runnerclient.ListInventory(r.Context(), s.cfg.RunnerHTTP, spec)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "resolve inventory: "+err.Error())
		return
	}
	if res.Error != "" {
		writeErr(w, http.StatusBadRequest, res.Error)
		return
	}
	out, perr := parseAnsibleInventory(res.JSON)
	if perr != nil {
		writeErr(w, http.StatusBadRequest, "parse inventory: "+perr.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// parseAnsibleInventory turns `ansible-inventory --list` JSON into a flat
// groups+hosts view. The top level is a map of group name → {hosts, children};
// "_meta" holds hostvars and is skipped.
func parseAnsibleInventory(raw string) (inventoryHostsResult, error) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return inventoryHostsResult{}, err
	}
	hostSet := map[string]struct{}{}
	groups := []inventoryGroup{}
	for name, body := range doc {
		if name == "_meta" {
			continue
		}
		var g struct {
			Hosts    []string `json:"hosts"`
			Children []string `json:"children"`
		}
		_ = json.Unmarshal(body, &g)
		// "all" is the implicit root — keep it only if it directly lists hosts.
		if name == "all" && len(g.Hosts) == 0 {
			continue
		}
		for _, h := range g.Hosts {
			hostSet[h] = struct{}{}
		}
		groups = append(groups, inventoryGroup{Name: name, Hosts: g.Hosts, Children: g.Children})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
	hosts := make([]string, 0, len(hostSet))
	for h := range hostSet {
		hosts = append(hosts, h)
	}
	sort.Strings(hosts)
	return inventoryHostsResult{Groups: groups, Hosts: hosts, Total: len(hosts)}, nil
}
