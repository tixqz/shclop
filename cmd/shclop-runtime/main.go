package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/mipopov/shclop/internal/claw"
	"github.com/mipopov/shclop/internal/gateway"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	gatewayURL := flag.String("gateway", env("SHCLOP_GATEWAY_URL", "ws://localhost:8080/runtime/ws"), "runtime websocket URL")
	agentID := flag.String("agent-id", os.Getenv("SHCLOP_AGENT_ID"), "agent id to register")
	token := flag.String("token", runtimeTokenFromEnv(), "runtime token returned by agent start")
	runtimeName := flag.String("runtime", env("SHCLOP_AGENT_FLAVOR", "demo"), "runtime flavor name")
	flag.Parse()

	if strings.TrimSpace(*agentID) == "" || strings.TrimSpace(*token) == "" {
		logger.Error("agent-id and token are required")
		os.Exit(1)
	}
	llmGatewayConfigured := strings.TrimSpace(os.Getenv("LLM_GATEWAY_BASE_URL")) != ""
	llmModel := strings.TrimSpace(os.Getenv("LLM_GATEWAY_MODEL"))

	header := map[string][]string{"Authorization": {"Bearer " + *token}}
	conn, _, err := websocket.DefaultDialer.Dial(*gatewayURL, header)
	if err != nil {
		logger.Error("failed to connect runtime websocket", "error", err)
		os.Exit(1)
	}
	defer conn.Close()

	if err := conn.WriteJSON(gateway.Envelope{Type: "runtime.hello", AgentID: *agentID, Payload: map[string]any{"runtime": *runtimeName}}); err != nil {
		logger.Error("failed to send runtime hello", "error", err)
		os.Exit(1)
	}
	var accepted gateway.Envelope
	if err := conn.ReadJSON(&accepted); err != nil {
		logger.Error("failed to read runtime acceptance", "error", err)
		os.Exit(1)
	}
	if accepted.Type != "runtime.accepted" {
		logger.Error("runtime rejected", "type", accepted.Type, "payload", accepted.Payload)
		os.Exit(1)
	}
	logger.Info("runtime registered", "agent_id", *agentID, "runtime", *runtimeName, "llm_gateway_configured", llmGatewayConfigured, "model", llmModel)

	for {
		var task gateway.Envelope
		if err := conn.ReadJSON(&task); err != nil {
			logger.Error("failed to read task", "error", err, "agent_id", *agentID)
			os.Exit(1)
		}
		if task.Type != "task.run" {
			continue
		}
		logger.Info("task received", "agent_id", *agentID, "runtime", *runtimeName, "message_id", task.MessageID)
		taskCtx, cancel := context.WithCancel(context.Background())
		events, err := adapterForRuntime(*runtimeName).Run(taskCtx, claw.Task{Text: taskText(task.Payload)})
		if err != nil {
			cancel()
			if writeErr := conn.WriteJSON(clawEventToEnvelope(claw.Event{Type: claw.EventError, Err: err}, task, 1)); writeErr != nil {
				logger.Error("failed to send task error", "error", writeErr, "agent_id", *agentID)
				os.Exit(1)
			}
			continue
		}
		seq := 1
		terminal := false
		for event := range events {
			envelope := clawEventToEnvelope(event, task, seq)
			seq++
			if err := conn.WriteJSON(envelope); err != nil {
				cancel()
				logger.Error("failed to send task event", "error", err, "agent_id", *agentID, "event", envelope.Type)
				os.Exit(1)
			}
			if isTerminalEnvelope(envelope.Type) {
				terminal = true
				cancel()
				break
			}
		}
		if !terminal {
			envelope := clawEventToEnvelope(claw.Event{Type: claw.EventError, Text: "claw adapter ended without terminal event"}, task, seq)
			cancel()
			if err := conn.WriteJSON(envelope); err != nil {
				logger.Error("failed to send terminal task event", "error", err, "agent_id", *agentID)
				os.Exit(1)
			}
		}
	}
}

func adapterForRuntime(runtimeName string) claw.Adapter {
	// If LLM gateway is configured, use the real OpenAI-compatible adapter.
	if strings.TrimSpace(os.Getenv("LLM_GATEWAY_BASE_URL")) != "" &&
		strings.TrimSpace(os.Getenv("LLM_GATEWAY_API_KEY")) != "" {
		return claw.OpenAIAdapter{}
	}
	// Otherwise fall back to the demo adapter that echoes the input.
	return claw.DemoAdapter{Flavor: runtimeName}
}

func taskText(payload map[string]any) string {
	text, _ := payload["text"].(string)
	return text
}

func clawEventToEnvelope(event claw.Event, task gateway.Envelope, seq int) gateway.Envelope {
	envelope := gateway.Envelope{AgentID: task.AgentID, SessionID: task.SessionID, MessageID: task.MessageID, Seq: int64(seq)}
	switch event.Type {
	case claw.EventStarted:
		envelope.Type = "message.started"
	case claw.EventDelta:
		envelope.Type = "message.delta"
		envelope.Payload = map[string]any{"text": event.Text}
	case claw.EventDone:
		envelope.Type = "message.done"
	case claw.EventError:
		envelope.Type = "message.error"
		text := event.Text
		if event.Err != nil {
			text = event.Err.Error()
		}
		payload := map[string]any{"text": text}
		if event.ExitCode != 0 {
			payload["exit_code"] = event.ExitCode
		}
		envelope.Payload = payload
	default:
		envelope.Type = "message.error"
		envelope.Payload = map[string]any{"text": "unknown claw event type: " + string(event.Type)}
	}
	return envelope
}

func isTerminalEnvelope(eventType string) bool {
	return eventType == "message.done" || eventType == "message.error"
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func runtimeTokenFromEnv() string {
	if path := strings.TrimSpace(os.Getenv("SHCLOP_RUNTIME_TOKEN_FILE")); path != "" {
		content, err := os.ReadFile(path)
		if err == nil {
			return strings.TrimSpace(string(content))
		}
	}
	return os.Getenv("SHCLOP_RUNTIME_TOKEN")
}
