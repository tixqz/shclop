package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"

	"github.com/mipopov/shclop/internal/domain"
)

type Service interface {
	Login(username, password string) (domain.User, string, error)
	Resolve(token string) (domain.User, bool)
}

type Memory struct {
	mu     sync.Mutex
	tokens map[string]domain.User
}

func NewMemory() *Memory { return &Memory{tokens: map[string]domain.User{}} }

func (m *Memory) Login(username, password string) (domain.User, string, error) {
	if username != "admin" || password != "admin" {
		return domain.User{}, "", errors.New("invalid credentials")
	}

	user := domain.User{ID: "user-admin", Username: "admin"}
	token, err := tokenID()
	if err != nil {
		return domain.User{}, "", err
	}

	m.mu.Lock()
	m.tokens[token] = user
	m.mu.Unlock()

	return user, token, nil
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
