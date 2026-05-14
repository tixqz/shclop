package auth

import "testing"

func TestMemoryAuthLogin(t *testing.T) {
	a := NewMemory()
	user, token, err := a.Login("admin", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if user.Username != "admin" {
		t.Fatalf("unexpected user %q", user.Username)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	resolved, ok := a.Resolve(token)
	if !ok {
		t.Fatal("expected token to resolve")
	}
	if resolved.ID != user.ID {
		t.Fatalf("expected %q got %q", user.ID, resolved.ID)
	}
}

func TestMemoryAuthRejectsBadPassword(t *testing.T) {
	a := NewMemory()
	_, _, err := a.Login("admin", "wrong")
	if err == nil {
		t.Fatal("expected bad password error")
	}
}

func TestMemoryAuthRejectsUnknownToken(t *testing.T) {
	a := NewMemory()
	if _, ok := a.Resolve("missing-token"); ok {
		t.Fatal("expected unknown token to not resolve")
	}
}
