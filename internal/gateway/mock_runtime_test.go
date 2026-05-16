package gateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestMockRuntimeStreamsResponse(t *testing.T) {
	r := MockRuntime{}
	events := r.Respond("agent-1", "session-1", "msg-1", "hello")
	if len(events) < 3 {
		t.Fatalf("expected at least 3 events got %d", len(events))
	}
	if events[0].Type != "message.started" {
		t.Fatalf("unexpected first type %q", events[0].Type)
	}
	if events[len(events)-1].Type != "message.done" {
		t.Fatalf("unexpected final type %q", events[len(events)-1].Type)
	}
}

func TestRegistryDropsEventsFromWrongAgent(t *testing.T) {
	registry := NewRuntimeRegistry()
	server, client := pipeWebSocket(t)
	defer server.Close()
	defer client.Close()

	registry.Register("agent-a", server)
	events, cancel, err := registry.SendTask("agent-a", Envelope{Type: "task.run", AgentID: "agent-a", MessageID: "msg-1"})
	if err != nil {
		t.Fatalf("send task: %v", err)
	}
	defer cancel()

	registry.Dispatch("agent-a", server, Envelope{Type: "message.done", AgentID: "agent-b", MessageID: "msg-1"})
	select {
	case event := <-events:
		t.Fatalf("unexpected event from wrong agent: %#v", event)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestRegistryDropsEventsFromStaleConnection(t *testing.T) {
	registry := NewRuntimeRegistry()
	oldServer, oldClient := pipeWebSocket(t)
	defer oldClient.Close()
	newServer, newClient := pipeWebSocket(t)
	defer newServer.Close()
	defer newClient.Close()

	registry.Register("agent-a", oldServer)
	registry.Register("agent-a", newServer)
	events, cancel, err := registry.SendTask("agent-a", Envelope{Type: "task.run", AgentID: "agent-a", MessageID: "msg-1"})
	if err != nil {
		t.Fatalf("send task: %v", err)
	}
	defer cancel()

	registry.Dispatch("agent-a", oldServer, Envelope{Type: "message.done", AgentID: "agent-a", MessageID: "msg-1"})
	select {
	case event := <-events:
		t.Fatalf("unexpected event from stale connection: %#v", event)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestRegistryCancelDoesNotPanicDispatch(t *testing.T) {
	registry := NewRuntimeRegistry()
	server, client := pipeWebSocket(t)
	defer server.Close()
	defer client.Close()

	registry.Register("agent-a", server)
	_, cancel, err := registry.SendTask("agent-a", Envelope{Type: "task.run", AgentID: "agent-a", MessageID: "msg-1"})
	if err != nil {
		t.Fatalf("send task: %v", err)
	}
	cancel()
	registry.Dispatch("agent-a", server, Envelope{Type: "message.done", AgentID: "agent-a", MessageID: "msg-1"})
}

func TestRegistryUnregisterCompletesPendingWaiter(t *testing.T) {
	registry := NewRuntimeRegistry()
	server, client := pipeWebSocket(t)
	defer server.Close()
	defer client.Close()

	registry.Register("agent-a", server)
	events, cancel, err := registry.SendTask("agent-a", Envelope{Type: "task.run", AgentID: "agent-a", MessageID: "msg-1"})
	if err != nil {
		t.Fatalf("send task: %v", err)
	}
	defer cancel()

	registry.Unregister("agent-a", server)
	select {
	case event := <-events:
		if event.Type != "message.error" {
			t.Fatalf("expected message.error, got %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("expected pending waiter to receive terminal error")
	}
}

func pipeWebSocket(t *testing.T) (*websocket.Conn, *websocket.Conn) {
	t.Helper()
	serverConn := make(chan *websocket.Conn, 1)
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade websocket: %v", err)
			return
		}
		serverConn <- conn
	}))
	t.Cleanup(server.Close)
	client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	select {
	case conn := <-serverConn:
		return conn, client
	case <-time.After(time.Second):
		t.Fatal("server websocket not accepted")
		return nil, nil
	}
}
