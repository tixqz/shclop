package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

type IdPProviderConfig struct {
	Name         string
	DisplayName  string
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURI  string
	Scopes       []string
	EmailClaim   string
	NameClaim    string
	GroupsClaim  string
}

type Config struct {
	Addr                  string
	Dev                   bool
	MockRuntime           bool
	MockLLM               bool
	MockSecrets           bool
	Store                 string
	PostgresDSN           string
	SandboxProvider       string
	DockerGatewayURL      string
	RuntimeImagePrefix    string
	AgentRuntimeClassName string
	RuntimeImages         map[string]string
	KubernetesNamespace   string
	KubernetesGatewayURL  string
	WorkspaceStorageClass string
	WorkspaceSize         string
	WorkspaceRetention    string
	NetworkPolicyEnabled  bool
	NetworkPolicyMode     string
	NetworkPolicyCIDRs    string
	SecretStore           string
	PodReadyTimeout       string
	LogLevel              string
	Metrics               bool
	StaticDir             string

	// LLM Gateway
	LLMGatewayBaseURL    string
	LLMGatewaySecretName string
	LLMGatewaySecretKey  string
	LLMGatewayAPIKey     string

	// Bootstrap admin
	BootstrapAdminUsername string

	// Integrations
	IntegrationEncryptionKey string

	// Plugin system
	PluginDir          string // directory for file-based plugin manifests
	PluginPollInterval string // DB poll interval for DBRegistry
	KubeconfigPath     string // optional path to kubeconfig; empty → in-cluster

	// Observability
	GrafanaURL string

	// OIDC SSO
	IdPProviders  []IdPProviderConfig
	AuthCookieKey string
}

func Default() (Config, error) {
	providers, err := parseIdPProviders()
	if err != nil {
		return Config{}, err
	}
	return Config{
		Addr:                  ":8080",
		Store:                 "inmemory",
		PostgresDSN:           os.Getenv("SHCLOP_POSTGRES_DSN"),
		SandboxProvider:       env("SHCLOP_SANDBOX_PROVIDER", "mock"),
		DockerGatewayURL:      env("SHCLOP_DOCKER_GATEWAY_URL", "ws://host.docker.internal:8080/runtime/ws"),
		RuntimeImagePrefix:    env("SHCLOP_RUNTIME_IMAGE_PREFIX", "shclop-runtime"),
		AgentRuntimeClassName: env("SHCLOP_AGENT_RUNTIME_CLASS", "kata"),
		RuntimeImages: map[string]string{
			"nanoclaw": env("SHCLOP_RUNTIME_IMAGE_NANOCLAW", "shclop-runtime-nanoclaw:latest"),
			"openclaw": env("SHCLOP_RUNTIME_IMAGE_OPENCLAW", "shclop-runtime-openclaw:latest"),
		},
		KubernetesNamespace:   env("SHCLOP_KUBERNETES_NAMESPACE", "default"),
		KubernetesGatewayURL:  env("SHCLOP_KUBERNETES_GATEWAY_URL", "ws://shclop-backend:8080/runtime/ws"),
		WorkspaceStorageClass: os.Getenv("SHCLOP_WORKSPACE_STORAGE_CLASS"),
		WorkspaceSize:         env("SHCLOP_WORKSPACE_SIZE", "10Gi"),
		WorkspaceRetention:    env("SHCLOP_WORKSPACE_RETENTION", "delete"),
		NetworkPolicyEnabled:  envBool("SHCLOP_NETWORK_POLICY_ENABLED", true),
		NetworkPolicyMode:     env("SHCLOP_NETWORK_POLICY_MODE", "restricted"),
		NetworkPolicyCIDRs:    os.Getenv("SHCLOP_NETWORK_POLICY_ALLOWED_CIDRS"),
		SecretStore:           env("SHCLOP_RUNTIME_SECRET_STORE", "kubernetes"),
		PodReadyTimeout:       os.Getenv("SHCLOP_POD_READY_TIMEOUT"),
		LogLevel:              "info",
		Metrics:               true,
		StaticDir:             "web/dist",

		// LLM Gateway
		LLMGatewayBaseURL:    os.Getenv("SHCLOP_LLM_GATEWAY_BASE_URL"),
		LLMGatewaySecretName: os.Getenv("SHCLOP_LLM_GATEWAY_SECRET_NAME"),
		LLMGatewaySecretKey:  os.Getenv("SHCLOP_LLM_GATEWAY_SECRET_KEY"),
		LLMGatewayAPIKey:     os.Getenv("SHCLOP_LLM_GATEWAY_API_KEY"),

		// Bootstrap admin
		BootstrapAdminUsername: env("SHCLOP_BOOTSTRAP_ADMIN_USERNAME", "admin"),

		// Integrations
		IntegrationEncryptionKey: os.Getenv("SHCLOP_INTEGRATION_ENCRYPTION_KEY"),

		// Plugin system
		PluginDir:          env("SHCLOP_PLUGIN_DIR", "/etc/shclop/plugins.d/"),
		PluginPollInterval: env("SHCLOP_PLUGIN_POLL_INTERVAL", "10s"),
		KubeconfigPath:     os.Getenv("KUBECONFIG"),

		// Observability
		GrafanaURL: os.Getenv("SHCLOP_GRAFANA_URL"),

		// OIDC SSO
		IdPProviders:  providers,
		AuthCookieKey: os.Getenv("SHCLOP_AUTH_COOKIE_KEY"),
	}, nil
}

func (c Config) Validate() error {
	if len(c.IdPProviders) > 0 && c.AuthCookieKey == "" && !c.Dev {
		return errors.New("SHCLOP_AUTH_COOKIE_KEY must be set when IdP providers are configured")
	}
	for i, p := range c.IdPProviders {
		if p.Name == "" {
			return fmt.Errorf("idp provider %d: missing NAME", i)
		}
		if p.Issuer == "" {
			return fmt.Errorf("idp provider %d: missing ISSUER", i)
		}
		if p.ClientID == "" {
			return fmt.Errorf("idp provider %d: missing CLIENT_ID", i)
		}
		if p.ClientSecret == "" {
			return fmt.Errorf("idp provider %d: missing CLIENT_SECRET", i)
		}
		if p.RedirectURI == "" {
			return fmt.Errorf("idp provider %d: missing REDIRECT_URI", i)
		}
	}
	return nil
}

func parseIdPProviders() ([]IdPProviderConfig, error) {
	var providers []IdPProviderConfig
	for n := range 32 {
		prefix := fmt.Sprintf("SHCLOP_IDP_PROVIDER_%d", n)
		name := os.Getenv(prefix + "_NAME")
		if name == "" {
			return providers, nil
		}
		issuer := os.Getenv(prefix + "_ISSUER")
		if issuer == "" {
			return nil, fmt.Errorf("idp provider %d: missing ISSUER", n)
		}
		clientID := os.Getenv(prefix + "_CLIENT_ID")
		if clientID == "" {
			return nil, fmt.Errorf("idp provider %d: missing CLIENT_ID", n)
		}
		clientSecret := os.Getenv(prefix + "_CLIENT_SECRET")
		if clientSecret == "" {
			return nil, fmt.Errorf("idp provider %d: missing CLIENT_SECRET", n)
		}
		redirectURI := os.Getenv(prefix + "_REDIRECT_URI")
		if redirectURI == "" {
			return nil, fmt.Errorf("idp provider %d: missing REDIRECT_URI", n)
		}

		displayName := os.Getenv(prefix + "_DISPLAY_NAME")
		if displayName == "" {
			displayName = "Sign in with " + name
		}

		scopes := parseScopes(os.Getenv(prefix + "_SCOPES"))

		emailClaim := env(prefix+"_EMAIL_CLAIM", "email")
		nameClaim := env(prefix+"_NAME_CLAIM", "name")
		groupsClaim := env(prefix+"_GROUPS_CLAIM", "groups")

		providers = append(providers, IdPProviderConfig{
			Name:         name,
			DisplayName:  displayName,
			Issuer:       issuer,
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURI:  redirectURI,
			Scopes:       scopes,
			EmailClaim:   emailClaim,
			NameClaim:    nameClaim,
			GroupsClaim:  groupsClaim,
		})
	}
	return nil, errors.New("idp provider limit of 32 exceeded")
}

func parseScopes(raw string) []string {
	if raw == "" {
		return []string{"openid", "email", "profile"}
	}
	parts := strings.Split(raw, ",")
	var scopes []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			scopes = append(scopes, p)
		}
	}
	if len(scopes) == 0 {
		return []string{"openid", "email", "profile"}
	}
	return scopes
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}
