package api

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/nikiv/ansible-ui/internal/model"
	"github.com/nikiv/ansible-ui/internal/runnerclient"
)

// hostFactSummary extracts a few human-friendly fields from a host's facts for
// the registry table (full facts stay in the per-host view).
func hostFactSummary(h *model.HostFacts) {
	f := h.Facts
	str := func(k string) string {
		if v, ok := f[k].(string); ok {
			return v
		}
		return ""
	}
	h.OS = str("ansible_os_family")
	h.Distro = str("ansible_distribution")
	if v := str("ansible_distribution_version"); v != "" && h.Distro != "" {
		h.Distro += " " + v
	}
	h.Kernel = str("ansible_kernel")
	if d, ok := f["ansible_default_ipv4"].(map[string]any); ok {
		if ip, ok := d["address"].(string); ok {
			h.IP = ip
		}
	}
}

func (s *Server) handleListHosts(w http.ResponseWriter, r *http.Request) {
	hosts, err := s.store.ListHostFacts(r.Context())
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	for _, h := range hosts {
		hostFactSummary(h)
		h.Facts = nil // keep the list lean — full facts via GET /api/hosts/{host}
	}
	writeJSON(w, http.StatusOK, hosts)
}

func (s *Server) handleGetHost(w http.ResponseWriter, r *http.Request) {
	h, err := s.store.GetHostFacts(r.Context(), r.PathValue("host"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	hostFactSummary(h)
	writeJSON(w, http.StatusOK, h)
}

func (s *Server) handleDeleteHost(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if err := s.store.DeleteHostFacts(r.Context(), r.PathValue("host")); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleGatherFacts runs `ansible … -m setup` for an inventory's hosts via the
// runner and stores the gathered facts in the central registry.
func (s *Server) handleGatherFacts(w http.ResponseWriter, r *http.Request) {
	inv, err := s.store.GetInventory(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	if !s.requireProjectCap(w, r, inv.ProjectID, model.CapRun) {
		return
	}
	proj, err := s.store.GetProject(r.Context(), inv.ProjectID)
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	dir := s.projectDir(proj)
	spec := model.FactsGatherSpec{Dir: dir, Pattern: r.URL.Query().Get("pattern")}
	switch inv.Type {
	case model.InventoryStatic:
		spec.InlineContent = inv.Content
		spec.Filename = ".aui-inv-facts-" + inv.ID
	case model.InventoryFile:
		spec.InventoryArg = inv.Content
	case model.InventoryDynamic:
		spec.InventoryArg = inv.Content
		_ = os.Chmod(filepath.Join(dir, filepath.FromSlash(inv.Content)), 0o755)
	default:
		writeErr(w, http.StatusBadRequest, "fact gathering isn't supported for "+inv.Type+" inventories yet")
		return
	}
	res, err := runnerclient.GatherFacts(r.Context(), s.cfg.RunnerHTTP, spec)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "gather facts: "+err.Error())
		return
	}
	stored := []string{}
	for host, facts := range res.Hosts {
		hf := &model.HostFacts{Host: host, InventoryID: inv.ID, Facts: facts}
		if err := s.store.UpsertHostFacts(r.Context(), hf); err == nil {
			stored = append(stored, host)
		}
	}
	if len(stored) == 0 && res.Error != "" {
		writeErr(w, http.StatusBadRequest, res.Error)
		return
	}
	s.recordActivity(r.Context(), s.actor(r.Context()), "facts.gathered", inv.Name, "")
	writeJSON(w, http.StatusOK, map[string]any{"gathered": stored, "count": len(stored)})
}
