package plugins

import (
	"strings"
	"testing"
)

const githubManifestYAML = `
apiVersion: shclop.io/v1alpha1
kind: IntegrationPlugin
metadata: { name: github }
spec:
  id: github
  display_name: GitHub
  description: Connect a GitHub PAT to expose gh CLI and MCP tools in agent runtime
  kind: http_template
  scopes_supported: [user]
  auth:
    type: pat_token
    fields:
      - { name: token, label: Personal Access Token, secret: true, placeholder: "ghp_...", help_url: "https://github.com/settings/tokens" }
  validate:
    http:
      method: GET
      url: "https://api.github.com/user"
      headers:
        Authorization: "Bearer {{.Token}}"
        User-Agent: "shclop/1.0"
      success_status: [200]
      extract: { external_account_id: ".id", external_login: ".login", account_type: ".type" }
      timeout: 5s
  contributions:
    env: { GITHUB_TOKEN: "{{.Token}}" }
    mcp_servers: []
`

const sidecarManifestYAML = `
apiVersion: shclop.io/v1alpha1
kind: IntegrationPlugin
metadata: { name: linear }
spec:
  id: linear
  display_name: Linear
  description: Connect Linear via a sidecar proxy
  kind: sidecar
  scopes_supported: [user]
  auth:
    type: pat_token
    fields:
      - { name: token, label: API Key, secret: true, placeholder: "lin_api_..." }
  sidecar:
    url: "http://linear-sidecar.svc.cluster.local:8080"
    timeout: 10s
  contributions:
    env: { LINEAR_TOKEN: "{{.Token}}" }
    mcp_servers: []
`

func TestParseManifest_GitHub(t *testing.T) {
	m, err := ParseManifest([]byte(githubManifestYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Metadata.Name != "github" {
		t.Errorf("expected metadata.name=github, got %q", m.Metadata.Name)
	}
	if m.Spec.ID != "github" {
		t.Errorf("expected spec.id=github, got %q", m.Spec.ID)
	}
	if m.Spec.PluginKind != "http_template" {
		t.Errorf("expected spec.kind=http_template, got %q", m.Spec.PluginKind)
	}
	if m.Spec.DisplayName != "GitHub" {
		t.Errorf("expected display_name=GitHub, got %q", m.Spec.DisplayName)
	}
	if len(m.Spec.ScopesSupported) != 1 || m.Spec.ScopesSupported[0] != "user" {
		t.Errorf("expected scopes_supported=[user], got %v", m.Spec.ScopesSupported)
	}
	if m.Spec.Auth.Type != "pat_token" {
		t.Errorf("expected auth.type=pat_token, got %q", m.Spec.Auth.Type)
	}
	if len(m.Spec.Auth.Fields) != 1 {
		t.Fatalf("expected 1 auth field, got %d", len(m.Spec.Auth.Fields))
	}
	f := m.Spec.Auth.Fields[0]
	if f.Name != "token" {
		t.Errorf("expected field name=token, got %q", f.Name)
	}
	if !f.Secret {
		t.Error("expected field secret=true")
	}
	if m.Spec.Validate.HTTP == nil {
		t.Fatal("expected validate.http to be set")
	}
	h := m.Spec.Validate.HTTP
	if h.Method != "GET" {
		t.Errorf("expected method=GET, got %q", h.Method)
	}
	if h.URL != "https://api.github.com/user" {
		t.Errorf("unexpected URL: %q", h.URL)
	}
	if len(h.SuccessStatus) != 1 || h.SuccessStatus[0] != 200 {
		t.Errorf("expected success_status=[200], got %v", h.SuccessStatus)
	}
	if h.Extract.ExternalAccountID != ".id" {
		t.Errorf("expected extract.external_account_id=.id, got %q", h.Extract.ExternalAccountID)
	}
	if h.Extract.ExternalLogin != ".login" {
		t.Errorf("expected extract.external_login=.login, got %q", h.Extract.ExternalLogin)
	}
	if h.Timeout != "5s" {
		t.Errorf("expected timeout=5s, got %q", h.Timeout)
	}
	envVal, ok := m.Spec.Contributions.Env["GITHUB_TOKEN"]
	if !ok {
		t.Error("expected contributions.env[GITHUB_TOKEN] to be set")
	}
	if envVal != "{{.Token}}" {
		t.Errorf("expected GITHUB_TOKEN={{.Token}}, got %q", envVal)
	}
}

func TestParseManifest_Sidecar(t *testing.T) {
	m, err := ParseManifest([]byte(sidecarManifestYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Spec.PluginKind != "sidecar" {
		t.Errorf("expected spec.kind=sidecar, got %q", m.Spec.PluginKind)
	}
	if m.Spec.Sidecar == nil {
		t.Fatal("expected sidecar spec to be set")
	}
	if m.Spec.Sidecar.URL != "http://linear-sidecar.svc.cluster.local:8080" {
		t.Errorf("unexpected sidecar URL: %q", m.Spec.Sidecar.URL)
	}
	if m.Spec.Sidecar.Timeout != "10s" {
		t.Errorf("expected timeout=10s, got %q", m.Spec.Sidecar.Timeout)
	}
}

func TestParseManifest_ScopesDefaulting(t *testing.T) {
	yaml := `
apiVersion: shclop.io/v1alpha1
kind: IntegrationPlugin
metadata: { name: test }
spec:
  id: test-plugin
  kind: http_template
  auth:
    type: pat_token
    fields:
      - { name: token }
  validate:
    http:
      method: GET
      url: "https://example.com"
      success_status: [200]
  contributions:
    mcp_servers: []
`
	m, err := ParseManifest([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m.Spec.ScopesSupported) != 1 || m.Spec.ScopesSupported[0] != "user" {
		t.Errorf("expected defaulted scopes_supported=[user], got %v", m.Spec.ScopesSupported)
	}
}

type rejectCase struct {
	name        string
	yaml        string
	errContains string
}

func TestParseManifest_Reject(t *testing.T) {
	cases := []rejectCase{
		{
			name: "missing apiVersion",
			yaml: `
kind: IntegrationPlugin
metadata: { name: x }
spec:
  id: x
  kind: http_template
  auth:
    type: pat_token
    fields: [{name: token}]
  validate:
    http: {method: GET, url: "https://example.com", success_status: [200]}
  contributions: {mcp_servers: []}
`,
			errContains: "apiVersion",
		},
		{
			name: "wrong kind value",
			yaml: `
apiVersion: shclop.io/v1alpha1
kind: WrongKind
metadata: { name: x }
spec:
  id: x
  kind: http_template
  auth:
    type: pat_token
    fields: [{name: token}]
  validate:
    http: {method: GET, url: "https://example.com", success_status: [200]}
  contributions: {mcp_servers: []}
`,
			errContains: "kind",
		},
		{
			name: "unknown PluginKind",
			yaml: `
apiVersion: shclop.io/v1alpha1
kind: IntegrationPlugin
metadata: { name: x }
spec:
  id: x
  kind: oauth2
  auth:
    type: pat_token
    fields: [{name: token}]
  contributions: {mcp_servers: []}
`,
			errContains: "spec.kind",
		},
		{
			name: "empty ID",
			yaml: `
apiVersion: shclop.io/v1alpha1
kind: IntegrationPlugin
metadata: { name: x }
spec:
  id: ""
  kind: http_template
  auth:
    type: pat_token
    fields: [{name: token}]
  validate:
    http: {method: GET, url: "https://example.com", success_status: [200]}
  contributions: {mcp_servers: []}
`,
			errContains: "spec.id",
		},
		{
			name: "ID with uppercase",
			yaml: `
apiVersion: shclop.io/v1alpha1
kind: IntegrationPlugin
metadata: { name: x }
spec:
  id: MyPlugin
  kind: http_template
  auth:
    type: pat_token
    fields: [{name: token}]
  validate:
    http: {method: GET, url: "https://example.com", success_status: [200]}
  contributions: {mcp_servers: []}
`,
			errContains: "spec.id",
		},
		{
			name: "ID with spaces",
			yaml: `
apiVersion: shclop.io/v1alpha1
kind: IntegrationPlugin
metadata: { name: x }
spec:
  id: "my plugin"
  kind: http_template
  auth:
    type: pat_token
    fields: [{name: token}]
  validate:
    http: {method: GET, url: "https://example.com", success_status: [200]}
  contributions: {mcp_servers: []}
`,
			errContains: "spec.id",
		},
		{
			name: "missing http spec when kind=http_template",
			yaml: `
apiVersion: shclop.io/v1alpha1
kind: IntegrationPlugin
metadata: { name: x }
spec:
  id: x
  kind: http_template
  auth:
    type: pat_token
    fields: [{name: token}]
  validate: {}
  contributions: {mcp_servers: []}
`,
			errContains: "spec.validate.http",
		},
		{
			name: "missing sidecar spec when kind=sidecar",
			yaml: `
apiVersion: shclop.io/v1alpha1
kind: IntegrationPlugin
metadata: { name: x }
spec:
  id: x
  kind: sidecar
  auth:
    type: pat_token
    fields: [{name: token}]
  contributions: {mcp_servers: []}
`,
			errContains: "spec.sidecar",
		},
		{
			name: "missing fields list",
			yaml: `
apiVersion: shclop.io/v1alpha1
kind: IntegrationPlugin
metadata: { name: x }
spec:
  id: x
  kind: http_template
  auth:
    type: pat_token
    fields: []
  validate:
    http: {method: GET, url: "https://example.com", success_status: [200]}
  contributions: {mcp_servers: []}
`,
			errContains: "spec.auth.fields",
		},
		{
			name: "MCPRef with both ref and external set",
			yaml: `
apiVersion: shclop.io/v1alpha1
kind: IntegrationPlugin
metadata: { name: x }
spec:
  id: x
  kind: http_template
  auth:
    type: pat_token
    fields: [{name: token}]
  validate:
    http: {method: GET, url: "https://example.com", success_status: [200]}
  contributions:
    mcp_servers:
      - name: my-mcp
        ref: some-crd
        external:
          url: "https://external.mcp.example.com"
          auth_header: "Bearer token"
`,
			errContains: "mcp_servers[0]",
		},
		{
			name: "MCPRef with neither ref nor external",
			yaml: `
apiVersion: shclop.io/v1alpha1
kind: IntegrationPlugin
metadata: { name: x }
spec:
  id: x
  kind: http_template
  auth:
    type: pat_token
    fields: [{name: token}]
  validate:
    http: {method: GET, url: "https://example.com", success_status: [200]}
  contributions:
    mcp_servers:
      - name: my-mcp
`,
			errContains: "mcp_servers[0]",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseManifest([]byte(tc.yaml))
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.errContains)
			}
			if !strings.Contains(err.Error(), tc.errContains) {
				t.Errorf("expected error to contain %q, got: %v", tc.errContains, err)
			}
		})
	}
}

func TestParseManifest_SandboxRejectsExec(t *testing.T) {
	yaml := `
apiVersion: shclop.io/v1alpha1
kind: IntegrationPlugin
metadata: { name: x }
spec:
  id: x
  kind: http_template
  auth:
    type: pat_token
    fields: [{name: token}]
  validate:
    http:
      method: GET
      url: '{{exec "rm" "-rf" "/"}}'
      success_status: [200]
  contributions: {mcp_servers: []}
`
	_, err := ParseManifest([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for exec in template, got nil")
	}
	if !strings.Contains(err.Error(), "spec.validate.http.url") {
		t.Errorf("expected error to mention field name, got: %v", err)
	}
}

func TestParseManifest_SandboxRejectsEnv(t *testing.T) {
	yaml := `
apiVersion: shclop.io/v1alpha1
kind: IntegrationPlugin
metadata: { name: x }
spec:
  id: x
  kind: http_template
  auth:
    type: pat_token
    fields: [{name: token}]
  validate:
    http:
      method: GET
      url: "https://example.com"
      headers:
        Authorization: '{{env "SECRET"}}'
      success_status: [200]
  contributions: {mcp_servers: []}
`
	_, err := ParseManifest([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for env in template, got nil")
	}
	if !strings.Contains(err.Error(), "spec.validate.http.headers") {
		t.Errorf("expected error to mention field name, got: %v", err)
	}
}

func TestParseTimeout(t *testing.T) {
	d, err := ParseTimeout("10s")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Seconds() != 10 {
		t.Errorf("expected 10s, got %v", d)
	}

	d, err = ParseTimeout("")
	if err != nil {
		t.Fatalf("unexpected error for empty: %v", err)
	}
	if d.Seconds() != 5 {
		t.Errorf("expected default 5s, got %v", d)
	}

	_, err = ParseTimeout("notaduration")
	if err == nil {
		t.Error("expected error for invalid duration")
	}
}
