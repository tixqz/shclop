// Package crypto provides shared cryptographic primitives used across
// shclop. The AESGCM type wraps the AES-256-GCM primitive that the
// integrations secretbox and the auth cookie codec share.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
)

// AESGCM authenticates and encrypts arbitrary byte slices with a 32-byte
// key. Output layout: nonce (12 bytes) || ciphertext || auth-tag.
type AESGCM struct {
	key []byte
}

// NewAESGCM returns an AESGCM bound to the given key. Keys that aren't
// exactly 32 bytes are SHA-256 derived to a deterministic 32-byte key.
// An empty key is rejected.
func NewAESGCM(rawKey []byte) (*AESGCM, error) {
	if len(rawKey) == 0 {
		return nil, errors.New("crypto: key must not be empty")
	}
	key := make([]byte, 32)
	if len(rawKey) == 32 {
		copy(key, rawKey)
	} else {
		h := sha256.Sum256(rawKey)
		copy(key, h[:])
	}
	return &AESGCM{key: key}, nil
}

// Seal encrypts plaintext and returns nonce||ciphertext||tag.
func (a *AESGCM) Seal(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(a.key)
	if err != nil {
		return nil, fmt.Errorf("aes new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("aes gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("nonce: %w", err)
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Open decrypts a payload produced by Seal. Input layout must be
// nonce||ciphertext||tag.
func (a *AESGCM) Open(payload []byte) ([]byte, error) {
	block, err := aes.NewCipher(a.key)
	if err != nil {
		return nil, fmt.Errorf("aes new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("aes gcm: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(payload) < nonceSize {
		return nil, errors.New("crypto: ciphertext too short")
	}
	nonce, ct := payload[:nonceSize], payload[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return plaintext, nil
}
