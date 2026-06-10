package api

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/nikiv/ansible-ui/internal/model"
)

// Hand-rolled RADIUS (RFC 2865) Access-Request with PAP, no third-party SDK.
// Just enough to authenticate a username/password against a RADIUS server and
// upsert the user locally — mirroring the LDAP path.

const (
	radiusAccessRequest = 1
	radiusAccessAccept  = 2
	radiusAccessReject  = 3
	radiusAttrUserName  = 1
	radiusAttrUserPass  = 2
	radiusAttrNASID     = 32
)

// authenticateRADIUS verifies username/password against the configured RADIUS
// server (PAP) and, on Access-Accept, upserts a local user row.
func (s *Server) authenticateRADIUS(ctx context.Context, username, password string) (*model.User, error) {
	cfg := s.cfg.RADIUS
	if cfg.Server == "" || cfg.Secret == "" {
		return nil, errors.New("radius not configured")
	}
	if password == "" {
		return nil, errors.New("empty password")
	}
	addr := cfg.Server
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = net.JoinHostPort(addr, "1812")
	}
	accept, err := radiusAccessCheck(addr, cfg.Secret, cfg.NASID, username, password)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errRADIUSUnavailable, err)
	}
	if !accept {
		return nil, errors.New("invalid radius credentials")
	}
	role := cfg.DefaultRole
	if role == "" {
		role = model.RoleUser
	}
	return s.store.UpsertExternalUser(ctx, username, "", role)
}

// errRADIUSUnavailable marks a connectivity/config failure (vs a credential
// rejection), so local accounts still sign in (local is tried first).
var errRADIUSUnavailable = errors.New("radius unavailable")

// radiusAccessCheck sends an Access-Request (PAP) and reports whether the server
// returned Access-Accept. Verifies the response authenticator.
func radiusAccessCheck(addr, secret, nasID, username, password string) (bool, error) {
	conn, err := net.DialTimeout("udp", addr, 5*time.Second)
	if err != nil {
		return false, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	// Request Authenticator: 16 random bytes.
	var reqAuth [16]byte
	if _, err := rand.Read(reqAuth[:]); err != nil {
		return false, err
	}
	// Attributes.
	var attrs []byte
	attrs = append(attrs, radiusAttr(radiusAttrUserName, []byte(username))...)
	attrs = append(attrs, radiusAttr(radiusAttrUserPass, radiusEncryptPassword(password, secret, reqAuth[:]))...)
	if nasID != "" {
		attrs = append(attrs, radiusAttr(radiusAttrNASID, []byte(nasID))...)
	}
	id := reqAuth[0] // any byte works as the packet identifier
	pkt := radiusPacket(radiusAccessRequest, id, reqAuth[:], attrs)

	if _, err := conn.Write(pkt); err != nil {
		return false, err
	}
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return false, err
	}
	if n < 20 {
		return false, errors.New("short radius response")
	}
	resp := buf[:n]
	// Verify the Response Authenticator = MD5(Code|ID|Length|RequestAuth|Attrs|Secret).
	respAuth := make([]byte, 16)
	copy(respAuth, resp[4:20])
	check := make([]byte, 0, n+len(secret))
	check = append(check, resp[:4]...)
	check = append(check, reqAuth[:]...)
	check = append(check, resp[20:n]...)
	check = append(check, []byte(secret)...)
	if sum := md5.Sum(check); !equalBytes(sum[:], respAuth) {
		return false, errors.New("bad radius response authenticator (wrong shared secret?)")
	}
	return resp[0] == radiusAccessAccept, nil
}

func radiusPacket(code, id byte, auth, attrs []byte) []byte {
	length := 20 + len(attrs)
	pkt := make([]byte, 0, length)
	pkt = append(pkt, code, id)
	var l [2]byte
	binary.BigEndian.PutUint16(l[:], uint16(length))
	pkt = append(pkt, l[:]...)
	pkt = append(pkt, auth...)
	pkt = append(pkt, attrs...)
	return pkt
}

func radiusAttr(typ byte, val []byte) []byte {
	a := make([]byte, 0, 2+len(val))
	a = append(a, typ, byte(2+len(val)))
	return append(a, val...)
}

// radiusEncryptPassword hides the password per RFC 2865 §5.2: pad to 16-byte
// blocks; b1 = MD5(secret + RequestAuth), c1 = p1 XOR b1; bN = MD5(secret + c(N-1)).
func radiusEncryptPassword(password, secret string, reqAuth []byte) []byte {
	pw := []byte(password)
	if len(pw)%16 != 0 {
		pw = append(pw, make([]byte, 16-len(pw)%16)...)
	}
	out := make([]byte, len(pw))
	prev := reqAuth
	for i := 0; i < len(pw); i += 16 {
		h := md5.Sum(append([]byte(secret), prev...))
		for j := 0; j < 16; j++ {
			out[i+j] = pw[i+j] ^ h[j]
		}
		prev = out[i : i+16]
	}
	return out
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var d byte
	for i := range a {
		d |= a[i] ^ b[i]
	}
	return d == 0
}

// radiusEnabled reports whether RADIUS auth is configured (for /api/auth/status).
func (s *Server) radiusEnabled() bool {
	return s.cfg.RADIUS.Enabled && s.cfg.RADIUS.Server != "" && strings.TrimSpace(s.cfg.RADIUS.Secret) != ""
}
