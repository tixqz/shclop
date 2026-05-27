package claw

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// NanoclawAdapter runs nano-claw as a subprocess for each task.
// nano-claw handles the full LLM ↔ tool execution loop internally.
// It reads LLM_GATEWAY_BASE_URL / LLM_GATEWAY_MODEL / LLM_GATEWAY_API_KEY and
// writes a nano-claw config file before invoking the binary.
type NanoclawAdapter struct{}

// OpenclawAdapter is the same wiring for the openclaw binary.
type OpenclawAdapter struct{}

func (a NanoclawAdapter) Run(ctx context.Context, task Task) (<-chan Event, error) {
	if err := writeNanoclawConfig(); err != nil {
		return nil, fmt.Errorf("nanoclaw: write config: %w", err)
	}
	args := []string{"agent", "-m", task.Text}
	if task.SessionID != "" {
		args = append(args, "--session", task.SessionID)
	}
	return SubprocessAdapter{
		Binary: "nano-claw",
		Args:   args,
		Env:    nanoclawEnv(),
	}.Run(ctx, task)
}

func (a OpenclawAdapter) Run(ctx context.Context, task Task) (<-chan Event, error) {
	if err := writeNanoclawConfig(); err != nil {
		return nil, fmt.Errorf("openclaw: write config: %w", err)
	}
	args := []string{"agent", "--message", task.Text}
	if task.SessionID != "" {
		args = append(args, "--session", task.SessionID)
	}
	return SubprocessAdapter{
		Binary: "openclaw",
		Args:   args,
		Env:    nanoclawEnv(),
	}.Run(ctx, task)
}

// writeNanoclawConfig writes ~/.nano-claw/config.json configured for the
// in-cluster LiteLLM gateway. Uses the openai provider with a custom apiBase
// so that the model name sent to LiteLLM matches what is registered there.
func writeNanoclawConfig() error {
	baseURL := strings.TrimRight(os.Getenv("LLM_GATEWAY_BASE_URL"), "/")
	model := os.Getenv("LLM_GATEWAY_MODEL")
	apiKey := os.Getenv("LLM_GATEWAY_API_KEY")
	if apiKey == "" {
		apiKey = "sk-shclop"
	}
	if baseURL == "" || model == "" {
		return fmt.Errorf("LLM_GATEWAY_BASE_URL and LLM_GATEWAY_MODEL must be set")
	}

	cfg := map[string]any{
		"providers": map[string]any{
			// openai provider with custom apiBase → OpenAIProvider strips the
			// "openai/" prefix when calling the API, giving LiteLLM the bare
			// model name it expects.
			"openai": map[string]any{
				"apiKey":  apiKey,
				"apiBase": baseURL,
			},
		},
		"agents": map[string]any{
			"defaults": map[string]any{
				"model": "openai/" + model,
			},
		},
		"tools": map[string]any{
			"restrictToWorkspace": false,
		},
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	configDir := nanoclawConfigDir()
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("create config dir %s: %w", configDir, err)
	}
	return os.WriteFile(filepath.Join(configDir, "config.json"), data, 0600)
}

// nanoclawConfigDir returns a writable directory for nano-claw config.
// Tries known writable mounts (/workspace, /memory) before falling back to
// the user home dir. The directory is used as HOME for the subprocess so that
// nano-claw resolves ~/.nano-claw/config.json inside it.
func nanoclawConfigDir() string {
	for _, base := range []string{"/workspace", "/memory"} {
		dir := filepath.Join(base, ".nano-claw")
		if testWritable(dir) {
			return dir
		}
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		dir := filepath.Join(home, ".nano-claw")
		if testWritable(dir) {
			return dir
		}
	}
	return "/tmp/.nano-claw"
}

func testWritable(dir string) bool {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return false
	}
	tmp := filepath.Join(dir, ".write-test")
	if err := os.WriteFile(tmp, []byte(""), 0600); err != nil {
		return false
	}
	_ = os.Remove(tmp)
	return true
}

// nanoclawEnv returns the environment for the nano-claw subprocess, ensuring
// HOME points to a directory that contains the written config.
func nanoclawEnv() []string {
	configDir := nanoclawConfigDir()
	home := filepath.Dir(configDir)

	env := make([]string, 0, len(os.Environ())+1)
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "HOME=") {
			continue
		}
		env = append(env, e)
	}
	env = append(env, "HOME="+home)
	return env
}
