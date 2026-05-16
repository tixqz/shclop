package auth

import (
	"context"
	"testing"

	"github.com/mipopov/shclop/internal/identity"
)

func TestMemoryAuthLogin(t *testing.T) {
	a := NewMemory()
	user, token, err := a.Login(context.Background(), "admin", "admin")
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
	_, _, err := a.Login(context.Background(), "admin", "wrong")
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

func TestMemoryAuthUsesIdentityProvider(t *testing.T) {
	provider := staticIdentityProvider{identity: identity.Identity{
		Subject: "oidc|alice",
		Email:   "alice@acme.test",
		Name:    "Alice Admin",
		Claims:  map[string]string{"tenant": "acme", "teams": "platform", "roles": "admin"},
	}}
	a := NewWithIdentity(provider, identity.StaticOrganizationMapper{})
	user, token, err := a.Login(context.Background(), "alice@acme.test", "alice")
	if err != nil {
		t.Fatal(err)
	}
	if token == "" {
		t.Fatal("expected token")
	}
	if user.ID != "oidc|alice" || user.Username != "alice@acme.test" || user.TenantID != "acme" {
		t.Fatalf("unexpected user: %#v", user)
	}
	if !contains(user.Roles, "admin") || !contains(user.TeamIDs, "platform") {
		t.Fatalf("expected mapped roles and teams: %#v", user)
	}
	resolved, ok := a.Resolve(token)
	if !ok || resolved.ID != user.ID || resolved.TenantID != "acme" {
		t.Fatalf("token did not resolve mapped user: %#v ok=%v", resolved, ok)
	}
}

func TestMemoryAuthPropagatesCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a := NewWithIdentity(staticIdentityProvider{}, identity.StaticOrganizationMapper{})
	if _, _, err := a.Login(ctx, "alice@acme.test", "alice"); err == nil {
		t.Fatal("expected canceled context error")
	}
}

type staticIdentityProvider struct{ identity identity.Identity }

func (p staticIdentityProvider) Name() string { return "static" }

func (p staticIdentityProvider) Authenticate(ctx context.Context, request identity.AuthRequest) (identity.Identity, error) {
	if err := ctx.Err(); err != nil {
		return identity.Identity{}, err
	}
	return p.identity, nil
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
