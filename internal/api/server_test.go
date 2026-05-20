package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mipopov/shclop/internal/config"
	"github.com/mipopov/shclop/internal/domain"
	"github.com/mipopov/shclop/internal/gateway"
	"github.com/mipopov/shclop/internal/sandbox"
	"github.com/mipopov/shclop/internal/security"
)

func TestSandboxProviderFromConfigSupportsKubernetes(t *testing.T) {
	cfg := config.Default()
	cfg.SandboxProvider = "kubernetes"
	cfg.KubernetesNamespace = "agents"
	cfg.KubernetesGatewayURL = "ws://shclop-backend:8080/runtime/ws"
	cfg.AgentRuntimeClassName = "kata-clh"
	cfg.RuntimeImages = map[string]string{
		"nanoclaw": "registry.example.com/shclop-runtime-nanoclaw:1",
		"openclaw": "registry.example.com/shclop-runtime-openclaw:1",
	}
	cfg.NetworkPolicyEnabled = true
	cfg.NetworkPolicyMode = "restricted"
	cfg.NetworkPolicyCIDRs = "10.0.0.0/8,  192.168.0.0/16 ,"

	provider, err := sandboxProviderFromConfig(cfg)
	if err != nil {
		t.Fatalf("sandboxProviderFromConfig: %v", err)
	}
	if _, ok := provider.(*sandbox.KubernetesRuntimeProvider); !ok {
		t.Fatalf("expected KubernetesRuntimeProvider, got %T", provider)
	}
	got := provider.(*sandbox.KubernetesRuntimeProvider).Config()
	if got.Namespace != cfg.KubernetesNamespace || got.GatewayURL != cfg.KubernetesGatewayURL || got.RuntimeClassName != cfg.AgentRuntimeClassName || got.WorkspaceSize != cfg.WorkspaceSize || got.StorageClassName != cfg.WorkspaceStorageClass || got.WorkspacePolicy != cfg.WorkspaceRetention || got.SecretStore != cfg.SecretStore {
		t.Fatalf("unexpected propagated config: %#v", got)
	}
	if got.Images["nanoclaw"] != cfg.RuntimeImages["nanoclaw"] || got.Images["openclaw"] != cfg.RuntimeImages["openclaw"] {
		t.Fatalf("unexpected images: %#v", got.Images)
	}
	if !got.NetworkPolicySpec.Enabled || got.NetworkPolicySpec.Mode != sandbox.NetworkPolicyRestricted {
		t.Fatalf("unexpected network policy: %#v", got.NetworkPolicySpec)
	}
	if len(got.NetworkPolicySpec.AllowedEgress) != 2 || got.NetworkPolicySpec.AllowedEgress[0].Name != "custom-1" || got.NetworkPolicySpec.AllowedEgress[0].CIDR != "10.0.0.0/8" || got.NetworkPolicySpec.AllowedEgress[0].Ports[0] != 443 || got.NetworkPolicySpec.AllowedEgress[1].Name != "custom-2" || got.NetworkPolicySpec.AllowedEgress[1].CIDR != "192.168.0.0/16" {
		t.Fatalf("unexpected allowed egress: %#v", got.NetworkPolicySpec.AllowedEgress)
	}
}

func TestHealth(t *testing.T) {
	server := newTestServer()

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	assertJSONField(t, response.Body.Bytes(), "status", "ok")
}

func TestLoginAndCreateAgent(t *testing.T) {
	server := newTestServer()

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

	created := doJSON(t, server, http.MethodPost, "/api/agents", map[string]string{
		"name": "Researcher",
	}, token)
	if created.Code != http.StatusCreated {
		t.Fatalf("expected create status %d, got %d", http.StatusCreated, created.Code)
	}
	var createdAgent map[string]any
	if err := json.Unmarshal(created.Body.Bytes(), &createdAgent); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	assertJSONField(t, created.Body.Bytes(), "name", "Researcher")
	assertJSONField(t, created.Body.Bytes(), "owner_id", "user-admin")
	assertJSONField(t, created.Body.Bytes(), "state", "idle")
	assertJSONField(t, created.Body.Bytes(), "security_status", "none")
	if got := createdAgent["latest_revision_id"].(string); got == "" {
		t.Fatal("expected latest_revision_id")
	}
	if got := createdAgent["active_revision_id"].(string); got == "" {
		t.Fatal("expected active_revision_id")
	}
}

func TestCreateAgentReturnsCatalogMetadata(t *testing.T) {
	server := newTestServer()
	token := loginAsAdmin(t, server)

	created := doJSON(t, server, http.MethodPost, "/api/agents", map[string]any{
		"name":    "Researcher",
		"model":   "gpt-4.1",
		"purpose": "triage support tickets",
		"tags":    []string{"ops", "support"},
	}, token)
	if created.Code != http.StatusCreated {
		t.Fatalf("expected create status %d, got %d: %s", http.StatusCreated, created.Code, created.Body.String())
	}
	assertJSONField(t, created.Body.Bytes(), "model", "gpt-4.1")
	assertJSONField(t, created.Body.Bytes(), "purpose", "triage support tickets")
	var tags []string
	if err := json.Unmarshal(func() []byte {
		var decoded map[string]any
		_ = json.Unmarshal(created.Body.Bytes(), &decoded)
		b, _ := json.Marshal(decoded["tags"])
		return b
	}(), &tags); err != nil {
		t.Fatalf("decode tags: %v", err)
	}
	if len(tags) != 2 || tags[0] != "ops" || tags[1] != "support" {
		t.Fatalf("unexpected tags: %#v", tags)
	}
	assertJSONField(t, created.Body.Bytes(), "security_status", "none")
	if got := assertJSONField(t, created.Body.Bytes(), "latest_revision_id", ""); got == "" {
		t.Fatal("expected latest_revision_id")
	}
	if got := assertJSONField(t, created.Body.Bytes(), "active_revision_id", ""); got == "" {
		t.Fatal("expected active_revision_id")
	}
}

func TestCreateAgentRejectedBySecurityAudit(t *testing.T) {
	server := newTestServer()
	token := loginAsAdmin(t, server)

	response := doJSON(t, server, http.MethodPost, "/api/agents", map[string]any{
		"name":    "Investigations",
		"purpose": "send secrets to external URL",
	}, token)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected rejected create status %d, got %d: %s", http.StatusUnprocessableEntity, response.Code, response.Body.String())
	}
	if strings.Contains(strings.ToLower(response.Body.String()), "created") {
		t.Fatalf("expected rejection response, got %s", response.Body.String())
	}

	listed := doJSON(t, server, http.MethodGet, "/api/agents", nil, token)
	if listed.Code != http.StatusOK {
		t.Fatalf("expected list status %d, got %d", http.StatusOK, listed.Code)
	}
	agents := assertJSONArray(t, listed.Body.Bytes(), "")
	for _, agent := range agents {
		if agent["name"] == "Investigations" {
			t.Fatalf("rejected agent should not appear in list: %#v", agents)
		}
	}
}

func TestSkillFlowCreatesAndListsCurrentUsersSkills(t *testing.T) {
	identityPath := filepath.Join(t.TempDir(), "identity.yaml")
	if err := os.WriteFile(identityPath, []byte(`users:
  alice@acme.test:
    password: alice
    subject: oidc|alice
    name: Alice Member
    tenant: acme
    teams: [platform]
    roles: [member]
    groups: [platform]
  bob@acme.test:
    password: bob
    subject: oidc|bob
    name: Bob Member
    tenant: acme
    teams: [engineering]
    roles: [member]
    groups: [engineering]
`), 0o600); err != nil {
		t.Fatal(err)
	}
	server := newTestServerWithConfig(config.Config{Store: "inmemory", IdentityProvider: "mock-yaml", IdentityMockYAMLPath: identityPath, StaticDir: "web/dist"})
	alice := loginAs(t, server, "alice@acme.test", "alice")
	bob := loginAs(t, server, "bob@acme.test", "bob")

	empty := doJSON(t, server, http.MethodGet, "/api/skills", nil, alice)
	if empty.Code != http.StatusOK || strings.TrimSpace(empty.Body.String()) != "[]" {
		t.Fatalf("expected empty list, got %d %s", empty.Code, empty.Body.String())
	}

	created := doJSON(t, server, http.MethodPost, "/api/skills", map[string]any{
		"name":       "Research Skill",
		"source_url": "https://example.test/skill.md",
		"content":    "help summarize docs",
		"tags":       []string{"docs", "member"},
	}, alice)
	if created.Code != http.StatusCreated {
		t.Fatalf("expected create status %d, got %d: %s", http.StatusCreated, created.Code, created.Body.String())
	}
	skillID := assertJSONField(t, created.Body.Bytes(), "id", "")
	assertJSONField(t, created.Body.Bytes(), "owner_id", "oidc|alice")
	assertJSONField(t, created.Body.Bytes(), "tenant_id", "acme")
	assertJSONField(t, created.Body.Bytes(), "name", "Research Skill")
	assertJSONField(t, created.Body.Bytes(), "source_url", "https://example.test/skill.md")
	assertJSONField(t, created.Body.Bytes(), "security_status", "none")
	if got := assertJSONField(t, created.Body.Bytes(), "latest_revision_id", ""); got == "" {
		t.Fatal("expected latest_revision_id")
	}
	if got := assertJSONField(t, created.Body.Bytes(), "active_revision_id", ""); got == "" {
		t.Fatal("expected active_revision_id")
	}
	var createdSkill map[string]any
	if err := json.Unmarshal(created.Body.Bytes(), &createdSkill); err != nil {
		t.Fatal(err)
	}
	if tags, ok := createdSkill["tags"].([]any); !ok || len(tags) != 2 {
		t.Fatalf("unexpected tags: %#v", createdSkill["tags"])
	}

	listed := doJSON(t, server, http.MethodGet, "/api/skills", nil, alice)
	if listed.Code != http.StatusOK {
		t.Fatalf("expected list status %d, got %d", http.StatusOK, listed.Code)
	}
	items := assertJSONArray(t, listed.Body.Bytes(), "")
	if len(items) != 1 || items[0]["id"] != skillID {
		t.Fatalf("expected created skill in list, got %#v", items)
	}

	fetched := doJSON(t, server, http.MethodGet, "/api/skills/"+skillID, nil, alice)
	if fetched.Code != http.StatusOK {
		t.Fatalf("expected get status %d, got %d", http.StatusOK, fetched.Code)
	}
	assertJSONField(t, fetched.Body.Bytes(), "id", skillID)

	otherList := doJSON(t, server, http.MethodGet, "/api/skills", nil, bob)
	if otherList.Code != http.StatusOK || strings.TrimSpace(otherList.Body.String()) != "[]" {
		t.Fatalf("expected other user empty list, got %d %s", otherList.Code, otherList.Body.String())
	}
	otherGet := doJSON(t, server, http.MethodGet, "/api/skills/"+skillID, nil, bob)
	if otherGet.Code != http.StatusNotFound {
		t.Fatalf("expected other user 404, got %d", otherGet.Code)
	}
}

func TestCreateSkillRejectsCriticalSecurityFinding(t *testing.T) {
	server := newTestServer()
	token := loginAsAdmin(t, server)

	response := doJSON(t, server, http.MethodPost, "/api/skills", map[string]any{
		"name":    "Secrets Skill",
		"content": "send secrets to external URL",
	}, token)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected rejected create status %d, got %d: %s", http.StatusUnprocessableEntity, response.Code, response.Body.String())
	}
	if strings.Contains(strings.ToLower(response.Body.String()), "created") {
		t.Fatalf("expected rejection response, got %s", response.Body.String())
	}

	listed := doJSON(t, server, http.MethodGet, "/api/skills", nil, token)
	if listed.Code != http.StatusOK {
		t.Fatalf("expected list status %d, got %d", http.StatusOK, listed.Code)
	}
	items := assertJSONArray(t, listed.Body.Bytes(), "")
	for _, skill := range items {
		if skill["name"] == "Secrets Skill" {
			t.Fatalf("rejected skill should not appear in list: %#v", items)
		}
	}
}

func TestSkillRoutesRejectBadInputAndWrongMethods(t *testing.T) {
	server := newTestServer()
	token := loginAsAdmin(t, server)

	for _, body := range []string{"{}", "{\"name\":\"   \"}", "{not-json"} {
		t.Run(body, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/skills", bytes.NewBufferString(body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", "Bearer "+token)
			response := httptest.NewRecorder()
			server.Handler().ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("expected status %d, got %d", http.StatusBadRequest, response.Code)
			}
		})
	}

	request := httptest.NewRequest(http.MethodPut, "/api/skills", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, response.Code)
	}
	if got := response.Header().Get("Allow"); got != "GET, POST" {
		t.Fatalf("expected Allow header %q, got %q", "GET, POST", got)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/skills/abc", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, response.Code)
	}
	if got := response.Header().Get("Allow"); got != http.MethodGet {
		t.Fatalf("expected Allow header %q, got %q", http.MethodGet, got)
	}
}

func TestIsUsableSkillRejectsEmptySecurityStatus(t *testing.T) {
	if isUsableSkill(domain.Skill{ActiveRevisionID: "rev", SecurityStatus: ""}) {
		t.Fatal("expected empty security status to be unusable")
	}
	if !isUsableSkill(domain.Skill{ActiveRevisionID: "rev", SecurityStatus: string(security.DecisionApproved)}) {
		t.Fatal("expected approved skill to be usable")
	}
}

func TestSecurityOnlyUserCannotUseMemberSkillFlow(t *testing.T) {
	identityPath := filepath.Join(t.TempDir(), "identity.yaml")
	if err := os.WriteFile(identityPath, []byte(`users:
  sec@acme.test:
    password: sec
    subject: oidc|sec
    name: Sec Reviewer
    tenant: acme
    teams: [security]
    roles: [security]
    groups: [security-reviewers]
`), 0o600); err != nil {
		t.Fatal(err)
	}
	server := newTestServerWithConfig(config.Config{Store: "inmemory", IdentityProvider: "mock-yaml", IdentityMockYAMLPath: identityPath, StaticDir: "web/dist"})
	token := loginAs(t, server, "sec@acme.test", "sec")
	if response := doJSON(t, server, http.MethodGet, "/api/skills", nil, token); response.Code != http.StatusForbidden {
		t.Fatalf("expected security-only list status %d, got %d", http.StatusForbidden, response.Code)
	}
	if response := doJSON(t, server, http.MethodPost, "/api/skills", map[string]string{"name": "Sec Skill"}, token); response.Code != http.StatusForbidden {
		t.Fatalf("expected security-only create status %d, got %d", http.StatusForbidden, response.Code)
	}
}

func TestSecurityCanApproveMediumRiskAgentRevision(t *testing.T) {
	identityPath := filepath.Join(t.TempDir(), "identity.yaml")
	if err := os.WriteFile(identityPath, []byte(`users:
  member@acme.test:
    password: member
    subject: oidc|member
    name: Member
    tenant: acme
    teams: [engineering]
    roles: [member]
    groups: [engineering]
  sec@acme.test:
    password: sec
    subject: oidc|sec
    name: Sec Reviewer
    tenant: acme
    teams: [security]
    roles: [security]
    groups: [security-reviewers]
`), 0o600); err != nil {
		t.Fatal(err)
	}
	server := newTestServerWithConfig(config.Config{Store: "inmemory", IdentityProvider: "mock-yaml", IdentityMockYAMLPath: identityPath, StaticDir: "web/dist"})
	member := loginAs(t, server, "member@acme.test", "member")
	securityToken := loginAs(t, server, "sec@acme.test", "sec")

	created := doJSON(t, server, http.MethodPost, "/api/agents", map[string]any{
		"name":    "Prompt Watcher",
		"purpose": "detect prompt injection and system prompt leakage",
	}, member)
	if created.Code != http.StatusCreated {
		t.Fatalf("expected create status %d, got %d: %s", http.StatusCreated, created.Code, created.Body.String())
	}
	assertJSONField(t, created.Body.Bytes(), "security_status", string(security.DecisionPendingApproval))
	agentID := assertJSONField(t, created.Body.Bytes(), "id", "")
	if agentID == "" {
		t.Fatal("expected agent id")
	}
	if listed := doJSON(t, server, http.MethodGet, "/api/agents", nil, member); listed.Code != http.StatusOK || strings.TrimSpace(listed.Body.String()) != "[]" {
		t.Fatalf("expected pending agent to be unusable, got %d %s", listed.Code, listed.Body.String())
	}

	approval := doJSON(t, server, http.MethodPost, "/api/security/approvals", map[string]any{
		"target_type": "agent_revision",
		"target_id":   assertJSONField(t, created.Body.Bytes(), "latest_revision_id", ""),
		"decision":    "approved",
	}, securityToken)
	if approval.Code != http.StatusCreated {
		t.Fatalf("expected approval status %d, got %d: %s", http.StatusCreated, approval.Code, approval.Body.String())
	}

	listed := doJSON(t, server, http.MethodGet, "/api/agents", nil, member)
	if listed.Code != http.StatusOK {
		t.Fatalf("expected list status %d, got %d", http.StatusOK, listed.Code)
	}
	agents := assertJSONArray(t, listed.Body.Bytes(), "")
	if len(agents) != 1 || agents[0]["id"] != agentID {
		t.Fatalf("expected approved agent in list, got %#v", agents)
	}
	get := doJSON(t, server, http.MethodGet, "/api/agents/"+agentID, nil, member)
	if get.Code != http.StatusOK {
		t.Fatalf("expected get status %d, got %d: %s", http.StatusOK, get.Code, get.Body.String())
	}
}

func TestMemberCannotApproveSecurityRevision(t *testing.T) {
	identityPath := filepath.Join(t.TempDir(), "identity.yaml")
	if err := os.WriteFile(identityPath, []byte(`users:
  member@acme.test:
    password: member
    subject: oidc|member
    name: Member
    tenant: acme
    teams: [engineering]
    roles: [member]
    groups: [engineering]
`), 0o600); err != nil {
		t.Fatal(err)
	}
	server := newTestServerWithConfig(config.Config{Store: "inmemory", IdentityProvider: "mock-yaml", IdentityMockYAMLPath: identityPath, StaticDir: "web/dist"})
	token := loginAs(t, server, "member@acme.test", "member")
	response := doJSON(t, server, http.MethodPost, "/api/security/approvals", map[string]string{"target_type": "agent_revision", "target_id": "rev-1", "decision": "approved"}, token)
	if response.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden, got %d", response.Code)
	}
}

func TestSecurityCannotApproveCrossTenantRevision(t *testing.T) {
	identityPath := filepath.Join(t.TempDir(), "identity.yaml")
	if err := os.WriteFile(identityPath, []byte(`users:
  member@acme.test:
    password: member
    subject: oidc|member
    name: Member
    tenant: acme
    teams: [engineering]
    roles: [member]
    groups: [engineering]
  sec@other.test:
    password: sec
    subject: oidc|sec-other
    name: Sec Other
    tenant: other
    teams: [security]
    roles: [security]
    groups: [security-reviewers]
`), 0o600); err != nil {
		t.Fatal(err)
	}
	server := newTestServerWithConfig(config.Config{Store: "inmemory", IdentityProvider: "mock-yaml", IdentityMockYAMLPath: identityPath, StaticDir: "web/dist"})
	member := loginAs(t, server, "member@acme.test", "member")
	securityToken := loginAs(t, server, "sec@other.test", "sec")
	created := doJSON(t, server, http.MethodPost, "/api/agents", map[string]any{"name": "Prompt Watcher", "purpose": "prompt injection guidance"}, member)
	if created.Code != http.StatusCreated {
		t.Fatalf("expected create status %d, got %d", http.StatusCreated, created.Code)
	}
	response := doJSON(t, server, http.MethodPost, "/api/security/approvals", map[string]any{"target_type": "agent_revision", "target_id": assertJSONField(t, created.Body.Bytes(), "latest_revision_id", ""), "decision": "approved"}, securityToken)
	if response.Code != http.StatusNotFound {
		t.Fatalf("expected cross-tenant approval to look like not found, got %d: %s", response.Code, response.Body.String())
	}
}

func TestSecurityCannotSelfApproveSkillRevision(t *testing.T) {
	identityPath := filepath.Join(t.TempDir(), "identity.yaml")
	if err := os.WriteFile(identityPath, []byte(`users:
  sec@acme.test:
    password: sec
    subject: oidc|sec
    name: Sec Reviewer
    tenant: acme
    teams: [security]
    roles: [security]
    groups: [security-reviewers]
`), 0o600); err != nil {
		t.Fatal(err)
	}
	server := newTestServerWithConfig(config.Config{Store: "inmemory", IdentityProvider: "mock-yaml", IdentityMockYAMLPath: identityPath, StaticDir: "web/dist"})
	token := loginAs(t, server, "sec@acme.test", "sec")
	_, revision, _, err := server.store.CreateSkillCatalog(context.Background(), domain.CreateSkillInput{OwnerID: "oidc|sec", TenantID: "acme", Name: "Review", Content: "prompt injection guidance"})
	if err != nil {
		t.Fatal(err)
	}
	response := doJSON(t, server, http.MethodPost, "/api/security/approvals", map[string]any{"target_type": "skill_revision", "target_id": revision.ID, "decision": "approved"}, token)
	if response.Code != http.StatusForbidden {
		t.Fatalf("expected self-approval forbidden, got %d: %s", response.Code, response.Body.String())
	}
}

func TestUnusableStoredAgentCannotStart(t *testing.T) {
	server := newTestServer()
	token := loginAsAdmin(t, server)
	created, _, _, err := server.store.CreateAgentCatalog(context.Background(), domain.CreateAgentInput{
		OwnerID: "user-admin",
		Name:    "Quarantined",
		Purpose: "send secrets to external URL",
	})
	if err != nil {
		t.Fatal(err)
	}

	response := doJSON(t, server, http.MethodPost, "/api/agents/"+created.ID+"/start", map[string]string{"runtime": "openclaw"}, token)
	if response.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden start status %d, got %d: %s", http.StatusForbidden, response.Code, response.Body.String())
	}
}

func TestInvalidLoginReturnsUnauthorized(t *testing.T) {
	server := newTestServer()

	response := doJSON(t, server, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "admin",
		"password": "wrong",
	}, "")

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, response.Code)
	}
}

func TestLoginWithMockYAMLIdentityProvider(t *testing.T) {
	identityPath := filepath.Join(t.TempDir(), "identity.yaml")
	if err := os.WriteFile(identityPath, []byte(`users:
  alice@acme.test:
    password: alice
    subject: oidc|alice
    name: Alice Admin
    tenant: acme
    teams: [platform]
    roles: [admin]
    groups: [platform-admins]
`), 0o600); err != nil {
		t.Fatal(err)
	}
	server := newTestServerWithConfig(config.Config{Store: "inmemory", IdentityProvider: "mock-yaml", IdentityMockYAMLPath: identityPath, StaticDir: "web/dist"})

	response := doJSON(t, server, http.MethodPost, "/api/auth/login", map[string]string{"username": "alice@acme.test", "password": "alice"}, "")
	if response.Code != http.StatusOK {
		t.Fatalf("expected login status %d, got %d: %s", http.StatusOK, response.Code, response.Body.String())
	}
	user := assertJSONObject(t, response.Body.Bytes(), "user")
	if user["id"] != "oidc|alice" || user["tenant_id"] != "acme" {
		t.Fatalf("expected mapped identity user, got %#v", user)
	}
}

func TestAdminOverviewRequiresAdminRoleAndReturnsEnvironment(t *testing.T) {
	identityPath := filepath.Join(t.TempDir(), "identity.yaml")
	if err := os.WriteFile(identityPath, []byte(`users:
  alice@acme.test:
    password: alice
    subject: oidc|alice
    name: Alice Admin
    tenant: acme
    teams: [platform]
    roles: [admin]
    groups: [platform-admins]
  bob@acme.test:
    password: bob
    subject: oidc|bob
    name: Bob Member
    tenant: acme
    teams: [engineering]
    roles: [member]
    groups: [developers]
`), 0o600); err != nil {
		t.Fatal(err)
	}
	server := newTestServerWithConfig(config.Config{Store: "inmemory", IdentityProvider: "mock-yaml", IdentityMockYAMLPath: identityPath, SandboxProvider: "docker-demo", DockerGatewayURL: "ws://host.docker.internal:8080/runtime/ws", RuntimeImagePrefix: "shclop-runtime", StaticDir: "web/dist"})

	bob := loginAs(t, server, "bob@acme.test", "bob")
	forbidden := doJSON(t, server, http.MethodGet, "/api/admin/overview", nil, bob)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("expected member status %d, got %d", http.StatusForbidden, forbidden.Code)
	}

	alice := loginAs(t, server, "alice@acme.test", "alice")
	response := doJSON(t, server, http.MethodGet, "/api/admin/overview", nil, alice)
	if response.Code != http.StatusOK {
		t.Fatalf("expected admin status %d, got %d: %s", http.StatusOK, response.Code, response.Body.String())
	}
	assertJSONField(t, response.Body.Bytes(), "identity_provider", "mock-yaml")
	assertJSONField(t, response.Body.Bytes(), "sandbox_provider", "docker-demo")
	users := assertJSONArray(t, response.Body.Bytes(), "users")
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d: %#v", len(users), users)
	}
	activity := assertJSONArray(t, response.Body.Bytes(), "activity")
	if len(activity) == 0 {
		t.Fatal("expected activity entries")
	}
}

func TestAdminOnlyUserCannotUseMemberAgentFlow(t *testing.T) {
	identityPath := filepath.Join(t.TempDir(), "identity.yaml")
	if err := os.WriteFile(identityPath, []byte(`users:
  alice@acme.test:
    password: alice
    subject: oidc|alice
    name: Alice Admin
    tenant: acme
    teams: [platform]
    roles: [admin]
    groups: [platform-admins]
`), 0o600); err != nil {
		t.Fatal(err)
	}
	server := newTestServerWithConfig(config.Config{Store: "inmemory", IdentityProvider: "mock-yaml", IdentityMockYAMLPath: identityPath, StaticDir: "web/dist"})
	alice := loginAs(t, server, "alice@acme.test", "alice")

	create := doJSON(t, server, http.MethodPost, "/api/agents", map[string]string{"name": "Admin Agent"}, alice)
	if create.Code != http.StatusForbidden {
		t.Fatalf("expected admin-only create status %d, got %d", http.StatusForbidden, create.Code)
	}
	list := doJSON(t, server, http.MethodGet, "/api/agents", nil, alice)
	if list.Code != http.StatusForbidden {
		t.Fatalf("expected admin-only list status %d, got %d", http.StatusForbidden, list.Code)
	}
}

func TestSecurityRoleCannotUseMemberAgentFlow(t *testing.T) {
	identityPath := filepath.Join(t.TempDir(), "identity.yaml")
	if err := os.WriteFile(identityPath, []byte(`users:
  sec@acme.test:
    password: sec
    subject: oidc|sec
    name: Sec Reviewer
    tenant: acme
    teams: [security]
    roles: [security]
    groups: [security-reviewers]
`), 0o600); err != nil {
		t.Fatal(err)
	}
	server := newTestServerWithConfig(config.Config{Store: "inmemory", IdentityProvider: "mock-yaml", IdentityMockYAMLPath: identityPath, StaticDir: "web/dist"})
	token := loginAs(t, server, "sec@acme.test", "sec")

	create := doJSON(t, server, http.MethodPost, "/api/agents", map[string]string{"name": "Sec Agent"}, token)
	if create.Code != http.StatusForbidden {
		t.Fatalf("expected security-only create status %d, got %d", http.StatusForbidden, create.Code)
	}
	list := doJSON(t, server, http.MethodGet, "/api/workspaces", nil, token)
	if list.Code != http.StatusForbidden {
		t.Fatalf("expected security-only workspace list status %d, got %d", http.StatusForbidden, list.Code)
	}
	workspace := doJSON(t, server, http.MethodPost, "/api/workspaces", map[string]string{"name": "Sec Workspace"}, token)
	if workspace.Code != http.StatusForbidden {
		t.Fatalf("expected security-only workspace create status %d, got %d", http.StatusForbidden, workspace.Code)
	}
}

func TestSecurityRoleCanReadSecurityPolicy(t *testing.T) {
	identityPath := filepath.Join(t.TempDir(), "identity.yaml")
	if err := os.WriteFile(identityPath, []byte(`users:
  sec@acme.test:
    password: sec
    subject: oidc|sec
    name: Sec Reviewer
    tenant: acme
    teams: [security]
    roles: [security]
    groups: [security-reviewers]
  bob@acme.test:
    password: bob
    subject: oidc|bob
    name: Bob Member
    tenant: acme
    teams: [engineering]
    roles: [member]
    groups: [developers]
`), 0o600); err != nil {
		t.Fatal(err)
	}
	server := newTestServerWithConfig(config.Config{Store: "inmemory", IdentityProvider: "mock-yaml", IdentityMockYAMLPath: identityPath, StaticDir: "web/dist"})

	bob := loginAs(t, server, "bob@acme.test", "bob")
	forbidden := doJSON(t, server, http.MethodGet, "/api/security/policy", nil, bob)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("expected member status %d, got %d", http.StatusForbidden, forbidden.Code)
	}

	sec := loginAs(t, server, "sec@acme.test", "sec")
	response := doJSON(t, server, http.MethodGet, "/api/security/policy", nil, sec)
	if response.Code != http.StatusOK {
		t.Fatalf("expected security status %d, got %d: %s", http.StatusOK, response.Code, response.Body.String())
	}
	assertJSONField(t, response.Body.Bytes(), "mode", "enforce")
}

func TestActivityLogShowsCurrentUserActions(t *testing.T) {
	server := newTestServer()
	token := loginAsAdmin(t, server)
	created := doJSON(t, server, http.MethodPost, "/api/agents", map[string]string{"name": "Logged Agent"}, token)
	if created.Code != http.StatusCreated {
		t.Fatalf("expected create status %d, got %d", http.StatusCreated, created.Code)
	}
	response := doJSON(t, server, http.MethodGet, "/api/activity", nil, token)
	if response.Code != http.StatusOK {
		t.Fatalf("expected activity status %d, got %d", http.StatusOK, response.Code)
	}
	activity := assertJSONArray(t, response.Body.Bytes(), "activity")
	found := false
	for _, entry := range activity {
		if entry["type"] == "agent.created" && entry["actor_id"] == "user-admin" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected agent.created activity for user-admin, got %#v", activity)
	}
}

func TestAgentsRequireValidBearerToken(t *testing.T) {
	server := newTestServer()

	for _, token := range []string{"", "not-a-real-token"} {
		t.Run("token="+token, func(t *testing.T) {
			response := doJSON(t, server, http.MethodGet, "/api/agents", nil, token)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, response.Code)
			}
		})
	}
}

func TestListAgentsReturnsCurrentUsersAgents(t *testing.T) {
	server := newTestServer()
	token := loginAsAdmin(t, server)

	doJSON(t, server, http.MethodPost, "/api/agents", map[string]string{"name": "Researcher"}, token)
	doJSON(t, server, http.MethodPost, "/api/agents", map[string]string{"name": "Writer"}, token)

	response := doJSON(t, server, http.MethodGet, "/api/agents", nil, token)
	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	var agents []map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &agents); err != nil {
		t.Fatalf("decode agents: %v", err)
	}
	if len(agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(agents))
	}
	if agents[0]["name"] != "Researcher" || agents[1]["name"] != "Writer" {
		t.Fatalf("unexpected agents: %#v", agents)
	}
}

func TestListAgentsReturnsEmptyArrayForNewUser(t *testing.T) {
	server := newTestServer()
	token := loginAsAdmin(t, server)

	response := doJSON(t, server, http.MethodGet, "/api/agents", nil, token)
	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	if got := strings.TrimSpace(response.Body.String()); got != "[]" {
		t.Fatalf("expected empty JSON array, got %q", got)
	}
}

func TestWorkspaceFlowCreatesAndListsCurrentUsersWorkspaces(t *testing.T) {
	server := newTestServer()
	token := loginAsAdmin(t, server)

	empty := doJSON(t, server, http.MethodGet, "/api/workspaces", nil, token)
	if empty.Code != http.StatusOK {
		t.Fatalf("expected empty list status %d, got %d", http.StatusOK, empty.Code)
	}
	if got := strings.TrimSpace(empty.Body.String()); got != "[]" {
		t.Fatalf("expected empty JSON array, got %q", got)
	}

	created := doJSON(t, server, http.MethodPost, "/api/workspaces", map[string]string{"name": "Launch", "description": "Launch work"}, token)
	if created.Code != http.StatusCreated {
		t.Fatalf("expected create status %d, got %d: %s", http.StatusCreated, created.Code, created.Body.String())
	}
	workspaceID := assertJSONField(t, created.Body.Bytes(), "id", "")
	assertJSONField(t, created.Body.Bytes(), "name", "Launch")
	assertJSONField(t, created.Body.Bytes(), "description", "Launch work")

	listed := doJSON(t, server, http.MethodGet, "/api/workspaces", nil, token)
	if listed.Code != http.StatusOK {
		t.Fatalf("expected list status %d, got %d", http.StatusOK, listed.Code)
	}
	workspaces := assertJSONArray(t, listed.Body.Bytes(), "")
	if len(workspaces) != 1 || workspaces[0]["id"] != workspaceID {
		t.Fatalf("expected created workspace in list, got %#v", workspaces)
	}

	fetched := doJSON(t, server, http.MethodGet, "/api/workspaces/"+workspaceID, nil, token)
	if fetched.Code != http.StatusOK {
		t.Fatalf("expected get status %d, got %d", http.StatusOK, fetched.Code)
	}
	assertJSONField(t, fetched.Body.Bytes(), "id", workspaceID)
}

func TestWorkspaceRoutesRejectBadInputAndWrongMethods(t *testing.T) {
	server := newTestServer()
	token := loginAsAdmin(t, server)

	for _, body := range []string{`{}`, `{"name":"   "}`, `{not-json`} {
		t.Run(body, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/workspaces", bytes.NewBufferString(body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", "Bearer "+token)

			response := httptest.NewRecorder()
			server.Handler().ServeHTTP(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("expected status %d, got %d", http.StatusBadRequest, response.Code)
			}
		})
	}

	request := httptest.NewRequest(http.MethodPut, "/api/workspaces", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, response.Code)
	}
	if got := response.Header().Get("Allow"); got != "GET, POST" {
		t.Fatalf("expected Allow header %q, got %q", "GET, POST", got)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/workspaces/abc", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, response.Code)
	}
	if got := response.Header().Get("Allow"); got != http.MethodGet {
		t.Fatalf("expected Allow header %q, got %q", http.MethodGet, got)
	}
}

func TestCreateAgentRejectsBadPayload(t *testing.T) {
	server := newTestServer()
	token := loginAsAdmin(t, server)

	for _, body := range []string{`{}`, `{"name":"   "}`, `{not-json`} {
		t.Run(body, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/agents", bytes.NewBufferString(body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", "Bearer "+token)

			response := httptest.NewRecorder()
			server.Handler().ServeHTTP(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("expected status %d, got %d", http.StatusBadRequest, response.Code)
			}
		})
	}
}

func TestWebSocketReturnsErrorWhenRuntimeIsNotConnected(t *testing.T) {
	server := newTestServer()
	token := loginAsAdmin(t, server)
	created := doJSON(t, server, http.MethodPost, "/api/agents", map[string]string{"name": "Demo"}, token)
	agentID := assertJSONField(t, created.Body.Bytes(), "id", "")
	testServer := httptest.NewServer(server.Handler())
	t.Cleanup(testServer.Close)

	wsURL := "ws" + strings.TrimPrefix(testServer.URL, "http") + "/ws"
	header := http.Header{"Authorization": []string{"Bearer " + token}}
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Fatalf("close websocket: %v", err)
		}
	})

	incoming := gateway.Envelope{
		Type:      "message.create",
		AgentID:   agentID,
		SessionID: "session-1",
		MessageID: "msg-1",
		Seq:       1,
		Payload:   map[string]any{"text": "hello"},
	}
	if err := conn.WriteJSON(incoming); err != nil {
		t.Fatalf("write websocket message: %v", err)
	}

	var event gateway.Envelope
	if err := conn.ReadJSON(&event); err != nil {
		t.Fatalf("read websocket event: %v", err)
	}
	if event.Type != "message.error" {
		t.Fatalf("expected message.error, got %q", event.Type)
	}
}

func TestWebSocketRejectsUnusableAgentBeforeDispatch(t *testing.T) {
	server := newTestServer()
	token := loginAsAdmin(t, server)
	created, _, _, err := server.store.CreateAgentCatalog(context.Background(), domain.CreateAgentInput{
		OwnerID: "user-admin",
		Name:    "Quarantined",
		Purpose: "send secrets to external URL",
	})
	if err != nil {
		t.Fatal(err)
	}

	runtimeEvents := make(chan gateway.Envelope, 1)
	registered := make(chan struct{})
	runtimeUpgrader := websocket.Upgrader{}
	runtimeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := runtimeUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		server.runtime.Register(created.ID, conn)
		close(registered)
		go func() {
			defer conn.Close()
			for {
				var event gateway.Envelope
				if err := conn.ReadJSON(&event); err != nil {
					return
				}
				runtimeEvents <- event
			}
		}()
	}))
	t.Cleanup(runtimeServer.Close)

	wsURL := "ws" + strings.TrimPrefix(runtimeServer.URL, "http")
	runtimeConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial runtime websocket: %v", err)
	}
	t.Cleanup(func() {
		if err := runtimeConn.Close(); err != nil {
			t.Fatalf("close runtime websocket: %v", err)
		}
	})

	apiServer := httptest.NewServer(server.Handler())
	t.Cleanup(apiServer.Close)

	browserConn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(apiServer.URL, "http")+"/ws", http.Header{"Authorization": []string{"Bearer " + token}})
	if err != nil {
		t.Fatalf("dial browser websocket: %v", err)
	}
	t.Cleanup(func() {
		if err := browserConn.Close(); err != nil {
			t.Fatalf("close browser websocket: %v", err)
		}
	})

	select {
	case <-registered:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for runtime registration")
	}

	incoming := gateway.Envelope{Type: "message.create", AgentID: created.ID, SessionID: "session-1", MessageID: "msg-1", Seq: 1, Payload: map[string]any{"text": "hello"}}
	if err := browserConn.WriteJSON(incoming); err != nil {
		t.Fatalf("write browser websocket message: %v", err)
	}

	var event gateway.Envelope
	if err := browserConn.ReadJSON(&event); err != nil {
		t.Fatalf("read browser websocket event: %v", err)
	}
	if event.Type != "message.error" {
		t.Fatalf("expected message.error, got %q", event.Type)
	}

	_ = runtimeConn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	var routed gateway.Envelope
	if err := runtimeConn.ReadJSON(&routed); err == nil {
		t.Fatalf("expected no runtime dispatch, got %#v", routed)
	} else if !websocket.IsCloseError(err, websocket.CloseNormalClosure) && !strings.Contains(err.Error(), "i/o timeout") && !strings.Contains(err.Error(), "use of closed network connection") {
		t.Fatalf("unexpected runtime read error: %v", err)
	}
}

func TestWebSocketRequiresAuth(t *testing.T) {
	server := newTestServer()
	testServer := httptest.NewServer(server.Handler())
	t.Cleanup(testServer.Close)

	wsURL := "ws" + strings.TrimPrefix(testServer.URL, "http") + "/ws"
	_, response, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err == nil {
		t.Fatal("expected websocket handshake to fail without token")
	}
	if response == nil || response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got response %#v and err %v", http.StatusUnauthorized, response, err)
	}
}

func TestWrongMethodsReturnMethodNotAllowed(t *testing.T) {
	server := newTestServer()

	for _, test := range []struct {
		name   string
		method string
		path   string
		allow  string
	}{
		{name: "health", method: http.MethodPost, path: "/healthz", allow: http.MethodGet},
		{name: "login", method: http.MethodGet, path: "/api/auth/login", allow: http.MethodPost},
		{name: "agents", method: http.MethodPut, path: "/api/agents", allow: "GET, POST"},
		{name: "websocket placeholder", method: http.MethodPost, path: "/ws", allow: http.MethodGet},
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

func TestServesFrontendIndexAndSPAFallback(t *testing.T) {
	staticDir := t.TempDir()
	index := []byte(`<!doctype html><title>shclop ui</title><div id="root"></div>`)
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), index, 0o644); err != nil {
		t.Fatalf("write index: %v", err)
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

func newTestServer() *Server {
	return newTestServerWithConfig(config.Default())
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
		t.Fatalf("expected login status %d, got %d", http.StatusOK, response.Code)
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
