package integrations

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/mipopov/shclop/internal/domain"
	"github.com/mipopov/shclop/internal/plugins"
	"github.com/mipopov/shclop/internal/store"
)

// ---------------------------------------------------------------------------
// fake registry (mirrors plugins.fakeRegistry; package plugins is internal)
// ---------------------------------------------------------------------------

type fakeRegistry struct {
	mu       sync.Mutex
	items    map[string]plugins.Manifest
	src      string
	notifyCh chan struct{}
}

func newFakeRegistry(src string) *fakeRegistry {
	return &fakeRegistry{
		src:      src,
		items:    make(map[string]plugins.Manifest),
		notifyCh: make(chan struct{}, 1),
	}
}

func (f *fakeRegistry) set(id string, m plugins.Manifest) {
	f.mu.Lock()
	f.items[id] = m
	f.mu.Unlock()
	select {
	case f.notifyCh <- struct{}{}:
	default:
	}
}

func (f *fakeRegistry) Get(id string) (plugins.Resolved, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, ok := f.items[id]
	if !ok {
		return plugins.Resolved{}, false
	}
	return plugins.Resolved{Manifest: m, Source: f.src}, true
}

func (f *fakeRegistry) List() []plugins.Resolved {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]plugins.Resolved, 0, len(f.items))
	for _, m := range f.items {
		out = append(out, plugins.Resolved{Manifest: m, Source: f.src})
	}
	return out
}

func (f *fakeRegistry) Subscribe() <-chan struct{} { return f.notifyCh }
func (f *fakeRegistry) Run(_ context.Context) error { return nil }

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// httpTemplateManifest builds a minimal http_template manifest pointing at baseURL.
func httpTemplateManifest(id, baseURL string) plugins.Manifest {
	return plugins.Manifest{
		APIVersion: "shclop.io/v1alpha1",
		Kind:       "IntegrationPlugin",
		Metadata:   plugins.Metadata{Name: id},
		Spec: plugins.ManifestSpec{
			ID:              id,
			DisplayName:     strings.ToUpper(id[:1]) + id[1:],
			Description:     "Test plugin " + id,
			PluginKind:      "http_template",
			ScopesSupported: []string{"user"},
			Auth: plugins.AuthSpec{
				Type: "pat_token",
				Fields: []plugins.FormField{
					{Name: "token", Label: "Personal Access Token", Secret: true, Placeholder: "tkn_..."},
				},
			},
			Validate: plugins.ValidateSpec{
				HTTP: &plugins.HTTPValidate{
					Method:        "GET",
					URL:           baseURL + "/validate",
					Headers:       map[string]string{"Authorization": "Bearer {{.Token}}"},
					SuccessStatus: []int{200},
					Extract: plugins.ExtractSpec{
						ExternalLogin: ".login",
					},
				},
			},
			Contributions: plugins.Contributions{
				Env: map[string]string{"TEST_TOKEN": "{{.Token}}"},
			},
		},
	}
}

// httpTemplateMCPManifest is like httpTemplateManifest but contributes one external MCP server.
func httpTemplateMCPManifest(id, baseURL string) plugins.Manifest {
	m := httpTemplateManifest(id, baseURL)
	m.Spec.Contributions.MCPServers = []plugins.MCPRef{
		{
			Name: "my-mcp",
			External: &plugins.ExternalMCP{
				URL:        "https://api.example.com/mcp",
				AuthHeader: "Bearer {{.Token}}",
			},
		},
	}
	return m
}

// newTestService builds a Service wired to an in-memory store + registry with a
// test encryption key.
func newTestService(t *testing.T, reg plugins.Registry) (*Service, *store.Memory) {
	t.Helper()
	st := store.NewMemory()
	key := make([]byte, 32)
	sb := NewSecretBox(key)
	// resolver with zero value — safe for external-only or empty MCP refs.
	resolver := &plugins.MCPResolver{}
	svc := NewService(st, sb, reg, resolver, nil)
	return svc, st
}

// fakeGitHubServer returns an httptest.Server that responds like GitHub's /user
// endpoint when the token is "valid-token", 401 otherwise. Caller must Close it.
func fakeGitHubServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/validate" {
			http.NotFound(w, r)
			return
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer valid-token" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"message": "bad credentials"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"login": "testuser", "id": 42, "type": "User"})
	}))
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// 1. Connect + GetSummary happy path.
func TestConnect_HappyPath_HTTPTemplate(t *testing.T) {
	srv := fakeGitHubServer(t)
	defer srv.Close()

	reg := newFakeRegistry("test")
	reg.set("myplugin", httpTemplateManifest("myplugin", srv.URL))

	svc, _ := newTestService(t, reg)
	ctx := context.Background()

	conn, err := svc.Connect(ctx, "user-1", "myplugin", map[string]string{"token": "valid-token"})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if conn.ProviderID != "myplugin" {
		t.Fatalf("expected provider_id myplugin, got %q", conn.ProviderID)
	}
	if conn.Status != "connected" {
		t.Fatalf("expected status connected, got %q", conn.Status)
	}

	summary, err := svc.BuildSummary(ctx, "user-1")
	if err != nil {
		t.Fatalf("BuildSummary: %v", err)
	}
	if len(summary.Providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(summary.Providers))
	}
	ps := summary.Providers[0]
	if !ps.Connected {
		t.Fatal("expected Connected=true")
	}
	if ps.AuthKind != "pat_token" {
		t.Fatalf("expected auth_kind=pat_token, got %q", ps.AuthKind)
	}
	if len(ps.FormFields) != 1 || ps.FormFields[0].Name != "token" {
		t.Fatalf("unexpected form_fields: %+v", ps.FormFields)
	}
	if ps.Description != "Test plugin myplugin" {
		t.Fatalf("unexpected description: %q", ps.Description)
	}
}

// 2. Connect failure: validation returns 401.
func TestConnect_ValidationFailure(t *testing.T) {
	srv := fakeGitHubServer(t)
	defer srv.Close()

	reg := newFakeRegistry("test")
	reg.set("myplugin", httpTemplateManifest("myplugin", srv.URL))

	svc, st := newTestService(t, reg)
	ctx := context.Background()

	_, err := svc.Connect(ctx, "user-1", "myplugin", map[string]string{"token": "bad-token"})
	if err == nil {
		t.Fatal("expected error for bad token")
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Fatalf("unexpected error message: %v", err)
	}

	// No connection should have been stored.
	_, storeErr := st.GetIntegrationConnection(ctx, "user-1", "myplugin")
	if storeErr != store.ErrNotFound {
		t.Fatalf("expected ErrNotFound after failed connect, got %v", storeErr)
	}
}

// 3. Connect with unregistered provider.
func TestConnect_UnregisteredProvider(t *testing.T) {
	reg := newFakeRegistry("test")
	svc, _ := newTestService(t, reg)

	_, err := svc.Connect(context.Background(), "user-1", "ghost", map[string]string{"token": "tok"})
	if err == nil {
		t.Fatal("expected error for unregistered provider")
	}
	if !strings.Contains(err.Error(), "not registered") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// 4. Disconnect: deletes connection; works even when plugin absent from registry.
func TestDisconnect_OrphanedConnection(t *testing.T) {
	reg := newFakeRegistry("test")
	svc, st := newTestService(t, reg)
	ctx := context.Background()

	// Plant a connection directly in the store (no registry entry).
	enc, _ := NewSecretBox(make([]byte, 32)).EncryptToString([]byte("tok"))
	_, err := st.UpsertIntegrationConnection(ctx, domain.IntegrationConnection{
		ProviderID: "old-plugin",
		UserID:     "user-1",
		Status:     "connected",
		Secret:     enc,
	})
	if err != nil {
		t.Fatalf("UpsertIntegrationConnection: %v", err)
	}

	if err := svc.Disconnect(ctx, "user-1", "old-plugin"); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}

	_, err = st.GetIntegrationConnection(ctx, "user-1", "old-plugin")
	if err != store.ErrNotFound {
		t.Fatalf("expected ErrNotFound after disconnect, got %v", err)
	}
}

// 5. BuildSummary surfaces orphaned credentials.
func TestBuildSummary_OrphanedCredential(t *testing.T) {
	reg := newFakeRegistry("test")
	svc, st := newTestService(t, reg)
	ctx := context.Background()

	// Plant orphaned connection.
	enc, _ := NewSecretBox(make([]byte, 32)).EncryptToString([]byte("old-tok"))
	_, err := st.UpsertIntegrationConnection(ctx, domain.IntegrationConnection{
		ProviderID: "old-plugin",
		UserID:     "user-1",
		Status:     "connected",
		Secret:     enc,
	})
	if err != nil {
		t.Fatalf("UpsertIntegrationConnection: %v", err)
	}

	summary, err := svc.BuildSummary(ctx, "user-1")
	if err != nil {
		t.Fatalf("BuildSummary: %v", err)
	}

	var found *domain.ProviderSummary
	for i := range summary.Providers {
		if summary.Providers[i].ProviderID == "old-plugin" {
			found = &summary.Providers[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected orphaned provider in summary")
	}
	if found.AuthKind != "unknown" {
		t.Fatalf("expected auth_kind=unknown, got %q", found.AuthKind)
	}
	if !found.Connected {
		t.Fatal("expected Connected=true for orphan")
	}
	if !strings.Contains(found.Description, "no longer registered") {
		t.Fatalf("unexpected orphan description: %q", found.Description)
	}
}

// 6. ToggleAgentIntegration.
func TestToggleAgentIntegration(t *testing.T) {
	reg := newFakeRegistry("test")
	svc, _ := newTestService(t, reg)
	ctx := context.Background()

	ai, err := svc.ToggleAgentIntegration(ctx, "agent-1", "myplugin", true)
	if err != nil {
		t.Fatalf("ToggleAgentIntegration enable: %v", err)
	}
	if !ai.Enabled {
		t.Fatal("expected enabled=true")
	}
	if ai.Status != "active" {
		t.Fatalf("expected status=active, got %q", ai.Status)
	}

	ai2, err := svc.ToggleAgentIntegration(ctx, "agent-1", "myplugin", false)
	if err != nil {
		t.Fatalf("ToggleAgentIntegration disable: %v", err)
	}
	if ai2.Enabled {
		t.Fatal("expected enabled=false")
	}
	if ai2.Status != "disabled" {
		t.Fatalf("expected status=disabled, got %q", ai2.Status)
	}
}

// 7. ResolveAgentRuntime happy path: http_template env contribution.
func TestResolveAgentRuntime_HTTPTemplate_Env(t *testing.T) {
	srv := fakeGitHubServer(t)
	defer srv.Close()

	reg := newFakeRegistry("test")
	reg.set("myplugin", httpTemplateManifest("myplugin", srv.URL))

	svc, _ := newTestService(t, reg)
	ctx := context.Background()

	// Connect first to store the credential.
	_, err := svc.Connect(ctx, "user-1", "myplugin", map[string]string{"token": "valid-token"})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	// Enable the integration for agent-1.
	if _, err := svc.ToggleAgentIntegration(ctx, "agent-1", "myplugin", true); err != nil {
		t.Fatalf("ToggleAgentIntegration: %v", err)
	}

	res, err := svc.ResolveAgentRuntime(ctx, "user-1", "agent-1")
	if err != nil {
		t.Fatalf("ResolveAgentRuntime: %v", err)
	}

	if res.Env["TEST_TOKEN"] != "valid-token" {
		t.Fatalf("expected TEST_TOKEN=valid-token, got %q", res.Env["TEST_TOKEN"])
	}
	if len(res.MCPServers) != 0 {
		t.Fatalf("expected 0 MCP servers, got %d", len(res.MCPServers))
	}
}

// 8. ResolveAgentRuntime with external MCP server contribution.
func TestResolveAgentRuntime_ExternalMCP(t *testing.T) {
	srv := fakeGitHubServer(t)
	defer srv.Close()

	reg := newFakeRegistry("test")
	reg.set("myplugin", httpTemplateMCPManifest("myplugin", srv.URL))

	svc, _ := newTestService(t, reg)
	ctx := context.Background()

	_, err := svc.Connect(ctx, "user-1", "myplugin", map[string]string{"token": "valid-token"})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if _, err := svc.ToggleAgentIntegration(ctx, "agent-1", "myplugin", true); err != nil {
		t.Fatalf("ToggleAgentIntegration: %v", err)
	}

	res, err := svc.ResolveAgentRuntime(ctx, "user-1", "agent-1")
	if err != nil {
		t.Fatalf("ResolveAgentRuntime: %v", err)
	}

	if len(res.MCPServers) != 1 {
		t.Fatalf("expected 1 MCP server, got %d", len(res.MCPServers))
	}
	mcp := res.MCPServers[0]
	if mcp.Name != "my-mcp" {
		t.Fatalf("expected name=my-mcp, got %q", mcp.Name)
	}
	if mcp.URL != "https://api.example.com/mcp" {
		t.Fatalf("expected URL=https://api.example.com/mcp, got %q", mcp.URL)
	}
	if mcp.Headers["Authorization"] != "Bearer valid-token" {
		t.Fatalf("expected Authorization header, got %v", mcp.Headers)
	}
}

// 9. ResolveAgentRuntime fails when enabled plugin is unregistered.
func TestResolveAgentRuntime_UnregisteredPlugin(t *testing.T) {
	reg := newFakeRegistry("test")
	svc, st := newTestService(t, reg)
	ctx := context.Background()

	// Toggle on an integration with no matching registry entry.
	if _, err := st.UpsertAgentIntegration(ctx, "agent-1", "ghost", true, 0, "active"); err != nil {
		t.Fatalf("UpsertAgentIntegration: %v", err)
	}

	_, err := svc.ResolveAgentRuntime(ctx, "user-1", "agent-1")
	if err == nil {
		t.Fatal("expected error for unregistered plugin")
	}
	if !strings.Contains(err.Error(), "unregistered") {
		t.Fatalf("unexpected error: %v", err)
	}
}
