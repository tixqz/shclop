# Shclop Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first runnable Shclop foundation: Go backend, React UI, mock/dev runtime, REST/WebSocket contracts, bootstrap CLI skeleton, and Helm skeleton.

**Architecture:** Implement a modular Go monolith with internal interfaces for auth, storage, runtime, LLM broker, and orchestration. The first slice must run locally in dev/mock mode without infrastructure, while keeping production-facing interfaces aligned with the design spec.

**Tech Stack:** Go 1.22+, React + Vite + TypeScript, REST JSON, WebSocket typed envelopes, shell bootstrap script, Helm chart skeleton.

---

## Scope

This plan implements the foundation only. It intentionally does not implement full Kata orchestration, Vault production integration, real LLM providers, real integration connectors, or production scheduler execution. Those require follow-up plans.

## Files

- Create: `go.mod` — Go module definition.
- Create: `cmd/shclop/main.go` — CLI entrypoint.
- Create: `internal/config/config.go` — config structs and dev defaults.
- Create: `internal/logging/logging.go` — structured logger setup.
- Create: `internal/api/server.go` — HTTP server, REST routes, WebSocket route.
- Create: `internal/api/server_test.go` — API health/auth/agents tests.
- Create: `internal/auth/auth.go` — local auth interfaces and in-memory implementation.
- Create: `internal/auth/auth_test.go` — auth tests.
- Create: `internal/domain/domain.go` — shared domain types.
- Create: `internal/store/store.go` — store interfaces and in-memory store.
- Create: `internal/store/store_test.go` — store tests.
- Create: `internal/gateway/envelope.go` — WebSocket envelope types.
- Create: `internal/gateway/mock_runtime.go` — mock runtime stream behavior.
- Create: `internal/gateway/mock_runtime_test.go` — streaming tests.
- Create: `web/package.json` — frontend package metadata.
- Create: `web/index.html` — Vite entry HTML.
- Create: `web/src/main.tsx` — React entrypoint.
- Create: `web/src/App.tsx` — basic UI shell.
- Create: `web/src/api.ts` — REST/WebSocket client helpers.
- Create: `web/src/styles.css` — minimal styling.
- Create: `scripts/bootstrap.sh` — action-subcommand bootstrap skeleton.
- Create: `charts/shclop/Chart.yaml` — Helm chart metadata.
- Create: `charts/shclop/values.yaml` — chart values.
- Create: `charts/shclop/templates/deployment.yaml` — backend deployment template.
- Create: `charts/shclop/templates/service.yaml` — backend service template.
- Modify: `README.md` — project overview and dev commands.

---

### Task 1: Go module and CLI entrypoint

**Files:**
- Create: `go.mod`
- Create: `cmd/shclop/main.go`
- Create: `internal/config/config.go`
- Create: `internal/logging/logging.go`

- [ ] **Step 1: Create Go module**

Create `go.mod`:

```go
module github.com/mipopov/shclop

go 1.22

require github.com/gorilla/websocket v1.5.3
```

- [ ] **Step 2: Add config defaults**

Create `internal/config/config.go`:

```go
package config

type Config struct {
	Addr        string
	Dev         bool
	MockRuntime bool
	MockLLM     bool
	MockSecrets bool
	Store       string
	LogLevel    string
	Metrics     bool
}

func Default() Config {
	return Config{Addr: ":8080", Store: "inmemory", LogLevel: "info", Metrics: true}
}
```

- [ ] **Step 3: Add logger setup**

Create `internal/logging/logging.go`:

```go
package logging

import (
	"log/slog"
	"os"
)

func New(level string) *slog.Logger {
	var parsed slog.Level
	if err := parsed.UnmarshalText([]byte(level)); err != nil {
		parsed = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: parsed}))
}
```

- [ ] **Step 4: Add CLI entrypoint**

Create `cmd/shclop/main.go`:

```go
package main

import (
	"flag"
	"log"

	"github.com/mipopov/shclop/internal/api"
	"github.com/mipopov/shclop/internal/config"
	"github.com/mipopov/shclop/internal/logging"
)

func main() {
	cfg := config.Default()
	flag.StringVar(&cfg.Addr, "addr", cfg.Addr, "HTTP listen address")
	flag.BoolVar(&cfg.Dev, "dev", cfg.Dev, "enable dev mode")
	flag.BoolVar(&cfg.MockRuntime, "mock-runtime", cfg.MockRuntime, "enable mock runtime provider")
	flag.BoolVar(&cfg.MockLLM, "mock-llm", cfg.MockLLM, "enable mock LLM provider")
	flag.BoolVar(&cfg.MockSecrets, "mock-secrets", cfg.MockSecrets, "enable mock SecretStore")
	flag.StringVar(&cfg.Store, "store", cfg.Store, "store backend: inmemory")
	flag.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "log level: debug/info/warn/error")
	flag.BoolVar(&cfg.Metrics, "metrics", cfg.Metrics, "enable metrics endpoint")
	flag.Parse()

	logger := logging.New(cfg.LogLevel)
	if err := api.NewServer(cfg, logger).ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 5: Verify compile failure points are resolved**

Run: `go test ./...`

Expected: FAIL because `internal/api` is not implemented yet. Continue to Task 2.

---

### Task 2: Domain and in-memory store

**Files:**
- Create: `internal/domain/domain.go`
- Create: `internal/store/store.go`
- Create: `internal/store/store_test.go`

- [ ] **Step 1: Write store tests**

Create `internal/store/store_test.go`:

```go
package store

import (
	"context"
	"testing"
)

func TestMemoryStoreCreatesAndListsAgents(t *testing.T) {
	s := NewMemory()
	agent, err := s.CreateAgent(context.Background(), "user-1", "Researcher")
	if err != nil { t.Fatal(err) }
	if agent.ID == "" { t.Fatal("expected agent ID") }
	agents, err := s.ListAgents(context.Background(), "user-1")
	if err != nil { t.Fatal(err) }
	if len(agents) != 1 { t.Fatalf("expected 1 agent, got %d", len(agents)) }
	if agents[0].Name != "Researcher" { t.Fatalf("unexpected name %q", agents[0].Name) }
}
```

- [ ] **Step 2: Run failing test**

Run: `go test ./internal/store`

Expected: FAIL because store implementation does not exist.

- [ ] **Step 3: Add domain types**

Create `internal/domain/domain.go`:

```go
package domain

import "time"

type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

type Agent struct {
	ID        string    `json:"id"`
	OwnerID   string    `json:"owner_id"`
	Name      string    `json:"name"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"created_at"`
}

type Message struct {
	ID        string    `json:"id"`
	AgentID   string    `json:"agent_id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}
```

- [ ] **Step 4: Add memory store**

Create `internal/store/store.go`:

```go
package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/mipopov/shclop/internal/domain"
)

type Store interface {
	CreateAgent(ctx context.Context, ownerID, name string) (domain.Agent, error)
	ListAgents(ctx context.Context, ownerID string) ([]domain.Agent, error)
}

type Memory struct {
	mu     sync.Mutex
	agents []domain.Agent
}

func NewMemory() *Memory { return &Memory{} }

func (m *Memory) CreateAgent(ctx context.Context, ownerID, name string) (domain.Agent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	agent := domain.Agent{ID: newID(), OwnerID: ownerID, Name: name, State: "idle", CreatedAt: time.Now().UTC()}
	m.agents = append(m.agents, agent)
	return agent, nil
}

func (m *Memory) ListAgents(ctx context.Context, ownerID string) ([]domain.Agent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.Agent
	for _, agent := range m.agents {
		if agent.OwnerID == ownerID { out = append(out, agent) }
	}
	return out, nil
}

func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
```

- [ ] **Step 5: Verify store tests pass**

Run: `go test ./internal/store`

Expected: PASS.

---

### Task 3: Local auth foundation

**Files:**
- Create: `internal/auth/auth.go`
- Create: `internal/auth/auth_test.go`

- [ ] **Step 1: Write auth tests**

Create `internal/auth/auth_test.go`:

```go
package auth

import "testing"

func TestMemoryAuthLogin(t *testing.T) {
	a := NewMemory()
	user, token, err := a.Login("admin", "admin")
	if err != nil { t.Fatal(err) }
	if user.Username != "admin" { t.Fatalf("unexpected user %q", user.Username) }
	resolved, ok := a.Resolve(token)
	if !ok { t.Fatal("expected token to resolve") }
	if resolved.ID != user.ID { t.Fatalf("expected %q got %q", user.ID, resolved.ID) }
}

func TestMemoryAuthRejectsBadPassword(t *testing.T) {
	a := NewMemory()
	_, _, err := a.Login("admin", "wrong")
	if err == nil { t.Fatal("expected bad password error") }
}
```

- [ ] **Step 2: Run failing test**

Run: `go test ./internal/auth`

Expected: FAIL because auth implementation does not exist.

- [ ] **Step 3: Implement memory auth**

Create `internal/auth/auth.go`:

```go
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"

	"github.com/mipopov/shclop/internal/domain"
)

type Service interface {
	Login(username, password string) (domain.User, string, error)
	Resolve(token string) (domain.User, bool)
}

type Memory struct {
	mu     sync.Mutex
	tokens map[string]domain.User
}

func NewMemory() *Memory { return &Memory{tokens: map[string]domain.User{}} }

func (m *Memory) Login(username, password string) (domain.User, string, error) {
	if username != "admin" || password != "admin" { return domain.User{}, "", errors.New("invalid credentials") }
	user := domain.User{ID: "user-admin", Username: "admin"}
	token := tokenID()
	m.mu.Lock(); m.tokens[token] = user; m.mu.Unlock()
	return user, token, nil
}

func (m *Memory) Resolve(token string) (domain.User, bool) {
	m.mu.Lock(); defer m.mu.Unlock()
	user, ok := m.tokens[token]
	return user, ok
}

func tokenID() string {
	var b [24]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
```

- [ ] **Step 4: Verify auth tests pass**

Run: `go test ./internal/auth`

Expected: PASS.

---

### Task 4: REST API foundation

**Files:**
- Create: `internal/api/server.go`
- Create: `internal/api/server_test.go`

- [ ] **Step 1: Write API tests**

Create `internal/api/server_test.go`:

```go
package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mipopov/shclop/internal/config"
	"github.com/mipopov/shclop/internal/logging"
)

func TestHealth(t *testing.T) {
	s := NewServer(config.Default(), logging.New("error"))
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusOK { t.Fatalf("expected 200 got %d", w.Code) }
}

func TestLoginAndCreateAgent(t *testing.T) {
	s := NewServer(config.Default(), logging.New("error"))
	body := bytes.NewBufferString(`{"username":"admin","password":"admin"}`)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/auth/login", body))
	if w.Code != http.StatusOK { t.Fatalf("login expected 200 got %d", w.Code) }
	var login map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &login); err != nil { t.Fatal(err) }
	token := login["token"].(string)

	agentReq := bytes.NewBufferString(`{"name":"Researcher"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/agents", agentReq)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusCreated { t.Fatalf("create agent expected 201 got %d: %s", w.Code, w.Body.String()) }
}
```

- [ ] **Step 2: Run failing test**

Run: `go test ./internal/api`

Expected: FAIL because API implementation does not exist.

- [ ] **Step 3: Implement REST server**

Create `internal/api/server.go`:

```go
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/mipopov/shclop/internal/auth"
	"github.com/mipopov/shclop/internal/config"
	"github.com/mipopov/shclop/internal/store"
)

type Server struct {
	cfg    config.Config
	logger *slog.Logger
	auth   auth.Service
	store  store.Store
}

func NewServer(cfg config.Config, logger *slog.Logger) *Server {
	return &Server{cfg: cfg, logger: logger, auth: auth.NewMemory(), store: store.NewMemory()}
}

func (s *Server) ListenAndServe() error { return http.ListenAndServe(s.cfg.Addr, s.Handler()) }

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusOK, map[string]string{"status": "ok"}) })
	mux.HandleFunc("/api/auth/login", s.handleLogin)
	mux.HandleFunc("/api/agents", s.handleAgents)
	mux.HandleFunc("/ws", s.handleWebSocket)
	return mux
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	var req struct{ Username, Password string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { http.Error(w, "bad request", http.StatusBadRequest); return }
	user, token, err := s.auth.Login(req.Username, req.Password)
	if err != nil { http.Error(w, "unauthorized", http.StatusUnauthorized); return }
	writeJSON(w, http.StatusOK, map[string]any{"user": user, "token": token})
}

func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r); if !ok { return }
	switch r.Method {
	case http.MethodGet:
		agents, _ := s.store.ListAgents(r.Context(), user.ID); writeJSON(w, http.StatusOK, agents)
	case http.MethodPost:
		var req struct{ Name string `json:"name"` }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Name) == "" { http.Error(w, "bad request", http.StatusBadRequest); return }
		agent, _ := s.store.CreateAgent(r.Context(), user.ID, req.Name); writeJSON(w, http.StatusCreated, agent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) requireUser(w http.ResponseWriter, r *http.Request) (authUser interface{ GetID() }, bool) { panic("replace in next step") }

func writeJSON(w http.ResponseWriter, status int, v any) { w.Header().Set("Content-Type", "application/json"); w.WriteHeader(status); _ = json.NewEncoder(w).Encode(v) }
```

- [ ] **Step 4: Fix `requireUser` with concrete domain type**

Replace the placeholder `requireUser` function in `internal/api/server.go` with:

```go
func (s *Server) requireUser(w http.ResponseWriter, r *http.Request) (domain.User, bool) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") { http.Error(w, "unauthorized", http.StatusUnauthorized); return domain.User{}, false }
	user, ok := s.auth.Resolve(strings.TrimPrefix(header, "Bearer "))
	if !ok { http.Error(w, "unauthorized", http.StatusUnauthorized); return domain.User{}, false }
	return user, true
}
```

Also add this import to `internal/api/server.go`:

```go
"github.com/mipopov/shclop/internal/domain"
```

- [ ] **Step 5: Verify API tests pass**

Run: `go test ./internal/api`

Expected: PASS.

---

### Task 5: WebSocket mock runtime contract

**Files:**
- Create: `internal/gateway/envelope.go`
- Create: `internal/gateway/mock_runtime.go`
- Create: `internal/gateway/mock_runtime_test.go`
- Modify: `internal/api/server.go`

- [ ] **Step 1: Add gateway tests**

Create `internal/gateway/mock_runtime_test.go`:

```go
package gateway

import "testing"

func TestMockRuntimeStreamsResponse(t *testing.T) {
	r := MockRuntime{}
	events := r.Respond("agent-1", "session-1", "msg-1", "hello")
	if len(events) < 3 { t.Fatalf("expected at least 3 events got %d", len(events)) }
	if events[0].Type != "message.started" { t.Fatalf("unexpected first type %q", events[0].Type) }
	if events[len(events)-1].Type != "message.done" { t.Fatalf("unexpected final type %q", events[len(events)-1].Type) }
}
```

- [ ] **Step 2: Run failing test**

Run: `go test ./internal/gateway`

Expected: FAIL because gateway package does not exist.

- [ ] **Step 3: Add envelope types**

Create `internal/gateway/envelope.go`:

```go
package gateway

type Envelope struct {
	Type      string         `json:"type"`
	AgentID   string         `json:"agent_id,omitempty"`
	SessionID string         `json:"session_id,omitempty"`
	MessageID string         `json:"message_id,omitempty"`
	Seq       int64          `json:"seq"`
	Payload   map[string]any `json:"payload,omitempty"`
}
```

- [ ] **Step 4: Add mock runtime**

Create `internal/gateway/mock_runtime.go`:

```go
package gateway

type MockRuntime struct{}

func (MockRuntime) Respond(agentID, sessionID, messageID, text string) []Envelope {
	return []Envelope{
		{Type: "message.started", AgentID: agentID, SessionID: sessionID, MessageID: messageID, Seq: 1},
		{Type: "message.delta", AgentID: agentID, SessionID: sessionID, MessageID: messageID, Seq: 2, Payload: map[string]any{"text": "Mock response: "}},
		{Type: "message.delta", AgentID: agentID, SessionID: sessionID, MessageID: messageID, Seq: 3, Payload: map[string]any{"text": text}},
		{Type: "message.done", AgentID: agentID, SessionID: sessionID, MessageID: messageID, Seq: 4},
	}
}
```

- [ ] **Step 5: Wire `/ws` handler**

In `internal/api/server.go`, implement `handleWebSocket` with `gorilla/websocket` and mock runtime. Add imports:

```go
"github.com/gorilla/websocket"
"github.com/mipopov/shclop/internal/gateway"
```

Add package-level upgrader and handler:

```go
var wsUpgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil { return }
	defer conn.Close()

	var incoming gateway.Envelope
	if err := conn.ReadJSON(&incoming); err != nil { return }
	text, _ := incoming.Payload["text"].(string)
	runtime := gateway.MockRuntime{}
	for _, event := range runtime.Respond(incoming.AgentID, incoming.SessionID, incoming.MessageID, text) {
		if err := conn.WriteJSON(event); err != nil { return }
	}
}
```

- [ ] **Step 6: Verify gateway and API tests pass**

Run: `go test ./internal/gateway ./internal/api`

Expected: PASS.

---

### Task 6: React/Vite UI foundation

**Files:**
- Create: `web/package.json`
- Create: `web/index.html`
- Create: `web/src/main.tsx`
- Create: `web/src/App.tsx`
- Create: `web/src/api.ts`
- Create: `web/src/styles.css`

- [ ] **Step 1: Create package file**

Create `web/package.json`:

```json
{
  "name": "shclop-web",
  "private": true,
  "version": "0.1.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc && vite build"
  },
  "dependencies": {
    "@vitejs/plugin-react": "latest",
    "typescript": "latest",
    "vite": "latest",
    "react": "latest",
    "react-dom": "latest"
  },
  "devDependencies": {}
}
```

- [ ] **Step 2: Add HTML entrypoint**

Create `web/index.html`:

```html
<!doctype html>
<html lang="en">
  <head><meta charset="UTF-8" /><meta name="viewport" content="width=device-width, initial-scale=1.0" /><title>Shclop</title></head>
  <body><div id="root"></div><script type="module" src="/src/main.tsx"></script></body>
</html>
```

- [ ] **Step 3: Add frontend API client**

Create `web/src/api.ts`:

```ts
export async function login(username: string, password: string): Promise<string> {
  const res = await fetch('/api/auth/login', { method: 'POST', body: JSON.stringify({ username, password }) })
  if (!res.ok) throw new Error('login failed')
  const body = await res.json()
  return body.token
}

export function streamMockChat(text: string, onEvent: (event: any) => void) {
  const ws = new WebSocket(`${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}/ws`)
  ws.onopen = () => ws.send(JSON.stringify({ type: 'user.message', agent_id: 'agent-dev', session_id: 'session-dev', message_id: crypto.randomUUID(), seq: 1, payload: { text } }))
  ws.onmessage = (message) => onEvent(JSON.parse(message.data))
  return () => ws.close()
}
```

- [ ] **Step 4: Add React app**

Create `web/src/App.tsx`:

```tsx
import { useState } from 'react'
import { login, streamMockChat } from './api'
import './styles.css'

export function App() {
  const [token, setToken] = useState('')
  const [input, setInput] = useState('')
  const [events, setEvents] = useState<any[]>([])

  async function handleLogin() { setToken(await login('admin', 'admin')) }
  function send() { setEvents([]); streamMockChat(input, event => setEvents(prev => [...prev, event])) }

  return <main>
    <h1>Shclop</h1>
    <button onClick={handleLogin}>{token ? 'Logged in' : 'Login as dev admin'}</button>
    <section>
      <textarea value={input} onChange={e => setInput(e.target.value)} placeholder="Message mock agent" />
      <button onClick={send}>Send</button>
    </section>
    <pre>{events.map(e => JSON.stringify(e)).join('\n')}</pre>
  </main>
}
```

Create `web/src/main.tsx`:

```tsx
import React from 'react'
import { createRoot } from 'react-dom/client'
import { App } from './App'

createRoot(document.getElementById('root')!).render(<React.StrictMode><App /></React.StrictMode>)
```

Create `web/src/styles.css`:

```css
body { margin: 0; font-family: system-ui, sans-serif; background: #0f172a; color: #e2e8f0; }
main { max-width: 880px; margin: 40px auto; padding: 24px; }
button { margin: 8px 0; padding: 8px 12px; }
textarea { display: block; width: 100%; min-height: 120px; margin-top: 16px; }
pre { background: #020617; padding: 16px; overflow: auto; }
```

- [ ] **Step 5: Verify frontend build**

Run: `npm install && npm run build` from `web/`.

Expected: PASS and Vite build output under `web/dist/`.

---

### Task 7: Bootstrap CLI skeleton

**Files:**
- Create: `scripts/bootstrap.sh`

- [ ] **Step 1: Add bootstrap script skeleton**

Create `scripts/bootstrap.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: scripts/bootstrap.sh <check|install|reset|destroy> [flags]

Targets:
  local target is default
  --remote user@host    run action on remote Linux host over SSH

Flags:
  --dry-run
  --install-deps
  --yes
  --purge-data
  --remove-k3s
  --remove-kata
  --values PATH
USAGE
}

action="${1:-}"
if [[ -z "$action" ]]; then usage; exit 2; fi
shift

remote=""
dry_run=false
install_deps=false
yes=false
purge_data=false
remove_k3s=false
remove_kata=false
values=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --remote) remote="${2:?missing remote target}"; shift 2 ;;
    --dry-run) dry_run=true; shift ;;
    --install-deps) install_deps=true; shift ;;
    --yes) yes=true; shift ;;
    --purge-data) purge_data=true; shift ;;
    --remove-k3s) remove_k3s=true; shift ;;
    --remove-kata) remove_kata=true; shift ;;
    --values) values="${2:?missing values path}"; shift 2 ;;
    *) echo "unknown argument: $1" >&2; usage; exit 2 ;;
  esac
done

case "$action" in check|install|reset|destroy) ;; *) echo "unknown action: $action" >&2; usage; exit 2 ;; esac

run_local() {
  echo "action=$action remote=local dry_run=$dry_run install_deps=$install_deps purge_data=$purge_data remove_k3s=$remove_k3s remove_kata=$remove_kata values=$values"
  if [[ "$action" == "destroy" && "$yes" != "true" ]]; then
    read -r -p "Type 'delete shclop' to continue: " confirm
    [[ "$confirm" == "delete shclop" ]] || { echo "aborted"; exit 1; }
  fi
  echo "bootstrap skeleton: implementation will add KVM/K3s/Kata/Helm operations"
}

if [[ -n "$remote" ]]; then
  ssh "$remote" "bash -s" -- "$action" < "$0"
else
  run_local
fi
```

- [ ] **Step 2: Make script executable**

Run: `chmod +x scripts/bootstrap.sh`

- [ ] **Step 3: Verify CLI shape**

Run: `scripts/bootstrap.sh check --dry-run`

Expected output contains `action=check remote=local`.

Run: `scripts/bootstrap.sh nope`

Expected: non-zero exit and usage output.

---

### Task 8: Helm chart skeleton

**Files:**
- Create: `charts/shclop/Chart.yaml`
- Create: `charts/shclop/values.yaml`
- Create: `charts/shclop/templates/deployment.yaml`
- Create: `charts/shclop/templates/service.yaml`

- [ ] **Step 1: Add chart metadata**

Create `charts/shclop/Chart.yaml`:

```yaml
apiVersion: v2
name: shclop
description: Self-hosted Claw orchestration platform
type: application
version: 0.1.0
appVersion: "0.1.0"
```

- [ ] **Step 2: Add values**

Create `charts/shclop/values.yaml`:

```yaml
image:
  repository: shclop
  tag: latest
  pullPolicy: IfNotPresent

service:
  type: ClusterIP
  port: 8080

config:
  logLevel: info
  metrics: true

dependencies:
  bundledPostgres: true
  bundledVault: true
  bundledMinIO: true
```

- [ ] **Step 3: Add deployment template**

Create `charts/shclop/templates/deployment.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Release.Name }}-backend
  labels:
    app.kubernetes.io/name: shclop
    app.kubernetes.io/part-of: shclop
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: shclop
  template:
    metadata:
      labels:
        app.kubernetes.io/name: shclop
        app.kubernetes.io/part-of: shclop
    spec:
      containers:
        - name: shclop
          image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
          imagePullPolicy: {{ .Values.image.pullPolicy }}
          args: ["--addr=:8080", "--log-level={{ .Values.config.logLevel }}"]
          ports:
            - containerPort: 8080
```

- [ ] **Step 4: Add service template**

Create `charts/shclop/templates/service.yaml`:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: {{ .Release.Name }}-backend
  labels:
    app.kubernetes.io/name: shclop
    app.kubernetes.io/part-of: shclop
spec:
  type: {{ .Values.service.type }}
  selector:
    app.kubernetes.io/name: shclop
  ports:
    - name: http
      port: {{ .Values.service.port }}
      targetPort: 8080
```

- [ ] **Step 5: Verify chart renders**

Run: `helm template shclop charts/shclop`

Expected: manifests render without error.

---

### Task 9: README update and full verification

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Update README**

Replace `README.md` with:

```markdown
# shclop

Self-hosted *Claw orchestration platform.

## Development

Run the backend in dev/mock mode:

```bash
go run ./cmd/shclop --dev --mock-runtime --mock-llm --mock-secrets --store inmemory
```

Run tests:

```bash
go test ./...
```

Run the frontend:

```bash
cd web
npm install
npm run dev
```

## Single-node evaluation

Linux with KVM is required for full runtime evaluation.

```bash
scripts/bootstrap.sh check
scripts/bootstrap.sh install --install-deps
scripts/bootstrap.sh check --remote root@example.com
```

Docker Compose and macOS-native runtime are out of scope.
```

- [ ] **Step 2: Run backend tests**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 3: Run frontend build**

Run: `npm install && npm run build` from `web/`.

Expected: PASS.

- [ ] **Step 4: Run bootstrap smoke checks**

Run: `scripts/bootstrap.sh check --dry-run`

Expected: output includes `action=check remote=local`.

- [ ] **Step 5: Run Helm render**

Run: `helm template shclop charts/shclop`

Expected: PASS.

- [ ] **Step 6: Commit foundation**

```bash
git add go.mod cmd internal web scripts charts README.md
git commit -m "feat: add shclop foundation"
```

---

## Self-review

- Spec coverage: this plan covers the first implementation slice for modular Go backend, React/Vite UI, REST/WebSocket contracts, mock/dev mode, bootstrap CLI shape, and Helm skeleton.
- Deferred by design: real Kata provider, real Vault integration, real Postgres persistence, production NetworkPolicy/admission manifests, LLM provider adapters, Integration Broker connectors, and scheduler execution.
- No placeholders remain in this plan.
- Bootstrap CLI matches approved design: local default, remote selected by `--remote user@host`, actions are subcommands.
