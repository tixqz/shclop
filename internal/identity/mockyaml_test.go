package identity

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestMockYAMLProviderAuthenticatesAndMapsPrincipal(t *testing.T) {
	path := writeMockIdentityConfig(t)
	provider, err := NewMockYAMLProvider(path)
	if err != nil {
		t.Fatal(err)
	}

	identity, err := provider.Authenticate(context.Background(), AuthRequest{Username: "alice@acme.test", Password: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	if identity.Subject != "oidc|alice" || identity.Email != "alice@acme.test" || identity.Name != "Alice Admin" {
		t.Fatalf("unexpected identity: %#v", identity)
	}
	if !contains(identity.Groups, "platform-admins") {
		t.Fatalf("expected platform-admins group: %#v", identity.Groups)
	}

	principal, err := StaticOrganizationMapper{}.Map(context.Background(), identity)
	if err != nil {
		t.Fatal(err)
	}
	if principal.UserID != "oidc|alice" || principal.TenantID != "acme" || principal.DisplayName != "Alice Admin" {
		t.Fatalf("unexpected mapped principal: %#v", principal)
	}
	if !contains(principal.TeamIDs, "platform") || !contains(principal.Roles, "admin") {
		t.Fatalf("expected team and role from yaml claims: %#v", principal)
	}
}

func TestMockYAMLProviderRejectsInvalidPassword(t *testing.T) {
	provider, err := NewMockYAMLProvider(writeMockIdentityConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Authenticate(context.Background(), AuthRequest{Username: "alice@acme.test", Password: "wrong"}); err == nil {
		t.Fatal("expected invalid credentials error")
	}
}

func TestMockYAMLProviderRejectsUsersWithoutPassword(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity.yaml")
	if err := os.WriteFile(path, []byte(`users:
  broken@example.test:
    subject: oidc|broken
    tenant: acme
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewMockYAMLProvider(path); err == nil {
		t.Fatal("expected missing password to fail config loading")
	}
}

func TestStaticOrganizationMapperRejectsMissingTenant(t *testing.T) {
	_, err := StaticOrganizationMapper{}.Map(context.Background(), Identity{Subject: "sub", Email: "user@example.test"})
	if err == nil {
		t.Fatal("expected missing tenant error")
	}
}

func TestStaticOrganizationMapperRejectsMissingSubject(t *testing.T) {
	_, err := StaticOrganizationMapper{}.Map(context.Background(), Identity{Email: "user@example.test", Claims: map[string]string{"tenant": "acme"}})
	if err == nil {
		t.Fatal("expected missing subject error")
	}
}

func writeMockIdentityConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "identity.yaml")
	content := []byte(`users:
  alice@acme.test:
    password: alice
    subject: oidc|alice
    name: Alice Admin
    tenant: acme
    teams: [platform]
    roles: [admin]
    groups: [platform-admins]
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
