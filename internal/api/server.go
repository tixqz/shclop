package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/mipopov/shclop/internal/auth"
	"github.com/mipopov/shclop/internal/config"
	"github.com/mipopov/shclop/internal/domain"
	"github.com/mipopov/shclop/internal/gateway"
	"github.com/mipopov/shclop/internal/store"
)

var wsUpgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

type Server struct {
	cfg     config.Config
	auth    auth.Service
	store   store.Store
	logger  *slog.Logger
	handler http.Handler
}

func NewServer(cfg config.Config, logger *slog.Logger) *Server {
	server := &Server{
		cfg:    cfg,
		auth:   auth.NewMemory(),
		store:  store.NewMemory(),
		logger: logger,
	}
	server.handler = server.routes()
	return server
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
	for _, event := range runtime.Respond(incoming.AgentID, incoming.SessionID, incoming.MessageID, text) {
		if err := conn.WriteJSON(event); err != nil {
			return
		}
	}
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
