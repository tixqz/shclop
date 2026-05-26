package integrations

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

// SecretBox provides AES-256-GCM encryption and decryption of sensitive data
// such as API tokens before they are stored in the database.
type SecretBox struct {
	key []byte // 32-byte AES-256 key
}

// NewSecretBox creates a SecretBox with the given raw key. If the key is
// not exactly 32 bytes, it is SHA-256 derived to produce a deterministic
// 32-byte key. In production, the key should be exactly 32 bytes from a
// secure source (e.g. SHCLOP_INTEGRATION_ENCRYPTION_KEY env var).
func NewSecretBox(rawKey []byte) *SecretBox {
	if len(rawKey) != 32 {
		// Derive a deterministic 32-byte key via SHA-256.
		h := sha256.Sum256(rawKey)
		rawKey = h[:]
	}
	key := make([]byte, 32)
	copy(key, rawKey[:32])
	return &SecretBox{key: key}
}

// NewSecretBoxFromConfig creates a SecretBox from a configuration string.
// If the string is empty, a dev/test fallback key is derived.
// Production deployments MUST set SHCLOP_INTEGRATION_ENCRYPTION_KEY to a
// secure value.
//
// Dev fallback: When the config string is empty, we derive a key from
// a hard-coded string so that tests and local development work without
// additional configuration. This is NOT suitable for production — the key
// must be set explicitly via the environment variable.
func NewSecretBoxFromConfig(configKey string) (*SecretBox, error) {
	if configKey == "" {
		// Dev/test fallback: derive a deterministic key. This allows the
		// server to start without an encryption key configured, which is
		// convenient for development and testing. Production deployments
		// MUST set SHCLOP_INTEGRATION_ENCRYPTION_KEY.
		return NewSecretBox([]byte("shclop-dev-fallback-integration-encryption-key-2026")), nil
	}
	return NewSecretBox([]byte(configKey)), nil
}

// Encrypt encrypts plaintext using AES-256-GCM. It returns a byte slice
// containing (nonce || ciphertext || auth tag). The nonce is 12 bytes,
// generated randomly for each encryption operation.
func (b *SecretBox) Encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(b.key)
	if err != nil {
		return nil, fmt.Errorf("aes new cipher: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("aes gcm: %w", err)
	}

	nonce := make([]byte, aesgcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("nonce: %w", err)
	}

	// Seal appends the encrypted data (with auth tag) to nonce.
	ciphertext := aesgcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts a ciphertext produced by Encrypt. The input must be
// (nonce || ciphertext || auth tag).
func (b *SecretBox) Decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(b.key)
	if err != nil {
		return nil, fmt.Errorf("aes new cipher: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("aes gcm: %w", err)
	}

	nonceSize := aesgcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	return plaintext, nil
}

// EncryptToString encrypts and returns hex-encoded string.
func (b *SecretBox) EncryptToString(plaintext []byte) (string, error) {
	data, err := b.Encrypt(plaintext)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

// DecryptFromString decodes a hex-encoded ciphertext and decrypts it.
func (b *SecretBox) DecryptFromString(encoded string) ([]byte, error) {
	data, err := hex.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("hex decode: %w", err)
	}
	return b.Decrypt(data)
}
