package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nikiv/ansible-ui/internal/model"
)

const (
	settingSyslogEnabled  = "syslog.enabled"
	settingSyslogAddress  = "syslog.address"  // host:port
	settingSyslogProtocol = "syslog.protocol" // udp | tcp
	settingSyslogTag      = "syslog.tag"      // APP-NAME field
	settingLogHTTPEnabled = "logexport.http_enabled"
	settingLogHTTPURL     = "logexport.http_url"   // e.g. a Splunk HEC / Elasticsearch _doc URL
	settingLogHTTPToken   = "logexport.http_token" // auth token (format-dependent)
	settingLogHTTPFormat  = "logexport.http_format" // splunk | elasticsearch | generic
)

const syslogFacility = 16 // local0

// forwardSyslog ships a run-completion event to an external syslog collector
// (RFC 5424) when enabled — the "log export to external systems" capability.
func (s *Server) forwardSyslog(run *model.Run, status string) {
	ctx := context.Background()
	if !s.boolSetting(ctx, settingSyslogEnabled) {
		return
	}
	addr, _, _ := s.store.GetSetting(ctx, settingSyslogAddress)
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return
	}
	proto, _, _ := s.store.GetSetting(ctx, settingSyslogProtocol)
	if proto != "tcp" {
		proto = "udp"
	}
	tag, _, _ := s.store.GetSetting(ctx, settingSyslogTag)
	if strings.TrimSpace(tag) == "" {
		tag = "ansible-ui"
	}

	severity := 5 // notice
	switch status {
	case model.StatusSuccess:
		severity = 6 // info
	case model.StatusFailed, model.StatusCanceled:
		severity = 3 // error
	}
	host, _ := os.Hostname()
	if host == "" {
		host = "ansible-ui"
	}
	exit := ""
	if run.ExitCode != nil {
		exit = strconv.Itoa(*run.ExitCode)
	}
	by := run.TriggeredBy
	if by == "" {
		by = "system"
	}
	msg := fmt.Sprintf("run=%q status=%s project=%q app=%s exit=%s by=%s id=%s",
		run.Name, status, run.ProjectName, run.App, exit, by, run.ID)
	// RFC 5424: <PRI>1 TIMESTAMP HOST APP PROCID MSGID SD MSG
	line := fmt.Sprintf("<%d>1 %s %s %s %s RUN - %s",
		syslogFacility*8+severity, time.Now().Format(time.RFC3339), host, tag, run.ID, msg)

	conn, err := net.DialTimeout(proto, addr, 5*time.Second)
	if err != nil {
		s.log.Warn("syslog forward failed", "addr", addr, "err", err)
		return
	}
	defer conn.Close()
	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write([]byte(line)); err != nil {
		s.log.Warn("syslog write failed", "addr", addr, "err", err)
	}
}

// forwardHTTPLog posts a run-completion event as JSON to an HTTP collector
// (Splunk HEC shape: {event, sourcetype, time}); auth via "Authorization: Splunk
// <token>" when a token is set. Generic collectors accept the same body.
func (s *Server) forwardHTTPLog(run *model.Run, status string) {
	ctx := context.Background()
	if !s.boolSetting(ctx, settingLogHTTPEnabled) {
		return
	}
	url, _, _ := s.store.GetSetting(ctx, settingLogHTTPURL)
	url = strings.TrimSpace(url)
	if url == "" {
		return
	}
	exit := 0
	if run.ExitCode != nil {
		exit = *run.ExitCode
	}
	by := run.TriggeredBy
	if by == "" {
		by = "system"
	}
	event := map[string]any{
		"id": run.ID, "run": run.Name, "status": status, "project": run.ProjectName,
		"app": run.App, "playbook": run.Playbook, "exitCode": exit, "by": by, "time": time.Now().Format(time.RFC3339),
	}
	format, _, _ := s.store.GetSetting(ctx, settingLogHTTPFormat)
	tok := strings.TrimSpace(func() string { v, _, _ := s.store.GetSetting(ctx, settingLogHTTPToken); return v }())

	var body []byte
	authHeader := ""
	switch format {
	case "elasticsearch", "generic":
		// Post the flat event document; the token (if any) is used verbatim as the
		// Authorization header (e.g. "ApiKey …" / "Bearer …" / "Basic …").
		body, _ = json.Marshal(event)
		authHeader = tok
	default: // splunk HEC
		body, _ = json.Marshal(map[string]any{"event": event, "sourcetype": "ansible-ui", "time": time.Now().Unix()})
		if tok != "" {
			authHeader = "Splunk " + tok
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		s.log.Warn("http log export failed", "url", url, "err", err)
		return
	}
	_ = resp.Body.Close()
}

type syslogConfig struct {
	Enabled     bool   `json:"enabled"`
	Address     string `json:"address"`
	Protocol    string `json:"protocol"`
	Tag         string `json:"tag"`
	HTTPEnabled bool   `json:"httpEnabled"`
	HTTPURL     string `json:"httpUrl"`
	HTTPToken   string `json:"httpToken"`
	HTTPFormat  string `json:"httpFormat"` // splunk | elasticsearch | generic
}

func (s *Server) handleGetSyslog(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	addr, _, _ := s.store.GetSetting(r.Context(), settingSyslogAddress)
	proto, _, _ := s.store.GetSetting(r.Context(), settingSyslogProtocol)
	if proto == "" {
		proto = "udp"
	}
	tag, _, _ := s.store.GetSetting(r.Context(), settingSyslogTag)
	if tag == "" {
		tag = "ansible-ui"
	}
	httpURL, _, _ := s.store.GetSetting(r.Context(), settingLogHTTPURL)
	httpTok, _, _ := s.store.GetSetting(r.Context(), settingLogHTTPToken)
	httpFmt, _, _ := s.store.GetSetting(r.Context(), settingLogHTTPFormat)
	if httpFmt == "" {
		httpFmt = "splunk"
	}
	writeJSON(w, http.StatusOK, syslogConfig{
		Enabled: s.boolSetting(r.Context(), settingSyslogEnabled), Address: addr, Protocol: proto, Tag: tag,
		HTTPEnabled: s.boolSetting(r.Context(), settingLogHTTPEnabled), HTTPURL: httpURL, HTTPToken: httpTok, HTTPFormat: httpFmt,
	})
}

func (s *Server) handleSetSyslog(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var in syslogConfig
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	if in.Protocol != "tcp" {
		in.Protocol = "udp"
	}
	if strings.TrimSpace(in.Tag) == "" {
		in.Tag = "ansible-ui"
	}
	_ = s.store.SetSetting(r.Context(), settingSyslogEnabled, boolStr(in.Enabled))
	_ = s.store.SetSetting(r.Context(), settingSyslogAddress, strings.TrimSpace(in.Address))
	_ = s.store.SetSetting(r.Context(), settingSyslogProtocol, in.Protocol)
	_ = s.store.SetSetting(r.Context(), settingSyslogTag, strings.TrimSpace(in.Tag))
	_ = s.store.SetSetting(r.Context(), settingLogHTTPEnabled, boolStr(in.HTTPEnabled))
	_ = s.store.SetSetting(r.Context(), settingLogHTTPURL, strings.TrimSpace(in.HTTPURL))
	_ = s.store.SetSetting(r.Context(), settingLogHTTPToken, strings.TrimSpace(in.HTTPToken))
	if in.HTTPFormat == "" {
		in.HTTPFormat = "splunk"
	}
	_ = s.store.SetSetting(r.Context(), settingLogHTTPFormat, in.HTTPFormat)
	s.recordActivity(r.Context(), s.actor(r.Context()), "syslog.updated",
		fmt.Sprintf("syslog=%v http=%v", in.Enabled, in.HTTPEnabled), "")
	writeJSON(w, http.StatusOK, in)
}
