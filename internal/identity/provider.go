package identity

import "context"

type AuthRequest struct {
	Username string
	Password string
	Headers  map[string]string
	Claims   map[string]string
}

type Identity struct {
	Subject string
	Email   string
	Name    string
	Groups  []string
	Claims  map[string]string
}

type MappedPrincipal struct {
	UserID      string
	Email       string
	DisplayName string
	TenantID    string
	TeamIDs     []string
	Roles       []string
}

type IdentityProvider interface {
	Name() string
	Authenticate(ctx context.Context, request AuthRequest) (Identity, error)
}

type OrganizationMapper interface {
	Map(ctx context.Context, identity Identity) (MappedPrincipal, error)
}
