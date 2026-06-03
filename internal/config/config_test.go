package config

import (
	"fmt"
	"strings"
	"testing"
)

func TestEnvBool(t *testing.T) {
	t.Setenv("SHCLOP_TEST_BOOL", "yes")
	if !envBool("SHCLOP_TEST_BOOL", false) {
		t.Fatal("expected yes to be true")
	}
	t.Setenv("SHCLOP_TEST_BOOL", "0")
	if envBool("SHCLOP_TEST_BOOL", true) {
		t.Fatal("expected 0 to be false")
	}
	t.Setenv("SHCLOP_TEST_BOOL", "maybe")
	if !envBool("SHCLOP_TEST_BOOL", true) {
		t.Fatal("expected fallback for unrecognized value")
	}
	if envBool("SHCLOP_TEST_BOOL", false) {
		t.Fatal("expected fallback false for unrecognized value")
	}
}

func idpPrefix(n int) string {
	return fmt.Sprintf("SHCLOP_IDP_PROVIDER_%d", n)
}

func setFullProvider(t *testing.T, n int, name string) {
	t.Helper()
	p := idpPrefix(n)
	t.Setenv(p+"_NAME", name)
	t.Setenv(p+"_ISSUER", "https://idp.example.com")
	t.Setenv(p+"_CLIENT_ID", "client-id")
	t.Setenv(p+"_CLIENT_SECRET", "client-secret")
	t.Setenv(p+"_REDIRECT_URI", "https://app.example.com/callback")
}

func clearProvider(t *testing.T, n int) {
	t.Helper()
	p := idpPrefix(n)
	t.Setenv(p+"_NAME", "")
	t.Setenv(p+"_ISSUER", "")
	t.Setenv(p+"_CLIENT_ID", "")
	t.Setenv(p+"_CLIENT_SECRET", "")
	t.Setenv(p+"_REDIRECT_URI", "")
	t.Setenv(p+"_DISPLAY_NAME", "")
	t.Setenv(p+"_SCOPES", "")
	t.Setenv(p+"_EMAIL_CLAIM", "")
	t.Setenv(p+"_NAME_CLAIM", "")
	t.Setenv(p+"_GROUPS_CLAIM", "")
}

func TestParseIdPProviders_Empty(t *testing.T) {
	clearProvider(t, 0)
	providers, err := parseIdPProviders()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(providers) != 0 {
		t.Fatalf("expected 0 providers, got %d", len(providers))
	}
}

func TestParseIdPProviders_Single(t *testing.T) {
	clearProvider(t, 0)
	clearProvider(t, 1)
	setFullProvider(t, 0, "keycloak")

	providers, err := parseIdPProviders()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(providers))
	}
	p := providers[0]
	if p.Name != "keycloak" {
		t.Errorf("expected Name keycloak, got %q", p.Name)
	}
	if p.DisplayName != "Sign in with keycloak" {
		t.Errorf("expected default DisplayName, got %q", p.DisplayName)
	}
	if p.EmailClaim != "email" {
		t.Errorf("expected default EmailClaim email, got %q", p.EmailClaim)
	}
	if p.NameClaim != "name" {
		t.Errorf("expected default NameClaim name, got %q", p.NameClaim)
	}
	if p.GroupsClaim != "groups" {
		t.Errorf("expected default GroupsClaim groups, got %q", p.GroupsClaim)
	}
	if len(p.Scopes) != 3 || p.Scopes[0] != "openid" {
		t.Errorf("expected default scopes [openid email profile], got %v", p.Scopes)
	}
}

func TestParseIdPProviders_Multiple(t *testing.T) {
	clearProvider(t, 0)
	clearProvider(t, 1)
	clearProvider(t, 2)
	setFullProvider(t, 0, "keycloak")
	setFullProvider(t, 1, "okta")

	providers, err := parseIdPProviders()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(providers))
	}
	if providers[0].Name != "keycloak" {
		t.Errorf("expected first provider keycloak, got %q", providers[0].Name)
	}
	if providers[1].Name != "okta" {
		t.Errorf("expected second provider okta, got %q", providers[1].Name)
	}
}

func TestParseIdPProviders_GapStops(t *testing.T) {
	clearProvider(t, 0)
	clearProvider(t, 1)
	clearProvider(t, 2)
	setFullProvider(t, 0, "keycloak")
	setFullProvider(t, 2, "okta")

	providers, err := parseIdPProviders()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(providers) != 1 {
		t.Fatalf("expected 1 provider (gap stops at N=1), got %d", len(providers))
	}
	if providers[0].Name != "keycloak" {
		t.Errorf("expected keycloak, got %q", providers[0].Name)
	}
}

func TestParseIdPProviders_MissingRequired(t *testing.T) {
	clearProvider(t, 0)
	clearProvider(t, 1)
	p := idpPrefix(0)
	t.Setenv(p+"_NAME", "keycloak")

	_, err := parseIdPProviders()
	if err == nil {
		t.Fatal("expected error for missing ISSUER")
	}
	if !strings.Contains(err.Error(), "ISSUER") {
		t.Errorf("expected error mentioning ISSUER, got: %v", err)
	}
}

func TestParseIdPProviders_ScopesCommaSplit(t *testing.T) {
	clearProvider(t, 0)
	clearProvider(t, 1)
	setFullProvider(t, 0, "keycloak")
	t.Setenv(idpPrefix(0)+"_SCOPES", "openid, email, profile, groups")

	providers, err := parseIdPProviders()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(providers))
	}
	if len(providers[0].Scopes) != 4 {
		t.Errorf("expected 4 scopes, got %v", providers[0].Scopes)
	}
	for _, s := range providers[0].Scopes {
		if s != strings.TrimSpace(s) {
			t.Errorf("scope %q not trimmed", s)
		}
	}
}

func TestConfig_Validate_RequiresCookieKey(t *testing.T) {
	cfg := Config{
		Dev: false,
		IdPProviders: []IdPProviderConfig{
			{Name: "keycloak", Issuer: "https://idp.example.com", ClientID: "id", ClientSecret: "secret", RedirectURI: "https://app/cb"},
		},
		AuthCookieKey: "",
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error when AuthCookieKey empty and providers set")
	}
}

func TestConfig_Validate_DevAllowsEmptyCookieKey(t *testing.T) {
	cfg := Config{
		Dev: true,
		IdPProviders: []IdPProviderConfig{
			{Name: "keycloak", Issuer: "https://idp.example.com", ClientID: "id", ClientSecret: "secret", RedirectURI: "https://app/cb"},
		},
		AuthCookieKey: "",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected error in dev mode: %v", err)
	}
}
