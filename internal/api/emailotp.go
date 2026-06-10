package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/smtp"
	"strings"
)

const (
	settingSMTPHost = "smtp.host"
	settingSMTPPort = "smtp.port"
	settingSMTPFrom = "smtp.from"
	settingSMTPUser = "smtp.username"
	settingSMTPPass = "smtp.password"
)

type smtpConfig struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	From     string `json:"from"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) smtpConfigured(ctx context.Context) bool {
	v, _, _ := s.store.GetSetting(ctx, settingSMTPHost)
	return strings.TrimSpace(v) != ""
}

func (s *Server) handleGetSMTP(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	get := func(k string) string { v, _, _ := s.store.GetSetting(r.Context(), k); return v }
	port := get(settingSMTPPort)
	if port == "" {
		port = "587"
	}
	writeJSON(w, http.StatusOK, smtpConfig{
		Host: get(settingSMTPHost), Port: port, From: get(settingSMTPFrom),
		Username: get(settingSMTPUser), Password: get(settingSMTPPass),
	})
}

func (s *Server) handleSetSMTP(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var in smtpConfig
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	if in.Port == "" {
		in.Port = "587"
	}
	_ = s.store.SetSetting(r.Context(), settingSMTPHost, strings.TrimSpace(in.Host))
	_ = s.store.SetSetting(r.Context(), settingSMTPPort, strings.TrimSpace(in.Port))
	_ = s.store.SetSetting(r.Context(), settingSMTPFrom, strings.TrimSpace(in.From))
	_ = s.store.SetSetting(r.Context(), settingSMTPUser, strings.TrimSpace(in.Username))
	_ = s.store.SetSetting(r.Context(), settingSMTPPass, in.Password)
	s.recordActivity(r.Context(), s.actor(r.Context()), "smtp.updated", in.Host, "")
	writeJSON(w, http.StatusOK, in)
}

// sendOTPEmail delivers a login code over the configured global SMTP.
func (s *Server) sendOTPEmail(ctx context.Context, to, code string) error {
	get := func(k string) string { v, _, _ := s.store.GetSetting(ctx, k); return strings.TrimSpace(v) }
	host := get(settingSMTPHost)
	if host == "" {
		return fmt.Errorf("SMTP is not configured")
	}
	port := get(settingSMTPPort)
	if port == "" {
		port = "587"
	}
	from := get(settingSMTPFrom)
	if from == "" {
		from = "ansible-ui@localhost"
	}
	var b strings.Builder
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + to + "\r\n")
	b.WriteString("Subject: Your ansible-ui login code\r\n")
	b.WriteString("MIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n")
	b.WriteString("Your login code is: " + code + "\r\n\r\nIt expires in 10 minutes.\r\n")

	var auth smtp.Auth
	if user := get(settingSMTPUser); user != "" {
		auth = smtp.PlainAuth("", user, get(settingSMTPPass), host)
	}
	return smtp.SendMail(net.JoinHostPort(host, port), auth, from, []string{to}, []byte(b.String()))
}

func newOTPCode() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(1000000))
	return fmt.Sprintf("%06d", n.Int64())
}

func hashOTP(code string) string {
	h := sha256.Sum256([]byte(code))
	return hex.EncodeToString(h[:])
}

// handleEnableEmailOTP turns on email-OTP login for the current user (needs an
// email on file + a configured SMTP).
func (s *Server) handleEnableEmailOTP(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u == nil {
		writeErr(w, http.StatusUnauthorized, "not signed in")
		return
	}
	if strings.TrimSpace(u.Email) == "" {
		writeErr(w, http.StatusBadRequest, "set an email address first")
		return
	}
	if !s.smtpConfigured(r.Context()) {
		writeErr(w, http.StatusBadRequest, "an admin must configure SMTP first")
		return
	}
	if err := s.store.SetUserEmailOTP(r.Context(), u.ID, true); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.recordActivity(r.Context(), u.Username, "auth.emailOtpEnabled", u.Username, "")
	writeJSON(w, http.StatusOK, map[string]bool{"emailOtpEnabled": true})
}

func (s *Server) handleDisableEmailOTP(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u == nil {
		writeErr(w, http.StatusUnauthorized, "not signed in")
		return
	}
	if err := s.store.SetUserEmailOTP(r.Context(), u.ID, false); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.recordActivity(r.Context(), u.Username, "auth.emailOtpDisabled", u.Username, "")
	writeJSON(w, http.StatusOK, map[string]bool{"emailOtpEnabled": false})
}
