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
	CreateWorkspace(ctx context.Context, ownerID, name, description string) (domain.Workspace, error)
	GetWorkspace(ctx context.Context, workspaceID string) (domain.Workspace, error)
	ListWorkspaces(ctx context.Context, ownerID string) ([]domain.Workspace, error)
	CreateAgent(ctx context.Context, ownerID, name string) (domain.Agent, error)
	GetAgent(ctx context.Context, agentID string) (domain.Agent, error)
	ListAgents(ctx context.Context, ownerID string) ([]domain.Agent, error)
	UpdateAgentState(ctx context.Context, agentID, state string) (domain.Agent, error)
}

var ErrNotFound = errors.New("not found")

type Memory struct {
	mu         sync.Mutex
	workspaces []domain.Workspace
	agents     []domain.Agent
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

func (m *Memory) CreateWorkspace(ctx context.Context, ownerID, name, description string) (domain.Workspace, error) {
	if err := ctx.Err(); err != nil {
		return domain.Workspace{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return domain.Workspace{}, err
	}
	id, err := newID()
	if err != nil {
		return domain.Workspace{}, err
	}
	now := time.Now().UTC()
	workspace := domain.Workspace{ID: id, OwnerID: ownerID, Name: name, Description: description, CreatedAt: now, UpdatedAt: now}
	m.workspaces = append(m.workspaces, workspace)
	return workspace, nil
}

func (m *Memory) ListWorkspaces(ctx context.Context, ownerID string) ([]domain.Workspace, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var out []domain.Workspace
	for _, workspace := range m.workspaces {
		if workspace.OwnerID == ownerID {
			out = append(out, workspace)
		}
	}
	return out, nil
}

func (m *Memory) GetWorkspace(ctx context.Context, workspaceID string) (domain.Workspace, error) {
	if err := ctx.Err(); err != nil {
		return domain.Workspace{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, workspace := range m.workspaces {
		if workspace.ID == workspaceID {
			return workspace, nil
		}
	}
	return domain.Workspace{}, ErrNotFound
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

func (m *Memory) GetAgent(ctx context.Context, agentID string) (domain.Agent, error) {
	if err := ctx.Err(); err != nil {
		return domain.Agent{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, agent := range m.agents {
		if agent.ID == agentID {
			return agent, nil
		}
	}
	return domain.Agent{}, ErrNotFound
}

func (m *Memory) UpdateAgentState(ctx context.Context, agentID, state string) (domain.Agent, error) {
	if err := ctx.Err(); err != nil {
		return domain.Agent{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, agent := range m.agents {
		if agent.ID == agentID {
			m.agents[i].State = state
			return m.agents[i], nil
		}
	}
	return domain.Agent{}, ErrNotFound
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
