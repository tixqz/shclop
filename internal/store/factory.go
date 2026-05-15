package store

import (
	"errors"
	"fmt"
)

type Config struct {
	Backend     string
	PostgresDSN string
}

func Open(cfg Config) (Store, error) {
	switch cfg.Backend {
	case "", "inmemory":
		return NewMemory(), nil
	case "postgres":
		if cfg.PostgresDSN == "" {
			return nil, errors.New("postgres dsn is required")
		}
		return NewPostgres(cfg.PostgresDSN)
	default:
		return nil, fmt.Errorf("unsupported store backend %q", cfg.Backend)
	}
}
