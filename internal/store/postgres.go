package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/mipopov/shclop/internal/domain"
	"github.com/mipopov/shclop/internal/security"
)

type Postgres struct{ db *sql.DB }

func NewPostgres(dsn string) (*Postgres, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	return &Postgres{db: db}, nil
}

func (p *Postgres) CreateAgent(ctx context.Context, ownerID, name string) (domain.Agent, error) {
	id, err := newID()
	if err != nil {
		return domain.Agent{}, err
	}
	agent := domain.Agent{ID: id, OwnerID: ownerID, Name: name, State: "idle", CreatedAt: time.Now().UTC()}
	_, err = p.db.ExecContext(ctx, `insert into agents (id, owner_id, tenant_id, name, model, purpose, tags, state, latest_revision_id, active_revision_id, security_status, created_at) values ($1,$2,'',$3,'','','[]',$4,'','','',$5)`, agent.ID, agent.OwnerID, agent.Name, agent.State, agent.CreatedAt)
	if err != nil {
		return domain.Agent{}, err
	}
	return agent, nil
}

func (p *Postgres) CreateAgentCatalog(ctx context.Context, input domain.CreateAgentInput) (domain.Agent, domain.AgentRevision, domain.AuditRun, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Agent{}, domain.AgentRevision{}, domain.AuditRun{}, err
	}
	defer tx.Rollback()
	return createAgentCatalogTx(ctx, tx, input)
}

func (p *Postgres) ListAgentRevisions(ctx context.Context, agentID string) ([]domain.AgentRevision, error) {
	rows, err := p.db.QueryContext(ctx, `select id, agent_id, revision_number, name, model, purpose, tags, content_digest, security_status, created_by, created_at from agent_revisions where agent_id = $1 order by revision_number asc, id asc`, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.AgentRevision
	for rows.Next() {
		var r domain.AgentRevision
		var tags string
		if err := rows.Scan(&r.ID, &r.AgentID, &r.RevisionNumber, &r.Name, &r.Model, &r.Purpose, &tags, &r.ContentDigest, &r.SecurityStatus, &r.CreatedBy, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.Tags = decodeTags(tags)
		out = append(out, r)
	}
	return out, rows.Err()
}

func (p *Postgres) CreateWorkspace(ctx context.Context, ownerID, name, description string) (domain.Workspace, error) {
	id, err := newID()
	if err != nil {
		return domain.Workspace{}, err
	}
	now := time.Now().UTC()
	workspace := domain.Workspace{ID: id, OwnerID: ownerID, Name: name, Description: description, CreatedAt: now, UpdatedAt: now}
	_, err = p.db.ExecContext(ctx, `insert into workspaces (id, owner_id, name, description, created_at, updated_at) values ($1, $2, $3, $4, $5, $6)`, workspace.ID, workspace.OwnerID, workspace.Name, workspace.Description, workspace.CreatedAt, workspace.UpdatedAt)
	if err != nil {
		return domain.Workspace{}, err
	}
	return workspace, nil
}

func (p *Postgres) ListWorkspaces(ctx context.Context, ownerID string) ([]domain.Workspace, error) {
	rows, err := p.db.QueryContext(ctx, `select id, owner_id, name, description, created_at, updated_at from workspaces where owner_id = $1 order by updated_at desc, id asc`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Workspace
	for rows.Next() {
		var w domain.Workspace
		if err := rows.Scan(&w.ID, &w.OwnerID, &w.Name, &w.Description, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (p *Postgres) GetWorkspace(ctx context.Context, workspaceID string) (domain.Workspace, error) {
	var w domain.Workspace
	err := p.db.QueryRowContext(ctx, `select id, owner_id, name, description, created_at, updated_at from workspaces where id = $1`, workspaceID).Scan(&w.ID, &w.OwnerID, &w.Name, &w.Description, &w.CreatedAt, &w.UpdatedAt)
	if err == sql.ErrNoRows {
		return domain.Workspace{}, ErrNotFound
	}
	if err != nil {
		return domain.Workspace{}, err
	}
	return w, nil
}

func (p *Postgres) ListAgents(ctx context.Context, ownerID string) ([]domain.Agent, error) {
	rows, err := p.db.QueryContext(ctx, `select id, owner_id, tenant_id, name, model, purpose, tags, state, latest_revision_id, active_revision_id, security_status, created_at from agents where owner_id = $1 order by created_at asc, id asc`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Agent
	for rows.Next() {
		var a domain.Agent
		var tags string
		if err := rows.Scan(&a.ID, &a.OwnerID, &a.TenantID, &a.Name, &a.Model, &a.Purpose, &tags, &a.State, &a.LatestRevisionID, &a.ActiveRevisionID, &a.SecurityStatus, &a.CreatedAt); err != nil {
			return nil, err
		}
		a.Tags = decodeTags(tags)
		out = append(out, a)
	}
	return out, rows.Err()
}

func (p *Postgres) GetAgent(ctx context.Context, agentID string) (domain.Agent, error) {
	var a domain.Agent
	var tags string
	err := p.db.QueryRowContext(ctx, `select id, owner_id, tenant_id, name, model, purpose, tags, state, latest_revision_id, active_revision_id, security_status, created_at from agents where id = $1`, agentID).Scan(&a.ID, &a.OwnerID, &a.TenantID, &a.Name, &a.Model, &a.Purpose, &tags, &a.State, &a.LatestRevisionID, &a.ActiveRevisionID, &a.SecurityStatus, &a.CreatedAt)
	if err == sql.ErrNoRows {
		return domain.Agent{}, ErrNotFound
	}
	if err != nil {
		return domain.Agent{}, err
	}
	a.Tags = decodeTags(tags)
	return a, nil
}

func (p *Postgres) UpdateAgentState(ctx context.Context, agentID, state string) (domain.Agent, error) {
	var a domain.Agent
	var tags string
	err := p.db.QueryRowContext(ctx, `update agents set state = $2 where id = $1 returning id, owner_id, tenant_id, name, model, purpose, tags, state, latest_revision_id, active_revision_id, security_status, created_at`, agentID, state).Scan(&a.ID, &a.OwnerID, &a.TenantID, &a.Name, &a.Model, &a.Purpose, &tags, &a.State, &a.LatestRevisionID, &a.ActiveRevisionID, &a.SecurityStatus, &a.CreatedAt)
	if err == sql.ErrNoRows {
		return domain.Agent{}, ErrNotFound
	}
	if err != nil {
		return domain.Agent{}, err
	}
	a.Tags = decodeTags(tags)
	return a, nil
}

func (p *Postgres) CreateSkillCatalog(ctx context.Context, input domain.CreateSkillInput) (domain.Skill, domain.SkillRevision, domain.AuditRun, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Skill{}, domain.SkillRevision{}, domain.AuditRun{}, err
	}
	defer tx.Rollback()
	return createSkillCatalogTx(ctx, tx, input)
}

func (p *Postgres) ListSkills(ctx context.Context, ownerID string) ([]domain.Skill, error) {
	rows, err := p.db.QueryContext(ctx, `select id, tenant_id, owner_id, name, source_url, tags, latest_revision_id, active_revision_id, security_status, created_at, updated_at from skills where owner_id = $1 order by updated_at desc, id asc`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Skill
	for rows.Next() {
		var s domain.Skill
		var tags string
		if err := rows.Scan(&s.ID, &s.TenantID, &s.OwnerID, &s.Name, &s.SourceURL, &tags, &s.LatestRevisionID, &s.ActiveRevisionID, &s.SecurityStatus, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		s.Tags = decodeTags(tags)
		out = append(out, s)
	}
	return out, rows.Err()
}

func (p *Postgres) GetSkill(ctx context.Context, skillID string) (domain.Skill, error) {
	var s domain.Skill
	var tags string
	err := p.db.QueryRowContext(ctx, `select id, tenant_id, owner_id, name, source_url, tags, latest_revision_id, active_revision_id, security_status, created_at, updated_at from skills where id = $1`, skillID).Scan(&s.ID, &s.TenantID, &s.OwnerID, &s.Name, &s.SourceURL, &tags, &s.LatestRevisionID, &s.ActiveRevisionID, &s.SecurityStatus, &s.CreatedAt, &s.UpdatedAt)
	if err == sql.ErrNoRows {
		return domain.Skill{}, ErrNotFound
	}
	if err != nil {
		return domain.Skill{}, err
	}
	s.Tags = decodeTags(tags)
	return s, nil
}

func (p *Postgres) ListSkillRevisions(ctx context.Context, skillID string) ([]domain.SkillRevision, error) {
	rows, err := p.db.QueryContext(ctx, `select id, skill_id, revision_number, name, description, content, tags, source, source_url, content_digest, security_status, created_by, created_at from skill_revisions where skill_id = $1 order by revision_number asc, id asc`, skillID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.SkillRevision
	for rows.Next() {
		var r domain.SkillRevision
		var tags string
		if err := rows.Scan(&r.ID, &r.SkillID, &r.RevisionNumber, &r.Name, &r.Description, &r.Content, &tags, &r.Source, &r.SourceURL, &r.ContentDigest, &r.SecurityStatus, &r.CreatedBy, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.Tags = decodeTags(tags)
		out = append(out, r)
	}
	return out, rows.Err()
}

func (p *Postgres) ApproveRevision(ctx context.Context, input ApprovalInput) (domain.Approval, domain.Agent, domain.Skill, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Approval{}, domain.Agent{}, domain.Skill{}, err
	}
	defer tx.Rollback()
	return approveRevisionTx(ctx, tx, input)
}

func (p *Postgres) Close() error { return p.db.Close() }

func decodeTags(s string) []string {
	var out []string
	if s == "" || s == "[]" {
		return nil
	}
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil
	}
	return out
}

func createAgentCatalogTx(ctx context.Context, tx *sql.Tx, input domain.CreateAgentInput) (domain.Agent, domain.AgentRevision, domain.AuditRun, error) {
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
	agent := domain.Agent{ID: agentID, OwnerID: input.OwnerID, TenantID: input.TenantID, Name: input.Name, Model: input.Model, Purpose: input.Purpose, Tags: cloneStrings(input.Tags), State: "idle", LatestRevisionID: revisionID, SecurityStatus: status, CreatedAt: now}
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
	encoded, err := json.Marshal(input.Tags)
	if err != nil {
		return domain.Agent{}, domain.AgentRevision{}, domain.AuditRun{}, err
	}
	if _, err := tx.ExecContext(ctx, `insert into agents (id, owner_id, tenant_id, name, model, purpose, tags, state, latest_revision_id, active_revision_id, security_status, created_at) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, agent.ID, agent.OwnerID, agent.TenantID, agent.Name, agent.Model, agent.Purpose, string(encoded), agent.State, agent.LatestRevisionID, agent.ActiveRevisionID, agent.SecurityStatus, agent.CreatedAt); err != nil {
		return domain.Agent{}, domain.AgentRevision{}, domain.AuditRun{}, err
	}
	if _, err := tx.ExecContext(ctx, `insert into agent_revisions (id, agent_id, revision_number, name, model, purpose, tags, content_digest, security_status, created_by, created_at) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, revision.ID, revision.AgentID, revision.RevisionNumber, revision.Name, revision.Model, revision.Purpose, string(encoded), revision.ContentDigest, revision.SecurityStatus, revision.CreatedBy, revision.CreatedAt); err != nil {
		return domain.Agent{}, domain.AgentRevision{}, domain.AuditRun{}, err
	}
	if _, err := tx.ExecContext(ctx, `insert into audit_runs (id, target_type, target_revision_id, content_digest, policy_version, scanner_version, risk_level, decision, findings_json, created_by, created_at) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, audit.ID, audit.TargetType, audit.TargetRevisionID, audit.ContentDigest, audit.PolicyVersion, audit.ScannerVersion, audit.RiskLevel, audit.Decision, audit.Findings, audit.CreatedBy, audit.CreatedAt); err != nil {
		return domain.Agent{}, domain.AgentRevision{}, domain.AuditRun{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Agent{}, domain.AgentRevision{}, domain.AuditRun{}, err
	}
	return cloneAgent(agent), cloneAgentRevision(revision), cloneAuditRun(audit), nil
}

func createSkillCatalogTx(ctx context.Context, tx *sql.Tx, input domain.CreateSkillInput) (domain.Skill, domain.SkillRevision, domain.AuditRun, error) {
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
	encoded, err := json.Marshal(input.Tags)
	if err != nil {
		return domain.Skill{}, domain.SkillRevision{}, domain.AuditRun{}, err
	}
	if _, err := tx.ExecContext(ctx, `insert into skills (id, tenant_id, owner_id, name, source_url, tags, latest_revision_id, active_revision_id, security_status, created_at, updated_at) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, skill.ID, skill.TenantID, skill.OwnerID, skill.Name, skill.SourceURL, string(encoded), skill.LatestRevisionID, skill.ActiveRevisionID, skill.SecurityStatus, skill.CreatedAt, skill.UpdatedAt); err != nil {
		return domain.Skill{}, domain.SkillRevision{}, domain.AuditRun{}, err
	}
	if _, err := tx.ExecContext(ctx, `insert into skill_revisions (id, skill_id, revision_number, name, description, content, tags, source, source_url, content_digest, security_status, created_by, created_at) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, revision.ID, revision.SkillID, revision.RevisionNumber, revision.Name, revision.Description, revision.Content, string(encoded), revision.Source, revision.SourceURL, revision.ContentDigest, revision.SecurityStatus, revision.CreatedBy, revision.CreatedAt); err != nil {
		return domain.Skill{}, domain.SkillRevision{}, domain.AuditRun{}, err
	}
	if _, err := tx.ExecContext(ctx, `insert into audit_runs (id, target_type, target_revision_id, content_digest, policy_version, scanner_version, risk_level, decision, findings_json, created_by, created_at) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, audit.ID, audit.TargetType, audit.TargetRevisionID, audit.ContentDigest, audit.PolicyVersion, audit.ScannerVersion, audit.RiskLevel, audit.Decision, audit.Findings, audit.CreatedBy, audit.CreatedAt); err != nil {
		return domain.Skill{}, domain.SkillRevision{}, domain.AuditRun{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Skill{}, domain.SkillRevision{}, domain.AuditRun{}, err
	}
	return cloneSkill(skill), cloneSkillRevision(revision), cloneAuditRun(audit), nil
}

func approveRevisionTx(ctx context.Context, tx *sql.Tx, input ApprovalInput) (domain.Approval, domain.Agent, domain.Skill, error) {
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
		var rev domain.AgentRevision
		var tags, tenantID string
		if err := tx.QueryRowContext(ctx, `select ar.id, ar.agent_id, ar.revision_number, ar.name, ar.model, ar.purpose, ar.tags, ar.content_digest, ar.security_status, ar.created_by, ar.created_at, a.tenant_id from agent_revisions ar join agents a on a.id = ar.agent_id where ar.id = $1`, input.TargetID).Scan(&rev.ID, &rev.AgentID, &rev.RevisionNumber, &rev.Name, &rev.Model, &rev.Purpose, &tags, &rev.ContentDigest, &rev.SecurityStatus, &rev.CreatedBy, &rev.CreatedAt, &tenantID); err != nil {
			if err == sql.ErrNoRows {
				return domain.Approval{}, domain.Agent{}, domain.Skill{}, ErrNotFound
			}
			return domain.Approval{}, domain.Agent{}, domain.Skill{}, err
		}
		rev.Tags = decodeTags(tags)
		if input.ActorTenantID != "" && tenantID != input.ActorTenantID {
			return domain.Approval{}, domain.Agent{}, domain.Skill{}, ErrNotFound
		}
		if rev.CreatedBy != "" && rev.CreatedBy == input.ActorID {
			return domain.Approval{}, domain.Agent{}, domain.Skill{}, ErrForbidden
		}
		if rev.SecurityStatus == "rejected" {
			return domain.Approval{}, domain.Agent{}, domain.Skill{}, ErrConflict
		}
		if _, err := tx.ExecContext(ctx, `update agent_revisions set security_status = 'approved' where id = $1`, rev.ID); err != nil {
			return domain.Approval{}, domain.Agent{}, domain.Skill{}, err
		}
		if _, err := tx.ExecContext(ctx, `update agents set active_revision_id = $2, security_status = 'approved' where id = $1`, rev.AgentID, rev.ID); err != nil {
			return domain.Approval{}, domain.Agent{}, domain.Skill{}, err
		}
		if _, err := tx.ExecContext(ctx, `insert into approvals (id, target_type, target_id, actor_id, decision, created_at) values ($1,$2,$3,$4,$5,$6)`, approval.ID, approval.TargetType, approval.TargetID, approval.ActorID, approval.Decision, approval.CreatedAt); err != nil {
			return domain.Approval{}, domain.Agent{}, domain.Skill{}, err
		}
		var agent domain.Agent
		var agentTags string
		if err := tx.QueryRowContext(ctx, `select id, owner_id, tenant_id, name, model, purpose, tags, state, latest_revision_id, active_revision_id, security_status, created_at from agents where id = $1`, rev.AgentID).Scan(&agent.ID, &agent.OwnerID, &agent.TenantID, &agent.Name, &agent.Model, &agent.Purpose, &agentTags, &agent.State, &agent.LatestRevisionID, &agent.ActiveRevisionID, &agent.SecurityStatus, &agent.CreatedAt); err != nil {
			return domain.Approval{}, domain.Agent{}, domain.Skill{}, err
		}
		agent.Tags = decodeTags(agentTags)
		if err := tx.Commit(); err != nil {
			return domain.Approval{}, domain.Agent{}, domain.Skill{}, err
		}
		return approval, agent, domain.Skill{}, nil
	case "skill_revision":
		var rev domain.SkillRevision
		var tags, tenantID string
		if err := tx.QueryRowContext(ctx, `select sr.id, sr.skill_id, sr.revision_number, sr.name, sr.description, sr.content, sr.tags, sr.source, sr.source_url, sr.content_digest, sr.security_status, sr.created_by, sr.created_at, s.tenant_id from skill_revisions sr join skills s on s.id = sr.skill_id where sr.id = $1`, input.TargetID).Scan(&rev.ID, &rev.SkillID, &rev.RevisionNumber, &rev.Name, &rev.Description, &rev.Content, &tags, &rev.Source, &rev.SourceURL, &rev.ContentDigest, &rev.SecurityStatus, &rev.CreatedBy, &rev.CreatedAt, &tenantID); err != nil {
			if err == sql.ErrNoRows {
				return domain.Approval{}, domain.Agent{}, domain.Skill{}, ErrNotFound
			}
			return domain.Approval{}, domain.Agent{}, domain.Skill{}, err
		}
		rev.Tags = decodeTags(tags)
		if input.ActorTenantID != "" && tenantID != input.ActorTenantID {
			return domain.Approval{}, domain.Agent{}, domain.Skill{}, ErrNotFound
		}
		if rev.CreatedBy != "" && rev.CreatedBy == input.ActorID {
			return domain.Approval{}, domain.Agent{}, domain.Skill{}, ErrForbidden
		}
		if rev.SecurityStatus == "rejected" {
			return domain.Approval{}, domain.Agent{}, domain.Skill{}, ErrConflict
		}
		if _, err := tx.ExecContext(ctx, `update skill_revisions set security_status = 'approved' where id = $1`, rev.ID); err != nil {
			return domain.Approval{}, domain.Agent{}, domain.Skill{}, err
		}
		if _, err := tx.ExecContext(ctx, `update skills set active_revision_id = $2, security_status = 'approved' where id = $1`, rev.SkillID, rev.ID); err != nil {
			return domain.Approval{}, domain.Agent{}, domain.Skill{}, err
		}
		if _, err := tx.ExecContext(ctx, `insert into approvals (id, target_type, target_id, actor_id, decision, created_at) values ($1,$2,$3,$4,$5,$6)`, approval.ID, approval.TargetType, approval.TargetID, approval.ActorID, approval.Decision, approval.CreatedAt); err != nil {
			return domain.Approval{}, domain.Agent{}, domain.Skill{}, err
		}
		var skill domain.Skill
		var skillTags string
		if err := tx.QueryRowContext(ctx, `select id, tenant_id, owner_id, name, source_url, tags, latest_revision_id, active_revision_id, security_status, created_at, updated_at from skills where id = $1`, rev.SkillID).Scan(&skill.ID, &skill.TenantID, &skill.OwnerID, &skill.Name, &skill.SourceURL, &skillTags, &skill.LatestRevisionID, &skill.ActiveRevisionID, &skill.SecurityStatus, &skill.CreatedAt, &skill.UpdatedAt); err != nil {
			return domain.Approval{}, domain.Agent{}, domain.Skill{}, err
		}
		skill.Tags = decodeTags(skillTags)
		if err := tx.Commit(); err != nil {
			return domain.Approval{}, domain.Agent{}, domain.Skill{}, err
		}
		return approval, domain.Agent{}, skill, nil
	default:
		return domain.Approval{}, domain.Agent{}, domain.Skill{}, fmt.Errorf("unsupported target type")
	}
}
