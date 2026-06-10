package api

import (
	"bytes"
	"crypto/md5"
	"testing"
)

// radiusDecryptPassword reverses radiusEncryptPassword (the server side does
// this) — used here only to round-trip-test the chaining.
func radiusDecryptPassword(enc []byte, secret string, reqAuth []byte) []byte {
	out := make([]byte, len(enc))
	prev := reqAuth
	for i := 0; i < len(enc); i += 16 {
		h := md5.Sum(append([]byte(secret), prev...))
		for j := 0; j < 16 && i+j < len(enc); j++ {
			out[i+j] = enc[i+j] ^ h[j]
		}
		prev = enc[i : i+16]
	}
	return out
}

func TestRadiusEncryptPasswordRoundTrip(t *testing.T) {
	reqAuth := []byte("0123456789abcdef") // 16 bytes
	secret := "s3cr3t-shared"
	for _, pw := range []string{"x", "hunter2", "exactly-16-bytes", "a-much-longer-password-over-32-bytes-long!!"} {
		enc := radiusEncryptPassword(pw, secret, reqAuth)
		if len(enc)%16 != 0 {
			t.Fatalf("ciphertext not padded to 16: len=%d", len(enc))
		}
		dec := radiusDecryptPassword(enc, secret, reqAuth)
		// Decrypted is NUL-padded to the block size; the prefix must match.
		if !bytes.HasPrefix(dec, []byte(pw)) {
			t.Fatalf("round-trip mismatch for %q: got %q", pw, dec)
		}
		for _, b := range dec[len(pw):] {
			if b != 0 {
				t.Fatalf("padding for %q should be NUL, got %v", pw, dec[len(pw):])
			}
		}
	}
}

func TestRadiusAttrEncoding(t *testing.T) {
	a := radiusAttr(radiusAttrUserName, []byte("alice"))
	if a[0] != radiusAttrUserName || int(a[1]) != len(a) || string(a[2:]) != "alice" {
		t.Fatalf("bad attr encoding: %v", a)
	}
}
