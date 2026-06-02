package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type SidecarClient struct {
	Client *http.Client
}

func NewSidecarClient() *SidecarClient {
	return &SidecarClient{Client: http.DefaultClient}
}

func (c *SidecarClient) httpClient() *http.Client {
	if c.Client != nil {
		return c.Client
	}
	return http.DefaultClient
}

func (c *SidecarClient) Validate(ctx context.Context, m *Manifest, vars TemplateContext) (ValidationResult, error) {
	if m.Spec.PluginKind != "sidecar" {
		return ValidationResult{}, fmt.Errorf("plugin %s: expected kind sidecar, got %q", m.Spec.ID, m.Spec.PluginKind)
	}
	if m.Spec.Sidecar == nil {
		return ValidationResult{}, fmt.Errorf("plugin %s: sidecar spec is nil", m.Spec.ID)
	}

	timeout, err := ParseTimeout(m.Spec.Sidecar.Timeout)
	if err != nil {
		return ValidationResult{}, fmt.Errorf("plugin %s: invalid timeout: %w", m.Spec.ID, err)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	body, err := json.Marshal(map[string]any{
		"fields":  vars.Fields,
		"context": map[string]string{"token": vars.Token},
	})
	if err != nil {
		return ValidationResult{}, fmt.Errorf("plugin %s: marshal request: %w", m.Spec.ID, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.Spec.Sidecar.URL+"/validate", bytes.NewReader(body))
	if err != nil {
		return ValidationResult{}, fmt.Errorf("plugin %s: build request: %w", m.Spec.ID, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return ValidationResult{}, fmt.Errorf("plugin %s unreachable: %w", m.Spec.ID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return ValidationResult{}, fmt.Errorf("plugin %s: validate returned %d: %s", m.Spec.ID, resp.StatusCode, bytes.TrimSpace(snippet))
	}

	var result struct {
		ExternalAccountID string `json:"external_account_id"`
		ExternalLogin     string `json:"external_login"`
		AccountType       string `json:"account_type"`
		Status            string `json:"status"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return ValidationResult{}, fmt.Errorf("plugin %s: decode response: %w", m.Spec.ID, err)
	}

	if result.Status != "connected" {
		return ValidationResult{}, fmt.Errorf("plugin %s: validate status %q", m.Spec.ID, result.Status)
	}

	return ValidationResult{
		ExternalAccountID: result.ExternalAccountID,
		ExternalLogin:     result.ExternalLogin,
		AccountType:       result.AccountType,
	}, nil
}

func (c *SidecarClient) BuildEnv(ctx context.Context, m *Manifest, vars TemplateContext) (map[string]string, error) {
	if m.Spec.PluginKind != "sidecar" {
		return nil, fmt.Errorf("plugin %s: expected kind sidecar, got %q", m.Spec.ID, m.Spec.PluginKind)
	}
	if m.Spec.Sidecar == nil {
		return nil, fmt.Errorf("plugin %s: sidecar spec is nil", m.Spec.ID)
	}

	timeout, err := ParseTimeout(m.Spec.Sidecar.Timeout)
	if err != nil {
		return nil, fmt.Errorf("plugin %s: invalid timeout: %w", m.Spec.ID, err)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	body, err := json.Marshal(map[string]any{
		"fields": vars.Fields,
	})
	if err != nil {
		return nil, fmt.Errorf("plugin %s: marshal request: %w", m.Spec.ID, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.Spec.Sidecar.URL+"/runtime-env", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("plugin %s: build request: %w", m.Spec.ID, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("plugin %s unreachable: %w", m.Spec.ID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("plugin %s: runtime-env returned %d: %s", m.Spec.ID, resp.StatusCode, bytes.TrimSpace(snippet))
	}

	var result struct {
		Env map[string]string `json:"env"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return nil, fmt.Errorf("plugin %s: decode response: %w", m.Spec.ID, err)
	}

	if result.Env == nil {
		return map[string]string{}, nil
	}
	return result.Env, nil
}
