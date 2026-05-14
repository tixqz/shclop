package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/mipopov/shclop/internal/domain"
)

type Store interface {
	CreateAgent(ctx context.Context, ownerID, name string) (domain.Agent, error)
	ListAgents(ctx context.Context, ownerID string) ([]domain.Agent, error)
}

type Memory struct {
	mu     sync.Mutex
	agents []domain.Agent
}

func NewMemory() *Memory { return &Memory{} }

func (m *Memory) CreateAgent(ctx context.Context, ownerID, name string) (domain.Agent, error) {
	if err := ctx.Err(); err != nil {
		return domain.Agent{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return domain.Agent{}, err
	}
	id, err := newID()
	if err != nil {
		return domain.Agent{}, err
	}
	agent := domain.Agent{ID: id, OwnerID: ownerID, Name: name, State: "idle", CreatedAt: time.Now().UTC()}
	m.agents = append(m.agents, agent)
	return agent, nil
}

func (m *Memory) ListAgents(ctx context.Context, ownerID string) ([]domain.Agent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var out []domain.Agent
	for _, agent := range m.agents {
		if agent.OwnerID == ownerID {
			out = append(out, agent)
		}
	}
	return out, nil
}

func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	if len(b) == 0 {
		return "", errors.New("empty id")
	}
	return hex.EncodeToString(b[:]), nil
}
