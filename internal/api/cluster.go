package api

import (
	"net/http"

	"github.com/nikiv/ansible-ui/internal/model"
	"github.com/nikiv/ansible-ui/internal/store"
)

// runnerCapacity is the live per-runner load shown on the Cluster dashboard.
type runnerCapacity struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Online        bool     `json:"online"`
	Builtin       bool     `json:"builtin"`
	Tags          []string `json:"tags"`
	Active        int      `json:"active"`        // runs currently occupying a slot
	MaxConcurrent int      `json:"maxConcurrent"` // 0 = unlimited
}

// handleCluster returns the dispatch/HA picture: whether this replica is the
// leader, per-runner capacity (load from the runs table), and the live queue /
// running tasks — the backing data for the Cluster dashboard.
func (s *Server) handleCluster(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	ctx := r.Context()
	runners, _ := s.store.ListRunners(ctx)
	load, _ := s.store.CountActiveRunsByRunner(ctx)
	byStatus, _ := s.store.CountRunsByStatus(ctx)
	queued, _ := s.store.ListRuns(ctx, store.RunFilter{Status: "queued", Limit: 100})
	running, _ := s.store.ListRuns(ctx, store.RunFilter{Status: "running", Limit: 100})
	if queued == nil {
		queued = []*model.Run{}
	}
	if running == nil {
		running = []*model.Run{}
	}

	caps := make([]runnerCapacity, 0, len(runners))
	online := 0
	for _, rn := range runners {
		on := runnerStatus(rn) == "online"
		if on {
			online++
		}
		tags := rn.Tags
		if tags == nil {
			tags = []string{}
		}
		caps = append(caps, runnerCapacity{
			ID: rn.ID, Name: rn.Name, Online: on, Builtin: rn.Builtin,
			Tags: tags, Active: load[rn.ID], MaxConcurrent: rn.MaxConcurrent,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"leader":  s.leader.Load(),
		"runners": caps,
		"queued":  queued,
		"running": running,
		"counts": map[string]int{
			"queued":        byStatus["queued"],
			"running":       byStatus["running"],
			"awaiting":      byStatus["awaiting"],
			"runnersOnline": online,
		},
	})
}
