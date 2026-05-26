package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mipopov/shclop/internal/config"
	"github.com/mipopov/shclop/internal/integrations"
)

func TestHealthz(t *testing.T) {
	server := newTestServer()

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	assertJSONField(t, response.Body.Bytes(), "status", "ok")
}

func TestReadyz(t *testing.T) {
	server := newTestServer()

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	assertJSONField(t, response.Body.Bytes(), "status", "ready")
}

func TestMetricsEnabled(t *testing.T) {
	server := newTestServerWithConfig(config.Config{Store: "inmemory", Metrics: true})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	if ct := response.Header().Get("Content-Type"); !strings.Contains(ct, "text/plain") {
		t.Fatalf("expected text/plain content type, got %q", ct)
	}
}

func TestMetricsDisabled(t *testing.T) {
	server := newTestServerWithConfig(config.Config{Store: "inmemory", Metrics: false})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, response.Code)
	}
}

func TestLoginAndGetMe(t *testing.T) {
	server := newTestServer()

	// Login with default admin credentials
	login := doJSON(t, server, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "admin",
		"password": "admin",
	}, "")
	if login.Code != http.StatusOK {
		t.Fatalf("expected login status %d, got %d", http.StatusOK, login.Code)
	}
	if cookie := login.Result().Cookies(); len(cookie) == 0 || cookie[0].Name != "shclop_session" {
		t.Fatalf("expected shclop_session cookie, got %#v", cookie)
	}
	token := assertJSONField(t, login.Body.Bytes(), "token", "")
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	user := assertJSONObject(t, login.Body.Bytes(), "user")
	if user["username"] != "admin" {
		t.Fatalf("expected username admin, got %v", user["username"])
	}
	if user["role"] != "admin" {
		t.Fatalf("expected role admin, got %v", user["role"])
	}

	// Get /api/me
	me := doJSON(t, server, http.MethodGet, "/api/me", nil, token)
	if me.Code != http.StatusOK {
		t.Fatalf("expected me status %d, got %d", http.StatusOK, me.Code)
	}
	meUser := assertJSONObject(t, me.Body.Bytes(), "user")
	if meUser["username"] != "admin" {
		t.Fatalf("expected username admin, got %v", meUser["username"])
	}
}

func TestBrowserWebSocketRejectsQueryToken(t *testing.T) {
	server := newTestServer()
	token := loginAsAdmin(t, server)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/ws?agent_id=agent-1&token="+token, nil)
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected query token to be rejected with status %d, got %d", http.StatusUnauthorized, response.Code)
	}
}

func TestLoginInvalidCredentials(t *testing.T) {
	server := newTestServer()

	response := doJSON(t, server, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "admin",
		"password": "wrong",
	}, "")
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, response.Code)
	}
}

func TestAdminCreateUser(t *testing.T) {
	server := newTestServer()
	token := loginAsAdmin(t, server)

	created := doJSON(t, server, http.MethodPost, "/api/admin/users", map[string]string{
		"username": "alice",
		"password": "secret",
		"role":     "user",
	}, token)
	if created.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, created.Code, created.Body.String())
	}
	assertJSONField(t, created.Body.Bytes(), "username", "alice")
	assertJSONField(t, created.Body.Bytes(), "role", "user")

	// List users should include both admin and alice
	listed := doJSON(t, server, http.MethodGet, "/api/admin/users", nil, token)
	if listed.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, listed.Code)
	}
	users := assertJSONArray(t, listed.Body.Bytes(), "")
	if len(users) < 2 {
		t.Fatalf("expected at least 2 users, got %d", len(users))
	}

	// Non-admin should be forbidden
	bobToken := loginAs(t, server, "alice", "secret")
	forbidden := doJSON(t, server, http.MethodPost, "/api/admin/users", map[string]string{
		"username": "charlie",
		"password": "secret",
		"role":     "user",
	}, bobToken)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, forbidden.Code)
	}
}

func TestAdminDisableUser(t *testing.T) {
	server := newTestServer()
	adminToken := loginAsAdmin(t, server)

	// Create a user
	created := doJSON(t, server, http.MethodPost, "/api/admin/users", map[string]string{
		"username": "alice",
		"password": "secret",
		"role":     "user",
	}, adminToken)
	userID := assertJSONField(t, created.Body.Bytes(), "id", "")

	// Disable the user
	disabled := true
	body, _ := json.Marshal(map[string]any{"disabled": disabled})
	req := httptest.NewRequest(http.MethodPatch, "/api/admin/users/"+userID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, response.Code, response.Body.String())
	}

	// Verify alice cannot login now
	aliceLogin := doJSON(t, server, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "alice",
		"password": "secret",
	}, "")
	if aliceLogin.Code != http.StatusUnauthorized {
		t.Fatalf("expected disabled user login to fail, got %d", aliceLogin.Code)
	}
}

func TestAgentsCreateAndList(t *testing.T) {
	server := newTestServer()
	token := loginAsAdmin(t, server)

	// Create an agent with a model (will be rejected since no models exist yet)
	created := doJSON(t, server, http.MethodPost, "/api/agents", map[string]any{
		"name":    "TestAgent",
		"runtime": "nanoclaw",
		"model":   "gpt-4",
	}, token)
	// Should fail because model doesn't exist
	if created.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request for unknown model, got %d: %s", created.Code, created.Body.String())
	}

	// Create model first
	doJSON(t, server, http.MethodPost, "/api/admin/models", map[string]any{
		"display_name":   "GPT-4",
		"provider_model": "gpt-4",
		"enabled":        true,
	}, token)

	// Now create agent should succeed
	created = doJSON(t, server, http.MethodPost, "/api/agents", map[string]any{
		"name":    "TestAgent",
		"runtime": "nanoclaw",
		"model":   "gpt-4",
	}, token)
	if created.Code != http.StatusCreated {
		t.Fatalf("expected create status %d, got %d: %s", http.StatusCreated, created.Code, created.Body.String())
	}
	agentID := assertJSONField(t, created.Body.Bytes(), "id", "")
	assertJSONField(t, created.Body.Bytes(), "name", "TestAgent")
	assertJSONField(t, created.Body.Bytes(), "runtime", "nanoclaw")
	assertJSONField(t, created.Body.Bytes(), "model", "gpt-4")
	assertJSONField(t, created.Body.Bytes(), "state", "idle")

	// List agents
	listed := doJSON(t, server, http.MethodGet, "/api/agents", nil, token)
	if listed.Code != http.StatusOK {
		t.Fatalf("expected list status %d, got %d", http.StatusOK, listed.Code)
	}
	agents := assertJSONArray(t, listed.Body.Bytes(), "")
	if len(agents) != 1 || agents[0]["id"] != agentID {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}

	// Get single agent
	fetched := doJSON(t, server, http.MethodGet, "/api/agents/"+agentID, nil, token)
	if fetched.Code != http.StatusOK {
		t.Fatalf("expected get status %d, got %d", http.StatusOK, fetched.Code)
	}
	assertJSONField(t, fetched.Body.Bytes(), "name", "TestAgent")
}

func TestAgentRequiresValidRuntime(t *testing.T) {
	server := newTestServer()
	token := loginAsAdmin(t, server)

	created := doJSON(t, server, http.MethodPost, "/api/agents", map[string]any{
		"name":    "BadRuntime",
		"runtime": "invalid",
	}, token)
	if created.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request for invalid runtime, got %d", created.Code)
	}
}

func TestAdminModels(t *testing.T) {
	server := newTestServer()
	token := loginAsAdmin(t, server)

	// Create model
	created := doJSON(t, server, http.MethodPost, "/api/admin/models", map[string]any{
		"display_name":   "GPT-4o",
		"provider_model": "openai/gpt-4o",
		"enabled":        true,
	}, token)
	if created.Code != http.StatusCreated {
		t.Fatalf("expected create status %d, got %d", http.StatusCreated, created.Code)
	}
	modelID := assertJSONField(t, created.Body.Bytes(), "id", "")

	// List models
	listed := doJSON(t, server, http.MethodGet, "/api/admin/models", nil, token)
	if listed.Code != http.StatusOK {
		t.Fatalf("expected list status %d, got %d", http.StatusOK, listed.Code)
	}
	models := assertJSONArray(t, listed.Body.Bytes(), "")
	if len(models) != 1 || models[0]["id"] != modelID {
		t.Fatalf("expected 1 model, got %d", len(models))
	}

	// Update model - disable it
	enabled := false
	body, _ := json.Marshal(map[string]any{"enabled": enabled})
	req := httptest.NewRequest(http.MethodPatch, "/api/admin/models/"+modelID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("expected update status %d, got %d", http.StatusOK, response.Code)
	}

	// Non-admin should be forbidden
	bobLogin := doJSON(t, server, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "admin",
		"password": "admin",
	}, "")
	bobToken := assertJSONField(t, bobLogin.Body.Bytes(), "token", "")
	_ = bobToken
	// Create a non-admin user
	createdUser := doJSON(t, server, http.MethodPost, "/api/admin/users", map[string]string{
		"username": "bob",
		"password": "bobpass",
		"role":     "user",
	}, token)
	bobToken2 := loginAs(t, server, "bob", "bobpass")
	forbidden := doJSON(t, server, http.MethodGet, "/api/admin/models", nil, bobToken2)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for non-admin, got %d", forbidden.Code)
	}
	_ = createdUser
}

func TestAdminLLMGateway(t *testing.T) {
	server := newTestServer()
	token := loginAsAdmin(t, server)

	// Get initial settings
	settings := doJSON(t, server, http.MethodGet, "/api/admin/llm-gateway", nil, token)
	if settings.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, settings.Code)
	}

	// Update settings
	updated := doJSON(t, server, http.MethodPatch, "/api/admin/llm-gateway", map[string]any{
		"enabled":     true,
		"base_url":    "https://llm.example.com",
		"secret_name": "llm-secret",
		"secret_key":  "api-key",
	}, token)
	if updated.Code != http.StatusOK {
		t.Fatalf("expected update status %d, got %d: %s", http.StatusOK, updated.Code, updated.Body.String())
	}

	// Verify
	settings = doJSON(t, server, http.MethodGet, "/api/admin/llm-gateway", nil, token)
	assertJSONField(t, settings.Body.Bytes(), "base_url", "https://llm.example.com")
}

func TestAdminOverview(t *testing.T) {
	server := newTestServer()
	token := loginAsAdmin(t, server)

	response := doJSON(t, server, http.MethodGet, "/api/admin/overview", nil, token)
	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	// Non-admin should be forbidden
	bobToken := loginAs(t, server, "admin", "admin")
	forbidden := doJSON(t, server, http.MethodGet, "/api/admin/overview", nil, bobToken)
	_ = forbidden
	// Actually bob is also admin since we only have admin. Let's create a regular user.
	createdUser := doJSON(t, server, http.MethodPost, "/api/admin/users", map[string]string{
		"username": "bob",
		"password": "bobpass",
		"role":     "user",
	}, token)
	bobToken = loginAs(t, server, "bob", "bobpass")
	forbidden = doJSON(t, server, http.MethodGet, "/api/admin/overview", nil, bobToken)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for non-admin, got %d", forbidden.Code)
	}
	_ = createdUser
}

func TestActivity(t *testing.T) {
	server := newTestServer()
	token := loginAsAdmin(t, server)

	// Create an agent to generate activity
	doJSON(t, server, http.MethodPost, "/api/admin/models", map[string]any{
		"display_name":   "GPT-4",
		"provider_model": "gpt-4",
		"enabled":        true,
	}, token)
	doJSON(t, server, http.MethodPost, "/api/agents", map[string]any{
		"name":    "TestAgent",
		"runtime": "nanoclaw",
		"model":   "gpt-4",
	}, token)

	response := doJSON(t, server, http.MethodGet, "/api/activity", nil, token)
	if response.Code != http.StatusOK {
		t.Fatalf("expected activity status %d, got %d", http.StatusOK, response.Code)
	}
	activity := assertJSONArray(t, response.Body.Bytes(), "activity")
	if len(activity) == 0 {
		t.Fatal("expected at least one activity entry")
	}
}

func TestModels_EnabledModels(t *testing.T) {
	server := newTestServer()
	adminToken := loginAsAdmin(t, server)

	// Create a regular (non-admin) user
	created := doJSON(t, server, http.MethodPost, "/api/admin/users", map[string]string{
		"username": "alice",
		"password": "secret",
		"role":     "user",
	}, adminToken)
	if created.Code != http.StatusCreated {
		t.Fatalf("create user: %d: %s", created.Code, created.Body.String())
	}
	userToken := loginAs(t, server, "alice", "secret")

	// Create enabled model
	doJSON(t, server, http.MethodPost, "/api/admin/models", map[string]any{
		"display_name":   "GPT-4o",
		"provider_model": "openai/gpt-4o",
		"enabled":        true,
	}, adminToken)

	// Create disabled model
	doJSON(t, server, http.MethodPost, "/api/admin/models", map[string]any{
		"display_name":   "Disabled Model",
		"provider_model": "test/disabled",
		"enabled":        false,
	}, adminToken)

	// Regular user GET /api/models — should see only enabled models
	listed := doJSON(t, server, http.MethodGet, "/api/models", nil, userToken)
	if listed.Code != http.StatusOK {
		t.Fatalf("expected status %d for GET /api/models, got %d: %s", http.StatusOK, listed.Code, listed.Body.String())
	}
	models := assertJSONArray(t, listed.Body.Bytes(), "")
	if len(models) != 1 {
		t.Fatalf("expected 1 enabled model, got %d: %+v", len(models), models)
	}
	if models[0]["provider_model"] != "openai/gpt-4o" {
		t.Fatalf("expected provider_model openai/gpt-4o, got %v", models[0]["provider_model"])
	}
	if models[0]["enabled"] != true {
		t.Fatalf("expected enabled=true, got %v", models[0]["enabled"])
	}

	// Unauthenticated GET /api/models — should be 401
	noAuth := doJSON(t, server, http.MethodGet, "/api/models", nil, "")
	if noAuth.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d for unauthenticated GET /api/models, got %d", http.StatusUnauthorized, noAuth.Code)
	}

	// POST /api/models — should be 405
	postResp := doJSON(t, server, http.MethodPost, "/api/models", map[string]any{
		"display_name": "should-not-work",
	}, userToken)
	if postResp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d for POST /api/models, got %d", http.StatusMethodNotAllowed, postResp.Code)
	}

	// PATCH /api/models — should be 405
	patchResp := doJSON(t, server, http.MethodPatch, "/api/models", map[string]any{}, userToken)
	if patchResp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d for PATCH /api/models, got %d", http.StatusMethodNotAllowed, patchResp.Code)
	}

	// Regular user GET /api/admin/models — should be 403
	adminForbidden := doJSON(t, server, http.MethodGet, "/api/admin/models", nil, userToken)
	if adminForbidden.Code != http.StatusForbidden {
		t.Fatalf("expected status %d for regular user GET /api/admin/models, got %d", http.StatusForbidden, adminForbidden.Code)
	}
}

func TestModels_GatewayDiscovery_FiltersByGateway(t *testing.T) {
	mockLLM := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// LiteLLM returns models at /v1/models
		if r.URL.Path != "/v1/models" {
			t.Errorf("expected /v1/models, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Errorf("expected Bearer test-api-key, got %s", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"object": "list",
			"data": []map[string]any{
				{"id": "gpt-4", "object": "model", "created": 123, "owned_by": "openai"},
				{"id": "claude-3", "object": "model", "created": 124, "owned_by": "anthropic"},
			},
		})
	}))
	defer mockLLM.Close()

	cfg := config.Config{
		Store:             "inmemory",
		LLMGatewayBaseURL: mockLLM.URL,
		LLMGatewayAPIKey:  "test-api-key",
	}
	server := newTestServerWithConfig(cfg)
	token := loginAsAdmin(t, server)

	// Create enabled model that matches a LiteLLM model
	doJSON(t, server, http.MethodPost, "/api/admin/models", map[string]any{
		"display_name":   "GPT-4",
		"provider_model": "gpt-4",
		"enabled":        true,
	}, token)

	// Create enabled model that does NOT appear in LiteLLM
	doJSON(t, server, http.MethodPost, "/api/admin/models", map[string]any{
		"display_name":   "Not in Gateway",
		"provider_model": "test/not-in-gateway",
		"enabled":        true,
	}, token)

	// Create disabled model that matches LiteLLM (should NOT appear)
	doJSON(t, server, http.MethodPost, "/api/admin/models", map[string]any{
		"display_name":   "Disabled GPT-4",
		"provider_model": "gpt-4",
		"enabled":        false,
	}, token)

	// Create a regular user
	doJSON(t, server, http.MethodPost, "/api/admin/users", map[string]string{
		"username": "alice",
		"password": "secret",
		"role":     "user",
	}, token)
	userToken := loginAs(t, server, "alice", "secret")

	listed := doJSON(t, server, http.MethodGet, "/api/models", nil, userToken)
	if listed.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", listed.Code, listed.Body.String())
	}
	models := assertJSONArray(t, listed.Body.Bytes(), "")
	if len(models) != 1 {
		t.Fatalf("expected 1 model (only gpt-4 matching gateway), got %d: %+v", len(models), models)
	}
	if models[0]["provider_model"] != "gpt-4" {
		t.Fatalf("expected provider_model gpt-4, got %v", models[0]["provider_model"])
	}
	if models[0]["enabled"] != true {
		t.Fatalf("expected enabled=true, got %v", models[0]["enabled"])
	}
}

func TestModels_GatewayDiscovery_FailsWith502(t *testing.T) {
	mockLLM := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer mockLLM.Close()

	cfg := config.Config{
		Store:             "inmemory",
		LLMGatewayBaseURL: mockLLM.URL,
		LLMGatewayAPIKey:  "test-api-key",
	}
	server := newTestServerWithConfig(cfg)
	token := loginAsAdmin(t, server)

	// Create enabled model
	doJSON(t, server, http.MethodPost, "/api/admin/models", map[string]any{
		"display_name":   "GPT-4",
		"provider_model": "gpt-4",
		"enabled":        true,
	}, token)

	// Create a regular user
	doJSON(t, server, http.MethodPost, "/api/admin/users", map[string]string{
		"username": "alice",
		"password": "secret",
		"role":     "user",
	}, token)
	userToken := loginAs(t, server, "alice", "secret")

	listed := doJSON(t, server, http.MethodGet, "/api/models", nil, userToken)
	if listed.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 Bad Gateway for LiteLLM failure, got %d: %s", listed.Code, listed.Body.String())
	}
}

func TestModels_GatewayDiscovery_NoGatewayConfig(t *testing.T) {
	// When no gateway is configured, /api/models returns all enabled store models.
	server := newTestServer()
	adminToken := loginAsAdmin(t, server)

	// Create a regular user
	doJSON(t, server, http.MethodPost, "/api/admin/users", map[string]string{
		"username": "alice",
		"password": "secret",
		"role":     "user",
	}, adminToken)
	userToken := loginAs(t, server, "alice", "secret")

	// Create enabled model
	doJSON(t, server, http.MethodPost, "/api/admin/models", map[string]any{
		"display_name":   "GPT-4o",
		"provider_model": "openai/gpt-4o",
		"enabled":        true,
	}, adminToken)

	// Create disabled model
	doJSON(t, server, http.MethodPost, "/api/admin/models", map[string]any{
		"display_name":   "Disabled Model",
		"provider_model": "test/disabled",
		"enabled":        false,
	}, adminToken)

	// Regular user GET /api/models — should see only enabled models (unchanged behavior)
	listed := doJSON(t, server, http.MethodGet, "/api/models", nil, userToken)
	if listed.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", listed.Code, listed.Body.String())
	}
	models := assertJSONArray(t, listed.Body.Bytes(), "")
	if len(models) != 1 {
		t.Fatalf("expected 1 enabled model, got %d: %+v", len(models), models)
	}
	if models[0]["provider_model"] != "openai/gpt-4o" {
		t.Fatalf("expected provider_model openai/gpt-4o, got %v", models[0]["provider_model"])
	}
}

func TestAgentsRequireAuth(t *testing.T) {
	server := newTestServer()

	for _, path := range []string{"/api/agents", "/api/me", "/api/admin/users", "/api/admin/models", "/api/admin/llm-gateway", "/api/admin/overview", "/api/activity", "/api/models"} {
		t.Run(path, func(t *testing.T) {
			response := doJSON(t, server, http.MethodGet, path, nil, "")
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("expected status %d for %s, got %d", http.StatusUnauthorized, path, response.Code)
			}
		})
	}
}

func TestWrongMethods(t *testing.T) {
	server := newTestServer()

	for _, test := range []struct {
		name   string
		method string
		path   string
		allow  string
	}{
		{name: "healthz post", method: http.MethodPost, path: "/healthz", allow: http.MethodGet},
		{name: "login get", method: http.MethodGet, path: "/api/auth/login", allow: http.MethodPost},
		{name: "me post", method: http.MethodPost, path: "/api/me", allow: http.MethodGet},
		{name: "agents put", method: http.MethodPut, path: "/api/agents", allow: "GET, POST"},
		{name: "ws post", method: http.MethodPost, path: "/ws", allow: http.MethodGet},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, nil)
			response := httptest.NewRecorder()
			server.Handler().ServeHTTP(response, request)
			if response.Code != http.StatusMethodNotAllowed {
				t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, response.Code)
			}
			if got := response.Header().Get("Allow"); got != test.allow {
				t.Fatalf("expected Allow header %q, got %q", test.allow, got)
			}
		})
	}
}

func TestServesFrontend(t *testing.T) {
	staticDir := t.TempDir()
	index := []byte(`<!doctype html><title>shclop ui</title><div id="root"></div>`)
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), index, 0o644); err != nil {
		t.Fatal(err)
	}

	server := newTestServerWithConfig(config.Config{
		Addr:      ":8080",
		Store:     "inmemory",
		LogLevel:  "info",
		Metrics:   true,
		StaticDir: staticDir,
	})

	for _, path := range []string{"/", "/agents/agent-1"} {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, path, nil)
			response := httptest.NewRecorder()
			server.Handler().ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
			}
			if got := response.Body.String(); got != string(index) {
				t.Fatalf("expected index body %q, got %q", string(index), got)
			}
		})
	}
}

func TestRequireBootstrapPassword_ProductionConfigRequiresPassword(t *testing.T) {
	// Simulate production: non-inmemory store + non-mock provider + no dev mode.
	server := &Server{
		cfg: config.Config{
			Store:           "postgres",
			SandboxProvider: "kubernetes",
			Dev:             false,
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	err := server.requireBootstrapPassword()
	if err == nil {
		t.Fatal("expected error for production config without SHCLOP_BOOTSTRAP_ADMIN_PASSWORD")
	}
	if !strings.Contains(err.Error(), "SHCLOP_BOOTSTRAP_ADMIN_PASSWORD") {
		t.Fatalf("expected error mentioning env var, got: %v", err)
	}
}

func TestRequireBootstrapPassword_DevConfigAllowsDefaultPassword(t *testing.T) {
	server := &Server{
		cfg: config.Config{
			Store:           "postgres",
			SandboxProvider: "kubernetes",
			Dev:             true, // dev mode overrides production check
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	if err := server.requireBootstrapPassword(); err != nil {
		t.Fatalf("dev config should allow default password: %v", err)
	}
}

func TestRequireBootstrapPassword_InmemoryStoreAllowsDefaultPassword(t *testing.T) {
	server := &Server{
		cfg: config.Config{
			Store:           "inmemory",
			SandboxProvider: "kubernetes",
			Dev:             false,
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	if err := server.requireBootstrapPassword(); err != nil {
		t.Fatalf("inmemory store should allow default password: %v", err)
	}
}

func TestRequireBootstrapPassword_MockProviderAllowsDefaultPassword(t *testing.T) {
	server := &Server{
		cfg: config.Config{
			Store:           "postgres",
			SandboxProvider: "mock",
			Dev:             false,
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	if err := server.requireBootstrapPassword(); err != nil {
		t.Fatalf("mock provider should allow default password: %v", err)
	}
}

func TestRequireBootstrapPassword_EnvVarSetAllowsProduction(t *testing.T) {
	t.Setenv("SHCLOP_BOOTSTRAP_ADMIN_PASSWORD", "s3cret!")
	server := &Server{
		cfg: config.Config{
			Store:           "postgres",
			SandboxProvider: "kubernetes",
			Dev:             false,
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	if err := server.requireBootstrapPassword(); err != nil {
		t.Fatalf("production config with env var set should pass: %v", err)
	}
}

func TestStartAgentWithModelRequiresFullyConfiguredGateway(t *testing.T) {
	server := newTestServer()
	token := loginAsAdmin(t, server)

	// Create an enabled model
	doJSON(t, server, http.MethodPost, "/api/admin/models", map[string]any{
		"display_name":   "GPT-4",
		"provider_model": "gpt-4",
		"enabled":        true,
	}, token)

	// Create an agent with that model
	created := doJSON(t, server, http.MethodPost, "/api/agents", map[string]any{
		"name":    "ModelAgent",
		"runtime": "nanoclaw",
		"model":   "gpt-4",
	}, token)
	if created.Code != http.StatusCreated {
		t.Fatalf("create agent: %d: %s", created.Code, created.Body.String())
	}
	agentID := assertJSONField(t, created.Body.Bytes(), "id", "")

	// Start agent — should fail because LLM gateway is not configured
	started := doJSON(t, server, http.MethodPost, "/api/agents/"+agentID+"/start", nil, token)
	if started.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing gateway config, got %d: %s", started.Code, started.Body.String())
	}
	if !strings.Contains(started.Body.String(), "LLM gateway") {
		t.Fatalf("expected error about LLM gateway, got: %s", started.Body.String())
	}
}

func TestStartAgentWithModelAcceptsInternalGatewayWithoutSecret(t *testing.T) {
	server := newTestServer()
	token := loginAsAdmin(t, server)

	doJSON(t, server, http.MethodPost, "/api/admin/models", map[string]any{
		"display_name":   "DeepSeek V4 Flash",
		"provider_model": "deepseek-v4-flash",
		"enabled":        true,
	}, token)

	updated := doJSON(t, server, http.MethodPatch, "/api/admin/llm-gateway", map[string]any{
		"enabled":  true,
		"base_url": "http://shclop-litellm:4000/v1",
	}, token)
	if updated.Code != http.StatusOK {
		t.Fatalf("configure internal gateway: %d: %s", updated.Code, updated.Body.String())
	}

	created := doJSON(t, server, http.MethodPost, "/api/agents", map[string]any{
		"name":    "ModelAgent",
		"runtime": "nanoclaw",
		"model":   "deepseek-v4-flash",
	}, token)
	if created.Code != http.StatusCreated {
		t.Fatalf("create agent: %d: %s", created.Code, created.Body.String())
	}
	agentID := assertJSONField(t, created.Body.Bytes(), "id", "")

	started := doJSON(t, server, http.MethodPost, "/api/agents/"+agentID+"/start", nil, token)
	if started.Code != http.StatusAccepted {
		t.Fatalf("start agent: %d: %s", started.Code, started.Body.String())
	}
}

func TestStopAgentRevokesRuntimeToken(t *testing.T) {
	server := newTestServer()
	token := loginAsAdmin(t, server)

	// Create agent
	created := doJSON(t, server, http.MethodPost, "/api/agents", map[string]any{
		"name":    "StopTest",
		"runtime": "nanoclaw",
		"model":   "",
	}, token)
	if created.Code != http.StatusCreated {
		t.Fatalf("create agent: %d: %s", created.Code, created.Body.String())
	}
	agentID := assertJSONField(t, created.Body.Bytes(), "id", "")

	// Start agent — mock provider succeeds, token is stored
	started := doJSON(t, server, http.MethodPost, "/api/agents/"+agentID+"/start", nil, token)
	if started.Code != http.StatusAccepted {
		t.Fatalf("start agent: %d: %s", started.Code, started.Body.String())
	}
	server.tokenMu.Lock()
	runtimeToken := server.tokens[agentID]
	server.tokenMu.Unlock()
	if runtimeToken == "" {
		t.Fatal("expected runtime token to be set after start")
	}

	// Stop agent
	stopped := doJSON(t, server, http.MethodPost, "/api/agents/"+agentID+"/stop", nil, token)
	if stopped.Code != http.StatusOK {
		t.Fatalf("stop agent: %d: %s", stopped.Code, stopped.Body.String())
	}

	// Verify token is revoked
	server.tokenMu.Lock()
	_, exists := server.tokens[agentID]
	server.tokenMu.Unlock()
	if exists {
		t.Fatal("expected runtime token to be deleted after stop")
	}
}

// --- Integrations ---

func TestIntegrations_ListReturnsProviders(t *testing.T) {
	server := newTestServer()
	token := loginAsAdmin(t, server)

	resp := doJSON(t, server, http.MethodGet, "/api/integrations", nil, token)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, resp.Code, resp.Body.String())
	}

	body := resp.Body.Bytes()
	providers := assertJSONArray(t, body, "providers")
	if len(providers) == 0 {
		t.Fatal("expected at least one provider")
	}
	found := false
	for _, p := range providers {
		if p["provider_id"] == "github" {
			found = true
			if p["name"] != "GitHub" {
				t.Fatalf("expected name GitHub, got %v", p["name"])
			}
			if p["connected"] != false {
				t.Fatal("expected connected=false initially")
			}
			break
		}
	}
	if !found {
		t.Fatal("expected github provider in list")
	}
}

func TestIntegrations_ConnectAndDisconnectGitHub(t *testing.T) {
	server := newTestServer()
	token := loginAsAdmin(t, server)

	// Start a test GitHub server that returns 200 for "ghp_test_valid" and 401 for anything else.
	githubCalled := false
	githubServer := newTestGitHubServer(t, func(w http.ResponseWriter, r *http.Request) {
		githubCalled = true
		auth := r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		if auth == "Bearer ghp_test_valid" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"login":"testuser","id":12345,"type":"User"}`))
		} else {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"message":"bad credentials"}`))
		}
	})
	defer githubServer.Close()

	// Override the GitHub provider with our test server
	server.integrations.RegisterProvider(newTestGitHubProvider(t, githubServer.URL))

	// Connect with valid token
	connectResp := doJSON(t, server, http.MethodPut, "/api/integrations/github/connection", map[string]string{
		"token": "ghp_test_valid",
	}, token)
	if connectResp.Code != http.StatusOK {
		t.Fatalf("expected connect status %d, got %d: %s", http.StatusOK, connectResp.Code, connectResp.Body.String())
	}
	if !githubCalled {
		t.Fatal("expected GitHub API to be called")
	}

	body := connectResp.Body.Bytes()
	assertJSONField(t, body, "provider_id", "github")
	assertJSONField(t, body, "external_login", "testuser")
	assertJSONField(t, body, "external_account_id", "12345")
	assertJSONField(t, body, "account_type", "User")
	assertJSONField(t, body, "status", "connected")

	// Verify token is NOT returned
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	if _, hasToken := decoded["token"]; hasToken {
		t.Fatal("response must not contain the token")
	}
	if _, hasSecret := decoded["secret"]; hasSecret {
		t.Fatal("response must not contain the secret")
	}

	// Connect with invalid token (should fail validation, not save/update)
	githubCalled = false
	failResp := doJSON(t, server, http.MethodPut, "/api/integrations/github/connection", map[string]string{
		"token": "ghp_bad",
	}, token)
	if failResp.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request for invalid token, got %d: %s", failResp.Code, failResp.Body.String())
	}

	// Verify the previous valid connection was NOT overwritten by the failed attempt
	listResp := doJSON(t, server, http.MethodGet, "/api/integrations", nil, token)
	listBody := listResp.Body.Bytes()
	providers := assertJSONArray(t, listBody, "providers")
	foundConnected := false
	for _, p := range providers {
		if p["provider_id"] == "github" {
			if p["connected"] == true {
				foundConnected = true
				// The connection should still have the original login (testuser), not reflect the failed attempt
				if conn, ok := p["connection"].(map[string]any); ok {
					if login, ok := conn["external_login"].(string); ok && login != "testuser" {
						t.Fatalf("expected preserved external_login testuser, got %q", login)
					}
				}
			}
		}
	}
	if !foundConnected {
		t.Fatal("expected previous valid connection to still be present after failed validation attempt")
	}

	// Disconnect
	disconnectResp := doJSON(t, server, http.MethodDelete, "/api/integrations/github/connection", nil, token)
	if disconnectResp.Code != http.StatusOK {
		t.Fatalf("expected disconnect status %d, got %d: %s", http.StatusOK, disconnectResp.Code, disconnectResp.Body.String())
	}

	// Verify disconnected
	listResp2 := doJSON(t, server, http.MethodGet, "/api/integrations", nil, token)
	listBody2 := listResp2.Body.Bytes()
	providers2 := assertJSONArray(t, listBody2, "providers")
	for _, p := range providers2 {
		if p["provider_id"] == "github" {
			if p["connected"] == true {
				t.Fatal("expected connected=false after disconnect")
			}
		}
	}
}

func TestIntegrations_RequiresAuth(t *testing.T) {
	server := newTestServer()

	// All integration endpoints should require auth
	endpoints := []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodGet, "/api/integrations", nil},
		{http.MethodPut, "/api/integrations/github/connection", map[string]string{"token": "test"}},
		{http.MethodDelete, "/api/integrations/github/connection", nil},
	}

	for _, ep := range endpoints {
		resp := doJSON(t, server, ep.method, ep.path, ep.body, "")
		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for %s %s, got %d", ep.method, ep.path, resp.Code)
		}
	}
}

func TestIntegrations_AgentToggleEnforceOwnership(t *testing.T) {
	server := newTestServer()
	adminToken := loginAsAdmin(t, server)

	// Create a non-admin user via admin API
	createdUser := doJSON(t, server, http.MethodPost, "/api/admin/users", map[string]string{
		"username": "alice",
		"password": "secret",
		"role":     "user",
	}, adminToken)
	if createdUser.Code != http.StatusCreated {
		t.Fatalf("create user: %d: %s", createdUser.Code, createdUser.Body.String())
	}
	aliceToken := loginAs(t, server, "alice", "secret")

	// Create another user for cross-ownership test
	createdBob := doJSON(t, server, http.MethodPost, "/api/admin/users", map[string]string{
		"username": "bob",
		"password": "secret",
		"role":     "user",
	}, adminToken)
	if createdBob.Code != http.StatusCreated {
		t.Fatalf("create bob: %d: %s", createdBob.Code, createdBob.Body.String())
	}
	bobToken := loginAs(t, server, "bob", "secret")

	// Create an agent as alice
	created := doJSON(t, server, http.MethodPost, "/api/agents", map[string]any{
		"name":    "AliceAgent",
		"runtime": "nanoclaw",
		"model":   "",
	}, aliceToken)
	if created.Code != http.StatusCreated {
		t.Fatalf("create agent: %d: %s", created.Code, created.Body.String())
	}
	agentID := assertJSONField(t, created.Body.Bytes(), "id", "")

	// Connect GitHub as alice (using test server)
	githubServer := newTestGitHubServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"login":"alice","id":999,"type":"User"}`))
	})
	defer githubServer.Close()
	server.integrations.RegisterProvider(newTestGitHubProvider(t, githubServer.URL))

	connectResp := doJSON(t, server, http.MethodPut, "/api/integrations/github/connection", map[string]string{
		"token": "ghp_alice_valid",
	}, aliceToken)
	if connectResp.Code != http.StatusOK {
		t.Fatalf("connect: %d: %s", connectResp.Code, connectResp.Body.String())
	}

	// Admin tries to toggle alice's agent integration (admin should be allowed)
	adminToggle := doJSON(t, server, http.MethodPut, "/api/agents/"+agentID+"/integrations/github", map[string]bool{
		"enabled": true,
	}, adminToken)
	if adminToggle.Code != http.StatusOK {
		t.Fatalf("admin toggle: %d: %s", adminToggle.Code, adminToggle.Body.String())
	}

	// Bob should NOT be able to toggle alice's agent
	bobToggle := doJSON(t, server, http.MethodPut, "/api/agents/"+agentID+"/integrations/github", map[string]bool{
		"enabled": true,
	}, bobToken)
	if bobToggle.Code != http.StatusNotFound {
		t.Fatalf("expected bob's toggle to be 404 (not found/not owner), got %d", bobToggle.Code)
	}

	// alice can toggle her own agent
	aliceToggle := doJSON(t, server, http.MethodPut, "/api/agents/"+agentID+"/integrations/github", map[string]bool{
		"enabled": true,
	}, aliceToken)
	if aliceToggle.Code != http.StatusOK {
		t.Fatalf("alice toggle: %d: %s", aliceToggle.Code, aliceToggle.Body.String())
	}
	body := aliceToggle.Body.Bytes()
	assertJSONField(t, body, "agent_id", agentID)
	assertJSONField(t, body, "provider_id", "github")
	assertJSONField(t, body, "status", "active")
	if v := assertJSONField(t, body, "enabled", ""); v != "true" {
		// Check bool field
		var decoded map[string]any
		json.Unmarshal(body, &decoded)
		if enabled, ok := decoded["enabled"].(bool); !ok || !enabled {
			t.Fatalf("expected enabled=true, got %#v", decoded["enabled"])
		}
	}

	// Toggle off
	aliceToggleOff := doJSON(t, server, http.MethodPut, "/api/agents/"+agentID+"/integrations/github", map[string]bool{
		"enabled": false,
	}, aliceToken)
	if aliceToggleOff.Code != http.StatusOK {
		t.Fatalf("alice toggle off: %d: %s", aliceToggleOff.Code, aliceToggleOff.Body.String())
	}
}

func TestIntegrations_AgentToggleRequiresConnection(t *testing.T) {
	server := newTestServer()
	token := loginAsAdmin(t, server)

	// Create agent
	created := doJSON(t, server, http.MethodPost, "/api/agents", map[string]any{
		"name":    "NoConnAgent",
		"runtime": "nanoclaw",
	}, token)
	agentID := assertJSONField(t, created.Body.Bytes(), "id", "")

	// Try to toggle without a connection first
	toggleResp := doJSON(t, server, http.MethodPut, "/api/agents/"+agentID+"/integrations/github", map[string]bool{
		"enabled": true,
	}, token)
	if toggleResp.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request (no connection), got %d: %s", toggleResp.Code, toggleResp.Body.String())
	}
}

// testGitHubServer creates an httptest.Server that responds to /user.
func newTestGitHubServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user" {
			handler(w, r)
			return
		}
		http.NotFound(w, r)
	}))
}

func newTestGitHubProvider(t *testing.T, baseURL string) *integrations.GitHubProvider {
	t.Helper()
	return integrations.NewGitHubProvider(integrations.GitHubProviderConfig{
		BaseURL:    baseURL,
		HTTPClient: &http.Client{},
	})
}

// --- Helpers ---

func newTestServer() *Server {
	return newTestServerWithConfig(config.Config{Store: "inmemory"})
}

func newTestServerWithConfig(cfg config.Config) *Server {
	server, err := NewServer(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		panic(err)
	}
	return server
}

func loginAsAdmin(t *testing.T, server *Server) string {
	t.Helper()
	return loginAs(t, server, "admin", "admin")
}

func loginAs(t *testing.T, server *Server, username, password string) string {
	t.Helper()
	response := doJSON(t, server, http.MethodPost, "/api/auth/login", map[string]string{
		"username": username,
		"password": password,
	}, "")
	if response.Code != http.StatusOK {
		t.Fatalf("login failed for %s: status %d: %s", username, response.Code, response.Body.String())
	}
	return assertJSONField(t, response.Body.Bytes(), "token", "")
}

func doJSON(t *testing.T, server *Server, method, path string, payload any, token string) *httptest.ResponseRecorder {
	t.Helper()

	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}
		body = bytes.NewReader(encoded)
	}

	request := httptest.NewRequest(method, path, body)
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	return response
}

func assertJSONField(t *testing.T, body []byte, key string, want string) string {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	got, _ := decoded[key].(string)
	if want != "" && got != want {
		t.Fatalf("expected %s %q, got %q", key, want, got)
	}
	return got
}

func assertJSONObject(t *testing.T, body []byte, key string) map[string]any {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	object, ok := decoded[key].(map[string]any)
	if !ok {
		t.Fatalf("expected %s object, got %#v", key, decoded[key])
	}
	return object
}

func assertJSONArray(t *testing.T, body []byte, key string) []map[string]any {
	t.Helper()
	if key == "" {
		var items []map[string]any
		if err := json.Unmarshal(body, &items); err != nil {
			t.Fatalf("decode json array: %v", err)
		}
		return items
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	items, ok := decoded[key].([]any)
	if !ok {
		t.Fatalf("expected %s array, got %#v", key, decoded[key])
	}
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		object, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("expected object in %s array, got %#v", key, item)
		}
		result = append(result, object)
	}
	return result
}
