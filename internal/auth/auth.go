package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"

	"github.com/mipopov/shclop/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

// PasswordHasher is implemented by stores that can provide password hashes.
type PasswordHasher interface {
	GetPasswordHash(ctx context.Context, username string) (string, error)
	SetPasswordHash(ctx context.Context, username, hash string) error
}

// UserStore is implemented by stores that manage users.
type UserStore interface {
	GetUserByUsername(ctx context.Context, username string) (domain.User, error)
	GetUser(ctx context.Context, userID string) (domain.User, error)
}

type Service struct {
	store  UserStore
	hasher PasswordHasher
	mu     sync.Mutex
	tokens map[string]domain.User
}

func NewService(store UserStore, hasher PasswordHasher) *Service {
	return &Service{
		store:  store,
		hasher: hasher,
		tokens: map[string]domain.User{},
	}
}

func (s *Service) Login(ctx context.Context, username, password string) (domain.User, string, error) {
	user, err := s.store.GetUserByUsername(ctx, username)
	if err != nil {
		return domain.User{}, "", errors.New("invalid credentials")
	}
	if user.Disabled {
		return domain.User{}, "", errors.New("account disabled")
	}

	hash, err := s.hasher.GetPasswordHash(ctx, username)
	if err != nil {
		return domain.User{}, "", errors.New("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return domain.User{}, "", errors.New("invalid credentials")
	}

	token, err := tokenID()
	if err != nil {
		return domain.User{}, "", err
	}

	s.mu.Lock()
	s.tokens[token] = user
	s.mu.Unlock()

	return user, token, nil
}

func (s *Service) Resolve(token string) (domain.User, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	user, ok := s.tokens[token]
	return user, ok
}

func tokenID() (string, error) {
	var b [24]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
