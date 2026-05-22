package main

import (
	"flag"
	"log"

	"github.com/mipopov/shclop/internal/api"
	"github.com/mipopov/shclop/internal/config"
	"github.com/mipopov/shclop/internal/logging"
)

func main() {
	cfg := config.Default()
	flag.StringVar(&cfg.Addr, "addr", cfg.Addr, "HTTP listen address")
	flag.BoolVar(&cfg.Dev, "dev", cfg.Dev, "enable dev mode")
	flag.BoolVar(&cfg.MockRuntime, "mock-runtime", cfg.MockRuntime, "enable mock runtime provider")
	flag.BoolVar(&cfg.MockLLM, "mock-llm", cfg.MockLLM, "enable mock LLM provider")
	flag.BoolVar(&cfg.MockSecrets, "mock-secrets", cfg.MockSecrets, "enable mock SecretStore")
	flag.StringVar(&cfg.Store, "store", cfg.Store, "store backend: inmemory or postgres")
	flag.StringVar(&cfg.PostgresDSN, "postgres-dsn", cfg.PostgresDSN, "PostgreSQL DSN (or SHCLOP_POSTGRES_DSN)")
	flag.StringVar(&cfg.SandboxProvider, "sandbox-provider", cfg.SandboxProvider, "sandbox provider: mock, docker-demo, or kubernetes")
	flag.StringVar(&cfg.DockerGatewayURL, "docker-gateway-url", cfg.DockerGatewayURL, "gateway URL passed to docker-demo runtime containers")
	flag.StringVar(&cfg.KubernetesNamespace, "kubernetes-namespace", cfg.KubernetesNamespace, "namespace for Kubernetes runtime sandboxes")
	flag.StringVar(&cfg.KubernetesGatewayURL, "kubernetes-gateway-url", cfg.KubernetesGatewayURL, "gateway websocket URL passed to Kubernetes runtime pods")
	flag.StringVar(&cfg.AgentRuntimeClassName, "agent-runtime-class", cfg.AgentRuntimeClassName, "Kubernetes RuntimeClass for agent runtime pods")
	flag.StringVar(&cfg.WorkspaceStorageClass, "workspace-storage-class", cfg.WorkspaceStorageClass, "storage class for runtime workspace PVCs")
	flag.StringVar(&cfg.WorkspaceSize, "workspace-size", cfg.WorkspaceSize, "runtime workspace PVC size")
	flag.StringVar(&cfg.WorkspaceRetention, "workspace-retention", cfg.WorkspaceRetention, "workspace PVC cleanup policy: delete or retain")
	flag.BoolVar(&cfg.NetworkPolicyEnabled, "network-policy", cfg.NetworkPolicyEnabled, "create NetworkPolicy for Kubernetes runtime pods")
	flag.StringVar(&cfg.NetworkPolicyMode, "network-policy-mode", cfg.NetworkPolicyMode, "NetworkPolicy mode: disabled, restricted, custom")
	flag.StringVar(&cfg.NetworkPolicyCIDRs, "network-policy-allowed-cidrs", cfg.NetworkPolicyCIDRs, "comma-separated CIDRs allowed for custom runtime egress")
	flag.StringVar(&cfg.SecretStore, "runtime-secret-store", cfg.SecretStore, "runtime token secret store: kubernetes")
	flag.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "log level: debug/info/warn/error")
	flag.BoolVar(&cfg.Metrics, "metrics", cfg.Metrics, "enable metrics endpoint")
	flag.StringVar(&cfg.StaticDir, "static-dir", cfg.StaticDir, "frontend static files directory")

	// LLM Gateway
	flag.StringVar(&cfg.LLMGatewayBaseURL, "llm-gateway-base-url", cfg.LLMGatewayBaseURL, "LLM gateway base URL (or SHCLOP_LLM_GATEWAY_BASE_URL)")
	flag.StringVar(&cfg.LLMGatewaySecretName, "llm-gateway-secret-name", cfg.LLMGatewaySecretName, "Kubernetes secret name for LLM gateway API key (or SHCLOP_LLM_GATEWAY_SECRET_NAME)")
	flag.StringVar(&cfg.LLMGatewaySecretKey, "llm-gateway-secret-key", cfg.LLMGatewaySecretKey, "Kubernetes secret key for LLM gateway API key (or SHCLOP_LLM_GATEWAY_SECRET_KEY)")

	// Bootstrap admin
	flag.StringVar(&cfg.BootstrapAdminUsername, "bootstrap-admin-username", cfg.BootstrapAdminUsername, "bootstrap admin username (or SHCLOP_BOOTSTRAP_ADMIN_USERNAME)")

	// Observability
	flag.StringVar(&cfg.GrafanaURL, "observability-grafana-url", cfg.GrafanaURL, "Grafana URL (or SHCLOP_GRAFANA_URL)")

	flag.Parse()

	logger := logging.New(cfg.LogLevel)
	server, err := api.NewServer(cfg, logger)
	if err != nil {
		log.Fatal(err)
	}
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
