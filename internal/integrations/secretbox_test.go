package integrations

import (
	"bytes"
	"testing"
)

func TestSecretBoxRoundTrip(t *testing.T) {
	box := NewSecretBox([]byte("this-is-a-32-byte-test-key-1234567")) // exactly 32 bytes
	plaintext := []byte("ghp_test_token_12345")

	ciphertext, err := box.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	// ciphertext must not contain plaintext
	if bytes.Contains(ciphertext, plaintext) {
		t.Fatal("ciphertext must not contain plaintext")
	}

	decrypted, err := box.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("round trip failed: got %q, want %q", string(decrypted), string(plaintext))
	}
}

func TestSecretBoxDifferentKeys(t *testing.T) {
	plaintext := []byte("ghp_test_token")
	box1 := NewSecretBox([]byte("this-is-a-32-byte-test-key-1234567"))
	box2 := NewSecretBox([]byte("this-is-another-32-byte-test-key-xxx"))

	ciphertext1, err := box1.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	// Decrypt with different key should fail
	if _, err := box2.Decrypt(ciphertext1); err == nil {
		t.Fatal("expected decryption to fail with different key")
	}
}

func TestSecretBoxTamperedCiphertext(t *testing.T) {
	box := NewSecretBox([]byte("this-is-a-32-byte-test-key-1234567"))
	plaintext := []byte("ghp_test_token")

	ciphertext, err := box.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	// Tamper with the ciphertext
	tampered := make([]byte, len(ciphertext))
	copy(tampered, ciphertext)
	tampered[len(tampered)-1] ^= 0xFF

	if _, err := box.Decrypt(tampered); err == nil {
		t.Fatal("expected decryption to fail with tampered ciphertext")
	}
}

func TestSecretBoxDeterministicKeyDerivation(t *testing.T) {
	// When given a key longer than 32 bytes, NewSecretBox derives via SHA-256.
	// Two boxes with the same long key should produce decryptable output.
	longKey := []byte("this-is-a-longer-key-that-exceeds-thirty-two-bytes-in-length-for-derivation")
	plaintext := []byte("some-secret-token")

	box1 := NewSecretBox(longKey)
	ciphertext, err := box1.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	box2 := NewSecretBox(longKey)
	decrypted, err := box2.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("decrypt with derived key: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("round trip with derived key failed: got %q, want %q", string(decrypted), string(plaintext))
	}
}

func TestNewSecretBoxFromConfigDevFallback(t *testing.T) {
	// NewSecretBoxFromConfig with empty string should produce a working box (dev fallback).
	box, err := NewSecretBoxFromConfig("")
	if err != nil {
		t.Fatalf("NewSecretBoxFromConfig with empty key: %v", err)
	}
	plaintext := []byte("test-token")
	ciphertext, err := box.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt with dev fallback: %v", err)
	}
	decrypted, err := box.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("decrypt with dev fallback: %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Fatal("round trip failed with dev fallback")
	}
}

func TestNewSecretBoxFromConfigWithEnvKey(t *testing.T) {
	// With a non-empty key string, NewSecretBoxFromConfig should use it as-is (after potential SHA-256).
	box, err := NewSecretBoxFromConfig("my-custom-encryption-key-for-testing-purposes")
	if err != nil {
		t.Fatalf("NewSecretBoxFromConfig: %v", err)
	}
	plaintext := []byte("test-token")
	ciphertext, err := box.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	decrypted, err := box.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Fatal("round trip failed")
	}
}
