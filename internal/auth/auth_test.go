package auth

import (
	"context"
	"testing"

	"github.com/mipopov/shclop/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

// testStore implements both UserStore and PasswordHasher for testing.
type testStore struct {
	users     map[string]domain.User // keyed by username
	passwords map[string]string      // username -> bcrypt hash
}

func newTestStore() *testStore {
	return &testStore{
		users:     make(map[string]domain.User),
		passwords: make(map[string]string),
	}
}

func (s *testStore) addUser(id, username, password, role string) {
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	s.users[username] = domain.User{
		ID:       id,
		Username: username,
		Role:     role,
	}
	s.passwords[username] = string(hash)
}

func (s *testStore) GetUserByUsername(_ context.Context, username string) (domain.User, error) {
	u, ok := s.users[username]
	if !ok {
		return domain.User{}, errNotFound
	}
	return u, nil
}

func (s *testStore) GetUser(_ context.Context, userID string) (domain.User, error) {
	for _, u := range s.users {
		if u.ID == userID {
			return u, nil
		}
	}
	return domain.User{}, errNotFound
}

func (s *testStore) GetPasswordHash(_ context.Context, username string) (string, error) {
	h, ok := s.passwords[username]
	if !ok {
		return "", errNotFound
	}
	return h, nil
}

func (s *testStore) SetPasswordHash(_ context.Context, username, hash string) error {
	s.passwords[username] = hash
	return nil
}

var errNotFound = &testError{"not found"}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }

func TestLoginSuccess(t *testing.T) {
	store := newTestStore()
	store.addUser("user-1", "alice", "password123", "admin")
	svc := NewService(store, store)

	user, token, err := svc.Login(context.Background(), "alice", "password123")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if user.Username != "alice" {
		t.Fatalf("expected username alice, got %q", user.Username)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	// Resolve token
	resolved, ok := svc.Resolve(token)
	if !ok {
		t.Fatal("expected token to resolve")
	}
	if resolved.ID != user.ID {
		t.Fatalf("expected user ID %q, got %q", user.ID, resolved.ID)
	}
}

func TestLoginInvalidPassword(t *testing.T) {
	store := newTestStore()
	store.addUser("user-1", "alice", "password123", "user")
	svc := NewService(store, store)

	_, _, err := svc.Login(context.Background(), "alice", "wrongpassword")
	if err == nil {
		t.Fatal("expected login to fail")
	}
}

func TestLoginUnknownUser(t *testing.T) {
	store := newTestStore()
	svc := NewService(store, store)

	_, _, err := svc.Login(context.Background(), "unknown", "password")
	if err == nil {
		t.Fatal("expected login to fail")
	}
}

func TestResolveInvalidToken(t *testing.T) {
	store := newTestStore()
	svc := NewService(store, store)

	_, ok := svc.Resolve("invalid-token")
	if ok {
		t.Fatal("expected invalid token to not resolve")
	}
}

func TestLoginDisabledUser(t *testing.T) {
	store := newTestStore()
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	store.users["disabled"] = domain.User{
		ID:       "user-disabled",
		Username: "disabled",
		Role:     "user",
		Disabled: true,
	}
	store.passwords["disabled"] = string(hash)
	svc := NewService(store, store)

	_, _, err := svc.Login(context.Background(), "disabled", "password")
	if err == nil {
		t.Fatal("expected login to fail for disabled user")
	}
}

func TestTokenIsolation(t *testing.T) {
	store := newTestStore()
	store.addUser("user-1", "alice", "pass1", "user")
	store.addUser("user-2", "bob", "pass2", "admin")
	svc := NewService(store, store)

	_, aliceToken, err := svc.Login(context.Background(), "alice", "pass1")
	if err != nil {
		t.Fatal(err)
	}
	_, bobToken, err := svc.Login(context.Background(), "bob", "pass2")
	if err != nil {
		t.Fatal(err)
	}

	aliceUser, ok := svc.Resolve(aliceToken)
	if !ok || aliceUser.Username != "alice" {
		t.Fatalf("expected alice token to resolve to alice")
	}
	bobUser, ok := svc.Resolve(bobToken)
	if !ok || bobUser.Username != "bob" {
		t.Fatalf("expected bob token to resolve to bob")
	}

	// Tokens should not be interchangeable
	if aliceUser.ID == bobUser.ID {
		t.Fatal("expected different users")
	}
}
