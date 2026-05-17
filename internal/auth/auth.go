package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"

	"github.com/mipopov/shclop/internal/domain"
	"github.com/mipopov/shclop/internal/identity"
)

type Service interface {
	Login(ctx context.Context, username, password string) (domain.User, string, error)
	Resolve(token string) (domain.User, bool)
}

type Memory struct {
	mu       sync.Mutex
	tokens   map[string]domain.User
	provider identity.IdentityProvider
	mapper   identity.OrganizationMapper
}

func NewMemory() *Memory { return &Memory{tokens: map[string]domain.User{}} }

func NewWithIdentity(provider identity.IdentityProvider, mapper identity.OrganizationMapper) *Memory {
	return &Memory{tokens: map[string]domain.User{}, provider: provider, mapper: mapper}
}

func (m *Memory) Login(ctx context.Context, username, password string) (domain.User, string, error) {
	user, err := m.authenticate(ctx, username, password)
	if err != nil {
		return domain.User{}, "", err
	}
	token, err := tokenID()
	if err != nil {
		return domain.User{}, "", err
	}

	m.mu.Lock()
	m.tokens[token] = user
	m.mu.Unlock()

	return user, token, nil
}

func (m *Memory) authenticate(ctx context.Context, username, password string) (domain.User, error) {
	if err := ctx.Err(); err != nil {
		return domain.User{}, err
	}
	if m.provider == nil {
		if username != "admin" || password != "admin" {
			return domain.User{}, errors.New("invalid credentials")
		}
		return domain.User{ID: "user-admin", Username: "admin", Roles: []string{"member"}}, nil
	}
	external, err := m.provider.Authenticate(ctx, identity.AuthRequest{Username: username, Password: password})
	if err != nil {
		return domain.User{}, err
	}
	principal, err := m.mapper.Map(ctx, external)
	if err != nil {
		return domain.User{}, err
	}
	return domain.User{
		ID:          principal.UserID,
		Username:    principal.Email,
		Email:       principal.Email,
		DisplayName: principal.DisplayName,
		TenantID:    principal.TenantID,
		TeamIDs:     append([]string(nil), principal.TeamIDs...),
		Roles:       append([]string(nil), principal.Roles...),
	}, nil
}

func (m *Memory) Resolve(token string) (domain.User, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	user, ok := m.tokens[token]
	return user, ok
}

func tokenID() (string, error) {
	var b [24]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
