package config

import (
	"os"
	"strings"
)

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

	// Bootstrap admin
	BootstrapAdminUsername string

	// Observability
	GrafanaURL string
}

func Default() Config {
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

		// Bootstrap admin
		BootstrapAdminUsername: env("SHCLOP_BOOTSTRAP_ADMIN_USERNAME", "admin"),

		// Observability
		GrafanaURL: os.Getenv("SHCLOP_GRAFANA_URL"),
	}
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
