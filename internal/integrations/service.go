package integrations

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/mipopov/shclop/internal/domain"
	"github.com/mipopov/shclop/internal/plugins"
	"github.com/mipopov/shclop/internal/store"
)

// Store is the subset of store.Store used by the integration service.
type Store = store.Store

// AgentRuntimeResolution holds the resolved env-vars and MCP server list for
// an agent start request.
type AgentRuntimeResolution struct {
	Env        map[string]string
	MCPServers []plugins.ResolvedMCPServer
}

// Service manages plugin-driven integrations: credential storage, connect/disconnect
// lifecycle, and runtime env + MCP resolution.
type Service struct {
	store    Store
	secret   *SecretBox
	registry plugins.Registry
	resolver *plugins.MCPResolver
	httpEval *plugins.HTTPEvaluator
	sidecar  *plugins.SidecarClient
	logger   *slog.Logger
}

// NewService creates an integration service backed by the given registry.
// resolver may be nil only when MCP ref-based servers are not used (e.g. external-only or tests).
func NewService(st Store, secret *SecretBox, registry plugins.Registry, resolver *plugins.MCPResolver, logger *slog.Logger) *Service {
	return &Service{
		store:    st,
		secret:   secret,
		registry: registry,
		resolver: resolver,
		httpEval: plugins.NewHTTPEvaluator(),
		sidecar:  plugins.NewSidecarClient(),
		logger:   logger,
	}
}

// Connect validates the supplied fields against the plugin spec, then encrypts
// and stores the credential. fields must contain at least "token" for pat_token
// auth kind. Returns the stored connection on success.
func (s *Service) Connect(ctx context.Context, userID, providerID string, fields map[string]string) (domain.IntegrationConnection, error) {
	r, ok := s.registry.Get(providerID)
	if !ok {
		return domain.IntegrationConnection{}, fmt.Errorf("plugin %s not registered", providerID)
	}
	m := r.Manifest
	vars := plugins.TemplateContext{Token: fields["token"], Fields: fields}

	var result plugins.ValidationResult
	var err error
	switch m.Spec.PluginKind {
	case "http_template":
		result, err = s.httpEval.Validate(ctx, &m, vars)
	case "sidecar":
		result, err = s.sidecar.Validate(ctx, &m, vars)
	default:
		return domain.IntegrationConnection{}, fmt.Errorf("plugin %s: unsupported kind %q", providerID, m.Spec.PluginKind)
	}
	if err != nil {
		return domain.IntegrationConnection{}, fmt.Errorf("plugin %s: validation failed: %w", providerID, err)
	}

	encrypted, err := s.secret.EncryptToString([]byte(fields["token"]))
	if err != nil {
		return domain.IntegrationConnection{}, fmt.Errorf("encrypt token: %w", err)
	}

	conn := domain.IntegrationConnection{
		ProviderID:        providerID,
		UserID:            userID,
		ExternalAccountID: result.ExternalAccountID,
		ExternalLogin:     result.ExternalLogin,
		AccountType:       result.AccountType,
		Status:            "connected",
		Secret:            encrypted,
	}
	saved, err := s.store.UpsertIntegrationConnection(ctx, conn)
	if err != nil {
		return domain.IntegrationConnection{}, fmt.Errorf("store connection: %w", err)
	}

	if s.logger != nil {
		s.logger.Info("integration connected",
			"provider", providerID,
			"user_id", userID,
			"external_login", result.ExternalLogin,
		)
	}
	return saved, nil
}

// Disconnect deletes the stored credential and associated agent integrations.
// Works for orphaned providers (no registry lookup needed).
func (s *Service) Disconnect(ctx context.Context, userID, providerID string) error {
	return s.store.DeleteIntegrationConnection(ctx, userID, providerID)
}

// DecryptToken decrypts the stored encrypted secret for a connection.
func (s *Service) DecryptToken(ctx context.Context, userID, providerID string) (string, error) {
	conn, err := s.store.GetIntegrationConnection(ctx, userID, providerID)
	if err != nil {
		return "", err
	}
	plaintext, err := s.secret.DecryptFromString(conn.Secret)
	if err != nil {
		return "", fmt.Errorf("decrypt token: %w", err)
	}
	return string(plaintext), nil
}

// GetConnection returns the connection metadata (without secret) for a user.
func (s *Service) GetConnection(ctx context.Context, userID, providerID string) (domain.IntegrationConnection, error) {
	return s.store.GetIntegrationConnection(ctx, userID, providerID)
}

// ToggleAgentIntegration enables or disables a provider integration for an agent.
func (s *Service) ToggleAgentIntegration(ctx context.Context, agentID, providerID string, enabled bool) (domain.AgentIntegration, error) {
	status := "active"
	if !enabled {
		status = "disabled"
	}
	return s.store.UpsertAgentIntegration(ctx, agentID, providerID, enabled, 0, status)
}

// ListAgentIntegrations returns all integrations for an agent.
func (s *Service) ListAgentIntegrations(ctx context.Context, agentID string) ([]domain.AgentIntegration, error) {
	return s.store.ListAgentIntegrations(ctx, agentID)
}

// GetAgentIntegration returns a specific agent integration.
func (s *Service) GetAgentIntegration(ctx context.Context, agentID, providerID string) (domain.AgentIntegration, error) {
	return s.store.GetAgentIntegration(ctx, agentID, providerID)
}

// BuildSummary returns an IntegrationSummary for the given user, populating
// manifest metadata, connection state, and per-agent bindings.
func (s *Service) BuildSummary(ctx context.Context, userID string) (domain.IntegrationSummary, error) {
	resolved := s.registry.List()
	summaries := make([]domain.ProviderSummary, 0, len(resolved))

	for _, r := range resolved {
		spec := r.Manifest.Spec
		ps := domain.ProviderSummary{
			ProviderID:  spec.ID,
			Name:        spec.DisplayName,
			Description: spec.Description,
			AuthKind:    spec.Auth.Type,
			FormFields:  toFormFieldSummaries(spec.Auth.Fields),
			MCPServers:  toMCPServerSummaries(spec.Contributions.MCPServers),
		}

		if conn, err := s.store.GetIntegrationConnection(ctx, userID, spec.ID); err == nil {
			ps.Connected = true
			ps.Connection = &domain.ConnectionMetadata{
				ExternalAccountID: conn.ExternalAccountID,
				ExternalLogin:     conn.ExternalLogin,
				AccountType:       conn.AccountType,
				Status:            conn.Status,
				Revision:          conn.Revision,
			}
		}

		ps.AgentBindings = s.buildAgentBindings(ctx, userID, spec.ID)
		summaries = append(summaries, ps)
	}

	// Surface orphaned credentials for providers no longer in the registry.
	allProviderIDs, err := s.store.ListUserIntegrationProviderIDs(ctx, userID)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("could not list user integration provider IDs", "user_id", userID, "err", err)
		}
	} else {
		for _, pid := range allProviderIDs {
			if _, known := s.registry.Get(pid); !known {
				summaries = append(summaries, domain.ProviderSummary{
					ProviderID:    pid,
					Name:          pid,
					AuthKind:      "unknown",
					Description:   "Provider is no longer registered. Disconnect available.",
					Connected:     true,
					AgentBindings: []domain.AgentBindingSummary{},
				})
			}
		}
	}

	return domain.IntegrationSummary{Providers: summaries}, nil
}

// ResolveAgentRuntime builds the env-var map and MCP server list for all enabled
// integrations on the given agent. Fails fast if any enabled plugin is unregistered
// or lacks a stored credential.
func (s *Service) ResolveAgentRuntime(ctx context.Context, userID, agentID string) (AgentRuntimeResolution, error) {
	ais, err := s.store.ListAgentIntegrations(ctx, agentID)
	if err != nil {
		return AgentRuntimeResolution{}, fmt.Errorf("list agent integrations: %w", err)
	}

	env := make(map[string]string)
	var mcps []plugins.ResolvedMCPServer

	for _, ai := range ais {
		if !ai.Enabled {
			continue
		}

		r, ok := s.registry.Get(ai.ProviderID)
		if !ok {
			return AgentRuntimeResolution{}, fmt.Errorf("plugin %s unregistered", ai.ProviderID)
		}
		m := r.Manifest

		conn, err := s.store.GetIntegrationConnection(ctx, userID, ai.ProviderID)
		if err != nil {
			return AgentRuntimeResolution{}, fmt.Errorf("no connection for plugin %s", ai.ProviderID)
		}

		tokenBytes, err := s.secret.DecryptFromString(conn.Secret)
		if err != nil {
			return AgentRuntimeResolution{}, fmt.Errorf("plugin %s: decrypt token: %w", ai.ProviderID, err)
		}
		token := string(tokenBytes)
		vars := plugins.TemplateContext{Token: token, Fields: map[string]string{"token": token}}

		var pluginEnv map[string]string
		switch m.Spec.PluginKind {
		case "http_template":
			pluginEnv, err = s.httpEval.BuildEnv(&m, vars)
		case "sidecar":
			pluginEnv, err = s.sidecar.BuildEnv(ctx, &m, vars)
		default:
			return AgentRuntimeResolution{}, fmt.Errorf("plugin %s: unsupported kind %q", ai.ProviderID, m.Spec.PluginKind)
		}
		if err != nil {
			return AgentRuntimeResolution{}, fmt.Errorf("plugin %s: build env: %w", ai.ProviderID, err)
		}

		for k, v := range pluginEnv {
			if _, collision := env[k]; collision && s.logger != nil {
				s.logger.Warn("env var collision between plugins",
					"key", k,
					"provider", ai.ProviderID,
				)
			}
			env[k] = v
		}

		if len(m.Spec.Contributions.MCPServers) > 0 {
			if s.resolver == nil {
				return AgentRuntimeResolution{}, fmt.Errorf("plugin %s: mcp resolver not configured (no kubernetes provider)", ai.ProviderID)
			}
			resolved, err := s.resolver.Resolve(ctx, m.Spec.Contributions.MCPServers, vars)
			if err != nil {
				return AgentRuntimeResolution{}, fmt.Errorf("plugin %s: resolve MCP servers: %w", ai.ProviderID, err)
			}
			mcps = append(mcps, resolved...)
		}
	}

	if mcps == nil {
		mcps = []plugins.ResolvedMCPServer{}
	}
	return AgentRuntimeResolution{Env: env, MCPServers: mcps}, nil
}

// buildAgentBindings returns per-agent integration bindings for the user's agents.
func (s *Service) buildAgentBindings(ctx context.Context, userID, providerID string) []domain.AgentBindingSummary {
	agents, err := s.store.ListAgents(ctx, userID)
	if err != nil {
		return []domain.AgentBindingSummary{}
	}
	bindings := make([]domain.AgentBindingSummary, 0, len(agents))
	for _, agent := range agents {
		ai, err := s.store.GetAgentIntegration(ctx, agent.ID, providerID)
		if err == nil {
			bindings = append(bindings, domain.AgentBindingSummary{
				AgentID:  ai.AgentID,
				Enabled:  ai.Enabled,
				Revision: ai.Revision,
				Status:   ai.Status,
			})
		}
	}
	return bindings
}

func toFormFieldSummaries(fields []plugins.FormField) []domain.FormFieldSummary {
	if len(fields) == 0 {
		return nil
	}
	out := make([]domain.FormFieldSummary, len(fields))
	for i, f := range fields {
		out[i] = domain.FormFieldSummary{
			Name:        f.Name,
			Label:       f.Label,
			Secret:      f.Secret,
			Placeholder: f.Placeholder,
			HelpURL:     f.HelpURL,
		}
	}
	return out
}

func toMCPServerSummaries(refs []plugins.MCPRef) []domain.MCPServerSummary {
	if len(refs) == 0 {
		return nil
	}
	out := make([]domain.MCPServerSummary, len(refs))
	for i, ref := range refs {
		kind := "external"
		if ref.Ref != "" {
			kind = "ref"
		}
		out[i] = domain.MCPServerSummary{Name: ref.Name, Kind: kind}
	}
	return out
}
