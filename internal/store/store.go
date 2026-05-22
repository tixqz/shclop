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
	// Users
	CreateUser(ctx context.Context, username, passwordHash, role string) (domain.User, error)
	GetUser(ctx context.Context, userID string) (domain.User, error)
	GetUserByUsername(ctx context.Context, username string) (domain.User, error)
	ListUsers(ctx context.Context) ([]domain.User, error)
	UpdateUser(ctx context.Context, userID string, disabled *bool, role *string) (domain.User, error)

	// Agents
	CreateAgent(ctx context.Context, ownerUserID, name, runtime, model string) (domain.Agent, error)
	GetAgent(ctx context.Context, agentID string) (domain.Agent, error)
	ListAgents(ctx context.Context, ownerUserID string) ([]domain.Agent, error)
	UpdateAgentState(ctx context.Context, agentID, state string) (domain.Agent, error)
	UpdateAgentError(ctx context.Context, agentID, lastError string) (domain.Agent, error)

	// LLM Models
	CreateLLMModel(ctx context.Context, displayName, providerModel string, enabled bool) (domain.LLMModel, error)
	GetLLMModel(ctx context.Context, modelID string) (domain.LLMModel, error)
	ListLLMModels(ctx context.Context) ([]domain.LLMModel, error)
	UpdateLLMModel(ctx context.Context, modelID string, displayName, providerModel *string, enabled *bool) (domain.LLMModel, error)

	// LLM Gateway
	GetLLMGatewaySettings(ctx context.Context) (domain.LLMGatewaySettings, error)
	UpsertLLMGatewaySettings(ctx context.Context, enabled bool, baseURL, secretName, secretKey string) (domain.LLMGatewaySettings, error)

	// Bootstrap
	BootstrapAdmin(ctx context.Context, username, passwordHash string) error
}

var ErrNotFound = errors.New("not found")
var ErrForbidden = errors.New("forbidden")
var ErrConflict = errors.New("conflict")
var ErrInvalidInput = errors.New("invalid input")

type Memory struct {
	mu             sync.Mutex
	users          []domain.User
	passwordHashes map[string]string // username -> bcrypt hash
	agents         []domain.Agent
	llmModels      []domain.LLMModel
	gatewayEnabled bool
	gatewayBaseURL string
	gatewaySecret  string
	gatewayKey     string
	gatewayUpdated time.Time
}

func NewMemory() *Memory {
	return &Memory{passwordHashes: make(map[string]string)}
}

// --- Users ---

func (m *Memory) CreateUser(ctx context.Context, username, passwordHash, role string) (domain.User, error) {
	if err := ctx.Err(); err != nil {
		return domain.User{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.users {
		if u.Username == username {
			return domain.User{}, ErrConflict
		}
	}
	id, err := newID()
	if err != nil {
		return domain.User{}, err
	}
	now := time.Now().UTC()
	user := domain.User{ID: id, Username: username, Role: role, Disabled: false, CreatedAt: now, UpdatedAt: now}
	m.users = append(m.users, user)
	if passwordHash != "" {
		m.passwordHashes[username] = passwordHash
	}
	return user, nil
}

func (m *Memory) GetUser(ctx context.Context, userID string) (domain.User, error) {
	if err := ctx.Err(); err != nil {
		return domain.User{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.users {
		if u.ID == userID {
			return u, nil
		}
	}
	return domain.User{}, ErrNotFound
}

func (m *Memory) GetUserByUsername(ctx context.Context, username string) (domain.User, error) {
	if err := ctx.Err(); err != nil {
		return domain.User{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.users {
		if u.Username == username {
			return u, nil
		}
	}
	return domain.User{}, ErrNotFound
}

func (m *Memory) ListUsers(ctx context.Context) ([]domain.User, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]domain.User, len(m.users))
	copy(out, m.users)
	return out, nil
}

func (m *Memory) UpdateUser(ctx context.Context, userID string, disabled *bool, role *string) (domain.User, error) {
	if err := ctx.Err(); err != nil {
		return domain.User{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, u := range m.users {
		if u.ID == userID {
			if disabled != nil {
				m.users[i].Disabled = *disabled
			}
			if role != nil {
				m.users[i].Role = *role
			}
			m.users[i].UpdatedAt = time.Now().UTC()
			return m.users[i], nil
		}
	}
	return domain.User{}, ErrNotFound
}

// GetPasswordHash returns the stored bcrypt hash for a username (used by auth service).
func (m *Memory) GetPasswordHash(_ context.Context, username string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	h, ok := m.passwordHashes[username]
	if !ok {
		return "", ErrNotFound
	}
	return h, nil
}

// SetPasswordHash stores a bcrypt hash for a username.
func (m *Memory) SetPasswordHash(_ context.Context, username, hash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.passwordHashes[username] = hash
	return nil
}

// --- Agents ---

func (m *Memory) CreateAgent(ctx context.Context, ownerUserID, name, runtime, model string) (domain.Agent, error) {
	if err := ctx.Err(); err != nil {
		return domain.Agent{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	id, err := newID()
	if err != nil {
		return domain.Agent{}, err
	}
	now := time.Now().UTC()
	agent := domain.Agent{
		ID:          id,
		OwnerUserID: ownerUserID,
		Name:        name,
		Runtime:     runtime,
		Model:       model,
		State:       "idle",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	m.agents = append(m.agents, agent)
	return agent, nil
}

func (m *Memory) GetAgent(ctx context.Context, agentID string) (domain.Agent, error) {
	if err := ctx.Err(); err != nil {
		return domain.Agent{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, a := range m.agents {
		if a.ID == agentID {
			return a, nil
		}
	}
	return domain.Agent{}, ErrNotFound
}

func (m *Memory) ListAgents(ctx context.Context, ownerUserID string) ([]domain.Agent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.Agent
	for _, a := range m.agents {
		if a.OwnerUserID == ownerUserID {
			out = append(out, a)
		}
	}
	return out, nil
}

func (m *Memory) UpdateAgentState(ctx context.Context, agentID, state string) (domain.Agent, error) {
	if err := ctx.Err(); err != nil {
		return domain.Agent{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, a := range m.agents {
		if a.ID == agentID {
			m.agents[i].State = state
			m.agents[i].UpdatedAt = time.Now().UTC()
			return m.agents[i], nil
		}
	}
	return domain.Agent{}, ErrNotFound
}

func (m *Memory) UpdateAgentError(ctx context.Context, agentID, lastError string) (domain.Agent, error) {
	if err := ctx.Err(); err != nil {
		return domain.Agent{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, a := range m.agents {
		if a.ID == agentID {
			m.agents[i].LastError = lastError
			m.agents[i].UpdatedAt = time.Now().UTC()
			return m.agents[i], nil
		}
	}
	return domain.Agent{}, ErrNotFound
}

// --- LLM Models ---

func (m *Memory) CreateLLMModel(ctx context.Context, displayName, providerModel string, enabled bool) (domain.LLMModel, error) {
	if err := ctx.Err(); err != nil {
		return domain.LLMModel{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	id, err := newID()
	if err != nil {
		return domain.LLMModel{}, err
	}
	now := time.Now().UTC()
	model := domain.LLMModel{
		ID:            id,
		DisplayName:   displayName,
		ProviderModel: providerModel,
		Enabled:       enabled,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	m.llmModels = append(m.llmModels, model)
	return model, nil
}

func (m *Memory) GetLLMModel(ctx context.Context, modelID string) (domain.LLMModel, error) {
	if err := ctx.Err(); err != nil {
		return domain.LLMModel{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, model := range m.llmModels {
		if model.ID == modelID {
			return model, nil
		}
	}
	return domain.LLMModel{}, ErrNotFound
}

func (m *Memory) ListLLMModels(ctx context.Context) ([]domain.LLMModel, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]domain.LLMModel, len(m.llmModels))
	copy(out, m.llmModels)
	return out, nil
}

func (m *Memory) UpdateLLMModel(ctx context.Context, modelID string, displayName, providerModel *string, enabled *bool) (domain.LLMModel, error) {
	if err := ctx.Err(); err != nil {
		return domain.LLMModel{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, model := range m.llmModels {
		if model.ID == modelID {
			if displayName != nil {
				m.llmModels[i].DisplayName = *displayName
			}
			if providerModel != nil {
				m.llmModels[i].ProviderModel = *providerModel
			}
			if enabled != nil {
				m.llmModels[i].Enabled = *enabled
			}
			m.llmModels[i].UpdatedAt = time.Now().UTC()
			return m.llmModels[i], nil
		}
	}
	return domain.LLMModel{}, ErrNotFound
}

// --- LLM Gateway ---

func (m *Memory) GetLLMGatewaySettings(ctx context.Context) (domain.LLMGatewaySettings, error) {
	if err := ctx.Err(); err != nil {
		return domain.LLMGatewaySettings{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return domain.LLMGatewaySettings{
		Enabled:    m.gatewayEnabled,
		BaseURL:    m.gatewayBaseURL,
		SecretName: m.gatewaySecret,
		SecretKey:  m.gatewayKey,
		UpdatedAt:  m.gatewayUpdated,
	}, nil
}

func (m *Memory) UpsertLLMGatewaySettings(ctx context.Context, enabled bool, baseURL, secretName, secretKey string) (domain.LLMGatewaySettings, error) {
	if err := ctx.Err(); err != nil {
		return domain.LLMGatewaySettings{}, err
	}
	m.mu.Lock()
	m.gatewayEnabled = enabled
	m.gatewayBaseURL = baseURL
	m.gatewaySecret = secretName
	m.gatewayKey = secretKey
	m.gatewayUpdated = time.Now().UTC()
	s := domain.LLMGatewaySettings{
		Enabled:    m.gatewayEnabled,
		BaseURL:    m.gatewayBaseURL,
		SecretName: m.gatewaySecret,
		SecretKey:  m.gatewayKey,
		UpdatedAt:  m.gatewayUpdated,
	}
	m.mu.Unlock()
	return s, nil
}

// --- Bootstrap ---

func (m *Memory) BootstrapAdmin(ctx context.Context, username, passwordHash string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, u := range m.users {
		if u.Username == username {
			m.users[i].Disabled = false
			m.users[i].Role = "admin"
			m.users[i].UpdatedAt = time.Now().UTC()
			if passwordHash != "" {
				m.passwordHashes[username] = passwordHash
			}
			return nil
		}
	}

	id, err := newID()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	m.users = append(m.users, domain.User{
		ID:        id,
		Username:  username,
		Role:      "admin",
		Disabled:  false,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if passwordHash != "" {
		m.passwordHashes[username] = passwordHash
	}
	return nil
}

func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
