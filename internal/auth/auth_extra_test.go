package auth

import (
	"testing"

	"github.com/mipopov/shclop/internal/domain"
)

func TestService_IssueTokenIsResolvable(t *testing.T) {
	store := newTestStore()
	svc := NewService(store, store)

	user := domain.User{ID: "user-42", Username: "carol", Role: "user"}
	token, err := svc.IssueToken(user)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	got, ok := svc.Resolve(token)
	if !ok {
		t.Fatal("Resolve returned not found for issued token")
	}
	if got.ID != user.ID {
		t.Errorf("Resolve ID = %q, want %q", got.ID, user.ID)
	}
}

func TestService_RevokeRemovesToken(t *testing.T) {
	store := newTestStore()
	svc := NewService(store, store)

	user := domain.User{ID: "user-99", Username: "dave", Role: "user"}
	token, err := svc.IssueToken(user)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	if !svc.Revoke(token) {
		t.Fatal("Revoke returned false for existing token")
	}

	_, ok := svc.Resolve(token)
	if ok {
		t.Error("Resolve found token after Revoke; expected not found")
	}
}

func TestService_RevokeUnknownToken(t *testing.T) {
	store := newTestStore()
	svc := NewService(store, store)

	if svc.Revoke("nope") {
		t.Error("Revoke returned true for unknown token; expected false")
	}
}
