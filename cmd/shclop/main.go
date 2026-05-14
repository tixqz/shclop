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
	flag.StringVar(&cfg.Store, "store", cfg.Store, "store backend: inmemory")
	flag.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "log level: debug/info/warn/error")
	flag.BoolVar(&cfg.Metrics, "metrics", cfg.Metrics, "enable metrics endpoint")
	flag.Parse()

	logger := logging.New(cfg.LogLevel)
	if err := api.NewServer(cfg, logger).ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
