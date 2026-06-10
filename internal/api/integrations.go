package api

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/nikiv/ansible-ui/internal/model"
)

func (s *Server) handleListIntegrations(w http.ResponseWriter, r *http.Request) {
	templateID := r.URL.Query().Get("templateId")
	ins, err := s.store.ListIntegrations(r.Context(), templateID)
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	if scope := projectScope(r); scope != "" && templateID == "" {
		out := ins[:0]
		for _, in := range ins {
			if in.ProjectID == scope {
				out = append(out, in)
			}
		}
		ins = out
	}
	for _, in := range ins {
		hydrateIntegration(in)
	}
	writeJSON(w, http.StatusOK, ins)
}

func (s *Server) handleCreateIntegration(w http.ResponseWriter, r *http.Request) {
	var in struct {
		TemplateID  string   `json:"templateId"`
		WorkflowID  string   `json:"workflowId"`
		Name        string   `json:"name"`
		AuthMethod  string   `json:"authMethod"`
		AuthHeader  string   `json:"authHeader"`
		AuthSecret  string   `json:"authSecret"`
		PassPayload bool     `json:"passPayload"`
		Aliases     []string `json:"aliases"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if in.TemplateID == "" && in.WorkflowID == "" {
		writeErr(w, http.StatusBadRequest, "templateId or workflowId is required")
		return
	}
	if in.WorkflowID != "" {
		if _, err := s.store.GetWorkflow(r.Context(), in.WorkflowID); err != nil {
			writeErr(w, http.StatusBadRequest, "workflow not found")
			return
		}
	} else if _, err := s.store.GetTemplate(r.Context(), in.TemplateID); err != nil {
		writeErr(w, http.StatusBadRequest, "template not found")
		return
	}
	raw := make([]byte, 18)
	_, _ = rand.Read(raw)
	integ := &model.Integration{
		TemplateID:  in.TemplateID,
		WorkflowID:  in.WorkflowID,
		Name:        in.Name,
		Token:       "whk_" + hex.EncodeToString(raw),
		Active:      true,
		AuthMethod:  in.AuthMethod,
		AuthHeader:  in.AuthHeader,
		PassPayload: in.PassPayload,
		Aliases:     in.Aliases,
	}
	if in.AuthSecret != "" {
		blob, err := s.cipher.Encrypt([]byte(in.AuthSecret))
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "encrypt secret: "+err.Error())
			return
		}
		integ.AuthSecretBlob = blob
	}
	if err := s.store.CreateIntegration(r.Context(), integ); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.recordActivity(r.Context(), "", "integration.created", integ.Name, integ.AuthMethod)
	hydrateIntegration(integ)
	writeJSON(w, http.StatusCreated, integ)
}

func (s *Server) handleUpdateIntegration(w http.ResponseWriter, r *http.Request) {
	integ, err := s.store.GetIntegration(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	var in struct {
		Name        *string   `json:"name"`
		Active      *bool     `json:"active"`
		AuthMethod  *string   `json:"authMethod"`
		AuthHeader  *string   `json:"authHeader"`
		AuthSecret  *string   `json:"authSecret"`
		PassPayload *bool     `json:"passPayload"`
		Aliases     *[]string `json:"aliases"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if in.Name != nil {
		integ.Name = *in.Name
	}
	if in.Active != nil {
		integ.Active = *in.Active
	}
	if in.AuthMethod != nil {
		integ.AuthMethod = *in.AuthMethod
	}
	if in.AuthHeader != nil {
		integ.AuthHeader = *in.AuthHeader
	}
	if in.PassPayload != nil {
		integ.PassPayload = *in.PassPayload
	}
	if in.Aliases != nil {
		integ.Aliases = *in.Aliases
	}
	if in.AuthSecret != nil && *in.AuthSecret != "" {
		blob, err := s.cipher.Encrypt([]byte(*in.AuthSecret))
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "encrypt secret: "+err.Error())
			return
		}
		integ.AuthSecretBlob = blob
	}
	if err := s.store.UpdateIntegration(r.Context(), integ); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	hydrateIntegration(integ)
	s.recordActivity(r.Context(), "", "integration.updated", integ.Name, integ.AuthMethod)
	writeJSON(w, http.StatusOK, integ)
}

func (s *Server) handleDeleteIntegration(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := id
	if integ, err := s.store.GetIntegration(r.Context(), id); err == nil {
		name = integ.Name
	}
	if err := s.store.DeleteIntegration(r.Context(), id); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.recordActivity(r.Context(), "", "integration.deleted", name, "")
	w.WriteHeader(http.StatusNoContent)
}

// handleWebhook is the PUBLIC inbound endpoint — the URL token authorises it,
// and an optional auth method (HMAC/token/basic) verifies the caller.
func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	integ, err := s.store.GetIntegrationByToken(r.Context(), r.PathValue("token"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "unknown or inactive webhook")
		return
	}
	if !s.verifyWebhookAuth(integ, r, body) {
		writeErr(w, http.StatusUnauthorized, "webhook authentication failed")
		return
	}
	var extra map[string]any
	if integ.PassPayload && len(body) > 0 {
		var payload any
		if json.Unmarshal(body, &payload) == nil {
			extra = map[string]any{"webhook": payload}
		}
	}

	// Workflow integration: start the pipeline (payload → workflow variables).
	if integ.WorkflowID != "" {
		wf, werr := s.store.GetWorkflow(r.Context(), integ.WorkflowID)
		if werr != nil {
			writeErr(w, http.StatusNotFound, "workflow not found")
			return
		}
		wr, lerr := s.startWorkflow(r.Context(), wf, "webhook", extra)
		if lerr != nil {
			writeErr(w, http.StatusBadRequest, lerr.Error())
			return
		}
		go s.store.TouchIntegration(r.Context(), integ.ID)
		s.log.Info("webhook triggered workflow", "integration", integ.ID, "workflow", wf.ID, "wfrun", wr.ID)
		writeJSON(w, http.StatusAccepted, map[string]any{"triggered": true, "workflowRunId": wr.ID})
		return
	}

	t, err := s.store.GetTemplate(r.Context(), integ.TemplateID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "template not found")
		return
	}
	run, err := s.runTemplateBy(r.Context(), t, " (webhook)", "webhook", extra)
	if err != nil {
		s.launchError(w, err)
		return
	}
	go s.store.TouchIntegration(r.Context(), integ.ID)
	s.log.Info("webhook triggered", "integration", integ.ID, "template", t.ID, "run", run.ID)
	writeJSON(w, http.StatusAccepted, map[string]any{"triggered": true, "runId": run.ID})
}

// verifyWebhookAuth checks the caller against the integration's auth method.
func (s *Server) verifyWebhookAuth(integ *model.Integration, r *http.Request, body []byte) bool {
	method := integ.AuthMethod
	if method == "" || method == model.AuthNone {
		return true
	}
	secret := ""
	if len(integ.AuthSecretBlob) > 0 {
		if plain, err := s.cipher.Decrypt(integ.AuthSecretBlob); err == nil {
			secret = string(plain)
		}
	}
	if secret == "" {
		return false // a method is configured but no secret — fail closed
	}
	switch method {
	case model.AuthToken:
		hdr := integ.AuthHeader
		if hdr == "" {
			hdr = "X-Auth-Token"
		}
		return constEq(r.Header.Get(hdr), secret)
	case model.AuthBasic:
		return constEq(r.Header.Get("Authorization"), "Basic "+base64.StdEncoding.EncodeToString([]byte(secret)))
	case model.AuthHMAC:
		hdr := integ.AuthHeader
		if hdr == "" {
			hdr = "X-Signature"
		}
		return hmacMatch(stripSHA256(r.Header.Get(hdr)), body, secret)
	case model.AuthGitHub:
		return hmacMatch(stripSHA256(r.Header.Get("X-Hub-Signature-256")), body, secret)
	case model.AuthBitbucket:
		sig := r.Header.Get("X-Hub-Signature")
		if sig == "" {
			sig = r.Header.Get("X-Hub-Signature-256")
		}
		return hmacMatch(stripSHA256(sig), body, secret)
	}
	return false
}

func stripSHA256(v string) string { return strings.TrimPrefix(strings.TrimSpace(v), "sha256=") }

func hmacMatch(got string, body []byte, secret string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	want := hex.EncodeToString(mac.Sum(nil))
	return constEq(strings.ToLower(got), want)
}

func constEq(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// hydrateIntegration strips secret material before returning to the client.
func hydrateIntegration(in *model.Integration) {
	in.HasSecret = len(in.AuthSecretBlob) > 0
	in.AuthSecretBlob = nil
	in.AuthSecret = ""
}
