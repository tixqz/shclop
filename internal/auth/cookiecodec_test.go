package auth

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func makeCodec(t *testing.T, key string) *CookieCodec {
	t.Helper()
	c, err := NewCookieCodecFromConfig(key)
	if err != nil {
		t.Fatalf("NewCookieCodecFromConfig: %v", err)
	}
	return c
}

var samplePayload = OIDCStateCookie{
	State:        "state123",
	Nonce:        "nonce456",
	CodeVerifier: "verifier789",
	Provider:     "google",
	ReturnTo:     "/dashboard",
	ExpiresAt:    time.Now().Add(5 * time.Minute),
}

func TestCookieCodec_RoundTrip(t *testing.T) {
	c := makeCodec(t, "test-key-roundtrip")
	encoded, err := c.Encode(samplePayload)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	got, err := c.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got.State != samplePayload.State ||
		got.Nonce != samplePayload.Nonce ||
		got.CodeVerifier != samplePayload.CodeVerifier ||
		got.Provider != samplePayload.Provider ||
		got.ReturnTo != samplePayload.ReturnTo ||
		!got.ExpiresAt.Equal(samplePayload.ExpiresAt) {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, samplePayload)
	}
}

func TestCookieCodec_TamperedCiphertext(t *testing.T) {
	c := makeCodec(t, "test-key-tamper")
	encoded, err := c.Encode(samplePayload)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	mid := len(raw) / 2
	raw[mid] ^= 0xFF
	tampered := base64.RawURLEncoding.EncodeToString(raw)

	_, err = c.Decode(tampered)
	if err == nil {
		t.Fatal("expected error for tampered ciphertext, got nil")
	}
	if !isInvalidCookie(err) {
		t.Errorf("expected ErrInvalidCookie, got: %v", err)
	}
}

func TestCookieCodec_Expired(t *testing.T) {
	c := makeCodec(t, "test-key-expired")
	payload := OIDCStateCookie{
		State:     "s",
		ExpiresAt: time.Now().Add(-1 * time.Minute),
	}
	encoded, err := c.Encode(payload)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	got, err := c.Decode(encoded)
	if err != ErrExpired {
		t.Errorf("expected ErrExpired, got: %v", err)
	}
	if got.State != payload.State {
		t.Errorf("expected payload to be populated on expiry, got: %+v", got)
	}
}

func TestCookieCodec_WrongKey(t *testing.T) {
	c1 := makeCodec(t, "key-one")
	c2 := makeCodec(t, "key-two")

	encoded, err := c1.Encode(samplePayload)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	_, err = c2.Decode(encoded)
	if err == nil {
		t.Fatal("expected error decoding with wrong key, got nil")
	}
	if !isInvalidCookie(err) {
		t.Errorf("expected ErrInvalidCookie, got: %v", err)
	}
}

func TestCookieCodec_EmptyKey(t *testing.T) {
	_, err := NewCookieCodec(nil)
	if err == nil {
		t.Error("NewCookieCodec(nil) should return error")
	}
	_, err = NewCookieCodec([]byte{})
	if err == nil {
		t.Error("NewCookieCodec(empty) should return error")
	}
	_, err = NewCookieCodecFromConfig("")
	if err == nil {
		t.Error("NewCookieCodecFromConfig(\"\") should return error")
	}
}

func TestCookieCodec_URLSafe(t *testing.T) {
	c := makeCodec(t, "test-key-urlsafe")
	encoded, err := c.Encode(samplePayload)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if strings.ContainsAny(encoded, "+/=") {
		t.Errorf("encoded value contains URL-unsafe characters: %q", encoded)
	}
}

func isInvalidCookie(err error) bool {
	return err != nil && strings.Contains(err.Error(), ErrInvalidCookie.Error())
}
