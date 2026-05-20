package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mipopov/shclop/internal/claw"
	"github.com/mipopov/shclop/internal/gateway"
)

func TestRuntimeTokenFromEnvPrefersFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "token")
	if err := os.WriteFile(path, []byte(" file-token \n"), 0o600); err != nil {
		t.Fatalf("write token file: %v", err)
	}
	t.Setenv("SHCLOP_RUNTIME_TOKEN_FILE", path)
	t.Setenv("SHCLOP_RUNTIME_TOKEN", "env-token")
	if got := runtimeTokenFromEnv(); got != "file-token" {
		t.Fatalf("expected file token, got %q", got)
	}
}

func TestRuntimeTokenFromEnvFallsBackToEnv(t *testing.T) {
	t.Setenv("SHCLOP_RUNTIME_TOKEN_FILE", filepath.Join(t.TempDir(), "missing"))
	t.Setenv("SHCLOP_RUNTIME_TOKEN", "env-token")
	if got := runtimeTokenFromEnv(); got != "env-token" {
		t.Fatalf("expected env token, got %q", got)
	}
}

func TestTaskTextExtractsPayloadText(t *testing.T) {
	text := taskText(map[string]any{"text": "hello"})
	if text != "hello" {
		t.Fatalf("got %q", text)
	}
}

func TestClawEventToEnvelopeMapsError(t *testing.T) {
	task := gateway.Envelope{AgentID: "agent", SessionID: "session", MessageID: "message"}
	envelope := clawEventToEnvelope(claw.Event{Type: claw.EventError, Err: assertError("boom"), ExitCode: 7}, task, 3)
	if envelope.Type != "message.error" || envelope.Seq != 3 {
		t.Fatalf("unexpected envelope: %#v", envelope)
	}
	if envelope.Payload["text"] != "boom" || envelope.Payload["exit_code"] != 7 {
		t.Fatalf("unexpected payload: %#v", envelope.Payload)
	}
}

func TestClawEventToEnvelopeMapsUnknownTypeToError(t *testing.T) {
	task := gateway.Envelope{AgentID: "agent", SessionID: "session", MessageID: "message"}
	envelope := clawEventToEnvelope(claw.Event{Type: claw.EventType("mystery")}, task, 1)
	if envelope.Type != "message.error" {
		t.Fatalf("unexpected type: %q", envelope.Type)
	}
	if got := envelope.Payload["text"]; got != "unknown claw event type: mystery" {
		t.Fatalf("unexpected payload text: %#v", got)
	}
}

func TestIsTerminalEnvelope(t *testing.T) {
	if !isTerminalEnvelope("message.done") || !isTerminalEnvelope("message.error") {
		t.Fatal("expected terminal envelope types")
	}
	if isTerminalEnvelope("message.delta") {
		t.Fatal("delta should not be terminal")
	}
}

func TestClawEventToEnvelopeMapsMissingTerminalToError(t *testing.T) {
	task := gateway.Envelope{AgentID: "agent", SessionID: "session", MessageID: "message"}
	envelope := clawEventToEnvelope(claw.Event{Type: claw.EventError, Text: "claw adapter ended without terminal event"}, task, 9)
	if envelope.Type != "message.error" || envelope.Seq != 9 {
		t.Fatalf("unexpected envelope: %#v", envelope)
	}
	if envelope.Payload["text"] != "claw adapter ended without terminal event" {
		t.Fatalf("unexpected payload: %#v", envelope.Payload)
	}
}

type assertError string

func (e assertError) Error() string { return string(e) }
