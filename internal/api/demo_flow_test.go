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

	created := doJSON(t, server, http.MethodPost, "/api/agents", map[string]any{
		"name":    "Demo agent",
		"runtime": "nanoclaw",
		"model":   "",
	}, token)
	if created.Code != http.StatusCreated {
		t.Fatalf("expected create status %d, got %d: %s", http.StatusCreated, created.Code, created.Body.String())
	}
	agentID := assertJSONField(t, created.Body.Bytes(), "id", "")
	if agentID == "" {
		t.Fatal("expected created agent id")
	}

	started := doJSON(t, server, http.MethodPost, "/api/agents/"+agentID+"/start", nil, token)
	if started.Code != http.StatusAccepted {
		t.Fatalf("expected start status %d, got %d: %s", http.StatusAccepted, started.Code, started.Body.String())
	}
	if got := assertJSONField(t, started.Body.Bytes(), "id", ""); got != agentID {
		t.Fatalf("expected start response agent id %s, got %q", agentID, got)
	}
	runtimeToken := server.tokens[agentID]
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
	if err := runtimeConn.WriteJSON(gateway.Envelope{Type: "runtime.hello", AgentID: agentID, Payload: map[string]any{"runtime": "nanoclaw"}}); err != nil {
		t.Fatalf("write runtime hello: %v", err)
	}
	var accepted gateway.Envelope
	if err := runtimeConn.ReadJSON(&accepted); err != nil {
		t.Fatalf("read runtime accepted: %v", err)
	}
	if accepted.Type != "runtime.accepted" {
		t.Fatalf("expected runtime.accepted, got %q", accepted.Type)
	}
	if accepted.AgentID != agentID {
		t.Fatalf("expected agent id %s, got %q", agentID, accepted.AgentID)
	}

	browserConn, _, err := websocket.DefaultDialer.Dial(wsBase+"/ws?agent_id="+agentID, http.Header{"Authorization": []string{"Bearer " + token}})
	if err != nil {
		t.Fatalf("dial browser websocket: %v", err)
	}
	t.Cleanup(func() { _ = browserConn.Close() })
	if err := browserConn.WriteJSON(map[string]any{"text": "hello", "session_id": "sess-1", "message_id": "msg-1"}); err != nil {
		t.Fatalf("write browser message: %v", err)
	}

	var runtimeEvent gateway.Envelope
	if err := runtimeConn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	if err := runtimeConn.ReadJSON(&runtimeEvent); err != nil {
		t.Fatalf("read runtime event: %v", err)
	}
	if runtimeEvent.Type != "task.run" {
		t.Fatalf("expected task.run, got %q", runtimeEvent.Type)
	}

	if err := runtimeConn.WriteJSON(gateway.Envelope{Type: "message.chunk", AgentID: agentID, SessionID: "sess-1", MessageID: "msg-1", Seq: 2, Payload: map[string]any{"text": "hello back"}}); err != nil {
		t.Fatalf("write runtime response: %v", err)
	}
	if err := runtimeConn.WriteJSON(gateway.Envelope{Type: "message.done", AgentID: agentID, SessionID: "sess-1", MessageID: "msg-1", Seq: 3}); err != nil {
		t.Fatalf("write runtime done: %v", err)
	}

	var browserEvent map[string]any
	if err := browserConn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	if err := browserConn.ReadJSON(&browserEvent); err != nil {
		t.Fatalf("read browser event: %v", err)
	}
	if browserEvent["type"] != "message.chunk" {
		t.Fatalf("expected message.chunk, got %#v", browserEvent["type"])
	}
	if browserEvent["text"] != "hello back" {
		t.Fatalf("expected browser text, got %#v", browserEvent["text"])
	}
}
