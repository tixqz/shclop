package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const gitHubDefaultBaseURL = "https://api.github.com"

// GitHubProvider implements Provider for the GitHub API using Personal Access
// Tokens.
type GitHubProvider struct {
	baseURL    string
	httpClient *http.Client
}

// GitHubProviderConfig allows injecting a custom base URL and HTTP client
// (useful for tests).
type GitHubProviderConfig struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewGitHubProvider creates a new GitHubProvider. If config.BaseURL is empty,
// it defaults to "https://api.github.com". If config.HTTPClient is nil,
// http.DefaultClient is used.
func NewGitHubProvider(config GitHubProviderConfig) *GitHubProvider {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = gitHubDefaultBaseURL
	}
	client := config.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	// Ensure no trailing slash for consistent path joining.
	if baseURL[len(baseURL)-1] == '/' {
		baseURL = baseURL[:len(baseURL)-1]
	}
	return &GitHubProvider{baseURL: baseURL, httpClient: client}
}

// ValidatePAT calls GET /user on the GitHub API with the given token.
// It returns metadata about the authenticated user on success, or an error
// if the token is invalid or the API call fails.
func (p *GitHubProvider) ValidatePAT(ctx context.Context, token string) (ValidationResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/user", nil)
	if err != nil {
		return ValidationResult{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "shclop-integrations/1.0")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return ValidationResult{}, fmt.Errorf("http call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ValidationResult{}, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var userInfo struct {
		Login string `json:"login"`
		ID    int64  `json:"id"`
		Type  string `json:"type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return ValidationResult{}, fmt.Errorf("decode response: %w", err)
	}

	return ValidationResult{
		ExternalAccountID: fmt.Sprintf("%d", userInfo.ID),
		ExternalLogin:     userInfo.Login,
		AccountType:       userInfo.Type,
		Status:            "connected",
	}, nil
}

// BuildRuntimeEnv returns environment variables for the agent runtime.
// For GitHub, it exports GITHUB_TOKEN so that tools like gh and git can
// authenticate with GitHub on behalf of the user.
func (p *GitHubProvider) BuildRuntimeEnv(token string) map[string]string {
	return map[string]string{
		"GITHUB_TOKEN": token,
	}
}

// ProviderID returns "github".
func (p *GitHubProvider) ProviderID() string {
	return "github"
}
