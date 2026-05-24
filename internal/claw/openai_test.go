package claw

import (
	"context"
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
}

func TestOpenAIAdapterStreamsSSEResponse(t *testing.T) {
	sseContent := `data: {"id":"1","object":"chat.completion.chunk","choices":[{"delta":{"content":"Hello"},"index":0}]}

data: {"id":"2","object":"chat.completion.chunk","choices":[{"delta":{"content":" world"},"index":0}]}

data: {"id":"3","object":"chat.completion.chunk","choices":[{"delta":{},"finish_reason":"stop","index":0}]}

data: [DONE]

`
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
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(sseContent))
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

	// Expected: started, "Hello", " world", done
	if len(got) < 4 {
		t.Fatalf("expected at least 4 events, got %d: %v", len(got), got)
	}
	if got[0].Type != EventStarted {
		t.Fatalf("first event = %v", got[0])
	}
	if got[1].Type != EventDelta || got[1].Text != "Hello" {
		t.Fatalf("second event = %v", got[1])
	}
	if got[2].Type != EventDelta || got[2].Text != " world" {
		t.Fatalf("third event = %v", got[2])
	}
	if got[3].Type != EventDone {
		t.Fatalf("last event = %v", got[3])
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
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
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
