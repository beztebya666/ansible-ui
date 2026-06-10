package api

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/nikiv/ansible-ui/internal/model"
)

// Hand-rolled TACACS+ (RFC 8907) ASCII-login authentication, no third-party SDK.
// Flow: AUTHEN START (with username) → server REPLY GETPASS → CONTINUE (password)
// → server REPLY PASS/FAIL. The body is obfuscated with the chained-MD5 pad.

const (
	tacVersion       = 0xc0 // major 12 (0xc), minor 0
	tacTypeAuthen    = 0x01
	tacFlagUnencrypt = 0x01

	tacAuthenLogin   = 0x01 // action
	tacAuthenASCII   = 0x01 // authen_type
	tacAuthenSvcLogin = 0x01

	tacStatusPass    = 0x01
	tacStatusFail    = 0x02
	tacStatusGetPass = 0x05
	tacStatusError   = 0x07
)

// errTACACSUnavailable marks a connectivity/config failure (vs a credential
// rejection), so local accounts still sign in (local is tried first).
var errTACACSUnavailable = errors.New("tacacs unavailable")

// authenticateTACACS verifies username/password against the TACACS+ server and,
// on PASS, upserts a local user row.
func (s *Server) authenticateTACACS(ctx context.Context, username, password string) (*model.User, error) {
	cfg := s.cfg.TACACS
	if cfg.Server == "" || cfg.Secret == "" {
		return nil, errors.New("tacacs not configured")
	}
	if password == "" {
		return nil, errors.New("empty password")
	}
	addr := cfg.Server
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = net.JoinHostPort(addr, "49")
	}
	pass, err := tacacsAuthCheck(addr, cfg.Secret, username, password)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errTACACSUnavailable, err)
	}
	if !pass {
		return nil, errors.New("invalid tacacs credentials")
	}
	role := cfg.DefaultRole
	if role == "" {
		role = model.RoleUser
	}
	return s.store.UpsertExternalUser(ctx, username, "", role)
}

// tacacsPad XORs body with the chained-MD5 pseudo-pad keyed by the session.
func tacacsPad(body []byte, sessionID uint32, key string, seqNo, version byte) []byte {
	var sid [4]byte
	binary.BigEndian.PutUint32(sid[:], sessionID)
	base := append(append(append(sid[:], []byte(key)...), version), seqNo)
	pad := make([]byte, 0, len(body)+md5.Size)
	var prev []byte
	for len(pad) < len(body) {
		h := md5.New()
		h.Write(base)
		h.Write(prev)
		prev = h.Sum(nil)
		pad = append(pad, prev...)
	}
	out := make([]byte, len(body))
	for i := range body {
		out[i] = body[i] ^ pad[i]
	}
	return out
}

func tacacsHeader(seqNo byte, sessionID uint32, bodyLen int) []byte {
	h := make([]byte, 12)
	h[0] = tacVersion
	h[1] = tacTypeAuthen
	h[2] = seqNo
	h[3] = 0 // flags: 0 = body is obfuscated
	binary.BigEndian.PutUint32(h[4:8], sessionID)
	binary.BigEndian.PutUint32(h[8:12], uint32(bodyLen))
	return h
}

// sendPacket obfuscates + writes a TACACS+ packet; readPacket reads + deobfuscates.
func tacacsSend(conn net.Conn, seqNo byte, sessionID uint32, key string, body []byte) error {
	pkt := append(tacacsHeader(seqNo, sessionID, len(body)), tacacsPad(body, sessionID, key, seqNo, tacVersion)...)
	_, err := conn.Write(pkt)
	return err
}

func tacacsRead(conn net.Conn, key string) (seqNo byte, body []byte, err error) {
	hdr := make([]byte, 12)
	if _, err = io.ReadFull(conn, hdr); err != nil {
		return 0, nil, err
	}
	seqNo = hdr[2]
	sessionID := binary.BigEndian.Uint32(hdr[4:8])
	blen := binary.BigEndian.Uint32(hdr[8:12])
	if blen > 1<<16 {
		return 0, nil, errors.New("tacacs body too large")
	}
	enc := make([]byte, blen)
	if _, err = io.ReadFull(conn, enc); err != nil {
		return 0, nil, err
	}
	if hdr[3]&tacFlagUnencrypt == 0 {
		body = tacacsPad(enc, sessionID, key, seqNo, hdr[0])
	} else {
		body = enc
	}
	return seqNo, body, nil
}

// tacacsAuthCheck runs the ASCII-login exchange and reports PASS/FAIL.
func tacacsAuthCheck(addr, key, username, password string) (bool, error) {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return false, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(8 * time.Second))

	var sidb [4]byte
	_, _ = rand.Read(sidb[:])
	sessionID := binary.BigEndian.Uint32(sidb[:])

	// AUTHEN START (seq 1): action/priv/type/service + lengths + username.
	start := []byte{tacAuthenLogin, 0x01, tacAuthenASCII, tacAuthenSvcLogin,
		byte(len(username)), 0, 0, 0}
	start = append(start, []byte(username)...)
	if err := tacacsSend(conn, 1, sessionID, key, start); err != nil {
		return false, err
	}
	// REPLY (seq 2): expect GETPASS.
	_, body, err := tacacsRead(conn, key)
	if err != nil {
		return false, err
	}
	st, err := tacacsReplyStatus(body)
	if err != nil {
		return false, err
	}
	if st == tacStatusPass {
		return true, nil // server accepted on the username alone (unusual but valid)
	}
	if st != tacStatusGetPass {
		return false, nil // FAIL/ERROR/GETUSER → treat as denied
	}
	// CONTINUE (seq 3): user_msg = password.
	cont := make([]byte, 5)
	binary.BigEndian.PutUint16(cont[0:2], uint16(len(password))) // user_msg_len
	// data_len = 0 (cont[2:4]); flags = 0 (cont[4])
	cont = append(cont, []byte(password)...)
	if err := tacacsSend(conn, 3, sessionID, key, cont); err != nil {
		return false, err
	}
	// REPLY (seq 4): PASS / FAIL.
	_, body, err = tacacsRead(conn, key)
	if err != nil {
		return false, err
	}
	st, err = tacacsReplyStatus(body)
	if err != nil {
		return false, err
	}
	return st == tacStatusPass, nil
}

// tacacsReplyStatus parses the status byte from an AUTHEN REPLY body.
func tacacsReplyStatus(body []byte) (byte, error) {
	if len(body) < 6 {
		return 0, errors.New("short tacacs reply")
	}
	return body[0], nil // status, flags, server_msg_len[2], data_len[2], …
}

func (s *Server) tacacsEnabled() bool {
	return s.cfg.TACACS.Enabled && s.cfg.TACACS.Server != "" && s.cfg.TACACS.Secret != ""
}
