package crypto

import (
	"bytes"
	"testing"
)

func TestAESGCMRoundTrip(t *testing.T) {
	a, err := NewAESGCM([]byte("test-key-test-key-test-key-test!")) // 32 bytes
	if err != nil {
		t.Fatal(err)
	}
	plain := []byte("hello, world")
	ct, err := a.Seal(plain)
	if err != nil {
		t.Fatal(err)
	}
	got, err := a.Open(ct)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plain) {
		t.Errorf("round-trip mismatch: got %q want %q", got, plain)
	}
}

func TestAESGCMShortPayload(t *testing.T) {
	a, _ := NewAESGCM([]byte("k"))
	if _, err := a.Open([]byte{0x01}); err == nil {
		t.Errorf("expected error for short payload")
	}
}

func TestAESGCMKeyDerivation(t *testing.T) {
	// Two AESGCMs built from the same non-32-byte key must produce
	// mutually decryptable ciphertext (SHA-256 derived deterministically).
	a, _ := NewAESGCM([]byte("short"))
	b, _ := NewAESGCM([]byte("short"))
	ct, _ := a.Seal([]byte("x"))
	got, err := b.Open(ct)
	if err != nil || !bytes.Equal(got, []byte("x")) {
		t.Errorf("cross-instance decrypt failed: err=%v got=%q", err, got)
	}
}

func TestAESGCMEmptyKey(t *testing.T) {
	if _, err := NewAESGCM(nil); err == nil {
		t.Errorf("expected error for empty key")
	}
}
