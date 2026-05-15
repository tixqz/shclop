package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

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
	mux.HandleFunc("/ws", s.handleWebSocket)
	mux.HandleFunc("/runtime/ws", s.handleRuntimeWebSocket)
	mux.HandleFunc("/", s.handleFrontend)
	return mux
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
	if _, ok := s.requireUserFromRequest(w, r); !ok {
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
	text, _ := incoming.Payload["text"].(string)
	runtime := gateway.MockRuntime{}
	for _, event := range runtime.Respond("mock-agent", incoming.SessionID, incoming.MessageID, text) {
		if err := conn.WriteJSON(event); err != nil {
			return
		}
	}
}

func (s *Server) handleRuntimeWebSocket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	expectedAgentID, expectedSecret, ok := strings.Cut(s.cfg.RuntimeToken, ":")
	if !ok || expectedAgentID == "" || expectedSecret == "" || r.Header.Get("Authorization") != "Bearer "+expectedSecret {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	var hello gateway.Envelope
	if err := conn.ReadJSON(&hello); err != nil {
		return
	}
	if hello.Type != "runtime.hello" || strings.TrimSpace(hello.AgentID) == "" || hello.AgentID != expectedAgentID {
		_ = conn.WriteJSON(gateway.Envelope{Type: "runtime.rejected", AgentID: hello.AgentID, Payload: map[string]any{"reason": "invalid runtime hello"}})
		return
	}

	_ = conn.WriteJSON(gateway.Envelope{Type: "runtime.accepted", AgentID: hello.AgentID, Seq: 1})
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
