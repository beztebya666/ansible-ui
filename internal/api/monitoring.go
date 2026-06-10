package api

import (
	"context"
	"net/http"
	"strconv"

	"github.com/nikiv/ansible-ui/internal/model"
	"github.com/nikiv/ansible-ui/internal/notify"
	"github.com/nikiv/ansible-ui/internal/runnerclient"
)

// handleEnableMonitor creates reachability monitors for an inventory's hosts.
func (s *Server) handleEnableMonitor(w http.ResponseWriter, r *http.Request) {
	inv, err := s.store.GetInventory(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	if !s.requireProjectCap(w, r, inv.ProjectID, model.CapEdit) {
		return
	}
	threshold := 3
	if v, e := strconv.Atoi(r.URL.Query().Get("threshold")); e == nil && v > 0 {
		threshold = v
	}
	dir, invArg, inline, filename, ok := s.invResolveParams(r.Context(), inv)
	if !ok {
		writeErr(w, http.StatusBadRequest, "monitoring isn't supported for "+inv.Type+" inventories yet")
		return
	}
	// Resolve the inventory's hosts and create a monitor per host.
	res, err := runnerclient.ListInventory(r.Context(), s.cfg.RunnerHTTP, model.InventoryListSpec{
		Dir: dir, InventoryArg: invArg, InlineContent: inline, Filename: filename,
	})
	if err != nil {
		writeErr(w, http.StatusBadGateway, "resolve inventory: "+err.Error())
		return
	}
	if res.Error != "" {
		writeErr(w, http.StatusBadRequest, res.Error)
		return
	}
	parsed, _ := parseAnsibleInventory(res.JSON)
	created := 0
	for _, h := range parsed.Hosts {
		if err := s.store.EnsureHostMonitor(r.Context(), h, inv.ID, threshold); err == nil {
			created++
		}
	}
	s.recordActivity(r.Context(), s.actor(r.Context()), "monitor.enabled", inv.Name, "")
	go s.checkHostMonitors(context.Background()) // kick an immediate check
	writeJSON(w, http.StatusOK, map[string]any{"monitored": created})
}

// handleDisableMonitor removes all monitors created from an inventory.
func (s *Server) handleDisableMonitor(w http.ResponseWriter, r *http.Request) {
	inv, err := s.store.GetInventory(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	if !s.requireProjectCap(w, r, inv.ProjectID, model.CapEdit) {
		return
	}
	n, err := s.store.DeleteHostMonitorsByInventory(r.Context(), inv.ID)
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.recordActivity(r.Context(), s.actor(r.Context()), "monitor.disabled", inv.Name, "")
	writeJSON(w, http.StatusOK, map[string]any{"removed": n})
}

func (s *Server) handleListMonitors(w http.ResponseWriter, r *http.Request) {
	mons, err := s.store.ListHostMonitors(r.Context())
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mons)
}

// handleCheckMonitors runs a reachability check now (admin; also the leader runs
// it periodically).
func (s *Server) handleCheckMonitors(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	s.checkHostMonitors(r.Context())
	writeJSON(w, http.StatusOK, map[string]bool{"checked": true})
}

// checkHostMonitors pings every monitored host (grouped by inventory) and
// updates status, alerting on up→down transitions and recoveries. Leader-only
// when called from the scheduler; safe to call ad-hoc.
func (s *Server) checkHostMonitors(ctx context.Context) {
	mons, err := s.store.ListHostMonitors(ctx)
	if err != nil || len(mons) == 0 {
		return
	}
	byInv := map[string][]*model.HostMonitor{}
	for _, m := range mons {
		byInv[m.InventoryID] = append(byInv[m.InventoryID], m)
	}
	for invID, list := range byInv {
		inv, err := s.store.GetInventory(ctx, invID)
		if err != nil {
			continue
		}
		dir, invArg, inline, filename, ok := s.invResolveParams(ctx, inv)
		if !ok {
			continue
		}
		res, perr := runnerclient.PingHosts(ctx, s.cfg.RunnerHTTP, model.PingSpec{
			Dir: dir, InventoryArg: invArg, InlineContent: inline, Filename: filename,
			Env: map[string]string{"ANSIBLE_TIMEOUT": "10"},
		})
		if perr != nil {
			s.log.Warn("monitor ping failed", "inventory", invID, "err", perr)
			continue
		}
		for _, m := range list {
			reachable := res.Reachable[m.Host]
			prev := m.Status
			if reachable {
				_ = s.store.UpdateHostMonitorStatus(ctx, m.Host, model.MonitorUp, 0, "", true)
				if prev == model.MonitorDown {
					s.alertHostMonitor(ctx, m, inv, "host-up", "recovered ✓ — reachable again")
				}
				continue
			}
			consec := m.ConsecFails + 1
			status := m.Status
			if consec >= m.Threshold {
				status = model.MonitorDown
			} else if status != model.MonitorDown {
				status = model.MonitorUnknown
			}
			_ = s.store.UpdateHostMonitorStatus(ctx, m.Host, status, consec, "unreachable", false)
			if prev != model.MonitorDown && status == model.MonitorDown {
				s.alertHostMonitor(ctx, m, inv, "host-down", "unreachable — host is DOWN")
			}
		}
	}
}

// alertHostMonitor fires a host-up/host-down notification to subscribed channels
// and logs the delivery (mirrors run alerts).
func (s *Server) alertHostMonitor(ctx context.Context, m *model.HostMonitor, inv *model.Inventory, event, detail string) {
	chans, err := s.store.ListEnabledNotificationChannels(ctx)
	if err != nil {
		return
	}
	title := "ansible·ui · host " + m.Host + " — " + map[string]string{"host-down": "DOWN", "host-up": "UP"}[event]
	text := detail + " · inventory=" + inv.Name
	vars := map[string]string{"run": m.Host, "status": event, "project": inv.Name, "app": "monitor", "id": m.Host}
	for _, ch := range chans {
		if !hasEvent(ch.Events, event) {
			continue
		}
		entry := &model.NotificationLog{ChannelID: ch.ID, ChannelName: ch.Name, ChannelType: ch.Type, RunName: m.Host, Event: event, OK: true}
		if derr := notify.Dispatch(ctx, ch, title, text, "", vars); derr != nil {
			entry.OK = false
			entry.Error = derr.Error()
		}
		_ = s.store.LogNotification(ctx, entry)
	}
	s.recordActivity(ctx, "", "host."+event, m.Host, inv.Name)
}
