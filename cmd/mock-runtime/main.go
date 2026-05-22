package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type Envelope struct {
	Type      string         `json:"type"`
	AgentID   string         `json:"agent_id,omitempty"`
	SessionID string         `json:"session_id,omitempty"`
	MessageID string         `json:"message_id,omitempty"`
	Seq       int64          `json:"seq,omitempty"`
	Payload   map[string]any `json:"payload,omitempty"`
}

func main() {
	gatewayURL := flag.String("gateway", env("SHCLOP_GATEWAY_URL", "ws://localhost:8080/runtime/ws"), "gateway websocket URL")
	agentID := flag.String("agent-id", env("SHCLOP_AGENT_ID", ""), "agent id")
	token := flag.String("token", env("SHCLOP_RUNTIME_TOKEN", ""), "runtime token")
	runtime := flag.String("runtime", env("SHCLOP_AGENT_FLAVOR", "openclaw"), "runtime flavor")
	flag.Parse()

	if *agentID == "" || *token == "" {
		log.Fatal("agent-id and token are required")
	}

	header := http.Header{"Authorization": {"Bearer " + *token}}
	conn, _, err := websocket.DefaultDialer.Dial(*gatewayURL, header)
	if err != nil {
		log.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	// Register
	if err := conn.WriteJSON(Envelope{Type: "runtime.hello", AgentID: *agentID, Payload: map[string]any{"runtime": *runtime}}); err != nil {
		log.Fatalf("failed to send hello: %v", err)
	}

	var accepted Envelope
	if err := conn.ReadJSON(&accepted); err != nil {
		log.Fatalf("failed to read accepted: %v", err)
	}
	if accepted.Type != "runtime.accepted" {
		log.Fatalf("runtime rejected: %+v", accepted)
	}
	log.Printf("registered: agent=%s flavor=%s", *agentID, *runtime)

	// Process tasks
	for {
		var task Envelope
		if err := conn.ReadJSON(&task); err != nil {
			log.Printf("read error: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}
		if task.Type != "task.run" {
			continue
		}

		text := taskText(task.Payload)
		log.Printf("task received: %q", text)

		// message.started
		conn.WriteJSON(Envelope{Type: "message.started", AgentID: task.AgentID, SessionID: task.SessionID, MessageID: task.MessageID, Seq: 1})

		// message.delta chunks
		response := generateResponse(text, *runtime)
		chunks := splitChunks(response)
		seq := int64(2)
		for _, chunk := range chunks {
			time.Sleep(200 * time.Millisecond)
			conn.WriteJSON(Envelope{Type: "message.delta", AgentID: task.AgentID, SessionID: task.SessionID, MessageID: task.MessageID, Seq: seq, Payload: map[string]any{"text": chunk}})
			seq++
		}

		// message.done
		conn.WriteJSON(Envelope{Type: "message.done", AgentID: task.AgentID, SessionID: task.SessionID, MessageID: task.MessageID, Seq: seq})
	}
}

func generateResponse(input, flavor string) string {
	input = strings.ToLower(strings.TrimSpace(input))
	switch {
	case strings.Contains(input, "hello") || strings.Contains(input, "hi") || strings.Contains(input, "привет"):
		return fmt.Sprintf("Hello! I'm %s, your AI assistant. I'm running inside a shclop sandbox. How can I help you today?", flavor)
	case strings.Contains(input, "who are you") || strings.Contains(input, "what are you"):
		return fmt.Sprintf("I'm %s, an AI agent running in a shclop sandbox. I'm isolated in my own runtime with no access to host secrets, network, or other agents. My communication goes through the shclop gateway.", flavor)
	case strings.Contains(input, "help"):
		return "I can help you with:\n- Answering questions\n- Writing code\n- Analyzing data\n- Automating tasks\n\nJust ask me anything!"
	case strings.Contains(input, "code") || strings.Contains(input, "program"):
		return "I can help with code! Here's a simple example:\n\n```go\nfunc main() {\n    fmt.Println(\"Hello from shclop!\")\n}\n```\n\nWhat would you like to build?"
	case strings.Contains(input, "status") || strings.Contains(input, "health"):
		return fmt.Sprintf("Status: Running\nRuntime: %s\nSandbox: shclop mock\nNetwork: isolated\nSecrets: none\n\nEverything looks good!", flavor)
	default:
		return fmt.Sprintf("I received your message: \"%s\"\n\nI'm a demo agent running in shclop's mock sandbox. In production, I'd be running as a Kata microVM with full isolation. For now, I'm demonstrating the chat flow through the shclop gateway.", input)
	}
}

func splitChunks(text string) []string {
	words := strings.Fields(text)
	var chunks []string
	var current string
	for _, w := range words {
		if len(current)+len(w)+1 > 30 {
			chunks = append(chunks, current)
			current = w
		} else if current == "" {
			current = w
		} else {
			current += " " + w
		}
	}
	if current != "" {
		chunks = append(chunks, current)
	}
	return chunks
}

func taskText(payload map[string]any) string {
	if payload == nil {
		return ""
	}
	if text, ok := payload["text"].(string); ok {
		return text
	}
	data, _ := json.Marshal(payload)
	return string(data)
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
