package claw

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// OpenAIAdapter implements Adapter for OpenAI-compatible chat completion APIs.
// It reads configuration from environment variables:
//   - LLM_GATEWAY_BASE_URL  — base URL for the API (e.g. https://openrouter.ai/api/v1)
//   - LLM_GATEWAY_MODEL     — model name (e.g. deepseek/deepseek-v4-flash:free)
//   - LLM_GATEWAY_API_KEY   — API key for authentication
type OpenAIAdapter struct{}

func (a OpenAIAdapter) Run(ctx context.Context, task Task) (<-chan Event, error) {
	baseURL := strings.TrimRight(os.Getenv("LLM_GATEWAY_BASE_URL"), "/")
	model := os.Getenv("LLM_GATEWAY_MODEL")
	apiKey := os.Getenv("LLM_GATEWAY_API_KEY")

	var missing []string
	if baseURL == "" {
		missing = append(missing, "LLM_GATEWAY_BASE_URL")
	}
	if model == "" {
		missing = append(missing, "LLM_GATEWAY_MODEL")
	}
	if apiKey == "" {
		missing = append(missing, "LLM_GATEWAY_API_KEY")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("OpenAIAdapter: required env vars not set: %s", strings.Join(missing, ", "))
	}

	chatURL := baseURL + "/chat/completions"

	reqBody := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": task.Text},
		},
		"stream":    true,
		"max_tokens": 2048,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("OpenAIAdapter: marshal request: %w", err)
	}

	out := make(chan Event, 16)
	go func() {
		defer close(out)

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, chatURL, bytes.NewReader(bodyBytes))
		if err != nil {
			out <- Event{Type: EventError, Err: fmt.Errorf("create request: %w", err)}
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)

		client := &http.Client{Timeout: 120 * time.Second}
		resp, err := client.Do(httpReq)
		if err != nil {
			out <- Event{Type: EventError, Err: fmt.Errorf("http request: %w", err)}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			errBody, _ := io.ReadAll(resp.Body)
			out <- Event{Type: EventError, Text: fmt.Sprintf("API error: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(errBody)))}
			return
		}

		out <- Event{Type: EventStarted}

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		var buf bytes.Buffer
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}
			var chunk struct {
				Choices []struct {
					Delta struct {
						Content      string `json:"content"`
						FinishReason string `json:"finish_reason"`
					} `json:"delta"`
				} `json:"choices"`
			}
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}
			for _, choice := range chunk.Choices {
				if choice.Delta.Content != "" {
					out <- Event{Type: EventDelta, Text: choice.Delta.Content}
				}
				if buf.Len() > 0 {
					buf.Reset()
				}
				_ = buf // keep for potential future buffering
			}
		}
		if err := scanner.Err(); err != nil {
			out <- Event{Type: EventError, Err: fmt.Errorf("read stream: %w", err)}
			return
		}

		out <- Event{Type: EventDone}
	}()

	return out, nil
}
