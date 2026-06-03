package auth

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"time"
)

var ErrExpired = errors.New("oidc state cookie expired")
var ErrInvalidCookie = errors.New("oidc state cookie invalid")

type OIDCStateCookie struct {
	State        string
	Nonce        string
	CodeVerifier string
	Provider     string
	ReturnTo     string
	ExpiresAt    time.Time
}

type CookieCodec struct {
	key []byte
	now func() time.Time
}

func NewCookieCodec(rawKey []byte) (*CookieCodec, error) {
	if len(rawKey) == 0 {
		return nil, errors.New("cookiecodec: key must not be empty")
	}
	var key []byte
	if len(rawKey) == 32 {
		key = make([]byte, 32)
		copy(key, rawKey)
	} else {
		h := sha256.Sum256(rawKey)
		key = h[:]
	}
	return &CookieCodec{key: key, now: time.Now}, nil
}

func NewCookieCodecFromConfig(configKey string) (*CookieCodec, error) {
	if configKey == "" {
		return nil, errors.New("cookiecodec: config key must not be empty")
	}
	return NewCookieCodec([]byte(configKey))
}

func (c *CookieCodec) SetClock(fn func() time.Time) {
	c.now = fn
}

func (c *CookieCodec) Encode(payload OIDCStateCookie) (string, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(payload); err != nil {
		return "", fmt.Errorf("cookiecodec encode: gob: %w", err)
	}

	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", fmt.Errorf("cookiecodec encode: aes: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("cookiecodec encode: gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("cookiecodec encode: nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, buf.Bytes(), nil)
	return base64.RawURLEncoding.EncodeToString(ciphertext), nil
}

func (c *CookieCodec) Decode(encoded string) (OIDCStateCookie, error) {
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return OIDCStateCookie{}, fmt.Errorf("%w: base64: %v", ErrInvalidCookie, err)
	}

	block, err := aes.NewCipher(c.key)
	if err != nil {
		return OIDCStateCookie{}, fmt.Errorf("%w: aes: %v", ErrInvalidCookie, err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return OIDCStateCookie{}, fmt.Errorf("%w: gcm: %v", ErrInvalidCookie, err)
	}

	nonceSize := gcm.NonceSize()
	if len(raw) < nonceSize {
		return OIDCStateCookie{}, fmt.Errorf("%w: too short", ErrInvalidCookie)
	}

	plaintext, err := gcm.Open(nil, raw[:nonceSize], raw[nonceSize:], nil)
	if err != nil {
		return OIDCStateCookie{}, fmt.Errorf("%w: decrypt: %v", ErrInvalidCookie, err)
	}

	var payload OIDCStateCookie
	if err := gob.NewDecoder(bytes.NewReader(plaintext)).Decode(&payload); err != nil {
		return OIDCStateCookie{}, fmt.Errorf("%w: gob: %v", ErrInvalidCookie, err)
	}

	if !payload.ExpiresAt.IsZero() && c.now().After(payload.ExpiresAt) {
		return payload, ErrExpired
	}

	return payload, nil
}
