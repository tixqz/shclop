# Shclop Backend Codemap

## Project responsibility

Shclop is a self-hosted control plane for OpenClaw/NanoClaw agents. The backend exposes a local-auth HTTP API, stores users/agents/model configuration in PostgreSQL, launches per-agent Kubernetes/Kata runtime pods, proxies browser chat to runtime WebSocket connections, and exports JSON logs plus Prometheus-compatible metrics.

## System entry points

| Path | Responsibility |
| --- | --- |
| `cmd/shclop/main.go` | API server binary. Loads `config.Default()`, binds CLI flags, configures JSON logging, constructs `api.Server`, and starts HTTP service. Production Helm passes PostgreSQL, Kubernetes sandbox, Kata RuntimeClass, bootstrap admin, LLM gateway, and Grafana flags here. |
| `cmd/shclop-runtime/main.go` | Runtime sidecar/process binary. Reads runtime token and agent/gateway env, connects to `/runtime/ws`, registers with the backend, runs the Claw adapter for incoming tasks, and emits structured JSON logs. |
| `migrations/0001_platform.sql` | Minimal production schema: `users`, `agents`, `llm_models`, and single-row `llm_gateway_settings`. Workspaces, skills, audit runs, approvals, and revision tables are intentionally absent. |
| `charts/shclop/` | Production Helm chart. Renders backend Deployment/Service, optional bundled PostgreSQL, bootstrap admin Secret, migration ConfigMap/initContainer, sandbox Namespace/RBAC, ServiceMonitor, and values for VictoriaMetrics/VictoriaLogs/Grafana guidance. |

## Directory map

| Directory | Responsibility summary |
| --- | --- |
| `internal/api/` | HTTP API, auth middleware, admin/user/model/gateway handlers, activity log, health/readiness/metrics endpoints, browser chat WebSocket, runtime registration WebSocket, static frontend serving. |
| `internal/auth/` | Local username/password auth using bcrypt hashes from the store and random bearer session tokens. Disabled-user enforcement is rechecked by API middleware against current store state. |
| `internal/config/` | Runtime configuration defaults and environment bindings for store backend, Kubernetes runtime (including `SHCLOP_POD_READY_TIMEOUT`), network policy, metrics, bootstrap admin, LLM gateway Secret metadata, and Grafana URL. |
| `internal/domain/` | Minimal API/domain DTOs: `User`, `Agent`, `LLMModel`, `LLMGatewaySettings`, `AdminOverview`, and `Message`. |
| `internal/store/` | Persistence abstraction plus in-memory test/dev store and PostgreSQL implementation. Owns user password hashes, agent state, enabled model allowlist, gateway settings, and bootstrap admin reconciliation. |
| `internal/sandbox/` | Runtime provider abstractions and Kubernetes pod/PVC/Secret/NetworkPolicy builders. Builds hardened Kata pod specs, injects `LLM_GATEWAY_BASE_URL`, `LLM_GATEWAY_MODEL`, and `LLM_GATEWAY_API_KEY` from a Kubernetes Secret key reference, and waits for pod readiness (Running + all containers Ready) before returning from Start. On timeout or Failed phase, collects warning Events and container waiting reasons for the error message. |
| `internal/gateway/` | In-memory runtime connection registry. Tracks runtime WebSocket connections per agent and routes `task.run` envelopes from browser chat to connected runtimes. |
| `internal/claw/` | Adapter abstraction for executing Claw tasks from runtime processes, including subprocess, demo, and OpenAI-compatible LLM adapter. The OpenAI adapter reads `LLM_GATEWAY_BASE_URL`, `LLM_GATEWAY_MODEL`, and `LLM_GATEWAY_API_KEY` environment variables and makes streaming HTTP requests to external LLM providers. |
| `internal/logging/` | `slog` JSON logger construction with configurable log level. |
| `internal/identity/` | Legacy identity interfaces/mock YAML provider retained for compatibility but not wired into the production API path. |
| `internal/security/` | Legacy security policy/audit helpers retained but not exposed by production routes. |
| `runtime/` | Runtime image definitions for NanoClaw/OpenClaw and shell adapter used inside agent runtime containers; the adapter reads the mounted runtime token file before launching `shclop-runtime`. |
| `cmd/mock-runtime/` | Existing local mock-runtime tooling outside the production Helm path. |

## Core data model

| Entity | Fields and role |
| --- | --- |
| `User` | `id`, `username`, `role` (`admin` or `user`), `disabled`, timestamps. Admin users manage users/models/gateway settings; normal users own agents. |
| `Agent` | `id`, `owner_user_id`, `name`, `runtime` (`openclaw` or `nanoclaw`), `model`, `state`, `last_error`, timestamps. Agents are user-owned runtime definitions and state records. |
| `LLMModel` | `id`, display name, provider model identifier, enabled flag, timestamps. Backend validates non-empty agent models against enabled provider identifiers before create/start. |
| `LLMGatewaySettings` | Enabled flag, external base URL, Kubernetes Secret name/key metadata, updated timestamp. Raw API keys are not stored in the backend DB. |

## HTTP and WebSocket surface

| Endpoint | Responsibility |
| --- | --- |
| `GET /healthz` | Process liveness: returns `{"status":"ok"}`. |
| `GET /readyz` | Store readiness: calls `store.ListUsers()` and returns ready or 503. |
| `GET /metrics` | Prometheus metrics via a custom registry when metrics are enabled; returns 404 when disabled. |
| `POST /api/auth/login` | Local login. Bootstraps admin on demand, sets `shclop_session` cookie, and returns `{user, token}`. |
| `GET /api/me` | Returns current store-backed user after bearer/cookie validation. |
| `/api/agents`, `/api/agents/{id}`, `/api/agents/{id}/start`, `/api/agents/{id}/stop` | User agent listing, creation, retrieval, start, and stop. Start returns the updated `Agent` directly with HTTP 202 and keeps runtime tokens internal. |
| `/api/admin/users`, `/api/admin/users/{id}` | Admin-only user creation/listing and role/disabled updates. |
| `/api/admin/models`, `/api/admin/models/{id}` | Admin-only model allowlist creation/listing/update and enable/disable. |
| `GET/PUT /api/admin/llm-gateway` | Admin-only gateway settings read/update for external base URL and Secret metadata. |
| `GET /api/admin/overview` | Admin runtime/observability/health summary, including Kubernetes runtime config and Grafana URL. |
| `GET /api/activity` | In-memory capped activity feed. |
| `GET /ws?agent_id=...&token=...` | Browser chat WebSocket. Accepts query token/cookie/bearer auth, receives `{text}`, forwards `task.run` to runtime registry, and streams top-level `{type,text}`/`{type,error,done}` messages. |
| `GET /runtime/ws` | Runtime WebSocket. Runtime authenticates with an internal bearer token and registers via `runtime.hello`. |

## Persistence flow

1. `api.NewServer()` opens `store.Open()` from `config.Config.Store` (`postgres` in Helm; `inmemory` allowed for tests/dev).
2. `Server.bootstrapAdmin()` hashes `SHCLOP_BOOTSTRAP_ADMIN_PASSWORD` with bcrypt and calls `Store.BootstrapAdmin()` to create or reconcile the admin user.
3. If gateway flags are present, bootstrap also upserts `llm_gateway_settings` metadata.
4. Auth login uses `auth.Service`, which loads the user by username and compares bcrypt hashes via store-provided password hash methods.
5. Authenticated API requests resolve the token and reload the current user from the store so disabled/deleted users are blocked after login.

## Runtime launch and chat flow

1. User creates an agent with runtime `openclaw` or `nanoclaw` and an optional enabled provider model.
2. `start` validates model allowlist and LLM gateway readiness when a model is set.
3. Backend creates an internal runtime token, stores it in memory by agent ID, logs the start request with agent_id, user_id, runtime, and model, builds a sandbox launch request, and calls the configured `RuntimeProvider`.
4. Kubernetes provider creates/updates Secret, PVC, optional NetworkPolicy, and a hardened runtime Pod with Kata `RuntimeClassName`, no host namespaces, no service account token, non-root user, read-only root filesystem, seccomp `RuntimeDefault`, and dropped Linux capabilities.
5. Provider then polls the Pod phase and container statuses until the pod is `Running` with all containers `Ready`, the pod enters `Failed` phase, or a configurable timeout (`PodReadyTimeout`, default 120s) expires. On timeout, warning Events for the pod and container waiting reasons are collected into the error message. On success, the agent state transitions to `running`.
6. Runtime process reads its token, connects to `/runtime/ws`, registers, receives `task.run`, invokes the Claw adapter (`OpenAIAdapter` when `LLM_GATEWAY_BASE_URL` and `LLM_GATEWAY_API_KEY` are set, `DemoAdapter` otherwise), and streams task events back.
7. Browser chat on `/ws` forwards user text to `RuntimeRegistry` and relays runtime payloads to the SPA.

## Observability

- Backend logs use `slog.JSONHandler(os.Stdout)` with configurable level. Runtime logs also use JSON `slog` and avoid printing API keys.
- Metrics are exported by `internal/api` using `github.com/prometheus/client_golang`: HTTP request count/duration, active connections, agent starts/stops/failures, runtime pod creation failures, chat/task events, model allowlist failures, and LLM gateway validation errors.
- Helm annotates the Service for Prometheus scraping by default and can render a `ServiceMonitor` when `monitoring.serviceMonitor.enabled=true`.
- Chart values document VictoriaMetrics k8s-stack, VictoriaLogs, and Grafana defaults with 7-day metrics/log retention.

## Helm production shape

- `postgresql.bundled: true` by default creates a single-node PostgreSQL Secret/PVC/Deployment/Service; external production DBs can be supplied through an existing DSN Secret.
- `agentRuntime.runtimeClassName` defaults to `kata` and fails Helm rendering if empty.
- Backend Deployment fails Helm rendering unless PostgreSQL is bundled or an external DSN Secret is configured, and unless `sandbox.provider=kubernetes`.
- Backend Deployment uses a PostgreSQL initContainer to apply the minimal schema before the API process starts, then passes `--store=postgres`, `--sandbox-provider=kubernetes`, runtime image flags/env, network policy mode, bootstrap admin env, LLM gateway flags, probes for `/healthz` and `/readyz`, and optional Grafana URL.
- The chart creates a backend ServiceAccount plus Role/RoleBinding in the sandbox namespace so the backend can manage runtime Pods, Secrets, PVCs, and NetworkPolicies.
- Mock, Docker demo, in-memory, identity federation, Vault, MinIO, workspace, skills, LDAP/OIDC, MCP, and security-policy features are not part of the charted production path.

## Verification baseline

After the production simplification, the following commands have passed during implementation:

```bash
go test ./...
go mod tidy && go vet ./... && go build ./...
npm run build   # from web/
helm template shclop charts/shclop
helm template shclop charts/shclop --set monitoring.serviceMonitor.enabled=true --set llmGateway.baseURL=https://llm.example.com --set llmGateway.existingSecret.name=llm-secret --set observability.grafana.url=https://grafana.example.com
helm template shclop charts/shclop --set agentRuntime.runtimeClassName=  # expected failure
```
