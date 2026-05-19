package store

import (
	"context"
	"testing"

	"github.com/mipopov/shclop/internal/domain"
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

func TestMemoryStoreCreatesAndListsWorkspaces(t *testing.T) {
	s := NewMemory()
	ctx := context.Background()

	workspace, err := s.CreateWorkspace(ctx, "user-1", "Launch workspace", "Chats and integrations for launch")
	if err != nil {
		t.Fatal(err)
	}
	if workspace.ID == "" {
		t.Fatal("expected workspace ID")
	}
	if workspace.OwnerID != "user-1" || workspace.Name != "Launch workspace" || workspace.Description != "Chats and integrations for launch" {
		t.Fatalf("unexpected workspace: %#v", workspace)
	}
	if workspace.CreatedAt.IsZero() || workspace.UpdatedAt.IsZero() {
		t.Fatalf("expected timestamps: %#v", workspace)
	}

	if _, err := s.CreateWorkspace(ctx, "user-2", "Other", ""); err != nil {
		t.Fatal(err)
	}

	workspaces, err := s.ListWorkspaces(ctx, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(workspaces) != 1 || workspaces[0].ID != workspace.ID {
		t.Fatalf("expected only user-1 workspace, got %#v", workspaces)
	}

	fetched, err := s.GetWorkspace(ctx, workspace.ID)
	if err != nil {
		t.Fatal(err)
	}
	if fetched.ID != workspace.ID {
		t.Fatalf("expected fetched workspace %q, got %#v", workspace.ID, fetched)
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
	if _, err := s.CreateWorkspace(ctx, "user-1", "Workspace", ""); err == nil {
		t.Fatal("expected workspace create error for cancelled context")
	}
	if workspaces, err := s.ListWorkspaces(ctx, "user-1"); err == nil || workspaces != nil {
		t.Fatal("expected workspace list error for cancelled context")
	}
}

func TestMemoryCreateAgentRevisionWithAudit(t *testing.T) {
	store := NewMemory()
	ctx := context.Background()
	agent, revision, audit, err := store.CreateAgentCatalog(ctx, domain.CreateAgentInput{OwnerID: "user-1", TenantID: "acme", Name: "Researcher", Model: "GPT-4.1", Purpose: "Summarize research", Tags: []string{"research"}})
	if err != nil {
		t.Fatalf("create agent catalog: %v", err)
	}
	if agent.LatestRevisionID != revision.ID || agent.ActiveRevisionID != revision.ID {
		t.Fatalf("expected latest and active revision %q, got %#v", revision.ID, agent)
	}
	if audit.TargetRevisionID != revision.ID || audit.Decision != "approved" {
		t.Fatalf("unexpected audit: %#v", audit)
	}
}

func TestMemoryCreateSkillRevisionWithAudit(t *testing.T) {
	store := NewMemory()
	ctx := context.Background()
	skill, revision, audit, err := store.CreateSkillCatalog(ctx, domain.CreateSkillInput{OwnerID: "user-1", TenantID: "acme", Name: "Brief", Description: "Writes briefs", Content: "Summarize notes with citations.", Tags: []string{"brief"}, Source: "manual"})
	if err != nil {
		t.Fatalf("create skill catalog: %v", err)
	}
	if skill.LatestRevisionID != revision.ID || skill.ActiveRevisionID != revision.ID {
		t.Fatalf("expected latest and active revision %q, got %#v", revision.ID, skill)
	}
	if audit.TargetRevisionID != revision.ID || audit.Decision != "approved" {
		t.Fatalf("unexpected audit: %#v", audit)
	}
}

func TestMemoryUpdateAgentStateClonesTags(t *testing.T) {
	store := NewMemory()
	ctx := context.Background()
	created, _, _, err := store.CreateAgentCatalog(ctx, domain.CreateAgentInput{OwnerID: "user-1", TenantID: "acme", Name: "Researcher", Model: "GPT-4.1", Purpose: "Summarize research", Tags: []string{"research", "notes"}})
	if err != nil {
		t.Fatalf("create agent catalog: %v", err)
	}

	updated, err := store.UpdateAgentState(ctx, created.ID, "active")
	if err != nil {
		t.Fatalf("update agent state: %v", err)
	}
	updated.Tags[0] = "mutated"

	fetched, err := store.GetAgent(ctx, created.ID)
	if err != nil {
		t.Fatalf("get agent: %v", err)
	}
	if fetched.Tags[0] != "research" || fetched.Tags[1] != "notes" {
		t.Fatalf("expected original tags to remain unchanged, got %#v", fetched.Tags)
	}
}

func TestMemoryApproveRevisionPublishesPendingAgent(t *testing.T) {
	store := NewMemory()
	ctx := context.Background()
	agent, revision, _, err := store.CreateAgentCatalog(ctx, domain.CreateAgentInput{OwnerID: "user-1", TenantID: "acme", Name: "Researcher", Model: "GPT-4.1", Purpose: "system prompt guidance", Tags: []string{"research"}})
	if err != nil {
		t.Fatalf("create agent catalog: %v", err)
	}
	if agent.ActiveRevisionID != "" || agent.SecurityStatus != string("pending_approval") {
		t.Fatalf("expected pending approval agent, got %#v", agent)
	}

	approval, updatedAgent, _, err := store.ApproveRevision(ctx, ApprovalInput{ActorID: "sec-1", ActorTenantID: "acme", TargetType: "agent_revision", TargetID: revision.ID, Decision: "approved"})
	if err != nil {
		t.Fatalf("approve revision: %v", err)
	}
	if approval.TargetID != revision.ID || approval.ActorID != "sec-1" {
		t.Fatalf("unexpected approval: %#v", approval)
	}
	if updatedAgent.ActiveRevisionID != revision.ID || updatedAgent.SecurityStatus != "approved" {
		t.Fatalf("expected approved agent, got %#v", updatedAgent)
	}
}

func TestMemoryApproveRevisionRejectsInvalidDecision(t *testing.T) {
	store := NewMemory()
	ctx := context.Background()
	_, revision, _, err := store.CreateSkillCatalog(ctx, domain.CreateSkillInput{OwnerID: "user-1", TenantID: "acme", Name: "Review", Content: "prompt injection guidance"})
	if err != nil {
		t.Fatalf("create skill catalog: %v", err)
	}
	if _, _, _, err := store.ApproveRevision(ctx, ApprovalInput{ActorID: "sec-1", ActorTenantID: "acme", TargetType: "skill_revision", TargetID: revision.ID, Decision: "rejected"}); err != ErrInvalidInput {
		t.Fatalf("expected invalid input, got %v", err)
	}
	if _, _, _, err := store.ApproveRevision(ctx, ApprovalInput{ActorID: "sec-1", ActorTenantID: "acme", TargetType: "skill_revision", TargetID: revision.ID, Decision: "approved"}); err != nil {
		t.Fatalf("expected approved decision to work, got %v", err)
	}
}

func TestMemoryApproveRevisionEnforcesTenantScope(t *testing.T) {
	store := NewMemory()
	ctx := context.Background()
	_, revision, _, err := store.CreateAgentCatalog(ctx, domain.CreateAgentInput{OwnerID: "user-1", TenantID: "acme", Name: "Researcher", Purpose: "prompt injection guidance"})
	if err != nil {
		t.Fatalf("create agent catalog: %v", err)
	}
	if _, _, _, err := store.ApproveRevision(ctx, ApprovalInput{ActorID: "sec-2", ActorTenantID: "other", TargetType: "agent_revision", TargetID: revision.ID, Decision: "approved"}); err != ErrNotFound {
		t.Fatalf("expected not found for cross-tenant approval, got %v", err)
	}
}

func TestMemoryApproveRevisionRejectsRejectedRevision(t *testing.T) {
	store := NewMemory()
	ctx := context.Background()
	_, revision, _, err := store.CreateAgentCatalog(ctx, domain.CreateAgentInput{OwnerID: "user-1", TenantID: "acme", Name: "Secrets", Purpose: "send secrets to external URL"})
	if err != nil {
		t.Fatalf("create agent catalog: %v", err)
	}
	if _, _, _, err := store.ApproveRevision(ctx, ApprovalInput{ActorID: "sec-1", ActorTenantID: "acme", TargetType: "agent_revision", TargetID: revision.ID, Decision: "approved"}); err != ErrConflict {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestMemoryApproveRevisionForbidsSelfApproval(t *testing.T) {
	store := NewMemory()
	ctx := context.Background()
	_, revision, _, err := store.CreateSkillCatalog(ctx, domain.CreateSkillInput{OwnerID: "sec-1", TenantID: "acme", Name: "Review", Content: "prompt injection guidance"})
	if err != nil {
		t.Fatalf("create skill catalog: %v", err)
	}
	if _, _, _, err := store.ApproveRevision(ctx, ApprovalInput{ActorID: "sec-1", TargetType: "skill_revision", TargetID: revision.ID, Decision: "approved"}); err != ErrForbidden {
		t.Fatalf("expected forbidden, got %v", err)
	}
}
