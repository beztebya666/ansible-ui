// Package crypto provides authenticated symmetric encryption for secrets at
// rest (Key Store entries). The key is derived from APP_SECRET via SHA-256.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"
)

// Cipher encrypts/decrypts secret blobs with AES-256-GCM.
type Cipher struct {
	aead cipher.AEAD
}

// New derives a Cipher from a secret string (any length).
func New(secret string) (*Cipher, error) {
	if secret == "" {
		return nil, errors.New("APP_SECRET must be set")
	}
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Cipher{aead: aead}, nil
}

// Encrypt returns nonce||ciphertext.
func (c *Cipher) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return c.aead.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt reverses Encrypt.
func (c *Cipher) Decrypt(blob []byte) ([]byte, error) {
	ns := c.aead.NonceSize()
	if len(blob) < ns {
		return nil, errors.New("ciphertext too short")
	}
	return c.aead.Open(nil, blob[:ns], blob[ns:], nil)
}
