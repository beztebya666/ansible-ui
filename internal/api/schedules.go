package api

import (
	"net/http"
	"time"

	"github.com/nikiv/ansible-ui/internal/model"
)

func (s *Server) handleListSchedules(w http.ResponseWriter, r *http.Request) {
	templateID := r.URL.Query().Get("templateId")
	scs, err := s.store.ListSchedules(r.Context(), templateID)
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	if scope := projectScope(r); scope != "" && templateID == "" {
		out := scs[:0]
		for _, sc := range scs {
			if sc.ProjectID == scope {
				out = append(out, sc)
			}
		}
		scs = out
	}
	writeJSON(w, http.StatusOK, scs)
}

func (s *Server) handleGetSchedule(w http.ResponseWriter, r *http.Request) {
	sc, err := s.store.GetSchedule(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sc)
}

func (s *Server) handleCreateSchedule(w http.ResponseWriter, r *http.Request) {
	var in struct {
		TemplateID string  `json:"templateId"`
		WorkflowID string  `json:"workflowId"`
		Name       string  `json:"name"`
		Cron       string  `json:"cron"`
		Once       bool    `json:"once"`
		RunAt      *string `json:"runAt"` // RFC3339, for one-time schedules
		Active     *bool   `json:"active"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if in.TemplateID == "" && in.WorkflowID == "" {
		writeErr(w, http.StatusBadRequest, "templateId or workflowId is required")
		return
	}
	// Resolve the owning project (for RBAC) from the template or workflow.
	var projectID string
	if in.WorkflowID != "" {
		wf, err := s.store.GetWorkflow(r.Context(), in.WorkflowID)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "workflow not found")
			return
		}
		projectID = wf.ProjectID
	} else {
		tpl, err := s.store.GetTemplate(r.Context(), in.TemplateID)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "template not found")
			return
		}
		projectID = tpl.ProjectID
	}
	if !s.requireProjectCap(w, r, projectID, model.CapEdit) {
		return
	}
	active := true
	if in.Active != nil {
		active = *in.Active
	}
	sc := &model.Schedule{TemplateID: in.TemplateID, WorkflowID: in.WorkflowID, Name: in.Name, Active: active}

	if in.Once {
		if in.RunAt == nil || *in.RunAt == "" {
			writeErr(w, http.StatusBadRequest, "runAt is required for a one-time schedule")
			return
		}
		at, perr := time.Parse(time.RFC3339, *in.RunAt)
		if perr != nil {
			writeErr(w, http.StatusBadRequest, "runAt must be an RFC3339 timestamp")
			return
		}
		sc.Once = true
		sc.NextRunAt = &at
	} else {
		if in.Cron == "" {
			writeErr(w, http.StatusBadRequest, "cron is required (or set once + runAt)")
			return
		}
		sched, err := parseCron(in.Cron)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "invalid cron expression (use 5 fields: min hour dom month dow)")
			return
		}
		next := sched.Next(time.Now())
		sc.Cron = in.Cron
		sc.NextRunAt = &next
	}
	if err := s.store.CreateSchedule(r.Context(), sc); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.recordActivity(r.Context(), "", "schedule.created", sc.Name, sc.Cron)
	writeJSON(w, http.StatusCreated, sc)
}

func (s *Server) handleUpdateSchedule(w http.ResponseWriter, r *http.Request) {
	sc, err := s.store.GetSchedule(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	if !s.requireProjectCap(w, r, sc.ProjectID, model.CapEdit) {
		return
	}
	var in struct {
		Name   *string `json:"name"`
		Cron   *string `json:"cron"`
		Active *bool   `json:"active"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if in.Name != nil {
		sc.Name = *in.Name
	}
	if in.Cron != nil {
		if _, err := parseCron(*in.Cron); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid cron expression")
			return
		}
		sc.Cron = *in.Cron
	}
	if in.Active != nil {
		sc.Active = *in.Active
	}
	if sched, err := parseCron(sc.Cron); err == nil {
		next := sched.Next(time.Now())
		sc.NextRunAt = &next
	}
	if err := s.store.UpdateSchedule(r.Context(), sc); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.recordActivity(r.Context(), "", "schedule.updated", sc.Name, sc.Cron)
	writeJSON(w, http.StatusOK, sc)
}

func (s *Server) handleDeleteSchedule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := id
	if sc, err := s.store.GetSchedule(r.Context(), id); err == nil {
		if !s.requireProjectCap(w, r, sc.ProjectID, model.CapEdit) {
			return
		}
		name = sc.Name
	}
	if err := s.store.DeleteSchedule(r.Context(), id); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.recordActivity(r.Context(), "", "schedule.deleted", name, "")
	w.WriteHeader(http.StatusNoContent)
}
