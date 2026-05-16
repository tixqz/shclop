package identity

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

var ErrInvalidCredentials = errors.New("invalid credentials")

type MockYAMLProvider struct {
	users map[string]mockYAMLUser
}

type mockYAMLConfig struct {
	Users map[string]mockYAMLUser `yaml:"users"`
}

type mockYAMLUser struct {
	Password string   `yaml:"password"`
	Subject  string   `yaml:"subject"`
	Name     string   `yaml:"name"`
	Tenant   string   `yaml:"tenant"`
	Teams    []string `yaml:"teams"`
	Roles    []string `yaml:"roles"`
	Groups   []string `yaml:"groups"`
}

func NewMockYAMLProvider(path string) (*MockYAMLProvider, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg mockYAMLConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if len(cfg.Users) == 0 {
		return nil, fmt.Errorf("mock identity config has no users")
	}
	for username, user := range cfg.Users {
		if strings.TrimSpace(username) == "" {
			return nil, fmt.Errorf("mock identity config has empty username")
		}
		if strings.TrimSpace(user.Password) == "" {
			return nil, fmt.Errorf("mock identity user %q has no password", username)
		}
		if strings.TrimSpace(user.Tenant) == "" {
			return nil, fmt.Errorf("mock identity user %q has no tenant", username)
		}
	}
	return &MockYAMLProvider{users: cfg.Users}, nil
}

func (p *MockYAMLProvider) Name() string { return "mock-yaml" }

func (p *MockYAMLProvider) Authenticate(ctx context.Context, request AuthRequest) (Identity, error) {
	if err := ctx.Err(); err != nil {
		return Identity{}, err
	}
	username := strings.TrimSpace(strings.ToLower(request.Username))
	user, ok := p.users[username]
	if !ok || user.Password != request.Password {
		return Identity{}, ErrInvalidCredentials
	}
	subject := user.Subject
	if subject == "" {
		subject = "mock-yaml|" + username
	}
	return Identity{
		Subject: subject,
		Email:   username,
		Name:    user.Name,
		Groups:  append([]string(nil), user.Groups...),
		Claims: map[string]string{
			"tenant": strings.TrimSpace(user.Tenant),
			"teams":  strings.Join(user.Teams, ","),
			"roles":  strings.Join(user.Roles, ","),
		},
	}, nil
}

type StaticOrganizationMapper struct{}

func (StaticOrganizationMapper) Map(ctx context.Context, identity Identity) (MappedPrincipal, error) {
	if err := ctx.Err(); err != nil {
		return MappedPrincipal{}, err
	}
	if strings.TrimSpace(identity.Subject) == "" {
		return MappedPrincipal{}, fmt.Errorf("identity has no subject")
	}
	if strings.TrimSpace(identity.Email) == "" {
		return MappedPrincipal{}, fmt.Errorf("identity %q has no email", identity.Subject)
	}
	tenant := strings.TrimSpace(identity.Claims["tenant"])
	if tenant == "" {
		return MappedPrincipal{}, fmt.Errorf("identity %q has no tenant claim", identity.Subject)
	}
	return MappedPrincipal{
		UserID:      identity.Subject,
		Email:       identity.Email,
		DisplayName: identity.Name,
		TenantID:    tenant,
		TeamIDs:     splitClaim(identity.Claims["teams"]),
		Roles:       splitClaim(identity.Claims["roles"]),
	}, nil
}

func splitClaim(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
