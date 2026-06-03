package identity

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	josejwt "github.com/go-jose/go-jose/v4/jwt"

	jose "github.com/go-jose/go-jose/v4"
)

// testIDP holds an httptest server acting as a minimal OIDC IdP.
type testIDP struct {
	server  *httptest.Server
	privKey *rsa.PrivateKey
	keyID   string
}

func newTestIDP(t *testing.T) *testIDP {
	t.Helper()
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}

	idp := &testIDP{privKey: privKey, keyID: "test-key-1"}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	idp.server = srv

	issuer := srv.URL

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		doc := map[string]any{
			"issuer":                                issuer,
			"authorization_endpoint":                issuer + "/auth",
			"token_endpoint":                        issuer + "/token",
			"jwks_uri":                              issuer + "/jwks",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(doc)
	})

	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		jwks := jose.JSONWebKeySet{
			Keys: []jose.JSONWebKey{
				{
					Key:       &privKey.PublicKey,
					KeyID:     idp.keyID,
					Algorithm: "RS256",
					Use:       "sig",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	})

	t.Cleanup(srv.Close)
	return idp
}

// mintToken creates and signs a JWT with the given claims.
func (idp *testIDP) mintToken(t *testing.T, claims map[string]any) string {
	t.Helper()
	sig, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: jose.JSONWebKey{
			Key:       idp.privKey,
			KeyID:     idp.keyID,
			Algorithm: string(jose.RS256),
			Use:       "sig",
		}},
		(&jose.SignerOptions{}).WithType("JWT"),
	)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	raw, err := josejwt.Signed(sig).Claims(claims).Serialize()
	if err != nil {
		t.Fatalf("serialize jwt: %v", err)
	}
	return raw
}

func buildProvider(t *testing.T, idp *testIDP, overrideCfg ...func(*OIDCProviderConfig)) *OIDCProvider {
	t.Helper()
	cfg := OIDCProviderConfig{
		Name:     "test",
		Issuer:   idp.server.URL,
		ClientID: "client-id",
	}
	for _, fn := range overrideCfg {
		fn(&cfg)
	}
	p, err := NewOIDCProvider(t.Context(), cfg)
	if err != nil {
		t.Fatalf("NewOIDCProvider: %v", err)
	}
	return p
}

func TestNewOIDCProvider_Discovery(t *testing.T) {
	idp := newTestIDP(t)
	p := buildProvider(t, idp)

	if p.Verifier == nil {
		t.Fatal("Verifier is nil")
	}
	if p.OAuth2.Endpoint.AuthURL == "" {
		t.Fatal("OAuth2.Endpoint.AuthURL is empty")
	}
	if p.Name() != "test" {
		t.Fatalf("Name() = %q, want %q", p.Name(), "test")
	}
}

func TestNewOIDCProvider_DiscoveryFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	cfg := OIDCProviderConfig{
		Name:     "bad",
		Issuer:   srv.URL,
		ClientID: "client-id",
	}
	_, err := NewOIDCProvider(t.Context(), cfg)
	if err == nil {
		t.Fatal("expected error from discovery failure, got nil")
	}
}

func TestNewOIDCProvider_MalformedDiscovery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Valid JSON but missing required fields including issuer.
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"authorization_endpoint": "http://example.com/auth",
		})
	}))
	t.Cleanup(srv.Close)

	cfg := OIDCProviderConfig{
		Name:     "malformed",
		Issuer:   srv.URL,
		ClientID: "client-id",
	}
	_, err := NewOIDCProvider(t.Context(), cfg)
	if err == nil {
		t.Fatal("expected error from malformed discovery doc, got nil")
	}
}

func TestIdentityFromIDToken_HappyPath(t *testing.T) {
	idp := newTestIDP(t)
	p := buildProvider(t, idp)

	now := time.Now()
	rawToken := idp.mintToken(t, map[string]any{
		"iss":    idp.server.URL,
		"aud":    "client-id",
		"sub":    "user-1",
		"iat":    now.Unix(),
		"exp":    now.Add(5 * time.Minute).Unix(),
		"email":  "a@b.com",
		"name":   "Alice",
		"groups": []string{"devs", "admins"},
	})

	idToken, err := p.Verifier.Verify(t.Context(), rawToken)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}

	got, err := p.IdentityFromIDToken(idToken)
	if err != nil {
		t.Fatalf("IdentityFromIDToken: %v", err)
	}

	if got.Subject != "user-1" {
		t.Errorf("Subject = %q, want %q", got.Subject, "user-1")
	}
	if got.Email != "a@b.com" {
		t.Errorf("Email = %q, want %q", got.Email, "a@b.com")
	}
	if got.Name != "Alice" {
		t.Errorf("Name = %q, want %q", got.Name, "Alice")
	}
	if len(got.Groups) != 2 || got.Groups[0] != "devs" || got.Groups[1] != "admins" {
		t.Errorf("Groups = %v, want [devs admins]", got.Groups)
	}
}

func TestIdentityFromIDToken_MissingSubject(t *testing.T) {
	idp := newTestIDP(t)
	p := buildProvider(t, idp)

	now := time.Now()
	rawToken := idp.mintToken(t, map[string]any{
		"iss":   idp.server.URL,
		"aud":   "client-id",
		"sub":   "",
		"iat":   now.Unix(),
		"exp":   now.Add(5 * time.Minute).Unix(),
		"email": "a@b.com",
	})

	idToken, err := p.Verifier.Verify(t.Context(), rawToken)
	if err != nil {
		t.Fatalf("Verify unexpectedly failed: %v", err)
	}

	_, err = p.IdentityFromIDToken(idToken)
	if err == nil {
		t.Fatal("expected error for missing sub, got nil")
	}
}

func TestIdentityFromIDToken_MissingEmail(t *testing.T) {
	idp := newTestIDP(t)
	p := buildProvider(t, idp)

	now := time.Now()
	rawToken := idp.mintToken(t, map[string]any{
		"iss": idp.server.URL,
		"aud": "client-id",
		"sub": "user-1",
		"iat": now.Unix(),
		"exp": now.Add(5 * time.Minute).Unix(),
	})

	idToken, err := p.Verifier.Verify(t.Context(), rawToken)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}

	_, err = p.IdentityFromIDToken(idToken)
	if err == nil {
		t.Fatal("expected error for missing email, got nil")
	}
}

func TestIdentityFromIDToken_GroupsAsStringList(t *testing.T) {
	idp := newTestIDP(t)
	p := buildProvider(t, idp)

	now := time.Now()
	rawToken := idp.mintToken(t, map[string]any{
		"iss":    idp.server.URL,
		"aud":    "client-id",
		"sub":    "user-1",
		"iat":    now.Unix(),
		"exp":    now.Add(5 * time.Minute).Unix(),
		"email":  "a@b.com",
		"groups": []string{"a", "b"},
	})

	idToken, err := p.Verifier.Verify(t.Context(), rawToken)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}

	got, err := p.IdentityFromIDToken(idToken)
	if err != nil {
		t.Fatalf("IdentityFromIDToken: %v", err)
	}

	if len(got.Groups) != 2 || got.Groups[0] != "a" || got.Groups[1] != "b" {
		t.Errorf("Groups = %v, want [a b]", got.Groups)
	}
}

func TestIdentityFromIDToken_GroupsMissing(t *testing.T) {
	idp := newTestIDP(t)
	p := buildProvider(t, idp)

	now := time.Now()
	rawToken := idp.mintToken(t, map[string]any{
		"iss":   idp.server.URL,
		"aud":   "client-id",
		"sub":   "user-1",
		"iat":   now.Unix(),
		"exp":   now.Add(5 * time.Minute).Unix(),
		"email": "a@b.com",
	})

	idToken, err := p.Verifier.Verify(t.Context(), rawToken)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}

	got, err := p.IdentityFromIDToken(idToken)
	if err != nil {
		t.Fatalf("IdentityFromIDToken: %v", err)
	}

	if len(got.Groups) != 0 {
		t.Errorf("Groups = %v, want empty", got.Groups)
	}
}

func TestIdentityFromIDToken_ExpiredToken(t *testing.T) {
	idp := newTestIDP(t)
	p := buildProvider(t, idp)

	past := time.Now().Add(-2 * time.Minute)
	rawToken := idp.mintToken(t, map[string]any{
		"iss":   idp.server.URL,
		"aud":   "client-id",
		"sub":   "user-1",
		"iat":   past.Add(-time.Minute).Unix(),
		"exp":   past.Unix(),
		"email": "a@b.com",
	})

	_, err := p.Verifier.Verify(t.Context(), rawToken)
	if err == nil {
		t.Fatal("expected Verifier to reject expired token, got nil error")
	}
}

func TestIdentityFromIDToken_CustomClaimNames(t *testing.T) {
	idp := newTestIDP(t)
	p := buildProvider(t, idp, func(cfg *OIDCProviderConfig) {
		cfg.EmailClaim = "preferred_username"
		cfg.NameClaim = "given_name"
	})

	now := time.Now()
	rawToken := idp.mintToken(t, map[string]any{
		"iss":                idp.server.URL,
		"aud":                "client-id",
		"sub":                "user-42",
		"iat":                now.Unix(),
		"exp":                now.Add(5 * time.Minute).Unix(),
		"preferred_username": "bob@example.com",
		"given_name":         "Bob",
	})

	idToken, err := p.Verifier.Verify(t.Context(), rawToken)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}

	got, err := p.IdentityFromIDToken(idToken)
	if err != nil {
		t.Fatalf("IdentityFromIDToken: %v", err)
	}

	if got.Email != "bob@example.com" {
		t.Errorf("Email = %q, want %q", got.Email, "bob@example.com")
	}
	if got.Name != "Bob" {
		t.Errorf("Name = %q, want %q", got.Name, "Bob")
	}
}

// TestAuthenticate_ReturnsError verifies the stub Authenticate method.
func TestAuthenticate_ReturnsError(t *testing.T) {
	idp := newTestIDP(t)
	p := buildProvider(t, idp)

	_, err := p.Authenticate(t.Context(), AuthRequest{Username: "x", Password: "y"})
	if err == nil {
		t.Fatal("expected error from Authenticate, got nil")
	}
}

// Compile-time check: OIDCProvider satisfies the IdentityProvider interface.
var _ IdentityProvider = (*OIDCProvider)(nil)

// Compile-time check: *oidc.IDToken is used correctly.
var _ *oidc.IDToken = (*oidc.IDToken)(nil)
