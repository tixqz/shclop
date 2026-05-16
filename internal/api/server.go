package api

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/mipopov/shclop/internal/auth"
	"github.com/mipopov/shclop/internal/config"
	"github.com/mipopov/shclop/internal/domain"
	"github.com/mipopov/shclop/internal/gateway"
	"github.com/mipopov/shclop/internal/store"
)

var wsUpgrader = websocket.Upgrader{CheckOrigin: sameOriginOrNoOrigin}

func sameOriginOrNoOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return parsed.Host == r.Host
}

type Server struct {
	cfg     config.Config
	auth    auth.Service
	store   store.Store
	runtime *gateway.RuntimeRegistry
	tokens  map[string]string
	tokenMu sync.Mutex
	logger  *slog.Logger
	handler http.Handler
}

func NewServer(cfg config.Config, logger *slog.Logger) (*Server, error) {
	openedStore, err := store.Open(store.Config{Backend: cfg.Store, PostgresDSN: cfg.PostgresDSN})
	if err != nil {
		return nil, err
	}
	server := &Server{
		cfg:    cfg,
		auth:   auth.NewMemory(),
		store:  openedStore,
		runtime: gateway.NewRuntimeRegistry(),
		tokens: map[string]string{},
		logger: logger,
	}
	server.handler = server.routes()
	return server, nil
}

func (s *Server) ListenAndServe() error {
	server := &http.Server{
		Addr:    s.cfg.Addr,
		Handler: s.Handler(),
	}
	return server.ListenAndServe()
}

func (s *Server) Handler() http.Handler {
	return s.handler
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/api/auth/login", s.handleLogin)
	mux.HandleFunc("/api/agents", s.handleAgents)
	mux.HandleFunc("/api/agents/", s.handleAgent)
	mux.HandleFunc("/ws", s.handleWebSocket)
	mux.HandleFunc("/runtime/ws", s.handleRuntimeWebSocket)
	mux.HandleFunc("/", s.handleFrontend)
	return mux
}

func (s *Server) handleAgent(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/agents/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 2 && parts[1] == "start" {
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}
		s.handleStartAgent(w, r, parts[0])
		return
	}
	if len(parts) == 1 && parts[0] != "" {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		s.handleGetAgent(w, r, parts[0])
		return
	}
	http.NotFound(w, r)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}

	var request struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	user, token, err := s.auth.Login(request.Username, request.Password)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "shclop_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	s.writeJSON(w, http.StatusOK, map[string]any{
		"user":  user,
		"token": token,
	})
}

func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handleCreateAgent(w, r)
	case http.MethodGet:
		s.handleListAgents(w, r)
	default:
		methodNotAllowed(w, "GET, POST")
	}
}

func (s *Server) handleCreateAgent(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}

	var request struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(request.Name)
	if name == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	agent, err := s.store.CreateAgent(r.Context(), user.ID, name)
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	s.writeJSON(w, http.StatusCreated, agent)
}

func (s *Server) handleListAgents(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}

	agents, err := s.store.ListAgents(r.Context(), user.ID)
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, agents)
}

func (s *Server) handleGetAgent(w http.ResponseWriter, r *http.Request, agentID string) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	agent, ok := s.requireOwnedAgent(w, r, user.ID, agentID)
	if !ok {
		return
	}
	s.writeJSON(w, http.StatusOK, agent)
}

func (s *Server) handleStartAgent(w http.ResponseWriter, r *http.Request, agentID string) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if _, ok := s.requireOwnedAgent(w, r, user.ID, agentID); !ok {
		return
	}
	var request struct {
		Runtime string `json:"runtime"`
	}
	_ = json.NewDecoder(r.Body).Decode(&request)
	if strings.TrimSpace(request.Runtime) == "" {
		request.Runtime = "openclaw"
	}
	secret, err := randomSecret()
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	s.tokenMu.Lock()
	s.tokens[agentID] = secret
	s.tokenMu.Unlock()
	agent, err := s.store.UpdateAgentState(r.Context(), agentID, "starting")
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	s.writeJSON(w, http.StatusAccepted, map[string]any{
		"agent":         agent,
		"runtime":       request.Runtime,
		"runtime_token": secret,
		"runtime_url":   "/runtime/ws",
	})
}

func (s *Server) requireOwnedAgent(w http.ResponseWriter, r *http.Request, ownerID, agentID string) (domain.Agent, bool) {
	agent, err := s.store.GetAgent(r.Context(), agentID)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return domain.Agent{}, false
	}
	if err != nil {
		s.writeStoreError(w, err)
		return domain.Agent{}, false
	}
	if agent.OwnerID != ownerID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return domain.Agent{}, false
	}
	return agent, true
}

func (s *Server) requireUser(w http.ResponseWriter, r *http.Request) (domain.User, bool) {
	const prefix = "Bearer "
	authorization := r.Header.Get("Authorization")
	if !strings.HasPrefix(authorization, prefix) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return domain.User{}, false
	}

	token := strings.TrimSpace(strings.TrimPrefix(authorization, prefix))
	if token == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return domain.User{}, false
	}

	user, ok := s.auth.Resolve(token)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return domain.User{}, false
	}
	return user, true
}

func (s *Server) writeStoreError(w http.ResponseWriter, err error) {
	if s.logger != nil {
		s.logger.Error("store operation failed", "error", err)
	}
	http.Error(w, "internal server error", http.StatusInternalServerError)
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, value any) {
	if err := writeJSON(w, status, value); err != nil && s.logger != nil {
		s.logger.Error("write response failed", "error", err)
	}
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	user, ok := s.requireUserFromRequest(w, r)
	if !ok {
		return
	}

	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	var incoming gateway.Envelope
	if err := conn.ReadJSON(&incoming); err != nil {
		return
	}
	agent, err := s.store.GetAgent(r.Context(), incoming.AgentID)
	if errors.Is(err, store.ErrNotFound) || agent.OwnerID != user.ID {
		_ = conn.WriteJSON(gateway.Envelope{Type: "message.error", AgentID: incoming.AgentID, SessionID: incoming.SessionID, MessageID: incoming.MessageID, Payload: map[string]any{"text": "agent not found"}})
		return
	}
	if err != nil {
		return
	}
	incoming.Type = "task.run"
	events, cancel, err := s.runtime.SendTask(incoming.AgentID, incoming)
	if errors.Is(err, gateway.ErrRuntimeNotConnected) {
		_ = conn.WriteJSON(gateway.Envelope{Type: "message.error", AgentID: incoming.AgentID, SessionID: incoming.SessionID, MessageID: incoming.MessageID, Payload: map[string]any{"text": "runtime not connected"}})
		return
	}
	if err != nil {
		return
	}
	defer cancel()
	for event := range events {
		if err := conn.WriteJSON(event); err != nil {
			return
		}
		if event.Type == "message.done" || event.Type == "message.error" {
			return
		}
	}
}

func (s *Server) handleRuntimeWebSocket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	const prefix = "Bearer "
	authorization := r.Header.Get("Authorization")
	if !strings.HasPrefix(authorization, prefix) || strings.TrimSpace(strings.TrimPrefix(authorization, prefix)) == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	secret := strings.TrimSpace(strings.TrimPrefix(authorization, prefix))

	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	var hello gateway.Envelope
	if err := conn.ReadJSON(&hello); err != nil {
		return
	}
	if hello.Type != "runtime.hello" || strings.TrimSpace(hello.AgentID) == "" || !s.validRuntimeToken(hello.AgentID, secret) {
		_ = conn.WriteJSON(gateway.Envelope{Type: "runtime.rejected", AgentID: hello.AgentID, Payload: map[string]any{"reason": "invalid runtime hello"}})
		return
	}

	s.runtime.Register(hello.AgentID, conn)
	defer s.runtime.Unregister(hello.AgentID, conn)
	_, _ = s.store.UpdateAgentState(r.Context(), hello.AgentID, "running")
	_ = conn.WriteJSON(gateway.Envelope{Type: "runtime.accepted", AgentID: hello.AgentID, Seq: 1})
	for {
		var event gateway.Envelope
		if err := conn.ReadJSON(&event); err != nil {
			_, _ = s.store.UpdateAgentState(r.Context(), hello.AgentID, "idle")
			return
		}
		s.runtime.Dispatch(hello.AgentID, conn, event)
	}
}

func (s *Server) validRuntimeToken(agentID, secret string) bool {
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()
	return s.tokens[agentID] != "" && s.tokens[agentID] == secret
}

func (s *Server) requireUserFromRequest(w http.ResponseWriter, r *http.Request) (domain.User, bool) {
	token := ""
	if cookie, err := r.Cookie("shclop_session"); err == nil {
		token = strings.TrimSpace(cookie.Value)
	}
	if token == "" {
		const prefix = "Bearer "
		if authorization := r.Header.Get("Authorization"); strings.HasPrefix(authorization, prefix) {
			token = strings.TrimSpace(strings.TrimPrefix(authorization, prefix))
		}
	}
	if token == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return domain.User{}, false
	}
	user, ok := s.auth.Resolve(token)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return domain.User{}, false
	}
	return user, true
}

func (s *Server) handleFrontend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		methodNotAllowed(w, "GET, HEAD")
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}
	if s.cfg.StaticDir == "" {
		http.NotFound(w, r)
		return
	}

	requested := filepath.Join(s.cfg.StaticDir, filepath.Clean(r.URL.Path))
	if info, err := os.Stat(requested); err == nil && !info.IsDir() {
		http.ServeFile(w, r, requested)
		return
	}

	index := filepath.Join(s.cfg.StaticDir, "index.html")
	if _, err := os.Stat(index); err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, index)
}

func writeJSON(w http.ResponseWriter, status int, value any) error {
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(value); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, err := w.Write(body.Bytes())
	return err
}

func methodNotAllowed(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func randomSecret() (string, error) {
	var b [24]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
