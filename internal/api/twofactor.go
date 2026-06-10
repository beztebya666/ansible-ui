package api

import (
	"encoding/base64"
	"net/http"

	qrcode "github.com/skip2/go-qrcode"
)

// qrDataURI renders text as a QR code PNG and returns it as a data: URI suitable
// for an <img src>. Returns "" on failure (the manual key entry still works).
func qrDataURI(text string) string {
	png, err := qrcode.Encode(text, qrcode.Medium, 256)
	if err != nil {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
}

// Two-factor auth (TOTP) for local accounts. Opt-in: a user enables it for their
// own account; password-only accounts are unaffected. The shared secret is
// AES-encrypted at rest and never returned after setup.

// handle2FASetup generates a fresh secret (pending, not yet enabled) and returns
// it + the provisioning URI for the user to add to an authenticator app.
func (s *Server) handle2FASetup(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u == nil {
		writeErr(w, http.StatusUnauthorized, "not signed in")
		return
	}
	secret := generateTOTPSecret()
	blob, err := s.cipher.Encrypt([]byte(secret))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "cannot store secret")
		return
	}
	if err := s.store.SetUserTOTP(r.Context(), u.ID, blob, false); err != nil { // pending until verified
		s.handleStoreErr(w, err)
		return
	}
	uri := otpauthURL(secret, u.Username)
	writeJSON(w, http.StatusOK, map[string]string{
		"secret":     secret,
		"otpauthUrl": uri,
		"qrDataUri":  qrDataURI(uri),
	})
}

// handle2FAEnable verifies a code against the pending secret and turns 2FA on.
func (s *Server) handle2FAEnable(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u == nil {
		writeErr(w, http.StatusUnauthorized, "not signed in")
		return
	}
	var in struct {
		Code string `json:"code"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	blob, _, err := s.store.GetUserTOTP(r.Context(), u.ID)
	if err != nil || len(blob) == 0 {
		writeErr(w, http.StatusBadRequest, "run 2FA setup first")
		return
	}
	secret, derr := s.cipher.Decrypt(blob)
	if derr != nil || !validateTOTP(string(secret), in.Code) {
		writeErr(w, http.StatusBadRequest, "incorrect code — try again")
		return
	}
	if err := s.store.SetUserTOTP(r.Context(), u.ID, blob, true); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.recordActivity(r.Context(), u.Username, "auth.2faEnabled", u.Username, "")
	writeJSON(w, http.StatusOK, map[string]bool{"enabled": true})
}

// handle2FADisable turns 2FA off (requires a current code as proof of possession).
func (s *Server) handle2FADisable(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u == nil {
		writeErr(w, http.StatusUnauthorized, "not signed in")
		return
	}
	var in struct {
		Code string `json:"code"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	blob, enabled, err := s.store.GetUserTOTP(r.Context(), u.ID)
	if err == nil && enabled && len(blob) > 0 {
		secret, derr := s.cipher.Decrypt(blob)
		if derr != nil || !validateTOTP(string(secret), in.Code) {
			writeErr(w, http.StatusBadRequest, "incorrect code")
			return
		}
	}
	if err := s.store.SetUserTOTP(r.Context(), u.ID, nil, false); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	s.recordActivity(r.Context(), u.Username, "auth.2faDisabled", u.Username, "")
	writeJSON(w, http.StatusOK, map[string]bool{"enabled": false})
}

// handleAdminReset2FA lets an admin clear a user's 2FA (account recovery).
func (s *Server) handleAdminReset2FA(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	id := r.PathValue("id")
	target, err := s.store.GetUser(r.Context(), id)
	if err != nil {
		s.handleStoreErr(w, err)
		return
	}
	if err := s.store.SetUserTOTP(r.Context(), id, nil, false); err != nil {
		s.handleStoreErr(w, err)
		return
	}
	_ = s.store.SetUserEmailOTP(r.Context(), id, false) // clear email-OTP too
	s.recordActivity(r.Context(), s.actor(r.Context()), "auth.2faReset", target.Username, "admin reset")
	writeJSON(w, http.StatusOK, map[string]bool{"enabled": false})
}
