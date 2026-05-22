package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/mipopov/shclop/internal/config"
	"github.com/mipopov/shclop/internal/gateway"
)

func TestRuntimeWebSocketAcceptsRuntimeHello(t *testing.T) {
	server := newTestServerWithConfig(config.Config{Store: "inmemory"})
	adminToken := loginAsAdmin(t, server)
	created := doJSON(t, server, http.MethodPost, "/api/agents", map[string]any{
		"name":    "Runtime",
		"runtime": "nanoclaw",
		"model":   "",
	}, adminToken)
	agentID := assertJSONField(t, created.Body.Bytes(), "id", "")
	started := doJSON(t, server, http.MethodPost, "/api/agents/"+agentID+"/start", nil, adminToken)
	if got := assertJSONField(t, started.Body.Bytes(), "id", ""); got != agentID {
		t.Fatalf("expected start response agent id %s, got %q", agentID, got)
	}
	runtimeToken := server.tokens[agentID]
	if runtimeToken == "" {
		t.Fatal("expected runtime token")
	}
	testServer := httptest.NewServer(server.Handler())
	t.Cleanup(testServer.Close)

	wsURL := "ws" + strings.TrimPrefix(testServer.URL, "http") + "/runtime/ws"
	header := http.Header{"Authorization": []string{"Bearer " + runtimeToken}}
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("dial runtime websocket: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	if err := conn.WriteJSON(gateway.Envelope{
		Type:    "runtime.hello",
		AgentID: agentID,
		Payload: map[string]any{"runtime": "nanoclaw"},
	}); err != nil {
		t.Fatalf("write runtime hello: %v", err)
	}

	var response gateway.Envelope
	if err := conn.ReadJSON(&response); err != nil {
		t.Fatalf("read runtime accepted: %v", err)
	}
	if response.Type != "runtime.accepted" {
		t.Fatalf("expected runtime.accepted, got %q", response.Type)
	}
	if response.AgentID != agentID {
		t.Fatalf("expected agent id %s, got %q", agentID, response.AgentID)
	}
}

func TestRuntimeWebSocketRequiresToken(t *testing.T) {
	server := newTestServerWithConfig(config.Config{Store: "inmemory"})
	testServer := httptest.NewServer(server.Handler())
	t.Cleanup(testServer.Close)

	wsURL := "ws" + strings.TrimPrefix(testServer.URL, "http") + "/runtime/ws"
	_, response, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err == nil {
		t.Fatal("expected runtime websocket handshake to fail without token")
	}
	if response == nil || response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got response %#v and err %v", http.StatusUnauthorized, response, err)
	}
}
