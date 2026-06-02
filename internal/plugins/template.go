package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"text/template"
)

type TemplateContext struct {
	Token  string
	Fields map[string]string
}

type ValidationResult struct {
	ExternalAccountID string
	ExternalLogin     string
	AccountType       string
}

type HTTPEvaluator struct {
	Client *http.Client
}

func NewHTTPEvaluator() *HTTPEvaluator {
	return &HTTPEvaluator{}
}

func (e *HTTPEvaluator) Validate(ctx context.Context, m *Manifest, vars TemplateContext) (ValidationResult, error) {
	if m.Spec.PluginKind != "http_template" || m.Spec.Validate.HTTP == nil {
		return ValidationResult{}, fmt.Errorf("manifest is not http_template kind or missing http spec")
	}

	h := m.Spec.Validate.HTTP

	timeout, err := ParseTimeout(h.Timeout)
	if err != nil {
		return ValidationResult{}, fmt.Errorf("parse timeout: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	rawURL, err := renderTemplate("url", h.URL, vars)
	if err != nil {
		return ValidationResult{}, fmt.Errorf("render url: %w", err)
	}

	method := h.Method
	if method == "" {
		method = http.MethodGet
	}

	req, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
	if err != nil {
		return ValidationResult{}, fmt.Errorf("build request: %w", err)
	}

	for k, v := range h.Headers {
		rendered, err := renderTemplate("header:"+k, v, vars)
		if err != nil {
			return ValidationResult{}, fmt.Errorf("render header %s: %w", k, err)
		}
		req.Header.Set(k, rendered)
	}

	client := e.Client
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return ValidationResult{}, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return ValidationResult{}, fmt.Errorf("read body: %w", err)
	}

	if !slices.Contains(h.SuccessStatus, resp.StatusCode) {
		snippet := bodyBytes
		if len(snippet) > 1024 {
			snippet = snippet[:1024]
		}
		return ValidationResult{}, fmt.Errorf("validate failed: status=%d, body=%s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}

	var body map[string]any
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		return ValidationResult{}, fmt.Errorf("unmarshal response: %w", err)
	}

	var result ValidationResult
	if h.Extract.ExternalAccountID != "" {
		v, _ := jsonPath(body, h.Extract.ExternalAccountID)
		result.ExternalAccountID = v
	}
	if h.Extract.ExternalLogin != "" {
		v, _ := jsonPath(body, h.Extract.ExternalLogin)
		result.ExternalLogin = v
	}
	if h.Extract.AccountType != "" {
		v, _ := jsonPath(body, h.Extract.AccountType)
		result.AccountType = v
	}

	return result, nil
}

func (e *HTTPEvaluator) BuildEnv(m *Manifest, vars TemplateContext) (map[string]string, error) {
	out := make(map[string]string, len(m.Spec.Contributions.Env))
	for k, v := range m.Spec.Contributions.Env {
		rendered, err := renderTemplate("env:"+k, v, vars)
		if err != nil {
			return nil, fmt.Errorf("render env %s: %w", k, err)
		}
		out[k] = rendered
	}
	return out, nil
}

func renderTemplate(name, tmpl string, vars TemplateContext) (string, error) {
	t, err := template.New(name).Funcs(sandboxedFuncs()).Option("missingkey=error").Parse(tmpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, vars); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func jsonPath(root any, path string) (string, error) {
	path = strings.TrimPrefix(path, ".")
	if path == "" {
		return fmt.Sprintf("%v", root), nil
	}
	segments := strings.Split(path, ".")
	cur := root
	for _, seg := range segments {
		m, ok := cur.(map[string]any)
		if !ok {
			return "", nil
		}
		val, exists := m[seg]
		if !exists {
			return "", nil
		}
		cur = val
	}
	return fmt.Sprintf("%v", cur), nil
}
