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
	server := newTestServerWithConfig(config.Config{
		Store:        "inmemory",
		RuntimeToken: "agent-1:runtime-test-token",
	})
	testServer := httptest.NewServer(server.Handler())
	t.Cleanup(testServer.Close)

	wsURL := "ws" + strings.TrimPrefix(testServer.URL, "http") + "/runtime/ws"
	header := http.Header{"Authorization": []string{"Bearer runtime-test-token"}}
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("dial runtime websocket: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	if err := conn.WriteJSON(gateway.Envelope{
		Type:    "runtime.hello",
		AgentID: "agent-1",
		Payload: map[string]any{"runtime": "openclaw"},
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
	if response.AgentID != "agent-1" {
		t.Fatalf("expected agent id agent-1, got %q", response.AgentID)
	}
}

func TestRuntimeWebSocketRequiresToken(t *testing.T) {
	server := newTestServerWithConfig(config.Config{Store: "inmemory", RuntimeToken: "agent-1:runtime-test-token"})
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
