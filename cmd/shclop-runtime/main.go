package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mipopov/shclop/internal/gateway"
)

func main() {
	gatewayURL := flag.String("gateway", env("SHCLOP_GATEWAY_URL", "ws://localhost:8080/runtime/ws"), "runtime websocket URL")
	agentID := flag.String("agent-id", os.Getenv("SHCLOP_AGENT_ID"), "agent id to register")
	token := flag.String("token", os.Getenv("SHCLOP_RUNTIME_TOKEN"), "runtime token returned by agent start")
	runtimeName := flag.String("runtime", env("SHCLOP_AGENT_FLAVOR", "demo"), "runtime flavor name")
	flag.Parse()

	if strings.TrimSpace(*agentID) == "" || strings.TrimSpace(*token) == "" {
		log.Fatal("agent-id and token are required")
	}

	header := map[string][]string{"Authorization": {"Bearer " + *token}}
	conn, _, err := websocket.DefaultDialer.Dial(*gatewayURL, header)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(gateway.Envelope{Type: "runtime.hello", AgentID: *agentID, Payload: map[string]any{"runtime": *runtimeName}}); err != nil {
		log.Fatal(err)
	}
	var accepted gateway.Envelope
	if err := conn.ReadJSON(&accepted); err != nil {
		log.Fatal(err)
	}
	if accepted.Type != "runtime.accepted" {
		log.Fatalf("runtime rejected: %#v", accepted)
	}
	log.Printf("runtime registered: agent=%s flavor=%s", *agentID, *runtimeName)

	for {
		var task gateway.Envelope
		if err := conn.ReadJSON(&task); err != nil {
			log.Fatal(err)
		}
		if task.Type != "task.run" {
			continue
		}
		text, _ := task.Payload["text"].(string)
		events := []gateway.Envelope{
			{Type: "message.started", AgentID: task.AgentID, SessionID: task.SessionID, MessageID: task.MessageID, Seq: 1},
			{Type: "message.delta", AgentID: task.AgentID, SessionID: task.SessionID, MessageID: task.MessageID, Seq: 2, Payload: map[string]any{"text": fmt.Sprintf("%s runtime received: %s\n", *runtimeName, text)}},
			{Type: "message.delta", AgentID: task.AgentID, SessionID: task.SessionID, MessageID: task.MessageID, Seq: 3, Payload: map[string]any{"text": "workspace=/workspace memory=/memory\n"}},
			{Type: "message.done", AgentID: task.AgentID, SessionID: task.SessionID, MessageID: task.MessageID, Seq: 4},
		}
		for _, event := range events {
			if err := conn.WriteJSON(event); err != nil {
				log.Fatal(err)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
