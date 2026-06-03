package store

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/mipopov/shclop/internal/domain"
	"github.com/mipopov/shclop/migrations"
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
	if err := runMigrations(ctx, db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}
	return &Postgres{db: db}, nil
}

func runMigrations(ctx context.Context, db *sql.DB) error {
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return err
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	for _, name := range files {
		data, err := migrations.FS.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		if _, err := db.ExecContext(ctx, string(data)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
	}
	return nil
}

func (p *Postgres) Close() error { return p.db.Close() }

// --- Users ---

func (p *Postgres) CreateUser(ctx context.Context, username, passwordHash, role string) (domain.User, error) {
	id, err := newID()
	if err != nil {
		return domain.User{}, err
	}
	now := time.Now().UTC()
	var u domain.User
	err = p.db.QueryRowContext(ctx,
		`insert into users (id, username, password_hash, role, disabled, created_at, updated_at) values ($1,$2,$3,$4,false,$5,$6)
		 returning id, username, role, disabled, created_at, updated_at`,
		id, username, passwordHash, role, now, now,
	).Scan(&u.ID, &u.Username, &u.Role, &u.Disabled, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			return domain.User{}, ErrConflict
		}
		return domain.User{}, fmt.Errorf("create user: %w", err)
	}
	return u, nil
}

func (p *Postgres) GetUser(ctx context.Context, userID string) (domain.User, error) {
	var u domain.User
	err := p.db.QueryRowContext(ctx,
		`select id, username, role, disabled, created_at, updated_at from users where id = $1`, userID,
	).Scan(&u.ID, &u.Username, &u.Role, &u.Disabled, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return domain.User{}, ErrNotFound
	}
	if err != nil {
		return domain.User{}, err
	}
	return u, nil
}

func (p *Postgres) GetUserByUsername(ctx context.Context, username string) (domain.User, error) {
	var u domain.User
	err := p.db.QueryRowContext(ctx,
		`select id, username, role, disabled, created_at, updated_at from users where username = $1`, username,
	).Scan(&u.ID, &u.Username, &u.Role, &u.Disabled, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return domain.User{}, ErrNotFound
	}
	if err != nil {
		return domain.User{}, err
	}
	return u, nil
}

func (p *Postgres) ListUsers(ctx context.Context) ([]domain.User, error) {
	rows, err := p.db.QueryContext(ctx,
		`select id, username, role, disabled, created_at, updated_at from users order by created_at asc, id asc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.User
	for rows.Next() {
		var u domain.User
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.Disabled, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (p *Postgres) UpdateUser(ctx context.Context, userID string, disabled *bool, role *string) (domain.User, error) {
	// Build dynamic update
	setClauses := ""
	args := []any{}
	argIdx := 1
	if disabled != nil {
		setClauses += fmt.Sprintf("disabled = $%d, ", argIdx)
		args = append(args, *disabled)
		argIdx++
	}
	if role != nil {
		setClauses += fmt.Sprintf("role = $%d, ", argIdx)
		args = append(args, *role)
		argIdx++
	}
	now := time.Now().UTC()
	setClauses += fmt.Sprintf("updated_at = $%d", argIdx)
	args = append(args, now)
	argIdx++

	query := fmt.Sprintf(`update users set %s where id = $%d returning id, username, role, disabled, created_at, updated_at`, setClauses, argIdx)
	args = append(args, userID)

	var u domain.User
	err := p.db.QueryRowContext(ctx, query, args...).Scan(&u.ID, &u.Username, &u.Role, &u.Disabled, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return domain.User{}, ErrNotFound
	}
	if err != nil {
		return domain.User{}, err
	}
	return u, nil
}

// GetPasswordHash retrieves the bcrypt hash for a username.
func (p *Postgres) GetPasswordHash(ctx context.Context, username string) (string, error) {
	var hash string
	err := p.db.QueryRowContext(ctx, `select password_hash from users where username = $1`, username).Scan(&hash)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return hash, nil
}

// SetPasswordHash updates the bcrypt hash for a user.
func (p *Postgres) SetPasswordHash(ctx context.Context, username, hash string) error {
	_, err := p.db.ExecContext(ctx, `update users set password_hash = $2, updated_at = $3 where username = $1`,
		username, hash, time.Now().UTC())
	return err
}

// --- Agents ---

func (p *Postgres) CreateAgent(ctx context.Context, ownerUserID, name, description, runtime, model, systemPrompt string) (domain.Agent, error) {
	id, err := newID()
	if err != nil {
		return domain.Agent{}, err
	}
	now := time.Now().UTC()
	var a domain.Agent
	err = p.db.QueryRowContext(ctx,
		`insert into agents (id, owner_user_id, name, description, runtime, model, system_prompt, state, created_at, updated_at)
		 values ($1,$2,$3,$4,$5,$6,$7,'idle',$8,$9)
		 returning id, owner_user_id, name, description, runtime, model, system_prompt, state, last_error, created_at, updated_at`,
		id, ownerUserID, name, description, runtime, model, systemPrompt, now, now,
	).Scan(&a.ID, &a.OwnerUserID, &a.Name, &a.Description, &a.Runtime, &a.Model, &a.SystemPrompt, &a.State, &a.LastError, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return domain.Agent{}, fmt.Errorf("create agent: %w", err)
	}
	return a, nil
}

func (p *Postgres) GetAgent(ctx context.Context, agentID string) (domain.Agent, error) {
	var a domain.Agent
	err := p.db.QueryRowContext(ctx,
		`select id, owner_user_id, name, description, runtime, model, system_prompt, state, last_error, created_at, updated_at from agents where id = $1`, agentID,
	).Scan(&a.ID, &a.OwnerUserID, &a.Name, &a.Description, &a.Runtime, &a.Model, &a.SystemPrompt, &a.State, &a.LastError, &a.CreatedAt, &a.UpdatedAt)
	if err == sql.ErrNoRows {
		return domain.Agent{}, ErrNotFound
	}
	if err != nil {
		return domain.Agent{}, err
	}
	return a, nil
}

func (p *Postgres) ListAgents(ctx context.Context, ownerUserID string) ([]domain.Agent, error) {
	rows, err := p.db.QueryContext(ctx,
		`select id, owner_user_id, name, description, runtime, model, system_prompt, state, last_error, created_at, updated_at from agents where owner_user_id = $1 order by created_at asc, id asc`, ownerUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Agent
	for rows.Next() {
		var a domain.Agent
		if err := rows.Scan(&a.ID, &a.OwnerUserID, &a.Name, &a.Description, &a.Runtime, &a.Model, &a.SystemPrompt, &a.State, &a.LastError, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (p *Postgres) UpdateAgentState(ctx context.Context, agentID, state string) (domain.Agent, error) {
	var a domain.Agent
	err := p.db.QueryRowContext(ctx,
		`update agents set state = $2, updated_at = $3 where id = $1
		 returning id, owner_user_id, name, description, runtime, model, system_prompt, state, last_error, created_at, updated_at`,
		agentID, state, time.Now().UTC(),
	).Scan(&a.ID, &a.OwnerUserID, &a.Name, &a.Description, &a.Runtime, &a.Model, &a.SystemPrompt, &a.State, &a.LastError, &a.CreatedAt, &a.UpdatedAt)
	if err == sql.ErrNoRows {
		return domain.Agent{}, ErrNotFound
	}
	if err != nil {
		return domain.Agent{}, err
	}
	return a, nil
}

func (p *Postgres) UpdateAgentError(ctx context.Context, agentID, lastError string) (domain.Agent, error) {
	var a domain.Agent
	err := p.db.QueryRowContext(ctx,
		`update agents set last_error = $2, updated_at = $3 where id = $1
		 returning id, owner_user_id, name, description, runtime, model, system_prompt, state, last_error, created_at, updated_at`,
		agentID, lastError, time.Now().UTC(),
	).Scan(&a.ID, &a.OwnerUserID, &a.Name, &a.Description, &a.Runtime, &a.Model, &a.SystemPrompt, &a.State, &a.LastError, &a.CreatedAt, &a.UpdatedAt)
	if err == sql.ErrNoRows {
		return domain.Agent{}, ErrNotFound
	}
	if err != nil {
		return domain.Agent{}, err
	}
	return a, nil
}

func (p *Postgres) DeleteAgent(ctx context.Context, agentID string) error {
	_, err := p.db.ExecContext(ctx, `delete from agent_integrations where agent_id = $1`, agentID)
	if err != nil {
		return fmt.Errorf("delete agent integrations: %w", err)
	}
	res, err := p.db.ExecContext(ctx, `delete from agents where id = $1`, agentID)
	if err != nil {
		return fmt.Errorf("delete agent: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// --- LLM Models ---

func (p *Postgres) CreateLLMModel(ctx context.Context, displayName, providerModel string, enabled bool) (domain.LLMModel, error) {
	id, err := newID()
	if err != nil {
		return domain.LLMModel{}, err
	}
	now := time.Now().UTC()
	var m domain.LLMModel
	err = p.db.QueryRowContext(ctx,
		`insert into llm_models (id, display_name, provider_model, enabled, created_at, updated_at)
		 values ($1,$2,$3,$4,$5,$6)
		 returning id, display_name, provider_model, enabled, created_at, updated_at`,
		id, displayName, providerModel, enabled, now, now,
	).Scan(&m.ID, &m.DisplayName, &m.ProviderModel, &m.Enabled, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return domain.LLMModel{}, fmt.Errorf("create llm model: %w", err)
	}
	return m, nil
}

func (p *Postgres) GetLLMModel(ctx context.Context, modelID string) (domain.LLMModel, error) {
	var m domain.LLMModel
	err := p.db.QueryRowContext(ctx,
		`select id, display_name, provider_model, enabled, created_at, updated_at from llm_models where id = $1`, modelID,
	).Scan(&m.ID, &m.DisplayName, &m.ProviderModel, &m.Enabled, &m.CreatedAt, &m.UpdatedAt)
	if err == sql.ErrNoRows {
		return domain.LLMModel{}, ErrNotFound
	}
	if err != nil {
		return domain.LLMModel{}, err
	}
	return m, nil
}

func (p *Postgres) ListLLMModels(ctx context.Context) ([]domain.LLMModel, error) {
	rows, err := p.db.QueryContext(ctx,
		`select id, display_name, provider_model, enabled, created_at, updated_at from llm_models order by created_at asc, id asc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.LLMModel
	for rows.Next() {
		var m domain.LLMModel
		if err := rows.Scan(&m.ID, &m.DisplayName, &m.ProviderModel, &m.Enabled, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (p *Postgres) UpdateLLMModel(ctx context.Context, modelID string, displayName, providerModel *string, enabled *bool) (domain.LLMModel, error) {
	setClauses := ""
	args := []any{}
	argIdx := 1
	if displayName != nil {
		setClauses += fmt.Sprintf("display_name = $%d, ", argIdx)
		args = append(args, *displayName)
		argIdx++
	}
	if providerModel != nil {
		setClauses += fmt.Sprintf("provider_model = $%d, ", argIdx)
		args = append(args, *providerModel)
		argIdx++
	}
	if enabled != nil {
		setClauses += fmt.Sprintf("enabled = $%d, ", argIdx)
		args = append(args, *enabled)
		argIdx++
	}
	now := time.Now().UTC()
	setClauses += fmt.Sprintf("updated_at = $%d", argIdx)
	args = append(args, now)
	argIdx++

	query := fmt.Sprintf(`update llm_models set %s where id = $%d returning id, display_name, provider_model, enabled, created_at, updated_at`, setClauses, argIdx)
	args = append(args, modelID)

	var m domain.LLMModel
	err := p.db.QueryRowContext(ctx, query, args...).Scan(&m.ID, &m.DisplayName, &m.ProviderModel, &m.Enabled, &m.CreatedAt, &m.UpdatedAt)
	if err == sql.ErrNoRows {
		return domain.LLMModel{}, ErrNotFound
	}
	if err != nil {
		return domain.LLMModel{}, err
	}
	return m, nil
}

// --- LLM Gateway ---

func (p *Postgres) GetLLMGatewaySettings(ctx context.Context) (domain.LLMGatewaySettings, error) {
	var s domain.LLMGatewaySettings
	err := p.db.QueryRowContext(ctx,
		`select enabled, base_url, secret_name, secret_key, updated_at from llm_gateway_settings where id = 1`,
	).Scan(&s.Enabled, &s.BaseURL, &s.SecretName, &s.SecretKey, &s.UpdatedAt)
	if err == sql.ErrNoRows {
		return domain.LLMGatewaySettings{}, nil
	}
	if err != nil {
		return domain.LLMGatewaySettings{}, err
	}
	return s, nil
}

func (p *Postgres) UpsertLLMGatewaySettings(ctx context.Context, enabled bool, baseURL, secretName, secretKey string) (domain.LLMGatewaySettings, error) {
	now := time.Now().UTC()
	var s domain.LLMGatewaySettings
	err := p.db.QueryRowContext(ctx,
		`insert into llm_gateway_settings (id, enabled, base_url, secret_name, secret_key, updated_at)
		 values (1, $1, $2, $3, $4, $5)
		 on conflict (id) do update set enabled = $1, base_url = $2, secret_name = $3, secret_key = $4, updated_at = $5
		 returning enabled, base_url, secret_name, secret_key, updated_at`,
		enabled, baseURL, secretName, secretKey, now,
	).Scan(&s.Enabled, &s.BaseURL, &s.SecretName, &s.SecretKey, &s.UpdatedAt)
	if err != nil {
		return domain.LLMGatewaySettings{}, fmt.Errorf("upsert llm gateway: %w", err)
	}
	return s, nil
}

// --- Integrations: Connections ---

func (p *Postgres) UpsertIntegrationConnection(ctx context.Context, connection domain.IntegrationConnection) (domain.IntegrationConnection, error) {
	if connection.Scope == "" {
		connection.Scope = "user"
	}
	if connection.ScopeID == "" {
		connection.ScopeID = connection.UserID
	}
	now := time.Now().UTC()
	var c domain.IntegrationConnection
	err := p.db.QueryRowContext(ctx,
		`insert into integration_connections (provider_id, user_id, scope, scope_id, external_account_id, external_login, account_type, status, secret, revision, created_at, updated_at)
		 values ($1, $2, $3, $4, $5, $6, $7, $8, $9, coalesce(nullif($10, 0), 1), $11, $11)
		 on conflict (provider_id, scope, scope_id) do update set
		   external_account_id = $5, external_login = $6, account_type = $7, status = $8, secret = $9,
		   revision = integration_connections.revision + 1, updated_at = $11
		 returning provider_id, user_id, scope, scope_id, external_account_id, external_login, account_type, status, secret, revision, created_at, updated_at`,
		connection.ProviderID, connection.UserID, connection.Scope, connection.ScopeID,
		connection.ExternalAccountID, connection.ExternalLogin,
		connection.AccountType, connection.Status, connection.Secret, connection.Revision, now,
	).Scan(&c.ProviderID, &c.UserID, &c.Scope, &c.ScopeID, &c.ExternalAccountID, &c.ExternalLogin, &c.AccountType,
		&c.Status, &c.Secret, &c.Revision, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return domain.IntegrationConnection{}, fmt.Errorf("upsert integration connection: %w", err)
	}
	return c, nil
}

func (p *Postgres) GetIntegrationConnection(ctx context.Context, userID, providerID string) (domain.IntegrationConnection, error) {
	var c domain.IntegrationConnection
	err := p.db.QueryRowContext(ctx,
		`select provider_id, user_id, scope, scope_id, external_account_id, external_login, account_type, status, secret, revision, created_at, updated_at
		 from integration_connections where provider_id = $1 and scope = 'user' and scope_id = $2`,
		providerID, userID,
	).Scan(&c.ProviderID, &c.UserID, &c.Scope, &c.ScopeID, &c.ExternalAccountID, &c.ExternalLogin, &c.AccountType,
		&c.Status, &c.Secret, &c.Revision, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return domain.IntegrationConnection{}, ErrNotFound
	}
	if err != nil {
		return domain.IntegrationConnection{}, err
	}
	return c, nil
}

func (p *Postgres) ListUserIntegrationProviderIDs(ctx context.Context, userID string) ([]string, error) {
	rows, err := p.db.QueryContext(ctx,
		`SELECT DISTINCT provider_id FROM integration_connections
		 WHERE scope = 'user' AND scope_id = $1
		 ORDER BY provider_id`,
		userID)
	if err != nil {
		return nil, fmt.Errorf("list user integration provider ids: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (p *Postgres) DeleteIntegrationConnection(ctx context.Context, userID, providerID string) error {
	_, err := p.db.ExecContext(ctx,
		`delete from integration_connections where provider_id = $1 and scope = 'user' and scope_id = $2`,
		providerID, userID)
	if err != nil {
		return fmt.Errorf("delete integration connection: %w", err)
	}
	_, _ = p.db.ExecContext(ctx,
		`delete from agent_integrations where provider_id = $1 and agent_id in (
		   select id from agents where owner_user_id = $2
		 )`, providerID, userID)
	return nil
}

// --- Integrations: Agent Integrations ---

func (p *Postgres) UpsertAgentIntegration(ctx context.Context, agentID, providerID string, enabled bool, revision int64, status string) (domain.AgentIntegration, error) {
	now := time.Now().UTC()
	var ai domain.AgentIntegration
	err := p.db.QueryRowContext(ctx,
		`insert into agent_integrations (agent_id, provider_id, enabled, revision, status, created_at, updated_at)
		 values ($1, $2, $3, coalesce(nullif($4, 0), 1), $5, $6, $6)
		 on conflict (agent_id, provider_id) do update set
		   enabled = $3, revision = agent_integrations.revision + 1, status = $5, updated_at = $6
		 returning agent_id, provider_id, enabled, revision, status, created_at, updated_at`,
		agentID, providerID, enabled, revision, status, now,
	).Scan(&ai.AgentID, &ai.ProviderID, &ai.Enabled, &ai.Revision, &ai.Status, &ai.CreatedAt, &ai.UpdatedAt)
	if err != nil {
		return domain.AgentIntegration{}, fmt.Errorf("upsert agent integration: %w", err)
	}
	return ai, nil
}

func (p *Postgres) ListAgentIntegrations(ctx context.Context, agentID string) ([]domain.AgentIntegration, error) {
	rows, err := p.db.QueryContext(ctx,
		`select agent_id, provider_id, enabled, revision, status, created_at, updated_at
		 from agent_integrations where agent_id = $1 order by provider_id asc`, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.AgentIntegration
	for rows.Next() {
		var ai domain.AgentIntegration
		if err := rows.Scan(&ai.AgentID, &ai.ProviderID, &ai.Enabled, &ai.Revision, &ai.Status, &ai.CreatedAt, &ai.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, ai)
	}
	if out == nil {
		return []domain.AgentIntegration{}, nil
	}
	return out, rows.Err()
}

func (p *Postgres) GetAgentIntegration(ctx context.Context, agentID, providerID string) (domain.AgentIntegration, error) {
	var ai domain.AgentIntegration
	err := p.db.QueryRowContext(ctx,
		`select agent_id, provider_id, enabled, revision, status, created_at, updated_at
		 from agent_integrations where agent_id = $1 and provider_id = $2`,
		agentID, providerID,
	).Scan(&ai.AgentID, &ai.ProviderID, &ai.Enabled, &ai.Revision, &ai.Status, &ai.CreatedAt, &ai.UpdatedAt)
	if err == sql.ErrNoRows {
		return domain.AgentIntegration{}, ErrNotFound
	}
	if err != nil {
		return domain.AgentIntegration{}, err
	}
	return ai, nil
}

// --- Plugin Manifests ---

func (p *Postgres) ListPluginManifests(ctx context.Context, enabledOnly bool) ([]domain.PluginManifest, error) {
	query := `select id, yaml, enabled, revision, created_at, updated_at from plugin_manifests`
	if enabledOnly {
		query += ` where enabled = true`
	}
	query += ` order by updated_at asc, id asc`
	rows, err := p.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.PluginManifest
	for rows.Next() {
		var pm domain.PluginManifest
		if err := rows.Scan(&pm.ID, &pm.YAML, &pm.Enabled, &pm.Revision, &pm.CreatedAt, &pm.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, pm)
	}
	return out, rows.Err()
}

func (p *Postgres) GetPluginManifest(ctx context.Context, id string) (domain.PluginManifest, error) {
	var pm domain.PluginManifest
	err := p.db.QueryRowContext(ctx,
		`select id, yaml, enabled, revision, created_at, updated_at from plugin_manifests where id = $1`, id,
	).Scan(&pm.ID, &pm.YAML, &pm.Enabled, &pm.Revision, &pm.CreatedAt, &pm.UpdatedAt)
	if err == sql.ErrNoRows {
		return domain.PluginManifest{}, ErrNotFound
	}
	if err != nil {
		return domain.PluginManifest{}, err
	}
	return pm, nil
}

func (p *Postgres) UpsertPluginManifest(ctx context.Context, m domain.PluginManifest) (domain.PluginManifest, error) {
	var pm domain.PluginManifest
	err := p.db.QueryRowContext(ctx,
		`insert into plugin_manifests (id, yaml, enabled, revision, created_at, updated_at)
		 values ($1, $2, $3, 1, now(), now())
		 on conflict (id) do update set
		   yaml = EXCLUDED.yaml, enabled = EXCLUDED.enabled,
		   revision = plugin_manifests.revision + 1, updated_at = now()
		 returning id, yaml, enabled, revision, created_at, updated_at`,
		m.ID, m.YAML, m.Enabled,
	).Scan(&pm.ID, &pm.YAML, &pm.Enabled, &pm.Revision, &pm.CreatedAt, &pm.UpdatedAt)
	if err != nil {
		return domain.PluginManifest{}, fmt.Errorf("upsert plugin manifest: %w", err)
	}
	return pm, nil
}

func (p *Postgres) DeletePluginManifest(ctx context.Context, id string) error {
	res, err := p.db.ExecContext(ctx, `delete from plugin_manifests where id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete plugin manifest: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// --- User Identities ---

func (p *Postgres) GetUserIDByIdentity(ctx context.Context, providerName, subject string) (string, bool, error) {
	var userID string
	err := p.db.QueryRowContext(ctx,
		`select user_id from user_identities where provider_name = $1 and subject = $2`,
		providerName, subject,
	).Scan(&userID)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return userID, true, nil
}

func (p *Postgres) TouchIdentityLogin(ctx context.Context, providerName, subject string) error {
	res, err := p.db.ExecContext(ctx,
		`update user_identities set last_login_at = now() where provider_name = $1 and subject = $2`,
		providerName, subject,
	)
	if err != nil {
		return fmt.Errorf("touch identity login: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *Postgres) CreateUserAndIdentity(ctx context.Context, args CreateUserAndIdentityArgs) (domain.User, error) {
	userID, err := newID()
	if err != nil {
		return domain.User{}, err
	}
	now := time.Now().UTC()

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.User{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var u domain.User
	err = tx.QueryRowContext(ctx,
		`insert into users (id, username, password_hash, role, disabled, created_at, updated_at)
		 values ($1, $2, '', $3, false, $4, $4)
		 returning id, username, role, disabled, created_at, updated_at`,
		userID, args.Username, args.Role, now,
	).Scan(&u.ID, &u.Username, &u.Role, &u.Disabled, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			return domain.User{}, ErrConflict
		}
		return domain.User{}, fmt.Errorf("create user in tx: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		`insert into user_identities (provider_name, subject, user_id, email, display_name, linked_at)
		 values ($1, $2, $3, $4, $5, $6)`,
		args.ProviderName, args.Subject, userID, args.Email, args.DisplayName, now,
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			return domain.User{}, ErrConflict
		}
		return domain.User{}, fmt.Errorf("insert identity in tx: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return domain.User{}, fmt.Errorf("commit tx: %w", err)
	}
	return u, nil
}

func (p *Postgres) LinkIdentityToUser(ctx context.Context, userID, providerName, subject, email, displayName string) error {
	var exists bool
	err := p.db.QueryRowContext(ctx, `select exists(select 1 from users where id = $1)`, userID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check user exists: %w", err)
	}
	if !exists {
		return ErrNotFound
	}

	_, err = p.db.ExecContext(ctx,
		`insert into user_identities (provider_name, subject, user_id, email, display_name, linked_at)
		 values ($1, $2, $3, $4, $5, now())`,
		providerName, subject, userID, email, displayName,
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			return ErrConflict
		}
		return fmt.Errorf("link identity: %w", err)
	}
	return nil
}

func (p *Postgres) ListUserIdentities(ctx context.Context, userID string) ([]domain.UserIdentity, error) {
	rows, err := p.db.QueryContext(ctx,
		`select provider_name, subject, user_id, email, display_name, linked_at, last_login_at
		 from user_identities where user_id = $1 order by provider_name asc, subject asc`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.UserIdentity
	for rows.Next() {
		var id domain.UserIdentity
		var lastLogin sql.NullTime
		if err := rows.Scan(&id.ProviderName, &id.Subject, &id.UserID, &id.Email, &id.DisplayName, &id.LinkedAt, &lastLogin); err != nil {
			return nil, err
		}
		if lastLogin.Valid {
			t := lastLogin.Time
			id.LastLoginAt = &t
		}
		out = append(out, id)
	}
	if out == nil {
		return []domain.UserIdentity{}, nil
	}
	return out, rows.Err()
}

func (p *Postgres) UnlinkIdentity(ctx context.Context, userID, providerName, subject string) error {
	res, err := p.db.ExecContext(ctx,
		`delete from user_identities where user_id = $1 and provider_name = $2 and subject = $3`,
		userID, providerName, subject,
	)
	if err != nil {
		return fmt.Errorf("unlink identity: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// --- App Settings ---

func (p *Postgres) GetAppSetting(ctx context.Context, key string) (string, error) {
	var value string
	err := p.db.QueryRowContext(ctx, `select value from app_settings where key = $1`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get app setting: %w", err)
	}
	return value, nil
}

func (p *Postgres) SetAppSetting(ctx context.Context, key, value string) error {
	_, err := p.db.ExecContext(ctx,
		`insert into app_settings (key, value, updated_at) values ($1, $2, now())
		 on conflict (key) do update set value = $2, updated_at = now()`,
		key, value,
	)
	if err != nil {
		return fmt.Errorf("set app setting: %w", err)
	}
	return nil
}

func (p *Postgres) ListAppSettings(ctx context.Context, prefix string) (map[string]string, error) {
	var rows *sql.Rows
	var err error
	if prefix == "" {
		rows, err = p.db.QueryContext(ctx, `select key, value from app_settings order by key asc`)
	} else {
		rows, err = p.db.QueryContext(ctx, `select key, value from app_settings where key like $1 || '%' order by key asc`, prefix)
	}
	if err != nil {
		return nil, fmt.Errorf("list app settings: %w", err)
	}
	defer rows.Close()
	out := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

// --- Bootstrap ---

func (p *Postgres) BootstrapAdmin(ctx context.Context, username, passwordHash string) error {
	id, err := newID()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	result, err := p.db.ExecContext(ctx,
		`insert into users (id, username, password_hash, role, disabled, created_at, updated_at)
		 values ($1, $2, $3, 'admin', false, $4, $4)
		 on conflict (username) do update set password_hash = case when $3 != '' then $3 else users.password_hash end,
		   role = 'admin', disabled = false, updated_at = $4`,
		id, username, passwordHash, now)
	if err != nil {
		return fmt.Errorf("bootstrap admin: %w", err)
	}
	_ = result
	return nil
}
