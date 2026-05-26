package api

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mipopov/shclop/internal/auth"
	"github.com/mipopov/shclop/internal/config"
	"github.com/mipopov/shclop/internal/domain"
	"github.com/mipopov/shclop/internal/gateway"
	"github.com/mipopov/shclop/internal/sandbox"
	"github.com/mipopov/shclop/internal/store"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/crypto/bcrypt"
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
	cfg         config.Config
	auth        *auth.Service
	store       store.Store
	runtime     *gateway.RuntimeRegistry
	sandbox     sandbox.RuntimeProvider
	tokens      map[string]string
	tokenMu     sync.Mutex
	activityMu  sync.Mutex
	activity    []activityEntry
	logger      *slog.Logger
	handler     http.Handler
	metrics     *MetricsCollectors
	bootstrapMu sync.Once
}

type MetricsCollectors struct {
	httpRequestsTotal    *prometheus.CounterVec
	httpRequestDuration  *prometheus.HistogramVec
	activeConnections    prometheus.Gauge
	agentStarts          prometheus.Counter
	agentStops           prometheus.Counter
	agentFailures        prometheus.Counter
	runtimePodFailures   prometheus.Counter
	chatEvents           prometheus.Counter
	taskEvents           prometheus.Counter
	modelAllowlistErrors prometheus.Counter
	gatewayErrors        prometheus.Counter
	registry             *prometheus.Registry
}

func newMetricsCollectors() *MetricsCollectors {
	reg := prometheus.NewRegistry()
	m := &MetricsCollectors{
		httpRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "shclop_http_requests_total",
			Help: "Total HTTP requests",
		}, []string{"method", "path", "status"}),
		httpRequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "shclop_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "path"}),
		activeConnections: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "shclop_active_connections",
			Help: "Number of active runtime/ws/chat connections",
		}),
		agentStarts: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "shclop_agent_starts_total",
			Help: "Total agent starts",
		}),
		agentStops: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "shclop_agent_stops_total",
			Help: "Total agent stops",
		}),
		agentFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "shclop_agent_failures_total",
			Help: "Total agent failures",
		}),
		runtimePodFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "shclop_runtime_pod_failures_total",
			Help: "Total runtime pod creation failures",
		}),
		chatEvents: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "shclop_chat_events_total",
			Help: "Total chat events",
		}),
		taskEvents: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "shclop_task_events_total",
			Help: "Total task events",
		}),
		modelAllowlistErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "shclop_model_allowlist_errors_total",
			Help: "Total model allowlist validation failures",
		}),
		gatewayErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "shclop_llm_gateway_errors_total",
			Help: "Total LLM gateway validation failures",
		}),
		registry: reg,
	}
	reg.MustRegister(
		m.httpRequestsTotal,
		m.httpRequestDuration,
		m.activeConnections,
		m.agentStarts,
		m.agentStops,
		m.agentFailures,
		m.runtimePodFailures,
		m.chatEvents,
		m.taskEvents,
		m.modelAllowlistErrors,
		m.gatewayErrors,
	)
	return m
}

type activityEntry struct {
	Time    time.Time      `json:"time"`
	Type    string         `json:"type"`
	ActorID string         `json:"actor_id,omitempty"`
	AgentID string         `json:"agent_id,omitempty"`
	Message string         `json:"message,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

func NewServer(cfg config.Config, logger *slog.Logger) (*Server, error) {
	openedStore, err := store.Open(store.Config{Backend: cfg.Store, PostgresDSN: cfg.PostgresDSN})
	if err != nil {
		return nil, err
	}

	var hasher auth.PasswordHasher
	var userStore auth.UserStore
	switch s := openedStore.(type) {
	case *store.Memory:
		hasher = s
		userStore = s
	case *store.Postgres:
		hasher = s
		userStore = s
	default:
		hasher = openedStore.(auth.PasswordHasher)
		userStore = openedStore.(auth.UserStore)
	}

	authService := auth.NewService(userStore, hasher)
	sandboxProvider, err := sandboxProviderFromConfig(cfg)
	if err != nil {
		return nil, err
	}

	metrics := newMetricsCollectors()

	server := &Server{
		cfg:     cfg,
		auth:    authService,
		store:   openedStore,
		runtime: gateway.NewRuntimeRegistry(),
		sandbox: sandboxProvider,
		tokens:  map[string]string{},
		logger:  logger,
		metrics: metrics,
	}
	server.handler = server.withMetrics(server.routes())
	return server, nil
}

// requireBootstrapPassword returns an error if the admin password env var is
// not set and the config is a production-grade deployment (non-inmemory store,
// non-mock sandbox provider, not in dev mode). Default password fallback is
// only acceptable in dev/inmemory/mock contexts.
func (s *Server) requireBootstrapPassword() error {
	if s.cfg.Store == "inmemory" || s.cfg.SandboxProvider == "mock" || s.cfg.Dev {
		return nil
	}
	if os.Getenv("SHCLOP_BOOTSTRAP_ADMIN_PASSWORD") == "" {
		return errors.New("SHCLOP_BOOTSTRAP_ADMIN_PASSWORD must be set for production configuration (non-inmemory store, non-mock sandbox provider)")
	}
	return nil
}

func (s *Server) bootstrapAdmin() {
	s.bootstrapMu.Do(func() {
		ctx := requestContext()
		username := s.cfg.BootstrapAdminUsername
		if username == "" {
			username = "admin"
		}
		password := os.Getenv("SHCLOP_BOOTSTRAP_ADMIN_PASSWORD")
		if password == "" {
			if s.logger != nil {
				s.logger.Warn("SHCLOP_BOOTSTRAP_ADMIN_PASSWORD not set, using default password")
			}
			password = "admin"
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			if s.logger != nil {
				s.logger.Error("failed to hash admin password", "error", err)
			}
			return
		}
		if err := s.store.BootstrapAdmin(ctx, username, string(hash)); err != nil {
			if s.logger != nil {
				s.logger.Error("failed to bootstrap admin", "error", err)
			}
			return
		}
		if s.cfg.LLMGatewayBaseURL != "" || s.cfg.LLMGatewaySecretName != "" || s.cfg.LLMGatewaySecretKey != "" {
			if _, err := s.store.UpsertLLMGatewaySettings(ctx, s.cfg.LLMGatewayBaseURL != "", s.cfg.LLMGatewayBaseURL, s.cfg.LLMGatewaySecretName, s.cfg.LLMGatewaySecretKey); err != nil && s.logger != nil {
				s.logger.Error("failed to seed llm gateway settings", "error", err)
			}
		}
		// Ensure the hash is stored in memory store too
		if mem, ok := s.store.(*store.Memory); ok {
			mem.SetPasswordHash(ctx, username, string(hash))
		} else if pg, ok := s.store.(*store.Postgres); ok {
			_ = pg.SetPasswordHash(ctx, username, string(hash))
		}
		if s.logger != nil {
			s.logger.Info("admin user bootstrapped", "username", username)
		}
	})
}

func requestContext() requestCtx {
	return requestCtx{}
}

type requestCtx struct{}

func (requestCtx) Deadline() (time.Time, bool) { return time.Time{}, false }
func (requestCtx) Done() <-chan struct{}       { return nil }
func (requestCtx) Err() error                  { return nil }
func (requestCtx) Value(key any) any           { return nil }

func sandboxProviderFromConfig(cfg config.Config) (sandbox.RuntimeProvider, error) {
	switch cfg.SandboxProvider {
	case "", "mock":
		return sandbox.MockRuntimeProvider{}, nil
	case "docker-demo":
		return sandbox.DockerDemoProvider{GatewayURL: cfg.DockerGatewayURL, ImagePrefix: cfg.RuntimeImagePrefix}, nil
	case "kubernetes":
		var podReadyTimeout time.Duration
		if cfg.PodReadyTimeout != "" {
			var err error
			podReadyTimeout, err = time.ParseDuration(cfg.PodReadyTimeout)
			if err != nil {
				return nil, fmt.Errorf("invalid pod_ready_timeout %q: %w", cfg.PodReadyTimeout, err)
			}
		}
		return sandbox.NewKubernetesRuntimeProvider(sandbox.KubernetesRuntimeProviderConfig{
			Namespace:          cfg.KubernetesNamespace,
			GatewayURL:         cfg.KubernetesGatewayURL,
			RuntimeClassName:   cfg.AgentRuntimeClassName,
			Images:             cfg.RuntimeImages,
			WorkspaceSize:      cfg.WorkspaceSize,
			StorageClassName:   cfg.WorkspaceStorageClass,
			WorkspacePolicy:    cfg.WorkspaceRetention,
			SecretStore:        cfg.SecretStore,
			NetworkPolicySpec:  sandbox.NetworkPolicySpecFromConfig(cfg.NetworkPolicyEnabled, cfg.NetworkPolicyMode, cfg.NetworkPolicyCIDRs),
			PodReadyTimeout:    podReadyTimeout,
		})
	default:
		return nil, errors.New("unsupported sandbox provider: " + cfg.SandboxProvider)
	}
}

func (s *Server) ListenAndServe() error {
	if err := s.requireBootstrapPassword(); err != nil {
		return err
	}
	s.bootstrapAdmin()
	if s.logger != nil {
		s.logger.Info("starting shclop server", "addr", s.cfg.Addr, "store", s.cfg.Store, "sandbox_provider", s.cfg.SandboxProvider, "static_dir", s.cfg.StaticDir)
	}
	server := &http.Server{
		Addr:    s.cfg.Addr,
		Handler: s.Handler(),
	}
	return server.ListenAndServe()
}

func (s *Server) Handler() http.Handler {
	return s.handler
}

func (s *Server) withMetrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		duration := time.Since(start).Seconds()
		s.metrics.httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, http.StatusText(sw.status)).Inc()
		s.metrics.httpRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (sw *statusWriter) WriteHeader(code int) {
	sw.status = code
	sw.ResponseWriter.WriteHeader(code)
}

func (sw *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := sw.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, errors.New("hijacking not supported")
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	// Health / readiness / metrics
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/readyz", s.handleReady)
	mux.Handle("/metrics", s.handleMetrics())

	// Auth
	mux.HandleFunc("/api/auth/login", s.handleLogin)

	// Current user
	mux.HandleFunc("/api/me", s.handleMe)

	// Agents
	mux.HandleFunc("/api/agents", s.handleAgents)
	mux.HandleFunc("/api/agents/", s.handleAgent)

	// Public models (enabled only for any authenticated user)
	mux.HandleFunc("/api/models", s.handleModels)

	// Admin
	mux.HandleFunc("/api/admin/users", s.handleAdminUsers)
	mux.HandleFunc("/api/admin/users/", s.handleAdminUser)
	mux.HandleFunc("/api/admin/models", s.handleAdminModels)
	mux.HandleFunc("/api/admin/models/", s.handleAdminModel)
	mux.HandleFunc("/api/admin/llm-gateway", s.handleAdminLLMGateway)
	mux.HandleFunc("/api/admin/overview", s.handleAdminOverview)

	// Activity
	mux.HandleFunc("/api/activity", s.handleActivity)

	// WebSocket
	mux.HandleFunc("/ws", s.handleWebSocket)
	mux.HandleFunc("/runtime/ws", s.handleRuntimeWebSocket)

	// Frontend
	mux.HandleFunc("/", s.handleFrontend)
	return mux
}

// --- Health / Readiness / Metrics ---

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	// Simple readiness check: can we access the store?
	_, err := s.store.ListUsers(r.Context())
	if err != nil {
		http.Error(w, `{"status":"not ready"}`, http.StatusServiceUnavailable)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) handleMetrics() http.Handler {
	if !s.cfg.Metrics {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		})
	}
	return promhttp.HandlerFor(s.metrics.registry, promhttp.HandlerOpts{})
}

// --- Auth ---

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	s.bootstrapAdmin()

	var request struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	user, token, err := s.auth.Login(r.Context(), request.Username, request.Password)
	if err != nil {
		s.recordActivity("auth.login_failed", "", "", "login failed", map[string]any{"username": request.Username})
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	s.recordActivity("auth.login", user.ID, "", "login succeeded", map[string]any{"username": user.Username})

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

// --- Current User ---

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

// --- Agents ---

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
	if len(parts) == 2 && parts[1] == "stop" {
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}
		s.handleStopAgent(w, r, parts[0])
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

func (s *Server) handleCreateAgent(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if user.Disabled {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var request domain.CreateAgentInput
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	request.Name = strings.TrimSpace(request.Name)
	request.Runtime = strings.TrimSpace(request.Runtime)
	request.Model = strings.TrimSpace(request.Model)

	if request.Name == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if request.Runtime == "" {
		request.Runtime = "nanoclaw"
	}
	if request.Runtime != "openclaw" && request.Runtime != "nanoclaw" {
		http.Error(w, "bad request: runtime must be openclaw or nanoclaw", http.StatusBadRequest)
		return
	}

	// Validate model exists and is enabled
	if request.Model != "" {
		models, err := s.store.ListLLMModels(r.Context())
		if err != nil {
			s.writeStoreError(w, err)
			return
		}
		found := false
		for _, m := range models {
			if m.ProviderModel == request.Model && m.Enabled {
				found = true
				break
			}
		}
		if !found {
			s.metrics.modelAllowlistErrors.Inc()
			http.Error(w, "bad request: model not found or not enabled", http.StatusBadRequest)
			return
		}
	}

	agent, err := s.store.CreateAgent(r.Context(), user.ID, request.Name, request.Runtime, request.Model)
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	s.recordActivity("agent.created", user.ID, agent.ID, "agent created", map[string]any{"name": agent.Name})
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
	if agents == nil {
		agents = []domain.Agent{}
	}
	s.writeJSON(w, http.StatusOK, agents)
}

func (s *Server) handleGetAgent(w http.ResponseWriter, r *http.Request, agentID string) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	agent, err := s.store.GetAgent(r.Context(), agentID)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	// Only owner or admin can view
	if agent.OwnerUserID != user.ID && user.Role != "admin" {
		http.NotFound(w, r)
		return
	}
	s.writeJSON(w, http.StatusOK, agent)
}

func (s *Server) handleStartAgent(w http.ResponseWriter, r *http.Request, agentID string) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	agent, err := s.store.GetAgent(r.Context(), agentID)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	if agent.OwnerUserID != user.ID && user.Role != "admin" {
		http.NotFound(w, r)
		return
	}

	// Validate model is enabled
	if agent.Model != "" {
		models, err := s.store.ListLLMModels(r.Context())
		if err != nil {
			s.writeStoreError(w, err)
			return
		}
		found := false
		for _, m := range models {
			if m.ProviderModel == agent.Model && m.Enabled {
				found = true
				break
			}
		}
		if !found {
			s.metrics.modelAllowlistErrors.Inc()
			http.Error(w, "model not enabled", http.StatusBadRequest)
			return
		}
	}

	// Get LLM gateway settings for the sandbox
	settings, err := s.store.GetLLMGatewaySettings(r.Context())
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	if agent.Model != "" && (!settings.Enabled || settings.BaseURL == "") {
		s.metrics.gatewayErrors.Inc()
		http.Error(w, "LLM gateway not fully configured: enabled and base URL are required when an agent model is set", http.StatusBadRequest)
		return
	}

	secret, err := randomSecret()
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	s.tokenMu.Lock()
	s.tokens[agentID] = secret
	s.tokenMu.Unlock()

	agent, err = s.store.UpdateAgentState(r.Context(), agentID, "starting")
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	s.recordActivity("agent.start_requested", user.ID, agentID, "agent start requested", map[string]any{"runtime": agent.Runtime})
	if s.logger != nil {
		s.logger.Info("agent start requested",
			"agent_id", agentID,
			"user_id", user.ID,
			"runtime", agent.Runtime,
			"model", agent.Model,
		)
	}
	lease, err := s.sandbox.Start(r.Context(), sandbox.StartRequest{
		AgentID:              agentID,
		OwnerID:              user.ID,
		Runtime:              agent.Runtime,
		RuntimeToken:         secret,
		LLMModel:             agent.Model,
		LLMGatewayBaseURL:    settings.BaseURL,
		LLMGatewaySecretName: settings.SecretName,
		LLMGatewaySecretKey:  settings.SecretKey,
	})
	if err != nil {
		_, _ = s.store.UpdateAgentState(r.Context(), agentID, "idle")
		_, _ = s.store.UpdateAgentError(r.Context(), agentID, err.Error())
		s.metrics.agentFailures.Inc()
		s.metrics.runtimePodFailures.Inc()
		s.recordActivity("sandbox.start_failed", user.ID, agentID, "sandbox start failed", map[string]any{"runtime": agent.Runtime, "error": err.Error()})
		if s.logger != nil {
			s.logger.Error("agent start failed",
				"agent_id", agentID,
				"user_id", user.ID,
				"runtime", agent.Runtime,
				"model", agent.Model,
				"error", err,
			)
		}
		s.writeStoreError(w, err)
		return
	}
	s.metrics.agentStarts.Inc()
	if s.logger != nil {
		s.logger.Info("sandbox started",
			"agent_id", agentID,
			"user_id", user.ID,
			"runtime", agent.Runtime,
			"model", agent.Model,
			"provider", lease.Provider,
			"runtime_id", lease.ExternalID,
		)
	}
	s.recordActivity("sandbox.started", user.ID, agentID, "sandbox started", map[string]any{"runtime": agent.Runtime, "provider": lease.Provider, "runtime_id": lease.ExternalID})
	agent, _ = s.store.UpdateAgentState(r.Context(), agentID, "running")
	s.writeJSON(w, http.StatusAccepted, agent)
}

func (s *Server) handleStopAgent(w http.ResponseWriter, r *http.Request, agentID string) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	agent, err := s.store.GetAgent(r.Context(), agentID)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	if agent.OwnerUserID != user.ID && user.Role != "admin" {
		http.NotFound(w, r)
		return
	}

	if err := s.sandbox.Stop(r.Context(), agentID); err != nil {
		s.writeStoreError(w, err)
		return
	}
	// Revoke the runtime token so the old runtime cannot reconnect
	s.tokenMu.Lock()
	delete(s.tokens, agentID)
	s.tokenMu.Unlock()

	agent, err = s.store.UpdateAgentState(r.Context(), agentID, "stopped")
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	s.metrics.agentStops.Inc()
	s.recordActivity("agent.stopped", user.ID, agentID, "agent stopped", nil)
	s.writeJSON(w, http.StatusOK, agent)
}

// --- Admin Users ---

func (s *Server) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleAdminListUsers(w, r)
	case http.MethodPost:
		s.handleAdminCreateUser(w, r)
	default:
		methodNotAllowed(w, "GET, POST")
	}
}

func (s *Server) handleAdminListUsers(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if user.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	users, err := s.store.ListUsers(r.Context())
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	if users == nil {
		users = []domain.User{}
	}
	s.writeJSON(w, http.StatusOK, users)
}

func (s *Server) handleAdminCreateUser(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if user.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var request struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	request.Username = strings.TrimSpace(request.Username)
	request.Role = strings.TrimSpace(request.Role)
	if request.Username == "" || request.Password == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if request.Role == "" {
		request.Role = "user"
	}
	if request.Role != "admin" && request.Role != "user" {
		http.Error(w, "bad request: role must be admin or user", http.StatusBadRequest)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(request.Password), bcrypt.DefaultCost)
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	created, err := s.store.CreateUser(r.Context(), request.Username, string(hash), request.Role)
	if errors.Is(err, store.ErrConflict) {
		http.Error(w, "username already exists", http.StatusConflict)
		return
	}
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	s.recordActivity("admin.user_created", user.ID, "", "user created", map[string]any{"username": created.Username, "role": created.Role})
	s.writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleAdminUser(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/admin/users/")
	userID := strings.TrimSpace(strings.Trim(path, "/"))
	if userID == "" {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodPatch {
		s.handleAdminUpdateUser(w, r, userID)
		return
	}
	methodNotAllowed(w, http.MethodPatch)
}

func (s *Server) handleAdminUpdateUser(w http.ResponseWriter, r *http.Request, targetUserID string) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if user.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var request struct {
		Disabled *bool   `json:"disabled"`
		Role     *string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if request.Role != nil {
		*request.Role = strings.TrimSpace(*request.Role)
		if *request.Role != "admin" && *request.Role != "user" {
			http.Error(w, "bad request: role must be admin or user", http.StatusBadRequest)
			return
		}
	}
	updated, err := s.store.UpdateUser(r.Context(), targetUserID, request.Disabled, request.Role)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, updated)
}

// --- Public Models (enabled only) ---

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleListEnabledModels(w, r)
	default:
		methodNotAllowed(w, "GET")
	}
}

func (s *Server) handleListEnabledModels(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	_ = user // any authenticated user (including non-admin) can list enabled models

	allModels, err := s.store.ListLLMModels(r.Context())
	if err != nil {
		s.writeStoreError(w, err)
		return
	}

	enabled := make([]domain.LLMModel, 0, len(allModels))
	for _, m := range allModels {
		if m.Enabled {
			enabled = append(enabled, m)
		}
	}

	// Check if gateway discovery is configured (enabled + baseURL + API key available)
	settings, err := s.store.GetLLMGatewaySettings(r.Context())
	if err != nil {
		s.writeStoreError(w, err)
		return
	}

	if settings.Enabled && settings.BaseURL != "" && s.cfg.LLMGatewayAPIKey != "" {
		gatewayModels, err := s.fetchLiteLLMModels(r.Context(), settings.BaseURL, s.cfg.LLMGatewayAPIKey)
		if err != nil {
			s.metrics.gatewayErrors.Inc()
			s.logger.Error("LiteLLM model discovery failed", "error", err)
			http.Error(w, `{"error":"LLM gateway unavailable"}`, http.StatusBadGateway)
			return
		}

		// Only keep enabled models whose provider_model exists in the LiteLLM model set
		filtered := make([]domain.LLMModel, 0, len(enabled))
		for _, m := range enabled {
			if gatewayModels[m.ProviderModel] {
				filtered = append(filtered, m)
			}
		}
		enabled = filtered
	}

	if enabled == nil {
		enabled = []domain.LLMModel{}
	}
	s.writeJSON(w, http.StatusOK, enabled)
}

// fetchLiteLLMModels queries the LiteLLM /v1/models endpoint and returns a set of
// model IDs (aliases) that the gateway reports as available.
// It handles both base URLs with and without a trailing /v1 path segment.
func (s *Server) fetchLiteLLMModels(ctx context.Context, baseURL, apiKey string) (map[string]bool, error) {
	// Normalise trailing slash
	u := strings.TrimSuffix(baseURL, "/")

	// Build the model list URL: if baseURL already ends with /v1, just append /models
	if strings.HasSuffix(u, "/v1") {
		u += "/models"
	} else {
		u += "/v1/models"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LiteLLM returned status %d", resp.StatusCode)
	}

	var result struct {
		Object string `json:"object"`
		Data   []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	models := make(map[string]bool, len(result.Data))
	for _, m := range result.Data {
		models[m.ID] = true
	}
	return models, nil
}

// --- Admin Models ---

func (s *Server) handleAdminModels(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleAdminListModels(w, r)
	case http.MethodPost:
		s.handleAdminCreateModel(w, r)
	default:
		methodNotAllowed(w, "GET, POST")
	}
}

func (s *Server) handleAdminListModels(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if user.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	models, err := s.store.ListLLMModels(r.Context())
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	if models == nil {
		models = []domain.LLMModel{}
	}
	s.writeJSON(w, http.StatusOK, models)
}

func (s *Server) handleAdminCreateModel(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if user.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var request struct {
		DisplayName   string `json:"display_name"`
		ProviderModel string `json:"provider_model"`
		Enabled       bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	request.DisplayName = strings.TrimSpace(request.DisplayName)
	request.ProviderModel = strings.TrimSpace(request.ProviderModel)
	if request.DisplayName == "" || request.ProviderModel == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	model, err := s.store.CreateLLMModel(r.Context(), request.DisplayName, request.ProviderModel, request.Enabled)
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	s.writeJSON(w, http.StatusCreated, model)
}

func (s *Server) handleAdminModel(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/admin/models/")
	modelID := strings.TrimSpace(strings.Trim(path, "/"))
	if modelID == "" {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodPatch {
		s.handleAdminUpdateModel(w, r, modelID)
		return
	}
	methodNotAllowed(w, http.MethodPatch)
}

func (s *Server) handleAdminUpdateModel(w http.ResponseWriter, r *http.Request, modelID string) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if user.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var request struct {
		DisplayName   *string `json:"display_name"`
		ProviderModel *string `json:"provider_model"`
		Enabled       *bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	model, err := s.store.UpdateLLMModel(r.Context(), modelID, request.DisplayName, request.ProviderModel, request.Enabled)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, model)
}

// --- Admin LLM Gateway ---

func (s *Server) handleAdminLLMGateway(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleAdminGetLLMGateway(w, r)
	case http.MethodPatch:
		s.handleAdminUpdateLLMGateway(w, r)
	default:
		methodNotAllowed(w, "GET, PATCH")
	}
}

func (s *Server) handleAdminGetLLMGateway(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if user.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	settings, err := s.store.GetLLMGatewaySettings(r.Context())
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, settings)
}

func (s *Server) handleAdminUpdateLLMGateway(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if user.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var request struct {
		Enabled    bool   `json:"enabled"`
		BaseURL    string `json:"base_url"`
		SecretName string `json:"secret_name"`
		SecretKey  string `json:"secret_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	settings, err := s.store.UpsertLLMGatewaySettings(r.Context(), request.Enabled, request.BaseURL, request.SecretName, request.SecretKey)
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, settings)
}

// --- Admin Overview ---

func (s *Server) handleAdminOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	if user.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	overview := domain.AdminOverview{
		Runtime: domain.AdminRuntimeConfig{
			Provider:         s.cfg.SandboxProvider,
			Namespace:        s.cfg.KubernetesNamespace,
			RuntimeClassName: s.cfg.AgentRuntimeClassName,
			Images:           s.cfg.RuntimeImages,
		},
		Observability: domain.AdminObservability{
			MetricsEnabled: s.cfg.Metrics,
			LoggingEnabled: true,
			GrafanaURL:     s.cfg.GrafanaURL,
		},
		Health: domain.AdminHealthStatus{
			Healthz: "ok",
			Readyz:  "ok",
		},
	}
	s.writeJSON(w, http.StatusOK, overview)
}

// --- Activity ---

func (s *Server) handleActivity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"activity": s.activityForUser(user)})
}

func (s *Server) recordActivity(eventType, actorID, agentID, message string, details map[string]any) {
	entry := activityEntry{Time: time.Now().UTC(), Type: eventType, ActorID: actorID, AgentID: agentID, Message: message, Details: details}
	if s.logger != nil {
		s.logger.Info("activity", "type", eventType, "actor_id", actorID, "agent_id", agentID, "message", message)
	}
	s.activityMu.Lock()
	defer s.activityMu.Unlock()
	s.activity = append(s.activity, entry)
	if len(s.activity) > 200 {
		s.activity = append([]activityEntry(nil), s.activity[len(s.activity)-200:]...)
	}
}

func (s *Server) activitySnapshot() []activityEntry {
	s.activityMu.Lock()
	defer s.activityMu.Unlock()
	return append([]activityEntry(nil), s.activity...)
}

func (s *Server) activityForUser(user domain.User) []activityEntry {
	if user.Role == "admin" {
		return s.activitySnapshot()
	}
	all := s.activitySnapshot()
	filtered := make([]activityEntry, 0, len(all))
	for _, entry := range all {
		if entry.ActorID == user.ID {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

// --- WebSocket ---

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	user, ok := s.requireUserFromRequest(w, r)
	if !ok {
		return
	}
	agentID := strings.TrimSpace(r.URL.Query().Get("agent_id"))
	if agentID == "" {
		http.Error(w, "agent_id is required", http.StatusBadRequest)
		return
	}
	agent, err := s.store.GetAgent(r.Context(), agentID)
	if errors.Is(err, store.ErrNotFound) || (err == nil && agent.OwnerUserID != user.ID && user.Role != "admin") {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.writeStoreError(w, err)
		return
	}

	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	s.metrics.activeConnections.Inc()
	defer s.metrics.activeConnections.Dec()

	var request struct {
		Text      string         `json:"text"`
		Type      string         `json:"type"`
		SessionID string         `json:"session_id"`
		MessageID string         `json:"message_id"`
		Payload   map[string]any `json:"payload"`
	}
	if err := conn.ReadJSON(&request); err != nil {
		return
	}
	text := strings.TrimSpace(request.Text)
	if text == "" && request.Payload != nil {
		text, _ = request.Payload["text"].(string)
		text = strings.TrimSpace(text)
	}
	if text == "" {
		_ = conn.WriteJSON(map[string]any{"type": "message.error", "error": "message text is required", "done": true})
		return
	}
	incoming := gateway.Envelope{
		Type:      "task.run",
		AgentID:   agentID,
		SessionID: request.SessionID,
		MessageID: request.MessageID,
		Payload:   map[string]any{"text": text},
	}
	if incoming.SessionID == "" {
		incoming.SessionID = randomHexID()
	}
	if incoming.MessageID == "" {
		incoming.MessageID = randomHexID()
	}
	events, cancel, err := s.runtime.SendTask(incoming.AgentID, incoming)
	if errors.Is(err, gateway.ErrRuntimeNotConnected) {
		_ = conn.WriteJSON(map[string]any{"type": "message.error", "error": "runtime not connected", "done": true})
		return
	}
	if err != nil {
		return
	}
	s.metrics.chatEvents.Inc()
	s.metrics.taskEvents.Inc()
	s.recordActivity("task.routed", user.ID, incoming.AgentID, "browser task routed to runtime", map[string]any{"message_id": incoming.MessageID})
	defer cancel()
	for event := range events {
		if err := conn.WriteJSON(chatEventResponse(event)); err != nil {
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
	s.metrics.activeConnections.Inc()
	defer s.metrics.activeConnections.Dec()

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
	s.recordActivity("runtime.connected", "", hello.AgentID, "runtime connected", map[string]any{"remote_addr": r.RemoteAddr})
	_ = conn.WriteJSON(gateway.Envelope{Type: "runtime.accepted", AgentID: hello.AgentID, Seq: 1})
	for {
		var event gateway.Envelope
		if err := conn.ReadJSON(&event); err != nil {
			_, _ = s.store.UpdateAgentState(r.Context(), hello.AgentID, "idle")
			s.recordActivity("runtime.disconnected", "", hello.AgentID, "runtime disconnected", map[string]any{"error": err.Error()})
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

// --- Helpers ---

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
	return s.enforceCurrentUser(w, r, user)
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
	return s.enforceCurrentUser(w, r, user)
}

func (s *Server) enforceCurrentUser(w http.ResponseWriter, r *http.Request, cached domain.User) (domain.User, bool) {
	current, err := s.store.GetUser(r.Context(), cached.ID)
	if errors.Is(err, store.ErrNotFound) || (err == nil && current.Disabled) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return domain.User{}, false
	}
	if err != nil {
		s.writeStoreError(w, err)
		return domain.User{}, false
	}
	return current, true
}

func chatEventResponse(event gateway.Envelope) map[string]any {
	response := map[string]any{"type": event.Type}
	if text, ok := event.Payload["text"].(string); ok && text != "" {
		if event.Type == "message.error" {
			response["error"] = text
		} else {
			response["text"] = text
		}
	}
	if event.Type == "message.done" || event.Type == "message.error" {
		response["done"] = true
	}
	return response
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

func randomHexID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(b[:])
}
