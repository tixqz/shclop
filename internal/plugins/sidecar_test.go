package plugins

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func sidecarManifest(url, timeout string) *Manifest {
	return &Manifest{
		APIVersion: "shclop.io/v1alpha1",
		Kind:       "IntegrationPlugin",
		Metadata:   Metadata{Name: "test-sidecar"},
		Spec: ManifestSpec{
			ID:          "test-sidecar",
			DisplayName: "Test Sidecar",
			PluginKind:  "sidecar",
			Auth: AuthSpec{
				Type:   "pat_token",
				Fields: []FormField{{Name: "token"}},
			},
			Sidecar: &SidecarSpec{
				URL:     url,
				Timeout: timeout,
			},
		},
	}
}

func TestSidecarClient_Validate_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/validate" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		var req struct {
			Fields  map[string]string `json:"fields"`
			Context struct {
				Token string `json:"token"`
			} `json:"context"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if req.Fields["token"] != "mytoken" {
			http.Error(w, "wrong fields token", http.StatusBadRequest)
			return
		}
		if req.Context.Token != "mytoken" {
			http.Error(w, "wrong context token", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"external_account_id": "42",
			"external_login":      "octocat",
			"account_type":        "User",
			"status":              "connected",
		})
	}))
	defer srv.Close()

	client := NewSidecarClient()
	m := sidecarManifest(srv.URL, "5s")
	vars := TemplateContext{
		Token:  "mytoken",
		Fields: map[string]string{"token": "mytoken"},
	}

	result, err := client.Validate(context.Background(), m, vars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExternalAccountID != "42" {
		t.Errorf("expected ExternalAccountID=42, got %q", result.ExternalAccountID)
	}
	if result.ExternalLogin != "octocat" {
		t.Errorf("expected ExternalLogin=octocat, got %q", result.ExternalLogin)
	}
	if result.AccountType != "User" {
		t.Errorf("expected AccountType=User, got %q", result.AccountType)
	}
}

func TestSidecarClient_Validate_StatusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "bad token",
		})
	}))
	defer srv.Close()

	client := NewSidecarClient()
	m := sidecarManifest(srv.URL, "5s")
	vars := TemplateContext{Token: "x", Fields: map[string]string{"token": "x"}}

	_, err := client.Validate(context.Background(), m, vars)
	if err == nil {
		t.Fatal("expected error for status=error, got nil")
	}
	if !strings.Contains(err.Error(), "error") {
		t.Errorf("expected error to mention status, got: %v", err)
	}
}

func TestSidecarClient_Validate_Non2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewSidecarClient()
	m := sidecarManifest(srv.URL, "5s")
	vars := TemplateContext{Token: "x", Fields: map[string]string{"token": "x"}}

	_, err := client.Validate(context.Background(), m, vars)
	if err == nil {
		t.Fatal("expected error for 500, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to contain 500, got: %v", err)
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("expected error to contain body 'boom', got: %v", err)
	}
}

func TestSidecarClient_BuildEnv_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/runtime-env" || r.Method != http.MethodPost {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"env": map[string]string{"FOO": "bar"},
		})
	}))
	defer srv.Close()

	client := NewSidecarClient()
	m := sidecarManifest(srv.URL, "5s")
	vars := TemplateContext{Token: "x", Fields: map[string]string{"token": "x"}}

	env, err := client.BuildEnv(context.Background(), m, vars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(env) != 1 || env["FOO"] != "bar" {
		t.Errorf("expected env={FOO:bar}, got %v", env)
	}
}

func TestSidecarClient_Validate_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewSidecarClient()
	m := sidecarManifest(srv.URL, "10ms")
	vars := TemplateContext{Token: "x", Fields: map[string]string{"token": "x"}}

	_, err := client.Validate(context.Background(), m, vars)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got: %v", err)
	}
}

func TestSidecarClient_Validate_NetworkUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()

	client := NewSidecarClient()
	m := sidecarManifest(url, "5s")
	vars := TemplateContext{Token: "x", Fields: map[string]string{"token": "x"}}

	_, err := client.Validate(context.Background(), m, vars)
	if err == nil {
		t.Fatal("expected error for closed server, got nil")
	}
	if !strings.Contains(err.Error(), "unreachable") {
		t.Errorf("expected error to contain 'unreachable', got: %v", err)
	}
}

func TestSidecarClient_Validate_WrongKind(t *testing.T) {
	client := NewSidecarClient()
	m := &Manifest{
		APIVersion: "shclop.io/v1alpha1",
		Kind:       "IntegrationPlugin",
		Metadata:   Metadata{Name: "github"},
		Spec: ManifestSpec{
			ID:         "github",
			PluginKind: "http_template",
			Sidecar:    nil,
		},
	}
	vars := TemplateContext{Token: "x", Fields: map[string]string{"token": "x"}}

	_, err := client.Validate(context.Background(), m, vars)
	if err == nil {
		t.Fatal("expected error for wrong kind, got nil")
	}
	if !strings.Contains(err.Error(), "kind") {
		t.Errorf("expected error to mention 'kind', got: %v", err)
	}
}
