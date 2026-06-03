package identity

import (
	"context"
	"errors"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// OIDCProviderConfig is the input config for an OIDC provider.
type OIDCProviderConfig struct {
	Name         string
	DisplayName  string
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURI  string
	Scopes       []string
	EmailClaim   string
	NameClaim    string
	GroupsClaim  string
}

// OIDCProvider is a materialized OIDC Relying Party client.
type OIDCProvider struct {
	Config   OIDCProviderConfig
	OIDC     *oidc.Provider
	OAuth2   *oauth2.Config
	Verifier *oidc.IDTokenVerifier
}

// NewOIDCProvider performs OIDC discovery and builds the materialized provider.
func NewOIDCProvider(ctx context.Context, cfg OIDCProviderConfig) (*OIDCProvider, error) {
	// Defensive copy of scopes so we don't mutate caller's slice.
	scopes := cfg.Scopes
	if len(scopes) == 0 {
		scopes = []string{oidc.ScopeOpenID, "email", "profile"}
	} else {
		s := make([]string, len(scopes))
		copy(s, scopes)
		scopes = s
	}

	if cfg.EmailClaim == "" {
		cfg.EmailClaim = "email"
	}
	if cfg.NameClaim == "" {
		cfg.NameClaim = "name"
	}
	if cfg.GroupsClaim == "" {
		cfg.GroupsClaim = "groups"
	}

	provider, err := oidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery for %q: %w", cfg.Issuer, err)
	}

	oauth2Cfg := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURI,
		Endpoint:     provider.Endpoint(),
		Scopes:       scopes,
	}

	verifier := provider.Verifier(&oidc.Config{ClientID: cfg.ClientID})

	return &OIDCProvider{
		Config:   cfg,
		OIDC:     provider,
		OAuth2:   oauth2Cfg,
		Verifier: verifier,
	}, nil
}

// Name implements IdentityProvider.
func (p *OIDCProvider) Name() string { return p.Config.Name }

// Authenticate implements IdentityProvider but is not the primary flow for OIDC.
// Callers must use the authorize-redirect flow.
func (p *OIDCProvider) Authenticate(_ context.Context, _ AuthRequest) (Identity, error) {
	return Identity{}, errors.New("OIDC providers do not support direct Authenticate; use authorize-redirect flow")
}

// IdentityFromIDToken extracts claims from a verified ID token into an Identity.
// The token must already be verified by the caller via p.Verifier.Verify.
func (p *OIDCProvider) IdentityFromIDToken(idToken *oidc.IDToken) (Identity, error) {
	subject := idToken.Subject
	if subject == "" {
		return Identity{}, errors.New("id token missing required sub claim")
	}

	var raw map[string]any
	if err := idToken.Claims(&raw); err != nil {
		return Identity{}, fmt.Errorf("extracting id token claims: %w", err)
	}

	email, err := stringClaim(raw, p.Config.EmailClaim)
	if err != nil || email == "" {
		if err == nil {
			err = fmt.Errorf("id token missing required %q claim", p.Config.EmailClaim)
		}
		return Identity{}, err
	}

	name, _ := stringClaim(raw, p.Config.NameClaim)

	groups := coerceGroups(raw[p.Config.GroupsClaim])

	return Identity{
		Subject: subject,
		Email:   email,
		Name:    name,
		Groups:  groups,
	}, nil
}

// stringClaim extracts a string value from raw claims by key.
func stringClaim(raw map[string]any, key string) (string, error) {
	v, ok := raw[key]
	if !ok {
		return "", nil
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("claim %q is not a string", key)
	}
	return s, nil
}

// coerceGroups normalizes a raw groups claim value into []string.
// Accepts []any (skips non-strings), []string, bare string, or nil.
func coerceGroups(v any) []string {
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case string:
		return []string{t}
	case []string:
		result := make([]string, len(t))
		copy(result, t)
		return result
	case []any:
		result := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		if len(result) == 0 {
			return nil
		}
		return result
	default:
		return nil
	}
}
