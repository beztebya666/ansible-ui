package api

import (
	"net/http"

	"github.com/nikiv/ansible-ui/internal/model"
)

// effectiveProjectRole resolves a user's effective role in a project:
// global admins are project admins; a project with no members is unrestricted
// (treated as admin); otherwise the member role, defaulting to viewer.
func (s *Server) effectiveProjectRole(r *http.Request, projectID string, u *model.User) string {
	if u == nil {
		return ""
	}
	if u.Role == model.RoleAdmin {
		return model.ProjectRoleAdmin
	}
	if role, _ := s.store.GetProjectRole(r.Context(), projectID, u.ID); role != "" {
		return role
	}
	if n, _ := s.store.ProjectMemberCount(r.Context(), projectID); n == 0 {
		return model.ProjectRoleAdmin // unrestricted project — backward compatible
	}
	return model.ProjectRoleViewer
}

// projectCaps resolves the set of capabilities the user holds in a project.
// Global admins and members of an unrestricted (memberless) project hold every
// capability; a built-in role expands to its fixed cap set; a custom-role id
// expands to that role's declared permissions.
func (s *Server) projectCaps(r *http.Request, projectID string, u *model.User) map[string]bool {
	caps := map[string]bool{}
	if u == nil {
		return caps
	}
	grantAll := func() map[string]bool {
		for _, c := range model.AllCaps {
			caps[c] = true
		}
		return caps
	}
	if u.Role == model.RoleAdmin {
		return grantAll()
	}
	role, _ := s.store.GetProjectRole(r.Context(), projectID, u.ID)
	if role == "" {
		if n, _ := s.store.ProjectMemberCount(r.Context(), projectID); n == 0 {
			return grantAll() // unrestricted project — backward compatible
		}
		role = model.ProjectRoleViewer
	}
	if bc := model.BuiltinRoleCaps(role); bc != nil {
		for _, c := range bc {
			caps[c] = true
		}
		return caps
	}
	// A custom role, referenced by id.
	if cr, err := s.store.GetCustomRole(r.Context(), role); err == nil {
		for _, c := range cr.Permissions {
			caps[c] = true
		}
	}
	return caps
}

// requireProjectCap guards a project-scoped action, writing 401/403 and
// returning false when the current user lacks the given capability.
func (s *Server) requireProjectCap(w http.ResponseWriter, r *http.Request, projectID, capability string) bool {
	u := currentUser(r)
	if u == nil {
		writeErr(w, http.StatusUnauthorized, "unauthorized")
		return false
	}
	if s.projectCaps(r, projectID, u)[capability] {
		return true
	}
	writeErr(w, http.StatusForbidden, "insufficient project permission (need "+capability+")")
	return false
}

func (s *Server) handleListMembers(w http.ResponseWriter, r *http.Request) {
	pid := r.PathValue("id")
	if _, err := s.store.GetProject(r.Context(), pid); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	members, err := s.store.ListProjectMembers(r.Context(), pid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if members == nil {
		members = []*model.ProjectMember{}
	}
	writeJSON(w, http.StatusOK, members)
}

func (s *Server) handleSetMember(w http.ResponseWriter, r *http.Request) {
	pid := r.PathValue("id")
	if _, err := s.store.GetProject(r.Context(), pid); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	if !s.requireProjectCap(w, r, pid, model.CapManage) {
		return
	}
	var in struct {
		UserID string `json:"userId"`
		Role   string `json:"role"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	// The role may be a built-in (viewer/editor/admin) or a custom-role id.
	if model.ProjectRoleRank(in.Role) == 0 {
		if _, err := s.store.GetCustomRole(r.Context(), in.Role); err != nil {
			writeErr(w, http.StatusBadRequest, "role must be viewer, editor, admin or a custom-role id")
			return
		}
	}
	if _, err := s.store.GetUser(r.Context(), in.UserID); err != nil {
		writeErr(w, http.StatusBadRequest, "user not found")
		return
	}
	if err := s.store.SetProjectMember(r.Context(), pid, in.UserID, in.Role); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.recordActivity(r.Context(), s.actor(r.Context()), "member.set", in.UserID, in.Role)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleRemoveMember(w http.ResponseWriter, r *http.Request) {
	pid := r.PathValue("id")
	if !s.requireProjectCap(w, r, pid, model.CapManage) {
		return
	}
	if err := s.store.RemoveProjectMember(r.Context(), pid, r.PathValue("userId")); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.recordActivity(r.Context(), s.actor(r.Context()), "member.removed", r.PathValue("userId"), "")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
