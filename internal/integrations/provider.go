package integrations

import "context"

// ValidationResult contains the result of validating a credential with an
// external provider. It holds non-sensitive metadata about the connected
// account.
type ValidationResult struct {
	ExternalAccountID string
	ExternalLogin     string
	AccountType       string
	Status            string // "connected" on success
}

// Provider is the interface that external integration providers (e.g. GitHub)
// must implement.
type Provider interface {
	// ValidatePAT checks that the given personal access token is valid by
	// calling the provider's API. It returns metadata about the authenticated
	// account or an error if the token is invalid/unreachable.
	ValidatePAT(ctx context.Context, token string) (ValidationResult, error)

	// BuildRuntimeEnv returns a map of environment variable names to values
	// that should be injected into an agent's runtime sandbox when this
	// integration is enabled for that agent.
	BuildRuntimeEnv(token string) map[string]string

	// ProviderID returns the unique identifier for this provider (e.g. "github").
	ProviderID() string
}
