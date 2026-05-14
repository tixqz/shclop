package store

import (
	"context"
	"testing"
)

func TestMemoryStoreCreatesAndListsAgents(t *testing.T) {
	s := NewMemory()
	ctx := context.Background()

	agent1, err := s.CreateAgent(ctx, "user-1", "Researcher")
	if err != nil {
		t.Fatal(err)
	}
	if agent1.ID == "" {
		t.Fatal("expected agent ID")
	}
	if agent1.State != "idle" {
		t.Fatalf("unexpected state %q", agent1.State)
	}
	if agent1.CreatedAt.IsZero() {
		t.Fatal("expected CreatedAt")
	}

	agent2, err := s.CreateAgent(ctx, "user-2", "Builder")
	if err != nil {
		t.Fatal(err)
	}
	if agent2.OwnerID != "user-2" {
		t.Fatalf("unexpected owner %q", agent2.OwnerID)
	}

	agents, err := s.ListAgents(ctx, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	if agents[0].Name != "Researcher" {
		t.Fatalf("unexpected name %q", agents[0].Name)
	}
	if agents[0].OwnerID != "user-1" {
		t.Fatalf("unexpected owner %q", agents[0].OwnerID)
	}

	otherAgents, err := s.ListAgents(ctx, "user-2")
	if err != nil {
		t.Fatal(err)
	}
	if len(otherAgents) != 1 {
		t.Fatalf("expected 1 agent for user-2, got %d", len(otherAgents))
	}
	if otherAgents[0].Name != "Builder" {
		t.Fatalf("unexpected name %q", otherAgents[0].Name)
	}

	if agents[0].OwnerID == otherAgents[0].OwnerID {
		t.Fatal("expected no cross-user leakage")
	}
}

func TestMemoryStoreRespectsCancelledContext(t *testing.T) {
	s := NewMemory()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := s.CreateAgent(ctx, "user-1", "Researcher"); err == nil {
		t.Fatal("expected create error for cancelled context")
	}
	if agents, err := s.ListAgents(ctx, "user-1"); err == nil || agents != nil {
		t.Fatal("expected list error for cancelled context")
	}
}
