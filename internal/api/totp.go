package api

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// TOTP (RFC 6238) — hand-rolled, no dependency. 6 digits, 30s step, SHA-1,
// the de-facto standard understood by Google Authenticator / Authy / 1Password.

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// generateTOTPSecret returns a fresh base32 shared secret (160 bits).
func generateTOTPSecret() string {
	b := make([]byte, 20)
	_, _ = rand.Read(b)
	return b32.EncodeToString(b)
}

// totpAt computes the 6-digit code for a given 30s counter.
func totpAt(secret string, counter uint64) string {
	key, err := b32.DecodeString(strings.ToUpper(strings.TrimSpace(secret)))
	if err != nil {
		return ""
	}
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)
	h := hmac.New(sha1.New, key)
	h.Write(buf[:])
	sum := h.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	code := (binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff) % 1_000_000
	return fmt.Sprintf("%06d", code)
}

// validateTOTP accepts the current code plus the adjacent windows (±30s clock
// skew). Constant-time compare.
func validateTOTP(secret, code string) bool {
	code = strings.TrimSpace(code)
	if len(code) != 6 || secret == "" {
		return false
	}
	counter := uint64(time.Now().Unix()) / 30
	for _, d := range []int64{0, -1, 1} {
		want := totpAt(secret, uint64(int64(counter)+d))
		if want != "" && hmac.Equal([]byte(want), []byte(code)) {
			return true
		}
	}
	return false
}

// otpauthURL is the provisioning URI an authenticator app scans / imports.
func otpauthURL(secret, account string) string {
	const issuer = "ansible-ui"
	v := url.Values{}
	v.Set("secret", secret)
	v.Set("issuer", issuer)
	v.Set("algorithm", "SHA1")
	v.Set("digits", "6")
	v.Set("period", "30")
	return "otpauth://totp/" + url.PathEscape(issuer+":"+account) + "?" + v.Encode()
}
