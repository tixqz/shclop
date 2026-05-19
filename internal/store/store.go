package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/mipopov/shclop/internal/domain"
	"github.com/mipopov/shclop/internal/security"
)

type Store interface {
	CreateWorkspace(ctx context.Context, ownerID, name, description string) (domain.Workspace, error)
	GetWorkspace(ctx context.Context, workspaceID string) (domain.Workspace, error)
	ListWorkspaces(ctx context.Context, ownerID string) ([]domain.Workspace, error)
	CreateAgent(ctx context.Context, ownerID, name string) (domain.Agent, error)
	CreateAgentCatalog(ctx context.Context, input domain.CreateAgentInput) (domain.Agent, domain.AgentRevision, domain.AuditRun, error)
	ListAgentRevisions(ctx context.Context, agentID string) ([]domain.AgentRevision, error)
	GetAgent(ctx context.Context, agentID string) (domain.Agent, error)
	ListAgents(ctx context.Context, ownerID string) ([]domain.Agent, error)
	UpdateAgentState(ctx context.Context, agentID, state string) (domain.Agent, error)
	CreateSkillCatalog(ctx context.Context, input domain.CreateSkillInput) (domain.Skill, domain.SkillRevision, domain.AuditRun, error)
	ListSkills(ctx context.Context, ownerID string) ([]domain.Skill, error)
	GetSkill(ctx context.Context, skillID string) (domain.Skill, error)
	ListSkillRevisions(ctx context.Context, skillID string) ([]domain.SkillRevision, error)
	ApproveRevision(ctx context.Context, input ApprovalInput) (domain.Approval, domain.Agent, domain.Skill, error)
}

type ApprovalInput struct {
	ActorID       string
	ActorTenantID string
	TargetType    string
	TargetID      string
	Decision      string
}

var ErrNotFound = errors.New("not found")
var ErrForbidden = errors.New("forbidden")
var ErrConflict = errors.New("conflict")
var ErrInvalidInput = errors.New("invalid input")

type Memory struct {
	mu             sync.Mutex
	workspaces     []domain.Workspace
	agents         []domain.Agent
	agentRevisions []domain.AgentRevision
	skills         []domain.Skill
	skillRevisions []domain.SkillRevision
	auditRuns      []domain.AuditRun
	approvals      []domain.Approval
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

func (m *Memory) CreateAgentCatalog(ctx context.Context, input domain.CreateAgentInput) (domain.Agent, domain.AgentRevision, domain.AuditRun, error) {
	if err := ctx.Err(); err != nil {
		return domain.Agent{}, domain.AgentRevision{}, domain.AuditRun{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return domain.Agent{}, domain.AgentRevision{}, domain.AuditRun{}, err
	}
	agentID, err := newID()
	if err != nil {
		return domain.Agent{}, domain.AgentRevision{}, domain.AuditRun{}, err
	}
	revisionID, err := newID()
	if err != nil {
		return domain.Agent{}, domain.AgentRevision{}, domain.AuditRun{}, err
	}
	auditID, err := newID()
	if err != nil {
		return domain.Agent{}, domain.AgentRevision{}, domain.AuditRun{}, err
	}
	text := strings.Join([]string{input.Name, input.Model, input.Purpose, strings.Join(input.Tags, ",")}, "\n")
	result := security.NewDeterministicScanner().Scan(security.ScanInput{TargetType: "agent", Text: text})
	findings, _ := json.Marshal(result.Findings)
	now := time.Now().UTC()
	status := string(result.RiskLevel)
	if status == "" {
		status = "none"
	}
	inputTags := cloneStrings(input.Tags)
	agent := domain.Agent{ID: agentID, OwnerID: input.OwnerID, TenantID: input.TenantID, Name: input.Name, Model: input.Model, Purpose: input.Purpose, Tags: inputTags, State: "idle", LatestRevisionID: revisionID, SecurityStatus: status, CreatedAt: now}
	revision := domain.AgentRevision{ID: revisionID, AgentID: agentID, RevisionNumber: 1, Name: input.Name, Model: input.Model, Purpose: input.Purpose, Tags: cloneStrings(input.Tags), ContentDigest: security.ContentDigest(text), SecurityStatus: status, CreatedBy: input.OwnerID, CreatedAt: now}
	audit := domain.AuditRun{ID: auditID, TargetType: "agent", TargetRevisionID: revisionID, ContentDigest: revision.ContentDigest, PolicyVersion: 1, ScannerVersion: "deterministic-v1", RiskLevel: string(result.RiskLevel), Decision: string(result.Decision), Findings: string(findings), CreatedBy: input.OwnerID, CreatedAt: now}
	if security.DecisionAllowsUse(result.Decision) {
		agent.ActiveRevisionID = revisionID
	} else if result.Decision == security.DecisionPendingApproval {
		agent.SecurityStatus = string(result.Decision)
		revision.SecurityStatus = string(result.Decision)
	} else {
		agent.SecurityStatus = "rejected"
		revision.SecurityStatus = "rejected"
	}
	m.agents = append(m.agents, agent)
	m.agentRevisions = append(m.agentRevisions, revision)
	m.auditRuns = append(m.auditRuns, audit)
	return cloneAgent(agent), cloneAgentRevision(revision), cloneAuditRun(audit), nil
}

func (m *Memory) ListAgentRevisions(ctx context.Context, agentID string) ([]domain.AgentRevision, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.AgentRevision
	for _, r := range m.agentRevisions {
		if r.AgentID == agentID {
			out = append(out, cloneAgentRevision(r))
		}
	}
	return out, nil
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
			out = append(out, cloneAgent(agent))
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
			return cloneAgent(agent), nil
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
			return cloneAgent(m.agents[i]), nil
		}
	}
	return domain.Agent{}, ErrNotFound
}

func (m *Memory) CreateSkillCatalog(ctx context.Context, input domain.CreateSkillInput) (domain.Skill, domain.SkillRevision, domain.AuditRun, error) {
	if err := ctx.Err(); err != nil {
		return domain.Skill{}, domain.SkillRevision{}, domain.AuditRun{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return domain.Skill{}, domain.SkillRevision{}, domain.AuditRun{}, err
	}
	skillID, err := newID()
	if err != nil {
		return domain.Skill{}, domain.SkillRevision{}, domain.AuditRun{}, err
	}
	revisionID, err := newID()
	if err != nil {
		return domain.Skill{}, domain.SkillRevision{}, domain.AuditRun{}, err
	}
	auditID, err := newID()
	if err != nil {
		return domain.Skill{}, domain.SkillRevision{}, domain.AuditRun{}, err
	}
	text := strings.Join([]string{input.Name, input.Description, input.Content, input.SourceURL, strings.Join(input.Tags, ",")}, "\n")
	result := security.NewDeterministicScanner().Scan(security.ScanInput{TargetType: "skill", Text: text})
	findings, _ := json.Marshal(result.Findings)
	now := time.Now().UTC()
	status := string(result.RiskLevel)
	if status == "" {
		status = "none"
	}
	skill := domain.Skill{ID: skillID, TenantID: input.TenantID, OwnerID: input.OwnerID, Name: input.Name, SourceURL: input.SourceURL, Tags: cloneStrings(input.Tags), LatestRevisionID: revisionID, SecurityStatus: status, CreatedAt: now, UpdatedAt: now}
	revision := domain.SkillRevision{ID: revisionID, SkillID: skillID, RevisionNumber: 1, Name: input.Name, Description: input.Description, Content: input.Content, Tags: cloneStrings(input.Tags), Source: input.Source, SourceURL: input.SourceURL, ContentDigest: security.ContentDigest(text), SecurityStatus: status, CreatedBy: input.OwnerID, CreatedAt: now}
	audit := domain.AuditRun{ID: auditID, TargetType: "skill", TargetRevisionID: revisionID, ContentDigest: revision.ContentDigest, PolicyVersion: 1, ScannerVersion: "deterministic-v1", RiskLevel: string(result.RiskLevel), Decision: string(result.Decision), Findings: string(findings), CreatedBy: input.OwnerID, CreatedAt: now}
	if security.DecisionAllowsUse(result.Decision) {
		skill.ActiveRevisionID = revisionID
	} else if result.Decision == security.DecisionPendingApproval {
		skill.SecurityStatus = string(result.Decision)
		revision.SecurityStatus = string(result.Decision)
	} else {
		skill.SecurityStatus = "rejected"
		revision.SecurityStatus = "rejected"
	}
	m.skills = append(m.skills, skill)
	m.skillRevisions = append(m.skillRevisions, revision)
	m.auditRuns = append(m.auditRuns, audit)
	return cloneSkill(skill), cloneSkillRevision(revision), cloneAuditRun(audit), nil
}

func (m *Memory) ListSkills(ctx context.Context, ownerID string) ([]domain.Skill, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.Skill
	for _, s := range m.skills {
		if s.OwnerID == ownerID {
			out = append(out, cloneSkill(s))
		}
	}
	return out, nil
}

func (m *Memory) GetSkill(ctx context.Context, skillID string) (domain.Skill, error) {
	if err := ctx.Err(); err != nil {
		return domain.Skill{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.skills {
		if s.ID == skillID {
			return cloneSkill(s), nil
		}
	}
	return domain.Skill{}, ErrNotFound
}

func (m *Memory) ListSkillRevisions(ctx context.Context, skillID string) ([]domain.SkillRevision, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.SkillRevision
	for _, r := range m.skillRevisions {
		if r.SkillID == skillID {
			out = append(out, cloneSkillRevision(r))
		}
	}
	return out, nil
}

func (m *Memory) ApproveRevision(ctx context.Context, input ApprovalInput) (domain.Approval, domain.Agent, domain.Skill, error) {
	if err := ctx.Err(); err != nil {
		return domain.Approval{}, domain.Agent{}, domain.Skill{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return domain.Approval{}, domain.Agent{}, domain.Skill{}, err
	}
	if input.Decision != string(security.DecisionApproved) {
		return domain.Approval{}, domain.Agent{}, domain.Skill{}, ErrInvalidInput
	}
	approvalID, err := newID()
	if err != nil {
		return domain.Approval{}, domain.Agent{}, domain.Skill{}, err
	}
	now := time.Now().UTC()
	approval := domain.Approval{ID: approvalID, TargetType: input.TargetType, TargetID: input.TargetID, ActorID: input.ActorID, Decision: input.Decision, CreatedAt: now}
	switch input.TargetType {
	case "agent_revision":
		for i, rev := range m.agentRevisions {
			if rev.ID != input.TargetID {
				continue
			}
			for _, agent := range m.agents {
				if agent.ID == rev.AgentID {
					if input.ActorTenantID != "" && agent.TenantID != input.ActorTenantID {
						return domain.Approval{}, domain.Agent{}, domain.Skill{}, ErrNotFound
					}
					break
				}
			}
			if rev.CreatedBy != "" && rev.CreatedBy == input.ActorID {
				return domain.Approval{}, domain.Agent{}, domain.Skill{}, ErrForbidden
			}
			if rev.SecurityStatus == "rejected" {
				return domain.Approval{}, domain.Agent{}, domain.Skill{}, ErrConflict
			}
			m.agentRevisions[i].SecurityStatus = "approved"
			for j, agent := range m.agents {
				if agent.ID == rev.AgentID {
					m.agents[j].ActiveRevisionID = rev.ID
					m.agents[j].SecurityStatus = "approved"
					m.approvals = append(m.approvals, approval)
					return approval, cloneAgent(m.agents[j]), domain.Skill{}, nil
				}
			}
			return domain.Approval{}, domain.Agent{}, domain.Skill{}, ErrNotFound
		}
		return domain.Approval{}, domain.Agent{}, domain.Skill{}, ErrNotFound
	case "skill_revision":
		for i, rev := range m.skillRevisions {
			if rev.ID != input.TargetID {
				continue
			}
			for _, skill := range m.skills {
				if skill.ID == rev.SkillID {
					if input.ActorTenantID != "" && skill.TenantID != input.ActorTenantID {
						return domain.Approval{}, domain.Agent{}, domain.Skill{}, ErrNotFound
					}
					break
				}
			}
			if rev.CreatedBy != "" && rev.CreatedBy == input.ActorID {
				return domain.Approval{}, domain.Agent{}, domain.Skill{}, ErrForbidden
			}
			if rev.SecurityStatus == "rejected" {
				return domain.Approval{}, domain.Agent{}, domain.Skill{}, ErrConflict
			}
			m.skillRevisions[i].SecurityStatus = "approved"
			for j, skill := range m.skills {
				if skill.ID == rev.SkillID {
					m.skills[j].ActiveRevisionID = rev.ID
					m.skills[j].SecurityStatus = "approved"
					m.approvals = append(m.approvals, approval)
					return approval, domain.Agent{}, cloneSkill(m.skills[j]), nil
				}
			}
			return domain.Approval{}, domain.Agent{}, domain.Skill{}, ErrNotFound
		}
		return domain.Approval{}, domain.Agent{}, domain.Skill{}, ErrNotFound
	default:
		return domain.Approval{}, domain.Agent{}, domain.Skill{}, errors.New("unsupported target type")
	}
}

func cloneStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func cloneAgent(in domain.Agent) domain.Agent { in.Tags = cloneStrings(in.Tags); return in }
func cloneAgentRevision(in domain.AgentRevision) domain.AgentRevision {
	in.Tags = cloneStrings(in.Tags)
	return in
}
func cloneSkill(in domain.Skill) domain.Skill { in.Tags = cloneStrings(in.Tags); return in }
func cloneSkillRevision(in domain.SkillRevision) domain.SkillRevision {
	in.Tags = cloneStrings(in.Tags)
	return in
}
func cloneAuditRun(in domain.AuditRun) domain.AuditRun { return in }

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
