package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mipopov/shclop/internal/gateway"
)

func TestFunctionalDemoRoutesBrowserTaskThroughRuntime(t *testing.T) {
	server := newTestServer()
	token := loginAsAdmin(t, server)

	created := doJSON(t, server, http.MethodPost, "/api/agents", map[string]string{"name": "Demo agent"}, token)
	if created.Code != http.StatusCreated {
		t.Fatalf("expected create status %d, got %d", http.StatusCreated, created.Code)
	}
	agentID := assertJSONField(t, created.Body.Bytes(), "id", "")
	if agentID == "" {
		t.Fatal("expected created agent id")
	}

	started := doJSON(t, server, http.MethodPost, "/api/agents/"+agentID+"/start", map[string]string{"runtime": "openclaw"}, token)
	if started.Code != http.StatusAccepted {
		t.Fatalf("expected start status %d, got %d: %s", http.StatusAccepted, started.Code, started.Body.String())
	}
	runtimeToken := assertJSONField(t, started.Body.Bytes(), "runtime_token", "")
	if runtimeToken == "" {
		t.Fatal("expected runtime token")
	}

	testServer := httptest.NewServer(server.Handler())
	t.Cleanup(testServer.Close)
	wsBase := "ws" + strings.TrimPrefix(testServer.URL, "http")

	runtimeConn, _, err := websocket.DefaultDialer.Dial(wsBase+"/runtime/ws", http.Header{"Authorization": []string{"Bearer " + runtimeToken}})
	if err != nil {
		t.Fatalf("dial runtime websocket: %v", err)
	}
	t.Cleanup(func() { _ = runtimeConn.Close() })
	if err := runtimeConn.WriteJSON(gateway.Envelope{Type: "runtime.hello", AgentID: agentID, Payload: map[string]any{"runtime": "openclaw"}}); err != nil {
		t.Fatalf("write runtime hello: %v", err)
	}
	var accepted gateway.Envelope
	if err := runtimeConn.ReadJSON(&accepted); err != nil {
		t.Fatalf("read runtime accepted: %v", err)
	}
	if accepted.Type != "runtime.accepted" {
		t.Fatalf("expected runtime.accepted, got %q", accepted.Type)
	}

	browserConn, _, err := websocket.DefaultDialer.Dial(wsBase+"/ws", http.Header{"Authorization": []string{"Bearer " + token}})
	if err != nil {
		t.Fatalf("dial browser websocket: %v", err)
	}
	t.Cleanup(func() { _ = browserConn.Close() })
	messageID := "msg-demo"
	if err := browserConn.WriteJSON(gateway.Envelope{Type: "user.message", AgentID: agentID, SessionID: "session-demo", MessageID: messageID, Payload: map[string]any{"text": "hello runtime"}}); err != nil {
		t.Fatalf("write browser message: %v", err)
	}

	var task gateway.Envelope
	if err := runtimeConn.ReadJSON(&task); err != nil {
		t.Fatalf("runtime did not receive task: %v", err)
	}
	if task.Type != "task.run" || task.AgentID != agentID || task.MessageID != messageID {
		t.Fatalf("unexpected task envelope: %#v", task)
	}

	for _, eventType := range []string{"message.started", "message.delta", "message.done"} {
		if err := runtimeConn.WriteJSON(gateway.Envelope{Type: eventType, AgentID: agentID, SessionID: task.SessionID, MessageID: messageID, Payload: map[string]any{"text": eventType}}); err != nil {
			t.Fatalf("write runtime event %s: %v", eventType, err)
		}
	}

	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("browser did not receive message.done")
		default:
		}
		var event gateway.Envelope
		if err := browserConn.ReadJSON(&event); err != nil {
			t.Fatalf("read browser event: %v", err)
		}
		if event.Type == "message.done" && event.MessageID == messageID {
			return
		}
	}
}
