package domain

import "time"

type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"` // "admin" or "user"
	Disabled  bool      `json:"disabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Agent struct {
	ID          string    `json:"id"`
	OwnerUserID string    `json:"owner_user_id"`
	Name        string    `json:"name"`
	Runtime     string    `json:"runtime"` // "openclaw" or "nanoclaw"
	Model       string    `json:"model"`
	State       string    `json:"state"` // idle, starting, running, stopped, error
	LastError   string    `json:"last_error,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
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
	Name    string `json:"name"`
	Runtime string `json:"runtime"`
	Model   string `json:"model"`
}

type Message struct {
	ID        string    `json:"id"`
	AgentID   string    `json:"agent_id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}
