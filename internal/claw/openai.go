package claw

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// OpenAIAdapter implements Adapter for OpenAI-compatible chat completion APIs.
// It reads configuration from environment variables:
//   - LLM_GATEWAY_BASE_URL  — base URL for the API (for example, http://litellm:4000/v1)
//   - LLM_GATEWAY_MODEL     — model name exposed by the gateway
//   - LLM_GATEWAY_API_KEY   — optional gateway authentication token
type OpenAIAdapter struct{}

const systemPrompt = `You are an AI agent running inside a Linux container. You have full shell access via the bash tool.

Environment:
- Your workspace is at /workspace (read/write)
- Persistent memory storage is at /memory (read/write)
- If GITHUB_TOKEN is set, you have GitHub API access via curl or git
- Internet access is available on ports 53, 80, 443, and 4000

Important rules:
- Use the bash tool to complete tasks — don't just describe what you'd do, actually do it
- For GitHub operations, use the REST API with curl and GITHUB_TOKEN, or use git with the token embedded in the remote URL
- Always show the actual output of commands you run
- If a command fails, investigate and fix the issue`

var bashTool = map[string]any{
	"type": "function",
	"function": map[string]any{
		"name":        "bash",
		"description": "Run a bash command in the container. Use for file operations, API calls, git, etc.",
		"parameters": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "The bash command to execute",
				},
			},
			"required": []string{"command"},
		},
	},
}

type chatMessage struct {
	Role       string     `json:"role"`
	Content    any        `json:"content,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	Name       string     `json:"name,omitempty"`
}

type toolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

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
	if len(missing) > 0 {
		return nil, fmt.Errorf("OpenAIAdapter: required env vars not set: %s", strings.Join(missing, ", "))
	}

	chatURL := baseURL + "/chat/completions"

	out := make(chan Event, 16)
	go func() {
		defer close(out)
		out <- Event{Type: EventStarted}

		messages := []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: task.Text},
		}

		client := &http.Client{Timeout: 120 * time.Second}

		// Agentic tool-use loop (max 10 iterations)
		for i := 0; i < 10; i++ {
			reqBody := map[string]any{
				"model":      model,
				"messages":   messages,
				"tools":      []any{bashTool},
				"max_tokens": 4096,
			}
			bodyBytes, err := json.Marshal(reqBody)
			if err != nil {
				out <- Event{Type: EventError, Err: fmt.Errorf("marshal request: %w", err)}
				return
			}

			httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, chatURL, bytes.NewReader(bodyBytes))
			if err != nil {
				out <- Event{Type: EventError, Err: fmt.Errorf("create request: %w", err)}
				return
			}
			httpReq.Header.Set("Content-Type", "application/json")
			if apiKey != "" {
				httpReq.Header.Set("Authorization", "Bearer "+apiKey)
			}

			resp, err := client.Do(httpReq)
			if err != nil {
				out <- Event{Type: EventError, Err: fmt.Errorf("http request: %w", err)}
				return
			}
			respBytes, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				out <- Event{Type: EventError, Err: fmt.Errorf("read response: %w", err)}
				return
			}

			if resp.StatusCode != http.StatusOK {
				out <- Event{Type: EventError, Text: fmt.Sprintf("API error: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBytes)))}
				return
			}

			var chatResp struct {
				Choices []struct {
					FinishReason string `json:"finish_reason"`
					Message      struct {
						Role      string     `json:"role"`
						Content   string     `json:"content"`
						ToolCalls []toolCall `json:"tool_calls"`
					} `json:"message"`
				} `json:"choices"`
			}
			if err := json.Unmarshal(respBytes, &chatResp); err != nil || len(chatResp.Choices) == 0 {
				out <- Event{Type: EventError, Err: fmt.Errorf("parse response: %w", err)}
				return
			}

			choice := chatResp.Choices[0]
			assistantMsg := chatMessage{
				Role:      choice.Message.Role,
				Content:   choice.Message.Content,
				ToolCalls: choice.Message.ToolCalls,
			}
			messages = append(messages, assistantMsg)

			if choice.FinishReason != "tool_calls" || len(choice.Message.ToolCalls) == 0 {
				// No more tool calls — stream the final text response
				if choice.Message.Content != "" {
					out <- Event{Type: EventDelta, Text: choice.Message.Content}
				}
				out <- Event{Type: EventDone}
				return
			}

			// Execute tool calls and collect results
			for _, tc := range choice.Message.ToolCalls {
				if tc.Function.Name != "bash" {
					messages = append(messages, chatMessage{
						Role:       "tool",
						ToolCallID: tc.ID,
						Name:       tc.Function.Name,
						Content:    "error: unknown tool",
					})
					continue
				}

				var args struct {
					Command string `json:"command"`
				}
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					messages = append(messages, chatMessage{
						Role:       "tool",
						ToolCallID: tc.ID,
						Name:       "bash",
						Content:    fmt.Sprintf("error: invalid arguments: %v", err),
					})
					continue
				}

				out <- Event{Type: EventDelta, Text: fmt.Sprintf("\n```bash\n$ %s\n", args.Command)}

				output, execErr := runBash(ctx, args.Command)
				if output != "" {
					out <- Event{Type: EventDelta, Text: output}
				}
				out <- Event{Type: EventDelta, Text: "```\n"}

				toolResult := output
				if execErr != nil {
					toolResult = output + "\nexit error: " + execErr.Error()
				}
				if toolResult == "" {
					toolResult = "(no output)"
				}

				messages = append(messages, chatMessage{
					Role:       "tool",
					ToolCallID: tc.ID,
					Name:       "bash",
					Content:    toolResult,
				})
			}
		}

		out <- Event{Type: EventError, Text: "agent reached maximum tool-use iterations"}
	}()

	return out, nil
}

func runBash(ctx context.Context, command string) (string, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "bash", "-c", command)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()

	output := buf.String()
	const maxOutput = 8 * 1024
	if len(output) > maxOutput {
		output = output[:maxOutput] + "\n...(truncated)"
	}
	return output, err
}
