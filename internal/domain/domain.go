package domain

import "time"

// IntegrationConnection represents a user's connection to an external provider
// (e.g., GitHub). The Secret field contains the encrypted credential token.
type IntegrationConnection struct {
	ProviderID        string    `json:"provider_id"`
	UserID            string    `json:"user_id"`
	ExternalAccountID string    `json:"external_account_id"` // e.g. GitHub user ID
	ExternalLogin     string    `json:"external_login"`      // e.g. GitHub login/username
	AccountType       string    `json:"account_type"`        // e.g. "User" or "Organization"
	Status            string    `json:"status"`              // "connected" or "error"
	Secret            string    `json:"-"`                   // encrypted token; never serialized
	Revision          int64     `json:"revision"`
	UpdatedAt         time.Time `json:"updated_at"`
	CreatedAt         time.Time `json:"created_at"`
}

// AgentIntegration links an agent to an integration provider's connection.
type AgentIntegration struct {
	AgentID    string    `json:"agent_id"`
	ProviderID string    `json:"provider_id"`
	Enabled    bool      `json:"enabled"`
	Revision   int64     `json:"revision"`
	Status     string    `json:"status"` // "active", "error", "disabled"
	UpdatedAt  time.Time `json:"updated_at"`
	CreatedAt  time.Time `json:"created_at"`
}

// IntegrationSummary is the response shape returned by the integrations API.
// It never includes secrets/tokens.
type IntegrationSummary struct {
	Providers []ProviderSummary `json:"providers"`
}

// ProviderSummary describes an available integration provider and the current
// user's connection state.
type ProviderSummary struct {
	ProviderID    string                  `json:"provider_id"`
	Name          string                  `json:"name"`
	Connected     bool                    `json:"connected"`
	Connection    *ConnectionMetadata     `json:"connection,omitempty"`
	AgentBindings []AgentBindingSummary   `json:"agent_bindings"`
}

// ConnectionMetadata holds non-sensitive details about a connection.
type ConnectionMetadata struct {
	ExternalAccountID string `json:"external_account_id"`
	ExternalLogin     string `json:"external_login"`
	AccountType       string `json:"account_type"`
	Status            string `json:"status"`
	Revision          int64  `json:"revision"`
}

// AgentBindingSummary is the per-agent integration toggle state.
type AgentBindingSummary struct {
	AgentID  string `json:"agent_id"`
	Enabled  bool   `json:"enabled"`
	Revision int64  `json:"revision"`
	Status   string `json:"status"`
}

type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"` // "admin" or "user"
	Disabled  bool      `json:"disabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Agent struct {
	ID           string    `json:"id"`
	OwnerUserID  string    `json:"owner_user_id"`
	Name         string    `json:"name"`
	Description  string    `json:"description,omitempty"`
	Runtime      string    `json:"runtime"` // "openclaw" or "nanoclaw"
	Model        string    `json:"model"`
	SystemPrompt string    `json:"system_prompt,omitempty"`
	State        string    `json:"state"` // idle, starting, running, stopped, error
	LastError    string    `json:"last_error,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type LLMModel struct {
	ID            string    `json:"id"`
	DisplayName   string    `json:"display_name"`
	ProviderModel string    `json:"provider_model"`
	Enabled       bool      `json:"enabled"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type LLMGatewaySettings struct {
	Enabled    bool      `json:"enabled"`
	BaseURL    string    `json:"base_url"`
	SecretName string    `json:"secret_name"`
	SecretKey  string    `json:"secret_key"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type AdminOverview struct {
	Runtime       AdminRuntimeConfig `json:"runtime"`
	Observability AdminObservability `json:"observability"`
	Health        AdminHealthStatus  `json:"health"`
}

type AdminRuntimeConfig struct {
	Provider         string            `json:"provider"`
	Namespace        string            `json:"namespace"`
	RuntimeClassName string            `json:"runtime_class_name"`
	Images           map[string]string `json:"images"`
}

type AdminObservability struct {
	MetricsEnabled bool   `json:"metrics_enabled"`
	LoggingEnabled bool   `json:"logging_enabled"`
	GrafanaURL     string `json:"grafana_url,omitempty"`
}

type AdminHealthStatus struct {
	Healthz string `json:"healthz"`
	Readyz  string `json:"readyz"`
}

// CreateAgentInput is used when creating an agent via the API.
type CreateAgentInput struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	Runtime      string `json:"runtime"`
	Model        string `json:"model"`
	SystemPrompt string `json:"system_prompt"`
}

type Message struct {
	ID        string    `json:"id"`
	AgentID   string    `json:"agent_id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}
