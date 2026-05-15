package config

import "os"

type Config struct {
	Addr        string
	Dev         bool
	MockRuntime bool
	MockLLM     bool
	MockSecrets bool
	Store       string
	PostgresDSN string
	RuntimeToken string
	LogLevel    string
	Metrics     bool
	StaticDir   string
}

func Default() Config {
	return Config{
		Addr:         ":8080",
		Store:        "inmemory",
		PostgresDSN:  os.Getenv("SHCLOP_POSTGRES_DSN"),
		RuntimeToken: os.Getenv("SHCLOP_RUNTIME_TOKEN"),
		LogLevel:     "info",
		Metrics:      true,
		StaticDir:    "web/dist",
	}
}
