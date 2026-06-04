package auth

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"errors"
	"fmt"
	"time"

	"github.com/mipopov/shclop/internal/crypto"
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
	aead *crypto.AESGCM
	now  func() time.Time
}

func NewCookieCodec(rawKey []byte) (*CookieCodec, error) {
	aead, err := crypto.NewAESGCM(rawKey)
	if err != nil {
		return nil, errors.New("cookiecodec: key must not be empty")
	}
	return &CookieCodec{aead: aead, now: time.Now}, nil
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

	ciphertext, err := c.aead.Seal(buf.Bytes())
	if err != nil {
		return "", fmt.Errorf("cookiecodec encode: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(ciphertext), nil
}

func (c *CookieCodec) Decode(encoded string) (OIDCStateCookie, error) {
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return OIDCStateCookie{}, fmt.Errorf("%w: base64: %v", ErrInvalidCookie, err)
	}

	plaintext, err := c.aead.Open(raw)
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
