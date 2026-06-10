package api

import (
	"encoding/json"
	"net/http"

	"github.com/nikiv/ansible-ui/internal/model"
	"github.com/nikiv/ansible-ui/internal/store"
)

// Minimal Model Context Protocol (MCP) server over Streamable HTTP at /api/mcp.
// It exposes the control plane to an AI agent as a set of tools (JSON-RPC 2.0:
// initialize / tools/list / tools/call). Auth reuses the API session/token guard,
// so the agent acts as a real user (project capabilities are enforced).

const mcpProtocolVersion = "2024-11-05"

type jsonRPCReq struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResp struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any           `json:"result,omitempty"`
	Error   *jsonRPCError `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type mcpTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

func obj(props map[string]any, required ...string) map[string]any {
	if props == nil {
		props = map[string]any{}
	}
	m := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		m["required"] = required
	}
	return m
}

func mcpTools() []mcpTool {
	strProp := map[string]any{"type": "string"}
	return []mcpTool{
		{Name: "list_projects", Description: "List all projects (tenants).", InputSchema: obj(nil)},
		{Name: "list_templates", Description: "List run templates, optionally scoped to a project.",
			InputSchema: obj(map[string]any{"projectId": strProp})},
		{Name: "run_template", Description: "Launch a run from a template (requires the run capability). Returns the new run id + status.",
			InputSchema: obj(map[string]any{"templateId": strProp}, "templateId")},
		{Name: "get_run", Description: "Get a run's status, app, exit code and timing.",
			InputSchema: obj(map[string]any{"runId": strProp}, "runId")},
		{Name: "list_runs", Description: "List recent runs, optionally scoped to a project.",
			InputSchema: obj(map[string]any{"projectId": strProp})},
	}
}

func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request) {
	var req jsonRPCReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusOK, jsonRPCResp{JSONRPC: "2.0", Error: &jsonRPCError{Code: -32700, Message: "parse error"}})
		return
	}
	// Notifications (no id) get no response body.
	switch req.Method {
	case "notifications/initialized", "notifications/cancelled":
		w.WriteHeader(http.StatusAccepted)
		return
	}
	resp := jsonRPCResp{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		resp.Result = map[string]any{
			"protocolVersion": mcpProtocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "ansible-ui", "version": appVersion},
		}
	case "ping":
		resp.Result = map[string]any{}
	case "tools/list":
		resp.Result = map[string]any{"tools": mcpTools()}
	case "tools/call":
		resp.Result = s.mcpCallTool(w, r, req.Params)
	default:
		resp.Error = &jsonRPCError{Code: -32601, Message: "method not found: " + req.Method}
	}
	writeJSON(w, http.StatusOK, resp)
}

// mcpResult wraps tool output as MCP content (text). isError marks tool failures
// (still a successful JSON-RPC response, per the MCP spec).
func mcpResult(text string, isErr bool) map[string]any {
	return map[string]any{"content": []map[string]any{{"type": "text", "text": text}}, "isError": isErr}
}

func mcpJSON(v any) map[string]any {
	b, _ := json.MarshalIndent(v, "", "  ")
	return mcpResult(string(b), false)
}

func (s *Server) mcpCallTool(w http.ResponseWriter, r *http.Request, raw json.RawMessage) map[string]any {
	var p struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	_ = json.Unmarshal(raw, &p)
	ctx := r.Context()
	str := func(k string) string {
		if v, ok := p.Arguments[k].(string); ok {
			return v
		}
		return ""
	}
	switch p.Name {
	case "list_projects":
		ps, err := s.store.ListProjects(ctx)
		if err != nil {
			return mcpResult(err.Error(), true)
		}
		return mcpJSON(ps)
	case "list_templates":
		ts, err := s.store.ListTemplates(ctx, str("projectId"))
		if err != nil {
			return mcpResult(err.Error(), true)
		}
		return mcpJSON(ts)
	case "list_runs":
		runs, err := s.store.ListRuns(ctx, store.RunFilter{ProjectID: str("projectId"), Limit: 50})
		if err != nil {
			return mcpResult(err.Error(), true)
		}
		return mcpJSON(runs)
	case "get_run":
		run, err := s.store.GetRun(ctx, str("runId"))
		if err != nil {
			return mcpResult(err.Error(), true)
		}
		return mcpJSON(run)
	case "run_template":
		t, err := s.store.GetTemplate(ctx, str("templateId"))
		if err != nil {
			return mcpResult("template not found", true)
		}
		if u := currentUser(r); !s.projectCaps(r, t.ProjectID, u)[model.CapRun] {
			return mcpResult("insufficient permission (need run) for this project", true)
		}
		by := "mcp"
		if u := currentUser(r); u != nil {
			by = u.Username
		}
		run, err := s.runTemplateBy(ctx, t, " (mcp)", by, nil)
		if err != nil {
			return mcpResult("launch failed: "+err.Error(), true)
		}
		return mcpJSON(map[string]string{"runId": run.ID, "status": run.Status, "name": run.Name})
	default:
		return mcpResult("unknown tool: "+p.Name, true)
	}
}
