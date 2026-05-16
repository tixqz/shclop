package config

import "os"

type Config struct {
	Addr                 string
	Dev                  bool
	MockRuntime          bool
	MockLLM              bool
	MockSecrets          bool
	Store                string
	PostgresDSN          string
	IdentityProvider     string
	IdentityMockYAMLPath string
	SandboxProvider      string
	DockerGatewayURL     string
	RuntimeImagePrefix   string
	LogLevel             string
	Metrics              bool
	StaticDir            string
}

func Default() Config {
	return Config{
		Addr:                 ":8080",
		Store:                "inmemory",
		PostgresDSN:          os.Getenv("SHCLOP_POSTGRES_DSN"),
		IdentityProvider:     env("SHCLOP_IDENTITY_PROVIDER", "local"),
		IdentityMockYAMLPath: env("SHCLOP_IDENTITY_MOCK_YAML", "config/identity.mock.yaml"),
		SandboxProvider:      env("SHCLOP_SANDBOX_PROVIDER", "mock"),
		DockerGatewayURL:     env("SHCLOP_DOCKER_GATEWAY_URL", "ws://host.docker.internal:8080/runtime/ws"),
		RuntimeImagePrefix:   env("SHCLOP_RUNTIME_IMAGE_PREFIX", "shclop-runtime"),
		LogLevel:             "info",
		Metrics:              true,
		StaticDir:            "web/dist",
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
