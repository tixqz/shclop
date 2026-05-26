package integrations

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/mipopov/shclop/internal/domain"
	"github.com/mipopov/shclop/internal/store"
)

// Service manages integration providers, encrypts/decrypts secrets, and
// coordinates the integration lifecycle (validation, storage, runtime env).
type Service struct {
	store   store.Store
	secret  *SecretBox
	logger  *slog.Logger
	mu      sync.Mutex
	providers map[string]Provider // providerID -> Provider
}

// NewService creates an integration service with the given store, secret box,
// and logger. Providers are registered via RegisterProvider.
func NewService(store store.Store, secret *SecretBox, logger *slog.Logger) *Service {
	return &Service{
		store:     store,
		secret:    secret,
		logger:    logger,
		providers: make(map[string]Provider),
	}
}

// RegisterProvider adds a provider to the service. Duplicate provider IDs
// will be silently overwritten.
func (s *Service) RegisterProvider(p Provider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.providers[p.ProviderID()] = p
}

// Provider returns the registered provider for the given ID, or nil.
func (s *Service) Provider(providerID string) Provider {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.providers[providerID]
}

// KnownProviders returns a list of all registered provider IDs.
func (s *Service) KnownProviders() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := make([]string, 0, len(s.providers))
	for id := range s.providers {
		ids = append(ids, id)
	}
	return ids
}

// Connect validates a PAT with the provider, encrypts it, and stores the
// connection. If validation fails, nothing is saved.
func (s *Service) Connect(ctx context.Context, userID, providerID, token string) (domain.IntegrationConnection, error) {
	provider := s.Provider(providerID)
	if provider == nil {
		return domain.IntegrationConnection{}, fmt.Errorf("unknown provider %q", providerID)
	}

	// Validate token with provider before saving anything.
	result, err := provider.ValidatePAT(ctx, token)
	if err != nil {
		return domain.IntegrationConnection{}, fmt.Errorf("token validation failed: %w", err)
	}

	// Encrypt the token
	encrypted, err := s.secret.EncryptToString([]byte(token))
	if err != nil {
		return domain.IntegrationConnection{}, fmt.Errorf("encrypt token: %w", err)
	}

	conn := domain.IntegrationConnection{
		ProviderID:        providerID,
		UserID:            userID,
		ExternalAccountID: result.ExternalAccountID,
		ExternalLogin:     result.ExternalLogin,
		AccountType:       result.AccountType,
		Status:            result.Status,
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

// Disconnect deletes the connection and associated agent integrations.
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

// ToggleAgentIntegration enables or disables a provider integration for a
// specific agent. It returns the updated AgentIntegration.
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

// BuildSummary returns an IntegrationSummary for the given user, assembling
// provider metadata, connection state, and per-agent bindings.
func (s *Service) BuildSummary(ctx context.Context, userID string) (domain.IntegrationSummary, error) {
	providers := s.KnownProviders()
	summaries := make([]domain.ProviderSummary, 0, len(providers))

	for _, pid := range providers {
		ps := domain.ProviderSummary{
			ProviderID: pid,
			Name:       providerDisplayName(pid),
		}

		conn, err := s.store.GetIntegrationConnection(ctx, userID, pid)
		if err == nil {
			ps.Connected = true
			meta := &domain.ConnectionMetadata{
				ExternalAccountID: conn.ExternalAccountID,
				ExternalLogin:     conn.ExternalLogin,
				AccountType:       conn.AccountType,
				Status:            conn.Status,
				Revision:          conn.Revision,
			}
			ps.Connection = meta
		}

		// Get agent bindings for all agents belonging to this user
		agents, err := s.store.ListAgents(ctx, userID)
		if err == nil {
			bindings := make([]domain.AgentBindingSummary, 0, len(agents))
			for _, agent := range agents {
				ai, err := s.store.GetAgentIntegration(ctx, agent.ID, pid)
				if err == nil {
					bindings = append(bindings, domain.AgentBindingSummary{
						AgentID:  ai.AgentID,
						Enabled:  ai.Enabled,
						Revision: ai.Revision,
						Status:   ai.Status,
					})
				}
			}
			if bindings == nil {
				bindings = []domain.AgentBindingSummary{}
			}
			ps.AgentBindings = bindings
		}

		summaries = append(summaries, ps)
	}

	return domain.IntegrationSummary{Providers: summaries}, nil
}

func providerDisplayName(pid string) string {
	switch pid {
	case "github":
		return "GitHub"
	default:
		return pid
	}
}
