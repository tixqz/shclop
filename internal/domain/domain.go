package domain

import "time"

type User struct {
	ID          string   `json:"id"`
	Username    string   `json:"username"`
	Email       string   `json:"email,omitempty"`
	DisplayName string   `json:"display_name,omitempty"`
	TenantID    string   `json:"tenant_id,omitempty"`
	TeamIDs     []string `json:"team_ids,omitempty"`
	Roles       []string `json:"roles,omitempty"`
}

type Agent struct {
	ID               string    `json:"id"`
	OwnerID          string    `json:"owner_id"`
	TenantID         string    `json:"tenant_id,omitempty"`
	Name             string    `json:"name"`
	Model            string    `json:"model,omitempty"`
	Purpose          string    `json:"purpose,omitempty"`
	Tags             []string  `json:"tags,omitempty"`
	State            string    `json:"state"`
	LatestRevisionID string    `json:"latest_revision_id,omitempty"`
	ActiveRevisionID string    `json:"active_revision_id,omitempty"`
	SecurityStatus   string    `json:"security_status,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

type CreateAgentInput struct {
	OwnerID, TenantID, Name, Model, Purpose string
	Tags                                    []string
}

type AgentRevision struct {
	ID             string    `json:"id"`
	AgentID        string    `json:"agent_id"`
	RevisionNumber int       `json:"revision_number"`
	Name           string    `json:"name"`
	Model          string    `json:"model,omitempty"`
	Purpose        string    `json:"purpose,omitempty"`
	Tags           []string  `json:"tags,omitempty"`
	ContentDigest  string    `json:"content_digest"`
	SecurityStatus string    `json:"security_status"`
	CreatedBy      string    `json:"created_by"`
	CreatedAt      time.Time `json:"created_at"`
}

type AuditRun struct {
	ID               string    `json:"id"`
	TargetType       string    `json:"target_type"`
	TargetRevisionID string    `json:"target_revision_id"`
	ContentDigest    string    `json:"content_digest"`
	PolicyVersion    int       `json:"policy_version"`
	ScannerVersion   string    `json:"scanner_version"`
	RiskLevel        string    `json:"risk_level"`
	Decision         string    `json:"decision"`
	Findings         string    `json:"findings_json"`
	CreatedBy        string    `json:"created_by"`
	CreatedAt        time.Time `json:"created_at"`
}

type Approval struct {
	ID         string    `json:"id"`
	TargetType string    `json:"target_type"`
	TargetID   string    `json:"target_id"`
	ActorID    string    `json:"actor_id"`
	Decision   string    `json:"decision"`
	CreatedAt  time.Time `json:"created_at"`
}

type CreateSkillInput struct {
	OwnerID     string
	TenantID    string
	Name        string
	Description string
	Content     string
	Tags        []string
	Source      string
	SourceURL   string
}

type Skill struct {
	ID               string    `json:"id"`
	TenantID         string    `json:"tenant_id,omitempty"`
	OwnerID          string    `json:"owner_id"`
	Name             string    `json:"name"`
	SourceURL        string    `json:"source_url,omitempty"`
	Tags             []string  `json:"tags,omitempty"`
	LatestRevisionID string    `json:"latest_revision_id"`
	ActiveRevisionID string    `json:"active_revision_id"`
	SecurityStatus   string    `json:"security_status,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type SkillRevision struct {
	ID             string    `json:"id"`
	SkillID        string    `json:"skill_id"`
	RevisionNumber int       `json:"revision_number"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	Content        string    `json:"content"`
	Tags           []string  `json:"tags,omitempty"`
	Source         string    `json:"source"`
	SourceURL      string    `json:"source_url,omitempty"`
	ContentDigest  string    `json:"content_digest"`
	SecurityStatus string    `json:"security_status"`
	CreatedBy      string    `json:"created_by"`
	CreatedAt      time.Time `json:"created_at"`
}

type Workspace struct {
	ID          string    `json:"id"`
	OwnerID     string    `json:"owner_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Message struct {
	ID        string    `json:"id"`
	AgentID   string    `json:"agent_id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}
