package plugins

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"text/template"
	"time"

	"gopkg.in/yaml.v3"
)

type Manifest struct {
	APIVersion string       `yaml:"apiVersion"`
	Kind       string       `yaml:"kind"`
	Metadata   Metadata     `yaml:"metadata"`
	Spec       ManifestSpec `yaml:"spec"`
}

type Metadata struct {
	Name   string `yaml:"name"`
	Source string `yaml:"-"` // populated by registry: "file" | "db" | "crd"
}

type ManifestSpec struct {
	ID              string        `yaml:"id"`
	DisplayName     string        `yaml:"display_name"`
	Description     string        `yaml:"description"`
	PluginKind      string        `yaml:"kind"`
	ScopesSupported []string      `yaml:"scopes_supported"`
	Auth            AuthSpec      `yaml:"auth"`
	Validate        ValidateSpec  `yaml:"validate"`
	Sidecar         *SidecarSpec  `yaml:"sidecar,omitempty"`
	Contributions   Contributions `yaml:"contributions"`
}

type AuthSpec struct {
	Type   string      `yaml:"type"`
	Fields []FormField `yaml:"fields"`
}

type FormField struct {
	Name        string `yaml:"name"`
	Label       string `yaml:"label"`
	Secret      bool   `yaml:"secret"`
	Placeholder string `yaml:"placeholder"`
	HelpURL     string `yaml:"help_url"`
}

type ValidateSpec struct {
	HTTP *HTTPValidate `yaml:"http,omitempty"`
}

type HTTPValidate struct {
	Method        string            `yaml:"method"`
	URL           string            `yaml:"url"`
	Headers       map[string]string `yaml:"headers"`
	SuccessStatus []int             `yaml:"success_status"`
	Extract       ExtractSpec       `yaml:"extract"`
	Timeout       string            `yaml:"timeout"`
}

type ExtractSpec struct {
	ExternalAccountID string `yaml:"external_account_id"`
	ExternalLogin     string `yaml:"external_login"`
	AccountType       string `yaml:"account_type"`
}

type SidecarSpec struct {
	URL     string `yaml:"url"`
	Timeout string `yaml:"timeout"`
}

type Contributions struct {
	Env        map[string]string `yaml:"env"`
	MCPServers []MCPRef          `yaml:"mcp_servers"`
}

type MCPRef struct {
	Name    string            `yaml:"name"`
	Ref     string            `yaml:"ref,omitempty"`
	External *ExternalMCP     `yaml:"external,omitempty"`
	EnvFrom map[string]string `yaml:"env_from"`
}

type ExternalMCP struct {
	URL        string `yaml:"url"`
	AuthHeader string `yaml:"auth_header"`
}

var idRegexp = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

func ParseManifest(data []byte) (*Manifest, error) {
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("yaml parse: %w", err)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

func (m *Manifest) Validate() error {
	if m.APIVersion != "shclop.io/v1alpha1" {
		return fmt.Errorf("apiVersion must be shclop.io/v1alpha1, got %q", m.APIVersion)
	}
	if m.Kind != "IntegrationPlugin" {
		return fmt.Errorf("kind must be IntegrationPlugin, got %q", m.Kind)
	}

	s := &m.Spec

	if s.ID == "" {
		return fmt.Errorf("spec.id is required")
	}
	if !idRegexp.MatchString(s.ID) {
		return fmt.Errorf("spec.id %q must match ^[a-z0-9][a-z0-9-]*$", s.ID)
	}

	if s.PluginKind != "http_template" && s.PluginKind != "sidecar" {
		return fmt.Errorf("spec.kind must be http_template or sidecar, got %q", s.PluginKind)
	}

	// Default before membership check so empty list becomes ["user"].
	if len(s.ScopesSupported) == 0 {
		s.ScopesSupported = []string{"user"}
	}
	for _, sc := range s.ScopesSupported {
		if sc != "user" && sc != "org" {
			return fmt.Errorf("spec.scopes_supported: unknown scope %q", sc)
		}
	}

	if s.Auth.Type != "pat_token" {
		return fmt.Errorf("spec.auth.type must be pat_token, got %q", s.Auth.Type)
	}
	if len(s.Auth.Fields) == 0 {
		return fmt.Errorf("spec.auth.fields must not be empty")
	}
	for i, f := range s.Auth.Fields {
		if f.Name == "" {
			return fmt.Errorf("spec.auth.fields[%d].name is required", i)
		}
	}

	switch s.PluginKind {
	case "http_template":
		if s.Validate.HTTP == nil {
			return fmt.Errorf("spec.validate.http is required for kind http_template")
		}
		h := s.Validate.HTTP
		if h.Method != "GET" && h.Method != "POST" && h.Method != "HEAD" {
			return fmt.Errorf("spec.validate.http.method must be GET, POST, or HEAD, got %q", h.Method)
		}
		if h.URL == "" {
			return fmt.Errorf("spec.validate.http.url is required")
		}
		if len(h.SuccessStatus) == 0 {
			return fmt.Errorf("spec.validate.http.success_status must not be empty")
		}
	case "sidecar":
		if s.Sidecar == nil {
			return fmt.Errorf("spec.sidecar is required for kind sidecar")
		}
		if s.Sidecar.URL == "" {
			return fmt.Errorf("spec.sidecar.url is required")
		}
	}

	for i, ref := range s.Contributions.MCPServers {
		if ref.Name == "" {
			return fmt.Errorf("spec.contributions.mcp_servers[%d].name is required", i)
		}
		hasRef := ref.Ref != ""
		hasExt := ref.External != nil
		if hasRef == hasExt {
			if hasRef {
				return fmt.Errorf("spec.contributions.mcp_servers[%d]: ref and external are mutually exclusive", i)
			}
			return fmt.Errorf("spec.contributions.mcp_servers[%d]: one of ref or external must be set", i)
		}
	}

	if err := m.validateTemplates(); err != nil {
		return err
	}

	return nil
}

func (m *Manifest) validateTemplates() error {
	fns := sandboxedFuncs()
	parse := func(field, tmpl string) error {
		_, err := template.New("").Option("missingkey=error").Funcs(fns).Parse(tmpl)
		if err != nil {
			return fmt.Errorf("%s: %w", field, err)
		}
		return nil
	}

	h := m.Spec.Validate.HTTP
	if h != nil {
		if err := parse("spec.validate.http.url", h.URL); err != nil {
			return err
		}
		for k, v := range h.Headers {
			if err := parse(fmt.Sprintf("spec.validate.http.headers[%s]", k), v); err != nil {
				return err
			}
		}
	}

	for k, v := range m.Spec.Contributions.Env {
		if err := parse(fmt.Sprintf("spec.contributions.env[%s]", k), v); err != nil {
			return err
		}
	}

	for i, ref := range m.Spec.Contributions.MCPServers {
		if ref.External != nil {
			if err := parse(fmt.Sprintf("spec.contributions.mcp_servers[%d].external.url", i), ref.External.URL); err != nil {
				return err
			}
			if err := parse(fmt.Sprintf("spec.contributions.mcp_servers[%d].external.auth_header", i), ref.External.AuthHeader); err != nil {
				return err
			}
		}
	}

	return nil
}

// sandbox: explicitly omit os/exec funcs so manifests cannot escape the process.
func sandboxedFuncs() template.FuncMap {
	return template.FuncMap{
		"lower":    strings.ToLower,
		"upper":    strings.ToUpper,
		"urlquery": url.QueryEscape,
		"trim":     strings.TrimSpace,
		"default": func(val, fallback string) string {
			if val == "" {
				return fallback
			}
			return val
		},
	}
}

func ParseTimeout(s string) (time.Duration, error) {
	if s == "" {
		return 5 * time.Second, nil
	}
	return time.ParseDuration(s)
}
