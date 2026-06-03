package api

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	josejwt "github.com/go-jose/go-jose/v4/jwt"
	"github.com/mipopov/shclop/internal/config"
	"github.com/mipopov/shclop/internal/domain"
)

// testIDP holds an httptest server acting as a minimal OIDC IdP with a token endpoint.
type testIDP struct {
	server  *httptest.Server
	privKey *rsa.PrivateKey
	keyID   string
	// codes maps authorization codes to claims maps for the token endpoint.
	codes map[string]map[string]any
}

func newTestIDP(t *testing.T) *testIDP {
	t.Helper()
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}

	idp := &testIDP{
		privKey: privKey,
		keyID:   "test-key-1",
		codes:   make(map[string]map[string]any),
	}

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

	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		code := r.FormValue("code")
		claims, ok := idp.codes[code]
		if !ok {
			http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
			return
		}
		delete(idp.codes, code)

		rawIDToken := idp.mintToken(t, claims)
		resp := map[string]any{
			"access_token": "access-token-opaque",
			"token_type":   "Bearer",
			"id_token":     rawIDToken,
			"expires_in":   3600,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	t.Cleanup(srv.Close)
	return idp
}

// addCode registers a code → claims mapping for the token endpoint.
func (idp *testIDP) addCode(code string, claims map[string]any) {
	idp.codes[code] = claims
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

// newTestServerWithIDP creates a test server wired to the given fake IdP.
func newTestServerWithIDP(t *testing.T, idp *testIDP) *Server {
	t.Helper()
	cfg := config.Config{
		Store: "inmemory",
		IdPProviders: []config.IdPProviderConfig{
			{
				Name:        "test-idp",
				DisplayName: "Test IdP",
				Issuer:      idp.server.URL,
				ClientID:    "client-id",
				ClientSecret: "client-secret",
				RedirectURI: "http://app.example.com/api/auth/oidc/callback/test-idp",
			},
		},
	}
	return newTestServerWithConfig(cfg)
}

// performOIDCLogin initiates the OIDC login flow and returns the state cookie
// and the redirect URL the IdP would call back with.
func performOIDCLogin(t *testing.T, server *Server, providerName string) (stateCookieValue string, authURL *url.URL) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/login/"+providerName, nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("expected redirect from login, got %d: %s", rec.Code, rec.Body.String())
	}

	var stateCookie string
	for _, c := range rec.Result().Cookies() {
		if c.Name == oidcStateCookieName {
			stateCookie = c.Value
		}
	}
	if stateCookie == "" {
		t.Fatal("expected oidc state cookie to be set")
	}

	redirectTo := rec.Header().Get("Location")
	parsed, err := url.Parse(redirectTo)
	if err != nil {
		t.Fatalf("parse redirect URL: %v", err)
	}
	return stateCookie, parsed
}

// performOIDCCallback simulates the IdP callback with the given code and state.
func performOIDCCallback(t *testing.T, server *Server, providerName, code, state, stateCookieValue string) *httptest.ResponseRecorder {
	t.Helper()
	callbackURL := fmt.Sprintf("/api/auth/oidc/callback/%s?code=%s&state=%s", providerName, url.QueryEscape(code), url.QueryEscape(state))
	req := httptest.NewRequest(http.MethodGet, callbackURL, nil)
	req.AddCookie(&http.Cookie{Name: oidcStateCookieName, Value: stateCookieValue})
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	return rec
}

// defaultClaims returns a valid set of ID token claims for the test IdP.
func defaultClaims(issuer, nonce string) map[string]any {
	now := time.Now()
	return map[string]any{
		"iss":   issuer,
		"aud":   "client-id",
		"sub":   "user-sub-1",
		"iat":   now.Unix(),
		"exp":   now.Add(5 * time.Minute).Unix(),
		"email": "alice@example.com",
		"name":  "Alice",
		"nonce": nonce,
	}
}

func TestOIDC_ListProviders_Empty(t *testing.T) {
	server := newTestServer()
	resp := doJSON(t, server, http.MethodGet, "/api/auth/providers", nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	providers := assertJSONArray(t, resp.Body.Bytes(), "providers")
	if len(providers) != 0 {
		t.Fatalf("expected 0 providers, got %d", len(providers))
	}
}

func TestOIDC_ListProviders_WithProvider(t *testing.T) {
	idp := newTestIDP(t)
	server := newTestServerWithIDP(t, idp)

	resp := doJSON(t, server, http.MethodGet, "/api/auth/providers", nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	providers := assertJSONArray(t, resp.Body.Bytes(), "providers")
	if len(providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(providers))
	}
	if providers[0]["name"] != "test-idp" {
		t.Fatalf("expected provider name test-idp, got %v", providers[0]["name"])
	}
	if providers[0]["status"] != "ready" {
		t.Fatalf("expected status ready, got %v", providers[0]["status"])
	}
}

func TestOIDC_LoginRedirect(t *testing.T) {
	idp := newTestIDP(t)
	server := newTestServerWithIDP(t, idp)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/login/test-idp", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected redirect, got %d: %s", rec.Code, rec.Body.String())
	}

	location := rec.Header().Get("Location")
	if !strings.Contains(location, idp.server.URL) {
		t.Fatalf("expected redirect to IdP, got %q", location)
	}

	parsed, _ := url.Parse(location)
	q := parsed.Query()
	if q.Get("code_challenge_method") != "S256" {
		t.Fatalf("expected code_challenge_method=S256, got %q", q.Get("code_challenge_method"))
	}
	if q.Get("code_challenge") == "" {
		t.Fatal("expected code_challenge to be set")
	}
	if q.Get("state") == "" {
		t.Fatal("expected state to be set")
	}
	if q.Get("nonce") == "" {
		t.Fatal("expected nonce to be set")
	}

	var stateCookie string
	for _, c := range rec.Result().Cookies() {
		if c.Name == oidcStateCookieName {
			stateCookie = c.Value
		}
	}
	if stateCookie == "" {
		t.Fatal("expected oidc state cookie to be set")
	}
}

func TestOIDC_LoginUnknownProvider(t *testing.T) {
	server := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/login/nonexistent", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestOIDC_CallbackHappyPath(t *testing.T) {
	idp := newTestIDP(t)
	server := newTestServerWithIDP(t, idp)

	stateCookieValue, authURL := performOIDCLogin(t, server, "test-idp")
	state := authURL.Query().Get("state")
	nonce := authURL.Query().Get("nonce")

	claims := defaultClaims(idp.server.URL, nonce)
	code := "auth-code-1"
	idp.addCode(code, claims)

	rec := performOIDCCallback(t, server, "test-idp", code, state, stateCookieValue)
	if rec.Code != http.StatusFound {
		t.Fatalf("expected redirect after callback, got %d: %s", rec.Code, rec.Body.String())
	}

	var sessionCookie string
	for _, c := range rec.Result().Cookies() {
		if c.Name == "shclop_session" {
			sessionCookie = c.Value
		}
	}
	if sessionCookie == "" {
		t.Fatal("expected shclop_session cookie to be set")
	}

	// Verify the session works (use Bearer with the token value from cookie)
	meResp := doJSON(t, server, http.MethodGet, "/api/me", nil, sessionCookie)
	if meResp.Code != http.StatusOK {
		t.Fatalf("expected /api/me 200 after OIDC login, got %d: %s", meResp.Code, meResp.Body.String())
	}
	meUser := assertJSONObject(t, meResp.Body.Bytes(), "user")
	if meUser["username"] != "alice@example.com" {
		t.Fatalf("expected username alice@example.com, got %v", meUser["username"])
	}
}

func TestOIDC_CallbackSecondLogin_ExistingUser(t *testing.T) {
	idp := newTestIDP(t)
	server := newTestServerWithIDP(t, idp)

	for i := range 2 {
		stateCookieValue, authURL := performOIDCLogin(t, server, "test-idp")
		state := authURL.Query().Get("state")
		nonce := authURL.Query().Get("nonce")

		claims := defaultClaims(idp.server.URL, nonce)
		code := fmt.Sprintf("auth-code-%d", i)
		idp.addCode(code, claims)

		rec := performOIDCCallback(t, server, "test-idp", code, state, stateCookieValue)
		if rec.Code != http.StatusFound {
			t.Fatalf("login %d: expected redirect, got %d: %s", i, rec.Code, rec.Body.String())
		}
	}

	// Confirm only one user was created
	adminToken := loginAsAdmin(t, server)
	listed := doJSON(t, server, http.MethodGet, "/api/admin/users", nil, adminToken)
	users := assertJSONArray(t, listed.Body.Bytes(), "")
	aliceCount := 0
	for _, u := range users {
		if u["username"] == "alice@example.com" {
			aliceCount++
		}
	}
	if aliceCount != 1 {
		t.Fatalf("expected 1 alice user, got %d", aliceCount)
	}
}

func TestOIDC_CallbackStateMismatch(t *testing.T) {
	idp := newTestIDP(t)
	server := newTestServerWithIDP(t, idp)

	stateCookieValue, authURL := performOIDCLogin(t, server, "test-idp")
	nonce := authURL.Query().Get("nonce")

	claims := defaultClaims(idp.server.URL, nonce)
	code := "auth-code-state-mismatch"
	idp.addCode(code, claims)

	rec := performOIDCCallback(t, server, "test-idp", code, "wrong-state", stateCookieValue)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for state mismatch, got %d", rec.Code)
	}
}

func TestOIDC_CallbackMissingStateCookie(t *testing.T) {
	idp := newTestIDP(t)
	server := newTestServerWithIDP(t, idp)

	callbackURL := "/api/auth/oidc/callback/test-idp?code=abc&state=xyz"
	req := httptest.NewRequest(http.MethodGet, callbackURL, nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing state cookie, got %d", rec.Code)
	}
}

func TestOIDC_CallbackNonceMismatch(t *testing.T) {
	idp := newTestIDP(t)
	server := newTestServerWithIDP(t, idp)

	stateCookieValue, authURL := performOIDCLogin(t, server, "test-idp")
	state := authURL.Query().Get("state")

	// Use wrong nonce in claims
	claims := defaultClaims(idp.server.URL, "wrong-nonce")
	code := "auth-code-nonce-mismatch"
	idp.addCode(code, claims)

	rec := performOIDCCallback(t, server, "test-idp", code, state, stateCookieValue)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for nonce mismatch, got %d", rec.Code)
	}
}

func TestOIDC_CallbackIdPError(t *testing.T) {
	idp := newTestIDP(t)
	server := newTestServerWithIDP(t, idp)

	stateCookieValue, authURL := performOIDCLogin(t, server, "test-idp")
	state := authURL.Query().Get("state")

	callbackURL := fmt.Sprintf("/api/auth/oidc/callback/test-idp?error=access_denied&state=%s", url.QueryEscape(state))
	req := httptest.NewRequest(http.MethodGet, callbackURL, nil)
	req.AddCookie(&http.Cookie{Name: oidcStateCookieName, Value: stateCookieValue})
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for IdP error, got %d", rec.Code)
	}
}

func TestOIDC_Logout(t *testing.T) {
	idp := newTestIDP(t)
	server := newTestServerWithIDP(t, idp)

	stateCookieValue, authURL := performOIDCLogin(t, server, "test-idp")
	state := authURL.Query().Get("state")
	nonce := authURL.Query().Get("nonce")

	claims := defaultClaims(idp.server.URL, nonce)
	code := "auth-code-logout"
	idp.addCode(code, claims)

	callbackRec := performOIDCCallback(t, server, "test-idp", code, state, stateCookieValue)
	if callbackRec.Code != http.StatusFound {
		t.Fatalf("expected redirect, got %d: %s", callbackRec.Code, callbackRec.Body.String())
	}

	var sessionCookie string
	for _, c := range callbackRec.Result().Cookies() {
		if c.Name == "shclop_session" {
			sessionCookie = c.Value
		}
	}

	// Logout via cookie
	logoutReq := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutReq.AddCookie(&http.Cookie{Name: "shclop_session", Value: sessionCookie})
	logoutRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusOK {
		t.Fatalf("expected 200 from logout, got %d", logoutRec.Code)
	}

	// Session should no longer work (use Bearer with the revoked token value)
	meResp := doJSON(t, server, http.MethodGet, "/api/me", nil, sessionCookie)
	if meResp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 after logout, got %d", meResp.Code)
	}
}

func TestOIDC_LogoutViaBearerToken(t *testing.T) {
	server := newTestServer()
	adminToken := loginAsAdmin(t, server)

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutReq.Header.Set("Authorization", "Bearer "+adminToken)
	logoutRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusOK {
		t.Fatalf("expected 200 from logout, got %d", logoutRec.Code)
	}

	// Session should no longer work
	meResp := doJSON(t, server, http.MethodGet, "/api/me", nil, adminToken)
	if meResp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 after logout, got %d", meResp.Code)
	}
}

func TestOIDC_AuthSettings_GetDefault(t *testing.T) {
	server := newTestServer()
	adminToken := loginAsAdmin(t, server)

	resp := doJSON(t, server, http.MethodGet, "/api/admin/auth-settings", nil, adminToken)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	if body["mode"] != string(domain.AuthModeLocal) {
		t.Fatalf("expected mode local, got %v", body["mode"])
	}
}

func TestOIDC_AuthSettings_PatchMode(t *testing.T) {
	server := newTestServer()
	adminToken := loginAsAdmin(t, server)

	resp := doJSON(t, server, http.MethodPatch, "/api/admin/auth-settings", map[string]any{
		"mode": "both",
	}, adminToken)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	if body["mode"] != "both" {
		t.Fatalf("expected mode both, got %v", body["mode"])
	}
}

func TestOIDC_AuthSettings_RequiresAdmin(t *testing.T) {
	server := newTestServer()
	adminToken := loginAsAdmin(t, server)

	doJSON(t, server, http.MethodPost, "/api/admin/users", map[string]string{
		"username": "bob",
		"password": "secret",
		"role":     "user",
	}, adminToken)
	bobToken := loginAs(t, server, "bob", "secret")

	resp := doJSON(t, server, http.MethodGet, "/api/admin/auth-settings", nil, bobToken)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.Code)
	}
}

func TestOIDC_LoginModeSSO_BlocksLocalLogin(t *testing.T) {
	server := newTestServer()
	adminToken := loginAsAdmin(t, server)

	// Switch to SSO-only mode
	resp := doJSON(t, server, http.MethodPatch, "/api/admin/auth-settings", map[string]any{
		"mode": "sso",
	}, adminToken)
	if resp.Code != http.StatusOK {
		t.Fatalf("patch auth settings: %d: %s", resp.Code, resp.Body.String())
	}

	// Subsequent local login should be forbidden
	login := doJSON(t, server, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "admin",
		"password": "admin",
	}, "")
	if login.Code != http.StatusForbidden {
		t.Fatalf("expected 403 in SSO-only mode, got %d", login.Code)
	}
}

func TestOIDC_LoginModeBoth_AllowsLocalLogin(t *testing.T) {
	server := newTestServer()
	adminToken := loginAsAdmin(t, server)

	doJSON(t, server, http.MethodPatch, "/api/admin/auth-settings", map[string]any{
		"mode": "both",
	}, adminToken)

	login := doJSON(t, server, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "admin",
		"password": "admin",
	}, "")
	if login.Code != http.StatusOK {
		t.Fatalf("expected 200 in both mode, got %d: %s", login.Code, login.Body.String())
	}
}

func TestOIDC_AdminIdentities_List(t *testing.T) {
	idp := newTestIDP(t)
	server := newTestServerWithIDP(t, idp)

	// First, create a user via OIDC so they have an identity
	stateCookieValue, authURL := performOIDCLogin(t, server, "test-idp")
	state := authURL.Query().Get("state")
	nonce := authURL.Query().Get("nonce")
	claims := defaultClaims(idp.server.URL, nonce)
	idp.addCode("code-list", claims)
	performOIDCCallback(t, server, "test-idp", "code-list", state, stateCookieValue)

	adminToken := loginAsAdmin(t, server)

	// Find alice's user ID
	listed := doJSON(t, server, http.MethodGet, "/api/admin/users", nil, adminToken)
	users := assertJSONArray(t, listed.Body.Bytes(), "")
	var aliceID string
	for _, u := range users {
		if u["username"] == "alice@example.com" {
			aliceID, _ = u["id"].(string)
		}
	}
	if aliceID == "" {
		t.Fatal("alice not found in user list")
	}

	// List alice's identities
	resp := doJSON(t, server, http.MethodGet, "/api/admin/users/"+aliceID+"/identities", nil, adminToken)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	identities := assertJSONArray(t, resp.Body.Bytes(), "identities")
	if len(identities) != 1 {
		t.Fatalf("expected 1 identity, got %d", len(identities))
	}
	if identities[0]["provider_name"] != "test-idp" {
		t.Fatalf("expected provider_name test-idp, got %v", identities[0]["provider_name"])
	}
}

func TestOIDC_AdminIdentities_LinkAndUnlink(t *testing.T) {
	server := newTestServer()
	adminToken := loginAsAdmin(t, server)

	// Create a user
	created := doJSON(t, server, http.MethodPost, "/api/admin/users", map[string]string{
		"username": "carol",
		"password": "secret",
		"role":     "user",
	}, adminToken)
	userID := assertJSONField(t, created.Body.Bytes(), "id", "")

	// Link an identity
	linkResp := doJSON(t, server, http.MethodPost, "/api/admin/users/"+userID+"/identities", map[string]any{
		"provider_name": "google",
		"subject":       "sub-carol-google",
		"email":         "carol@gmail.com",
		"display_name":  "Carol",
	}, adminToken)
	if linkResp.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", linkResp.Code, linkResp.Body.String())
	}
	identities := assertJSONArray(t, linkResp.Body.Bytes(), "identities")
	if len(identities) != 1 {
		t.Fatalf("expected 1 identity after link, got %d", len(identities))
	}

	// Unlink the identity
	unlinkPath := "/api/admin/users/" + userID + "/identities/google/sub-carol-google"
	unlinkReq := httptest.NewRequest(http.MethodDelete, unlinkPath, nil)
	unlinkReq.Header.Set("Authorization", "Bearer "+adminToken)
	unlinkRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(unlinkRec, unlinkReq)
	if unlinkRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 from unlink, got %d: %s", unlinkRec.Code, unlinkRec.Body.String())
	}

	// Verify identity gone
	listResp := doJSON(t, server, http.MethodGet, "/api/admin/users/"+userID+"/identities", nil, adminToken)
	identities2 := assertJSONArray(t, listResp.Body.Bytes(), "identities")
	if len(identities2) != 0 {
		t.Fatalf("expected 0 identities after unlink, got %d", len(identities2))
	}
}

func TestOIDC_AdminIdentities_LinkConflict(t *testing.T) {
	server := newTestServer()
	adminToken := loginAsAdmin(t, server)

	// Create two users
	u1Resp := doJSON(t, server, http.MethodPost, "/api/admin/users", map[string]string{
		"username": "user1",
		"password": "secret",
		"role":     "user",
	}, adminToken)
	user1ID := assertJSONField(t, u1Resp.Body.Bytes(), "id", "")

	u2Resp := doJSON(t, server, http.MethodPost, "/api/admin/users", map[string]string{
		"username": "user2",
		"password": "secret",
		"role":     "user",
	}, adminToken)
	user2ID := assertJSONField(t, u2Resp.Body.Bytes(), "id", "")

	// Link identity to user1
	doJSON(t, server, http.MethodPost, "/api/admin/users/"+user1ID+"/identities", map[string]any{
		"provider_name": "okta",
		"subject":       "shared-subject",
		"email":         "user1@example.com",
	}, adminToken)

	// Try to link same identity to user2 — should conflict
	conflictResp := doJSON(t, server, http.MethodPost, "/api/admin/users/"+user2ID+"/identities", map[string]any{
		"provider_name": "okta",
		"subject":       "shared-subject",
		"email":         "user2@example.com",
	}, adminToken)
	if conflictResp.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", conflictResp.Code, conflictResp.Body.String())
	}
}

func TestOIDC_CallbackEmailCollision(t *testing.T) {
	idp := newTestIDP(t)
	server := newTestServerWithIDP(t, idp)

	// Pre-create a local user with the same email
	adminToken := loginAsAdmin(t, server)
	doJSON(t, server, http.MethodPost, "/api/admin/users", map[string]string{
		"username": "alice@example.com",
		"password": "secret",
		"role":     "user",
	}, adminToken)

	// Now attempt OIDC login as alice@example.com (same email)
	stateCookieValue, authURL := performOIDCLogin(t, server, "test-idp")
	state := authURL.Query().Get("state")
	nonce := authURL.Query().Get("nonce")

	claims := defaultClaims(idp.server.URL, nonce)
	code := "auth-code-collision"
	idp.addCode(code, claims)

	rec := performOIDCCallback(t, server, "test-idp", code, state, stateCookieValue)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for email collision, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestOIDC_Logout_WithoutSession(t *testing.T) {
	server := newTestServer()

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for logout without session, got %d", rec.Code)
	}
}

func TestOIDC_LoginReturnTo(t *testing.T) {
	idp := newTestIDP(t)
	server := newTestServerWithIDP(t, idp)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/login/test-idp?return_to=/agents", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("expected redirect, got %d", rec.Code)
	}

	var stateCookie string
	for _, c := range rec.Result().Cookies() {
		if c.Name == oidcStateCookieName {
			stateCookie = c.Value
		}
	}
	authURL, _ := url.Parse(rec.Header().Get("Location"))
	state := authURL.Query().Get("state")
	nonce := authURL.Query().Get("nonce")

	claims := defaultClaims(idp.server.URL, nonce)
	code := "auth-code-returnto"
	idp.addCode(code, claims)

	cbRec := performOIDCCallback(t, server, "test-idp", code, state, stateCookie)
	if cbRec.Code != http.StatusFound {
		t.Fatalf("expected redirect from callback, got %d: %s", cbRec.Code, cbRec.Body.String())
	}
	if cbRec.Header().Get("Location") != "/agents" {
		t.Fatalf("expected redirect to /agents, got %q", cbRec.Header().Get("Location"))
	}
}

func TestOIDC_AuthSettings_InvalidMode(t *testing.T) {
	server := newTestServer()
	adminToken := loginAsAdmin(t, server)

	resp := doJSON(t, server, http.MethodPatch, "/api/admin/auth-settings", map[string]any{
		"mode": "invalid-mode",
	}, adminToken)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid mode, got %d", resp.Code)
	}
}

func TestOIDC_ProviderRouteNotFound(t *testing.T) {
	server := newTestServer()

	// Unknown action
	req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/unknown/test-idp", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestOIDC_AdminIdentities_UnlinkNonexistent(t *testing.T) {
	server := newTestServer()
	adminToken := loginAsAdmin(t, server)

	created := doJSON(t, server, http.MethodPost, "/api/admin/users", map[string]string{
		"username": "dave",
		"password": "secret",
		"role":     "user",
	}, adminToken)
	userID := assertJSONField(t, created.Body.Bytes(), "id", "")

	unlinkPath := "/api/admin/users/" + userID + "/identities/google/nonexistent-sub"
	req := httptest.NewRequest(http.MethodDelete, unlinkPath, nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for nonexistent identity, got %d", rec.Code)
	}
}

func TestOIDC_AdminIdentities_RequiresMissingFields(t *testing.T) {
	server := newTestServer()
	adminToken := loginAsAdmin(t, server)

	created := doJSON(t, server, http.MethodPost, "/api/admin/users", map[string]string{
		"username": "eve",
		"password": "secret",
		"role":     "user",
	}, adminToken)
	userID := assertJSONField(t, created.Body.Bytes(), "id", "")

	// Missing provider_name
	resp := doJSON(t, server, http.MethodPost, "/api/admin/users/"+userID+"/identities", map[string]any{
		"subject": "sub-eve",
	}, adminToken)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing provider_name, got %d", resp.Code)
	}
}

func TestOIDC_UserIdentity_Serialization(t *testing.T) {
	idp := newTestIDP(t)
	server := newTestServerWithIDP(t, idp)

	stateCookieValue, authURL := performOIDCLogin(t, server, "test-idp")
	state := authURL.Query().Get("state")
	nonce := authURL.Query().Get("nonce")
	claims := defaultClaims(idp.server.URL, nonce)
	idp.addCode("code-serial", claims)
	performOIDCCallback(t, server, "test-idp", "code-serial", state, stateCookieValue)

	adminToken := loginAsAdmin(t, server)
	listed := doJSON(t, server, http.MethodGet, "/api/admin/users", nil, adminToken)
	users := assertJSONArray(t, listed.Body.Bytes(), "")
	var aliceID string
	for _, u := range users {
		if u["username"] == "alice@example.com" {
			aliceID, _ = u["id"].(string)
		}
	}

	resp := doJSON(t, server, http.MethodGet, "/api/admin/users/"+aliceID+"/identities", nil, adminToken)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	identities := assertJSONArray(t, resp.Body.Bytes(), "identities")
	if len(identities) == 0 {
		t.Fatal("expected non-empty identities array")
	}
	if identities[0]["provider_name"] != "test-idp" {
		t.Fatalf("expected provider_name test-idp, got %v", identities[0]["provider_name"])
	}
	if identities[0]["subject"] == nil || identities[0]["subject"] == "" {
		t.Fatal("expected non-empty subject")
	}
}

// Regression: handleAdminUser PATCH on user ID still works after sub-dispatch refactor.
func TestOIDC_AdminUserPatch_StillWorks(t *testing.T) {
	server := newTestServer()
	adminToken := loginAsAdmin(t, server)

	created := doJSON(t, server, http.MethodPost, "/api/admin/users", map[string]string{
		"username": "frank",
		"password": "secret",
		"role":     "user",
	}, adminToken)
	userID := assertJSONField(t, created.Body.Bytes(), "id", "")

	body, _ := json.Marshal(map[string]any{"role": "admin"})
	req := httptest.NewRequest(http.MethodPatch, "/api/admin/users/"+userID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from PATCH user, got %d: %s", rec.Code, rec.Body.String())
	}
	assertJSONField(t, rec.Body.Bytes(), "role", "admin")
}
