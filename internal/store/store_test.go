package store

import (
	"context"
	"testing"
)

func TestMemoryStoreCreatesAndListsUsers(t *testing.T) {
	s := NewMemory()
	ctx := context.Background()

	u1, err := s.CreateUser(ctx, "alice", "hash1", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if u1.ID == "" {
		t.Fatal("expected user ID")
	}
	if u1.Username != "alice" {
		t.Fatalf("expected username alice, got %q", u1.Username)
	}
	if u1.Role != "admin" {
		t.Fatalf("expected role admin, got %q", u1.Role)
	}
	if u1.Disabled {
		t.Fatal("expected user not disabled")
	}

	u2, err := s.CreateUser(ctx, "bob", "hash2", "user")
	if err != nil {
		t.Fatal(err)
	}
	if u2.Username != "bob" || u2.Role != "user" {
		t.Fatalf("unexpected user: %#v", u2)
	}

	// Duplicate username
	_, err = s.CreateUser(ctx, "alice", "hash3", "admin")
	if err != ErrConflict {
		t.Fatalf("expected ErrConflict, got %v", err)
	}

	// List
	users, err := s.ListUsers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}

	// Get by username
	got, err := s.GetUserByUsername(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != u1.ID {
		t.Fatalf("expected user %q, got %q", u1.ID, got.ID)
	}

	// Get by ID
	got, err = s.GetUser(ctx, u1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Username != "alice" {
		t.Fatalf("expected username alice, got %q", got.Username)
	}
}

func TestMemoryStoreUpdateUser(t *testing.T) {
	s := NewMemory()
	ctx := context.Background()

	u, err := s.CreateUser(ctx, "alice", "hash", "user")
	if err != nil {
		t.Fatal(err)
	}

	disabled := true
	updated, err := s.UpdateUser(ctx, u.ID, &disabled, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !updated.Disabled {
		t.Fatal("expected user to be disabled")
	}

	role := "admin"
	updated, err = s.UpdateUser(ctx, u.ID, nil, &role)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Role != "admin" {
		t.Fatalf("expected role admin, got %q", updated.Role)
	}
	if !updated.Disabled {
		t.Fatal("expected user to still be disabled")
	}

	// NotFound
	_, err = s.UpdateUser(ctx, "nonexistent", nil, nil)
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMemoryStoreCreatesAndListsAgents(t *testing.T) {
	s := NewMemory()
	ctx := context.Background()

	// First create a user
	user, err := s.CreateUser(ctx, "testuser", "hash", "user")
	if err != nil {
		t.Fatal(err)
	}

	a1, err := s.CreateAgent(ctx, user.ID, "Researcher", "nanoclaw", "gpt-4")
	if err != nil {
		t.Fatal(err)
	}
	if a1.ID == "" {
		t.Fatal("expected agent ID")
	}
	if a1.State != "idle" {
		t.Fatalf("unexpected state %q", a1.State)
	}
	if a1.OwnerUserID != user.ID {
		t.Fatalf("unexpected owner %q", a1.OwnerUserID)
	}
	if a1.Runtime != "nanoclaw" {
		t.Fatalf("unexpected runtime %q", a1.Runtime)
	}

	a2, err := s.CreateAgent(ctx, user.ID, "Builder", "openclaw", "claude-3")
	if err != nil {
		t.Fatal(err)
	}
	if a2.OwnerUserID != user.ID {
		t.Fatalf("unexpected owner %q", a2.OwnerUserID)
	}

	agents, err := s.ListAgents(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(agents))
	}

	// Test isolation
	otherUser, err := s.CreateUser(ctx, "other", "hash", "user")
	if err != nil {
		t.Fatal(err)
	}
	otherAgents, err := s.ListAgents(ctx, otherUser.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(otherAgents) != 0 {
		t.Fatalf("expected 0 agents for other user, got %d", len(otherAgents))
	}
}

func TestMemoryStoreUpdateAgentState(t *testing.T) {
	s := NewMemory()
	ctx := context.Background()

	user, err := s.CreateUser(ctx, "testuser", "hash", "user")
	if err != nil {
		t.Fatal(err)
	}

	a, err := s.CreateAgent(ctx, user.ID, "Test", "nanoclaw", "gpt-4")
	if err != nil {
		t.Fatal(err)
	}

	updated, err := s.UpdateAgentState(ctx, a.ID, "running")
	if err != nil {
		t.Fatal(err)
	}
	if updated.State != "running" {
		t.Fatalf("expected state running, got %q", updated.State)
	}

	updated, err = s.UpdateAgentError(ctx, a.ID, "something went wrong")
	if err != nil {
		t.Fatal(err)
	}
	if updated.LastError != "something went wrong" {
		t.Fatalf("expected last_error, got %q", updated.LastError)
	}

	// Verify persistence
	fetched, err := s.GetAgent(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if fetched.State != "running" {
		t.Fatalf("expected state running, got %q", fetched.State)
	}
	if fetched.LastError != "something went wrong" {
		t.Fatalf("expected last_error, got %q", fetched.LastError)
	}
}

func TestMemoryStoreLLMModels(t *testing.T) {
	s := NewMemory()
	ctx := context.Background()

	m1, err := s.CreateLLMModel(ctx, "GPT-4o", "openai/gpt-4o", true)
	if err != nil {
		t.Fatal(err)
	}
	if m1.ID == "" {
		t.Fatal("expected model ID")
	}
	if !m1.Enabled {
		t.Fatal("expected model enabled")
	}

	m2, err := s.CreateLLMModel(ctx, "Claude 3.5 Sonnet", "anthropic/claude-3.5-sonnet", false)
	if err != nil {
		t.Fatal(err)
	}
	if m2.Enabled {
		t.Fatal("expected model disabled")
	}

	models, err := s.ListLLMModels(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}

	// Update
	enabled := true
	updated, err := s.UpdateLLMModel(ctx, m2.ID, nil, nil, &enabled)
	if err != nil {
		t.Fatal(err)
	}
	if !updated.Enabled {
		t.Fatal("expected model now enabled")
	}

	fetched, err := s.GetLLMModel(ctx, m1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if fetched.DisplayName != "GPT-4o" {
		t.Fatalf("expected display name GPT-4o, got %q", fetched.DisplayName)
	}
}

func TestMemoryStoreLLMGateway(t *testing.T) {
	s := NewMemory()
	ctx := context.Background()

	// Default empty
	settings, err := s.GetLLMGatewaySettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if settings.Enabled {
		t.Fatal("expected gateway disabled by default")
	}

	// Upsert
	updated, err := s.UpsertLLMGatewaySettings(ctx, true, "https://llm.example.com", "llm-secret", "api-key")
	if err != nil {
		t.Fatal(err)
	}
	if !updated.Enabled || updated.BaseURL != "https://llm.example.com" {
		t.Fatalf("unexpected settings: %#v", updated)
	}

	// Verify persistence
	settings, err = s.GetLLMGatewaySettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !settings.Enabled || settings.BaseURL != "https://llm.example.com" {
		t.Fatalf("unexpected settings: %#v", settings)
	}
}

func TestMemoryStoreBootstrapAdmin(t *testing.T) {
	s := NewMemory()
	ctx := context.Background()

	err := s.BootstrapAdmin(ctx, "admin", "$2a$10$hash")
	if err != nil {
		t.Fatal(err)
	}

	user, err := s.GetUserByUsername(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if user.Role != "admin" {
		t.Fatalf("expected role admin, got %q", user.Role)
	}
	if user.Disabled {
		t.Fatal("expected admin not disabled")
	}

	// Get password hash
	hash, err := s.GetPasswordHash(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if hash != "$2a$10$hash" {
		t.Fatalf("unexpected hash: %q", hash)
	}

	// Re-bootstrap should not error
	err = s.BootstrapAdmin(ctx, "admin", "$2a$10$newhash")
	if err != nil {
		t.Fatal(err)
	}

	// Hash should be updated
	hash, err = s.GetPasswordHash(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if hash != "$2a$10$newhash" {
		t.Fatalf("expected updated hash, got %q", hash)
	}
}

func TestMemoryStoreRespectsCancelledContext(t *testing.T) {
	s := NewMemory()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := s.ListUsers(ctx); err == nil {
		t.Fatal("expected error for cancelled context")
	}
	if _, err := s.ListAgents(ctx, "user-1"); err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestMemoryGetPasswordHashNotFound(t *testing.T) {
	s := NewMemory()
	_, err := s.GetPasswordHash(context.Background(), "nonexistent")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
