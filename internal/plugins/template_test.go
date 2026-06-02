package plugins

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func makeHTTPManifest(t *testing.T, method, urlTemplate string, extraHeaders map[string]string, extract map[string]string) *Manifest {
	t.Helper()
	h := &HTTPValidate{
		Method:        method,
		URL:           urlTemplate,
		Headers:       extraHeaders,
		SuccessStatus: []int{200},
		Timeout:       "5s",
	}
	if extract != nil {
		h.Extract = ExtractSpec{
			ExternalAccountID: extract["external_account_id"],
			ExternalLogin:     extract["external_login"],
			AccountType:       extract["account_type"],
		}
	}
	return &Manifest{
		APIVersion: "shclop.io/v1alpha1",
		Kind:       "IntegrationPlugin",
		Metadata:   Metadata{Name: "test"},
		Spec: ManifestSpec{
			ID:              "test",
			PluginKind:      "http_template",
			ScopesSupported: []string{"user"},
			Auth: AuthSpec{
				Type:   "pat_token",
				Fields: []FormField{{Name: "token"}},
			},
			Validate:      ValidateSpec{HTTP: h},
			Contributions: Contributions{},
		},
	}
}

func TestValidate_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer mytoken" {
			t.Errorf("expected Authorization header 'Bearer mytoken', got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"id": 42, "login": "octocat", "type": "User"}`))
	}))
	defer srv.Close()

	m := makeHTTPManifest(t, "GET", srv.URL+"/user", map[string]string{
		"Authorization": "Bearer {{.Token}}",
	}, map[string]string{
		"external_account_id": ".id",
		"external_login":      ".login",
		"account_type":        ".type",
	})

	ev := NewHTTPEvaluator()
	ev.Client = srv.Client()

	vars := TemplateContext{
		Token:  "mytoken",
		Fields: map[string]string{},
	}

	result, err := ev.Validate(context.Background(), m, vars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExternalLogin != "octocat" {
		t.Errorf("expected ExternalLogin=octocat, got %q", result.ExternalLogin)
	}
	if result.ExternalAccountID != "42" {
		t.Errorf("expected ExternalAccountID=42, got %q", result.ExternalAccountID)
	}
	if result.AccountType != "User" {
		t.Errorf("expected AccountType=User, got %q", result.AccountType)
	}
}

func TestValidate_NonSuccessStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"message": "Bad credentials"}`))
	}))
	defer srv.Close()

	m := makeHTTPManifest(t, "GET", srv.URL, nil, nil)

	ev := &HTTPEvaluator{Client: srv.Client()}
	_, err := ev.Validate(context.Background(), m, TemplateContext{})
	if err == nil {
		t.Fatal("expected error for 401, got nil")
	}
	if !strings.Contains(err.Error(), "status=401") {
		t.Errorf("expected error to contain 'status=401', got: %v", err)
	}
}

func TestValidate_BadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	m := makeHTTPManifest(t, "GET", srv.URL, nil, nil)

	ev := &HTTPEvaluator{Client: srv.Client()}
	_, err := ev.Validate(context.Background(), m, TemplateContext{})
	if err == nil {
		t.Fatal("expected error for bad JSON, got nil")
	}
	if !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("expected error to mention unmarshal, got: %v", err)
	}
}

func TestValidate_ContextCancellation(t *testing.T) {
	ready := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(ready)
		<-r.Context().Done()
	}))
	defer srv.Close()

	m := makeHTTPManifest(t, "GET", srv.URL, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	ev := &HTTPEvaluator{Client: srv.Client()}

	done := make(chan error, 1)
	go func() {
		_, err := ev.Validate(ctx, m, TemplateContext{})
		done <- err
	}()

	<-ready
	cancel()

	err := <-done
	if err == nil {
		t.Fatal("expected error after context cancellation, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

func TestValidate_TemplateRenderingInURL(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	m := makeHTTPManifest(t, "GET", srv.URL+"/orgs/{{.Fields.org}}/members", nil, nil)

	ev := &HTTPEvaluator{Client: srv.Client()}
	vars := TemplateContext{
		Fields: map[string]string{"org": "myorg"},
	}

	_, err := ev.Validate(context.Background(), m, vars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/orgs/myorg/members" {
		t.Errorf("expected path /orgs/myorg/members, got %q", gotPath)
	}
}

func TestBuildEnv(t *testing.T) {
	m := &Manifest{
		Spec: ManifestSpec{
			Contributions: Contributions{
				Env: map[string]string{
					"GITHUB_TOKEN": "{{.Token}}",
					"EXTRA":        "{{.Fields.team}}",
				},
			},
		},
	}

	ev := NewHTTPEvaluator()
	vars := TemplateContext{
		Token:  "tok123",
		Fields: map[string]string{"team": "platform"},
	}

	env, err := ev.BuildEnv(m, vars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env["GITHUB_TOKEN"] != "tok123" {
		t.Errorf("expected GITHUB_TOKEN=tok123, got %q", env["GITHUB_TOKEN"])
	}
	if env["EXTRA"] != "platform" {
		t.Errorf("expected EXTRA=platform, got %q", env["EXTRA"])
	}
}
