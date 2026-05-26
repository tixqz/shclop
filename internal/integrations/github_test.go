package integrations

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGitHubProviderValidatePAT_Success(t *testing.T) {
	// Start a test GitHub server that returns a valid user response.
	githubServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		// Verify auth header
		if r.Header.Get("Authorization") != "Bearer ghp_test_valid" {
			t.Fatalf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}
		if r.Header.Get("X-GitHub-Api-Version") != "2022-11-28" {
			t.Fatalf("missing api version header")
		}
		if r.Header.Get("User-Agent") == "" {
			t.Fatal("missing user-agent header")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"login": "testuser",
			"id":    12345,
			"type":  "User",
		})
	}))
	defer githubServer.Close()

	provider := NewGitHubProvider(GitHubProviderConfig{
		BaseURL:  githubServer.URL,
		HTTPClient: githubServer.Client(),
	})

	result, err := provider.ValidatePAT(t.Context(), "ghp_test_valid")
	if err != nil {
		t.Fatalf("ValidatePAT: %v", err)
	}
	if result.ExternalAccountID != "12345" {
		t.Fatalf("expected external_account_id 12345, got %q", result.ExternalAccountID)
	}
	if result.ExternalLogin != "testuser" {
		t.Fatalf("expected external_login testuser, got %q", result.ExternalLogin)
	}
	if result.AccountType != "User" {
		t.Fatalf("expected account_type User, got %q", result.AccountType)
	}
	if result.Status != "connected" {
		t.Fatalf("expected status connected, got %q", result.Status)
	}
}

func TestGitHubProviderValidatePAT_NonOKStatus(t *testing.T) {
	githubServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"message": "bad credentials"})
	}))
	defer githubServer.Close()

	provider := NewGitHubProvider(GitHubProviderConfig{
		BaseURL:  githubServer.URL,
		HTTPClient: githubServer.Client(),
	})

	_, err := provider.ValidatePAT(t.Context(), "ghp_invalid")
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
}

func TestGitHubProviderValidatePAT_NetworkError(t *testing.T) {
	// Use a server that closes immediately
	githubServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Close without responding
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("server does not support hijack")
		}
		conn, _, err := hj.Hijack()
		if err != nil {
			t.Fatal(err)
		}
		conn.Close()
	}))
	defer githubServer.Close()

	provider := NewGitHubProvider(GitHubProviderConfig{
		BaseURL:  githubServer.URL,
		HTTPClient: githubServer.Client(),
	})

	_, err := provider.ValidatePAT(t.Context(), "ghp_test")
	if err == nil {
		t.Fatal("expected error for network failure")
	}
}

func TestGitHubProviderBuildRuntimeEnv(t *testing.T) {
	provider := NewGitHubProvider(GitHubProviderConfig{})
	env := provider.BuildRuntimeEnv("ghp_test_token_value")
	if env == nil {
		t.Fatal("expected non-nil env map")
	}
	if env["GITHUB_TOKEN"] != "ghp_test_token_value" {
		t.Fatalf("expected GITHUB_TOKEN=ghp_test_token_value, got %v", env)
	}
}

func TestGitHubProviderValidatePAT_DefaultBaseURL(t *testing.T) {
	provider := NewGitHubProvider(GitHubProviderConfig{})
	if provider.baseURL != "https://api.github.com" {
		t.Fatalf("expected default base URL https://api.github.com, got %q", provider.baseURL)
	}
}
