package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/mipopov/shclop/internal/config"
	"github.com/mipopov/shclop/internal/gateway"
)

func TestHealth(t *testing.T) {
	server := newTestServer()

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	assertJSONField(t, response.Body.Bytes(), "status", "ok")
}

func TestLoginAndCreateAgent(t *testing.T) {
	server := newTestServer()

	login := doJSON(t, server, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "admin",
		"password": "admin",
	}, "")
	if login.Code != http.StatusOK {
		t.Fatalf("expected login status %d, got %d", http.StatusOK, login.Code)
	}
	token := assertJSONField(t, login.Body.Bytes(), "token", "")
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	user := assertJSONObject(t, login.Body.Bytes(), "user")
	if user["username"] != "admin" {
		t.Fatalf("expected username admin, got %v", user["username"])
	}

	created := doJSON(t, server, http.MethodPost, "/api/agents", map[string]string{
		"name": "Researcher",
	}, token)
	if created.Code != http.StatusCreated {
		t.Fatalf("expected create status %d, got %d", http.StatusCreated, created.Code)
	}
	assertJSONField(t, created.Body.Bytes(), "name", "Researcher")
	assertJSONField(t, created.Body.Bytes(), "owner_id", "user-admin")
}

func TestInvalidLoginReturnsUnauthorized(t *testing.T) {
	server := newTestServer()

	response := doJSON(t, server, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "admin",
		"password": "wrong",
	}, "")

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, response.Code)
	}
}

func TestAgentsRequireValidBearerToken(t *testing.T) {
	server := newTestServer()

	for _, token := range []string{"", "not-a-real-token"} {
		t.Run("token="+token, func(t *testing.T) {
			response := doJSON(t, server, http.MethodGet, "/api/agents", nil, token)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, response.Code)
			}
		})
	}
}

func TestListAgentsReturnsCurrentUsersAgents(t *testing.T) {
	server := newTestServer()
	token := loginAsAdmin(t, server)

	doJSON(t, server, http.MethodPost, "/api/agents", map[string]string{"name": "Researcher"}, token)
	doJSON(t, server, http.MethodPost, "/api/agents", map[string]string{"name": "Writer"}, token)

	response := doJSON(t, server, http.MethodGet, "/api/agents", nil, token)
	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	var agents []map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &agents); err != nil {
		t.Fatalf("decode agents: %v", err)
	}
	if len(agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(agents))
	}
	if agents[0]["name"] != "Researcher" || agents[1]["name"] != "Writer" {
		t.Fatalf("unexpected agents: %#v", agents)
	}
}

func TestCreateAgentRejectsBadPayload(t *testing.T) {
	server := newTestServer()
	token := loginAsAdmin(t, server)

	for _, body := range []string{`{}`, `{"name":"   "}`, `{not-json`} {
		t.Run(body, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/agents", bytes.NewBufferString(body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", "Bearer "+token)

			response := httptest.NewRecorder()
			server.Handler().ServeHTTP(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("expected status %d, got %d", http.StatusBadRequest, response.Code)
			}
		})
	}
}

func TestWebSocketStreamsMockResponse(t *testing.T) {
	server := newTestServer()
	testServer := httptest.NewServer(server.Handler())
	t.Cleanup(testServer.Close)

	wsURL := "ws" + strings.TrimPrefix(testServer.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Fatalf("close websocket: %v", err)
		}
	})

	incoming := gateway.Envelope{
		Type:      "message.create",
		AgentID:   "agent-1",
		SessionID: "session-1",
		MessageID: "msg-1",
		Seq:       1,
		Payload:   map[string]any{"text": "hello"},
	}
	if err := conn.WriteJSON(incoming); err != nil {
		t.Fatalf("write websocket message: %v", err)
	}

	var first gateway.Envelope
	if err := conn.ReadJSON(&first); err != nil {
		t.Fatalf("read first websocket event: %v", err)
	}
	if first.Type != "message.started" {
		t.Fatalf("expected first event message.started, got %q", first.Type)
	}

	for {
		var event gateway.Envelope
		if err := conn.ReadJSON(&event); err != nil {
			t.Fatalf("read websocket event: %v", err)
		}
		if event.Type == "message.done" {
			return
		}
	}
}

func TestWrongMethodsReturnMethodNotAllowed(t *testing.T) {
	server := newTestServer()

	for _, test := range []struct {
		name   string
		method string
		path   string
		allow  string
	}{
		{name: "health", method: http.MethodPost, path: "/healthz", allow: http.MethodGet},
		{name: "login", method: http.MethodGet, path: "/api/auth/login", allow: http.MethodPost},
		{name: "agents", method: http.MethodPut, path: "/api/agents", allow: "GET, POST"},
		{name: "websocket placeholder", method: http.MethodPost, path: "/ws", allow: http.MethodGet},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, nil)
			response := httptest.NewRecorder()
			server.Handler().ServeHTTP(response, request)

			if response.Code != http.StatusMethodNotAllowed {
				t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, response.Code)
			}
			if got := response.Header().Get("Allow"); got != test.allow {
				t.Fatalf("expected Allow header %q, got %q", test.allow, got)
			}
		})
	}
}

func newTestServer() *Server {
	return NewServer(config.Default(), slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func loginAsAdmin(t *testing.T, server *Server) string {
	t.Helper()
	response := doJSON(t, server, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "admin",
		"password": "admin",
	}, "")
	if response.Code != http.StatusOK {
		t.Fatalf("expected login status %d, got %d", http.StatusOK, response.Code)
	}
	return assertJSONField(t, response.Body.Bytes(), "token", "")
}

func doJSON(t *testing.T, server *Server, method, path string, payload any, token string) *httptest.ResponseRecorder {
	t.Helper()

	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}
		body = bytes.NewReader(encoded)
	}

	request := httptest.NewRequest(method, path, body)
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	return response
}

func assertJSONField(t *testing.T, body []byte, key string, want string) string {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	got, _ := decoded[key].(string)
	if want != "" && got != want {
		t.Fatalf("expected %s %q, got %q", key, want, got)
	}
	return got
}

func assertJSONObject(t *testing.T, body []byte, key string) map[string]any {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	object, ok := decoded[key].(map[string]any)
	if !ok {
		t.Fatalf("expected %s object, got %#v", key, decoded[key])
	}
	return object
}
