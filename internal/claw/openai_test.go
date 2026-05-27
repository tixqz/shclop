package claw

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestOpenAIAdapterReturnsErrorWithoutEnv(t *testing.T) {
	// Unset any previously set env vars
	os.Unsetenv("LLM_GATEWAY_BASE_URL")
	os.Unsetenv("LLM_GATEWAY_MODEL")
	os.Unsetenv("LLM_GATEWAY_API_KEY")

	adapter := OpenAIAdapter{}
	_, err := adapter.Run(context.Background(), Task{Text: "hello"})
	if err == nil {
		t.Fatal("expected error when env vars are unset")
	}
	if !strings.Contains(err.Error(), "LLM_GATEWAY_BASE_URL") {
		t.Fatalf("expected error about missing base URL, got: %v", err)
	}
	if strings.Contains(err.Error(), "LLM_GATEWAY_API_KEY") {
		t.Fatalf("gateway API key must be optional for internal LiteLLM deployments, got: %v", err)
	}
}

func TestOpenAIAdapterReturnsTextResponse(t *testing.T) {
	chatResp := map[string]any{
		"choices": []any{
			map[string]any{
				"finish_reason": "stop",
				"message": map[string]any{
					"role":    "assistant",
					"content": "Hello world",
				},
			},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-key" {
			t.Fatalf("unexpected auth: %s", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(chatResp)
	}))
	defer srv.Close()

	t.Setenv("LLM_GATEWAY_BASE_URL", srv.URL+"/v1")
	t.Setenv("LLM_GATEWAY_MODEL", "test-model")
	t.Setenv("LLM_GATEWAY_API_KEY", "test-key")
	defer func() {
		os.Unsetenv("LLM_GATEWAY_BASE_URL")
		os.Unsetenv("LLM_GATEWAY_MODEL")
		os.Unsetenv("LLM_GATEWAY_API_KEY")
	}()

	adapter := OpenAIAdapter{}
	events, err := adapter.Run(context.Background(), Task{Text: "hello"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	var got []Event
	for event := range events {
		got = append(got, event)
	}

	// Expected: started, "Hello world", done
	if len(got) < 3 {
		t.Fatalf("expected at least 3 events, got %d: %v", len(got), got)
	}
	if got[0].Type != EventStarted {
		t.Fatalf("first event = %v", got[0])
	}
	var text string
	for _, e := range got {
		if e.Type == EventDelta {
			text += e.Text
		}
	}
	if text != "Hello world" {
		t.Fatalf("expected text 'Hello world', got %q", text)
	}
	if got[len(got)-1].Type != EventDone {
		t.Fatalf("last event = %v", got[len(got)-1])
	}
}

func TestOpenAIAdapterExecutesBashTool(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		callCount++
		if callCount == 1 {
			// First call: return tool_calls
			resp := map[string]any{
				"choices": []any{
					map[string]any{
						"finish_reason": "tool_calls",
						"message": map[string]any{
							"role":    "assistant",
							"content": "",
							"tool_calls": []any{
								map[string]any{
									"id":   "call_1",
									"type": "function",
									"function": map[string]any{
										"name":      "bash",
										"arguments": `{"command":"echo hello_from_bash"}`,
									},
								},
							},
						},
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
		} else {
			// Second call: return final text
			resp := map[string]any{
				"choices": []any{
					map[string]any{
						"finish_reason": "stop",
						"message": map[string]any{
							"role":    "assistant",
							"content": "Done.",
						},
					},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer srv.Close()

	t.Setenv("LLM_GATEWAY_BASE_URL", srv.URL)
	t.Setenv("LLM_GATEWAY_MODEL", "test-model")
	t.Setenv("LLM_GATEWAY_API_KEY", "test-key")
	defer func() {
		os.Unsetenv("LLM_GATEWAY_BASE_URL")
		os.Unsetenv("LLM_GATEWAY_MODEL")
		os.Unsetenv("LLM_GATEWAY_API_KEY")
	}()

	adapter := OpenAIAdapter{}
	events, err := adapter.Run(context.Background(), Task{Text: "run echo"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	var allText string
	var done bool
	for event := range events {
		if event.Type == EventDelta {
			allText += event.Text
		}
		if event.Type == EventDone {
			done = true
		}
		if event.Type == EventError {
			t.Fatalf("unexpected error: %v / %v", event.Err, event.Text)
		}
	}
	if !done {
		t.Fatal("expected EventDone")
	}
	if callCount != 2 {
		t.Fatalf("expected 2 LLM calls, got %d", callCount)
	}
	if !strings.Contains(allText, "hello_from_bash") {
		t.Fatalf("expected bash output in events, got: %q", allText)
	}
}

func TestOpenAIAdapterHandlesAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid api key"}`))
	}))
	defer srv.Close()

	t.Setenv("LLM_GATEWAY_BASE_URL", srv.URL)
	t.Setenv("LLM_GATEWAY_MODEL", "test-model")
	t.Setenv("LLM_GATEWAY_API_KEY", "bad-key")
	defer func() {
		os.Unsetenv("LLM_GATEWAY_BASE_URL")
		os.Unsetenv("LLM_GATEWAY_MODEL")
		os.Unsetenv("LLM_GATEWAY_API_KEY")
	}()

	adapter := OpenAIAdapter{}
	events, err := adapter.Run(context.Background(), Task{Text: "hello"})
	if err != nil {
		t.Fatalf("Run should not return error directly for HTTP errors: %v", err)
	}

	var last Event
	for event := range events {
		last = event
	}
	if last.Type != EventError {
		t.Fatalf("expected EventError, got %v", last)
	}
	if !strings.Contains(last.Text, "401") && !strings.Contains(last.Text, "invalid api key") {
		t.Fatalf("error text should mention failure, got: %s", last.Text)
	}
}

func TestOpenAIAdapterContextCancellation(t *testing.T) {
	// Server that holds connection open by never writing
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		select {
		case <-r.Context().Done():
			return
		case <-time.After(10 * time.Second):
		}
	}))
	defer srv.Close()

	t.Setenv("LLM_GATEWAY_BASE_URL", srv.URL)
	t.Setenv("LLM_GATEWAY_MODEL", "test-model")
	t.Setenv("LLM_GATEWAY_API_KEY", "test-key")
	defer func() {
		os.Unsetenv("LLM_GATEWAY_BASE_URL")
		os.Unsetenv("LLM_GATEWAY_MODEL")
		os.Unsetenv("LLM_GATEWAY_API_KEY")
	}()

	ctx, cancel := context.WithCancel(context.Background())
	adapter := OpenAIAdapter{}
	events, err := adapter.Run(ctx, Task{Text: "hello"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	cancel()

	select {
	case _, ok := <-events:
		if ok {
			// Channel should close after context cancel
		}
	case <-time.After(3 * time.Second):
		t.Fatal("channel did not close after context cancellation")
	}
}
