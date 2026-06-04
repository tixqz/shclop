# shclop

Shclop is a self-hosted production control plane for users and their OpenClaw/NanoClaw agents.

The current product scope is intentionally small: local users log in, create agents, select an admin-enabled LLM model, start/stop the runtime, and chat with the agent. Administrators manage local users, model availability, LLM gateway settings, and runtime/observability status.

## Architecture

```text
Browser UI
  | HTTPS / WebSocket
  v
Shclop backend + embedded UI
  | stores users, agents, models, sessions, settings
  v
PostgreSQL

Shclop backend
  | creates Pod/PVC/Secret/NetworkPolicy
  v
Kubernetes runtime provider
  | RuntimeClass: kata
  v
Hardened OpenClaw/NanoClaw runtime pods
  | WebSocket: /runtime/ws
  v
Shclop backend

Runtime pod
  | SHCLOP_LLM_GATEWAY_BASE_URL
  | SHCLOP_LLM_MODEL
  | API key SecretKeyRef metadata
  v
Operator-provided LLM gateway
```

Production uses Kubernetes runtime pods with a Kata RuntimeClass. Docker images are packaging only; they are not the production isolation boundary for agent workloads.

## Current scope

Implemented UI/API path:

- Local authentication only.
- Bootstrap admin from environment variables or a Kubernetes Secret.
- Admin-created users with `admin` or `user` role; users can be disabled.
- Admin-managed enabled/disabled LLM models.
- Admin-managed LLM gateway base URL and Kubernetes Secret name/key metadata.
- User-owned OpenClaw/NanoClaw agents.
- Agent create/start/stop/chat flow.
- Kubernetes provider for production runtimes.
- JSON stdout logs, `/healthz`, `/readyz`, and Prometheus-compatible `/metrics`.

Not in the current UI/API path:

- Workspaces, teams, catalogs, or skills.
- LDAP, OIDC, header auth, or SCIM.
- MCP servers or third-party integration management.
- Security policy approval workflows.
- Built-in LLM proxying.

## Production install with Helm

### Prerequisites

- Kubernetes cluster with a Kata Containers RuntimeClass, usually named `kata`.
- StorageClass for PostgreSQL and per-agent workspace PVCs.
- NetworkPolicy-capable CNI.
- Runtime images for OpenClaw and NanoClaw reachable by the cluster.
- PostgreSQL. The chart can deploy a bundled single-node PostgreSQL instance by default, or read an external DSN from an existing Secret.
- An operator-managed LLM gateway and a Kubernetes Secret containing its API key.

### Provider-agnostic single-node sizing

These requirements are for a single-node test or small production installation. They apply to any VM, bare-metal host, or cloud instance provider.

| Size | CPU | Memory | Disk | Expected use |
| --- | --- | --- | --- | --- |
| Minimum | 2 vCPU | 4 GiB RAM | 30 GiB free disk | Bootstrap validation, UI, PostgreSQL, and one light runtime pod. |
| Recommended | 4 vCPU | 8 GiB RAM | 50 GiB free SSD/NVMe disk | A usable single-node test installation with one or two active runtime pods. |
| Larger workloads | Add capacity per concurrent runtime pod | Add 1-2 GiB RAM per active runtime pod | Add workspace PVC capacity per agent | More concurrent agents, larger workspaces, or heavier model/tool workloads. |

Additional requirements:

- Hardware virtualization should be available as `/dev/kvm` for Kata Containers. Without KVM, Kata may run slowly or may not be suitable for production isolation.
- The K3s baseline resource requirements do not include application workloads. Plan extra CPU, memory, and disk for shclop, PostgreSQL, runtime pods, observability, and workspaces.
- Keep PostgreSQL data and workspace PVCs on persistent storage. For single-node tests, the bundled PostgreSQL is acceptable; for durable production use, prefer managed or separately backed up PostgreSQL.
- If Ingress TLS is enabled, ports `80/tcp` and `443/tcp` must be reachable by the ACME HTTP-01 issuer.

The bootstrap script checks CPU, memory, disk, and KVM availability during `check` and `install`. Override the default thresholds with `MIN_CPU_CORES`, `MIN_MEMORY_MIB`, and `MIN_DISK_GIB` when testing on intentionally smaller hosts.

### Minimal values

Create the runtime namespace if it does not exist:

```bash
kubectl create namespace shclop-sandbox
```

Create Secrets for the bootstrap admin password and LLM gateway API key:

```bash
kubectl create secret generic shclop-bootstrap-admin \
  --from-literal=password='replace-with-a-long-random-password'

kubectl create secret generic shclop-llm-gateway \
  --from-literal=api-key='replace-with-gateway-api-key'
```

Example `production-values.yaml` using the bundled single-node PostgreSQL:

```yaml
image:
  repository: registry.example.com/shclop/shclop
  tag: 0.1.0

postgresql:
  bundled: true
  persistence:
    size: 20Gi
    storageClass: fast-retain

sandbox:
  provider: kubernetes
  kubernetes:
    namespace: shclop-sandbox
    gatewayURL: ws://shclop:8080/runtime/ws
    workspace:
      size: 10Gi
      storageClassName: fast-retain
      retention: retain
    networkPolicy:
      enabled: true
      allowedCIDRs: "10.0.0.0/8"

agentRuntime:
  runtimeClassName: kata
  images:
    openclaw: registry.example.com/shclop/runtime-openclaw:0.1.0
    nanoclaw: registry.example.com/shclop/runtime-nanoclaw:0.1.0

bootstrapAdmin:
  username: admin
  existingSecret:
    name: shclop-bootstrap-admin
    key: password

llmGateway:
  baseURL: https://llm-gateway.example.com/v1
  existingSecret:
    name: shclop-llm-gateway
    key: api-key

observability:
  retentionDays: 7
  grafana:
    enabled: true
    url: https://grafana.example.com
```

Install or upgrade:

```bash
helm upgrade --install shclop charts/shclop \
  --namespace shclop --create-namespace \
  -f production-values.yaml
```

Render before applying:

```bash
helm template shclop charts/shclop \
  --namespace shclop \
  -f production-values.yaml
```

### External PostgreSQL

For production databases managed outside the chart, create a Secret containing the full DSN:

```bash
kubectl create secret generic shclop-postgres-dsn \
  --namespace shclop \
  --from-literal=dsn='postgres://shclop:password@postgres.example.com:5432/shclop?sslmode=require'
```

Then override bundled PostgreSQL:

```yaml
postgresql:
  bundled: false
  existingSecret:
    name: shclop-postgres-dsn
    key: dsn
```

PostgreSQL is required for production. In-memory storage is for development and tests only.

## LLM gateway behavior

Shclop does not include an LLM proxy. The backend stores:

- gateway base URL;
- Kubernetes Secret name;
- Kubernetes Secret key;
- enabled model list.

When a user creates or starts an agent, the backend validates that the selected model is enabled. When the Kubernetes runtime pod is created, Shclop passes the base URL, model, and API key SecretKeyRef to the pod. The gateway itself is operated outside Shclop.

## OIDC SSO (optional)

Shclop supports OpenID Connect single sign-on as an alternative or supplement to local password login. Configure one or more identity providers (Keycloak, Auth0, Okta, Google Workspace, Azure AD, etc.) via Helm values.

### Prerequisites

1. The IdP must have an OAuth2 client registered with redirect URI `https://<your-shclop-host>/api/auth/oidc/<provider-name>/callback`.
2. A Kubernetes Secret containing the OIDC client secret.
3. A 32-byte random key (base64-encoded) stored in a Kubernetes Secret, used to encrypt the OIDC state cookie.

Generate the cookie key:

```bash
openssl rand -base64 32 | kubectl create secret generic shclop-auth-cookie --from-literal=cookie-key=-
```

(Adjust to your namespace.)

### Helm values

```yaml
auth:
  mode: both    # local | sso | both
  cookieKey:
    existingSecret:
      name: shclop-auth-cookie
      key: cookie-key
  idp:
    providers:
      - name: keycloak
        displayName: "Sign in with Keycloak"
        issuer: https://keycloak.example.com/realms/main
        clientID: shclop
        clientSecret:
          existingSecret:
            name: shclop-keycloak
            key: client-secret
        redirectURI: https://shclop.example.com/api/auth/oidc/keycloak/callback
        scopes: [openid, email, profile, groups]
```

After `helm upgrade --install`, the login screen shows a "Sign in with Keycloak" button. First successful sign-in JIT-creates a local user with role `user`. The admin can promote via the Users panel.

### Modes

- `local` — only local username+password login. SSO buttons hidden.
- `sso` — only SSO. Local login disabled except for the bootstrap admin (`SHCLOP_BOOTSTRAP_ADMIN_USERNAME`), accessible via `?break_glass=1` on the login URL.
- `both` — both paths active. SSO users JIT-provisioned, local users still log in with password.

The admin can switch modes at runtime via the Admin → Auth tab. The initial seed value comes from `auth.mode` in Helm values; subsequent changes are stored in the database.

### Account linking

If an IdP returns an email that matches an existing local user, the SSO flow returns `409 local_user_with_same_email_exists`. The admin must explicitly link the identity via the Admin → Users → Identities panel before the user can sign in via that IdP. This protects against IdP-side email-spoofing capturing existing local accounts.

## Health, readiness, metrics, and logs

Endpoints:

- `GET /healthz` returns process liveness.
- `GET /readyz` verifies that the backend can access its store.
- `GET /metrics` exposes Prometheus-compatible metrics when metrics are enabled.

Logs are structured JSON on stdout/stderr and should be collected by the cluster logging stack.

The chart includes Service annotations and an optional ServiceMonitor. It documents, but does not install, a recommended observability stack:

- VictoriaMetrics k8s-stack for metrics.
- VictoriaLogs for logs.
- Grafana for dashboards.
- 7-day retention defaults via `observability.retentionDays`.

## Development

Development can still use the in-memory store and mock or Docker demo runtime providers. See [`DEVELOPMENT.md`](DEVELOPMENT.md) for local commands, tests, builds, and Helm rendering.

## Guides

- [`ADMIN_GUIDE.md`](ADMIN_GUIDE.md): admin flows for users, models, LLM gateway, runtime, and observability.
- [`USER_GUIDE.md`](USER_GUIDE.md): user flow for creating and chatting with OpenClaw/NanoClaw agents.
- [`docs/kubernetes-claw-runtime-design.md`](docs/kubernetes-claw-runtime-design.md): Kubernetes runtime provider design notes.

## License

See [`LICENSE`](LICENSE).
- Owners can configure within those bounds: autonomy mode, schedules, allowed domains, model preference, workspace size, and integration grants.

Secrets must not be mounted into an agent runtime. A compromised agent should not be able to read Vault, provider tokens, Kubernetes credentials, or another tenant’s data. Integration connectors request short-lived, tenant-scoped secret access per typed action. The secret path is derived by the platform from verified grant metadata, never supplied by the agent.

For larger installations, shared connector pools are expected. They should still use request-scoped Vault access, exact-path policies, no list permission, short TTLs, and audit metadata containing tenant, integration, grant, action, and request IDs. High-risk tenants can be moved to dedicated connector pools or Vault namespaces.

See [`USER_GUIDE.md`](USER_GUIDE.md) for the workspace user flow and [`ADMIN_GUIDE.md`](ADMIN_GUIDE.md) for the read-only admin area.

## Monitoring

Monitoring has to cover more than uptime. Agents fail in ways that look like normal automation until you inspect intent, approval, credentials, and egress.

Recommended signals:

- API and gateway request counts, latency, status codes, and WebSocket disconnects.
- Agent lifecycle events: created, starting, hot, idle, restoring, archived, deleted.
- Runtime resource use: CPU, memory, disk, PID count, restarts, startup time.
- Tool and integration ledger: requested action, approval state, provider response class, duration, error code.
- Secret access: token exchange, Vault read, lease, revoke, rotation, denied access.
- Network events: denied egress, DNS anomalies, private CIDR attempts, proxy allow/deny decisions.
- Scheduler events: due, leased, started, completed, failed, skipped, disabled.
- Security events: unexpected runtime network target, policy violation, suspicious package download, privilege escalation attempt.

Suggested stack:

- Prometheus for metrics.
- Loki or ELK for structured logs.
- OpenTelemetry with Tempo or Jaeger for traces.
- Alertmanager or the organization’s existing paging system.
- A durable audit store for action, approval, grant, and secret-use records.

The application should emit structured logs and metrics; the cluster decides where they go.

## Security model

Shclop assumes agent runtime compromise is possible. That includes root inside the guest/container. The platform design must make that insufficient for host access, tenant breakout, secret theft, or unrestricted network access.

Security boundaries:

- Kubernetes Restricted pod settings for agent runtime pods.
- Kata Containers RuntimeClass for microVM isolation.
- No host network, host PID, host IPC, hostPath, privileged mode, host devices, or mounted Kubernetes service account token for agent runtimes.
- Deny-by-default egress with explicit proxy-mediated exceptions.
- Runtime-to-platform traffic over authenticated encrypted channels.
- No direct runtime access to Vault, Postgres, Kubernetes API, provider APIs, or cloud metadata endpoints.
- Integration Broker exposes typed provider actions, not a generic HTTP proxy.
- LLM Broker owns model credentials and policy; runtimes do not receive provider keys.

## Status

This repository currently contains the foundation slice:

- Go backend entrypoint, config, logging, REST API, local auth, in-memory and Postgres-backed agent store, and WebSocket endpoints for browser chat and runtime registration.
- React/Vite/TypeScript UI served separately in dev or embedded in the built container image.
- Dockerfile for a single backend+UI image.
- Helm chart skeleton for the backend service, Postgres DSN wiring, identity settings, sandbox settings, and runtime image settings.
- Bootstrap script skeleton with local default and explicit `--remote user@host` execution.
- Runtime image skeletons for NanoClaw and OpenClaw using their official install paths, plus a demo runtime process that connects to the Shclop runtime WebSocket and streams task events. The `docker-demo` sandbox provider can launch those images through a local Docker daemon for single-machine demos. (NemoClaw is removed for now; it will return after a review of its NVIDIA-based installer dependencies.)
- Kata sandbox provider foundation that builds the hardened agent pod spec shape: RuntimeClass, no service account token, no privileged mode, read-only root filesystem, dropped capabilities, workspace and memory mounts.

Schema migrations currently live under `migrations/`. The first migration creates the `agents` table used by the Postgres store.

The runtime images are bootstrap skeletons. They intentionally follow the upstream install paths for now, but they are not yet a pinned/signed supply-chain baseline. Treat them as a starting point for reproducible runtime images, not as final production images.

Not implemented yet:

- **Kubernetes sandbox controller.** The repository now has the agent pod spec builder and Kata runtime image catalog boundary, but the backend still does not talk to the Kubernetes API. This layer should create, start, idle, and delete runtime pods and PVCs; watch pod status; collect logs; attach NetworkPolicies; and clean up abandoned resources.

  Planned direction: implement this first as an **embedded Kubernetes controller MVP** inside the backend, with extraction-ready interfaces for a future standalone controller. The MVP should create Pod + credential reference + generated NetworkPolicy + PVC/workspace resources, use Kata RuntimeClass, treat runtime WebSocket registration as readiness, and perform idempotent cleanup. NetworkPolicy should be configured through sandbox config/Helm values (`disabled`, `restricted`, or `custom`), not by requiring users to write raw per-agent policy YAML. Kubernetes Secret delivery is only a dev/MVP fallback; production secret delivery should go through a `RuntimeSecretStore`/`SecretRef` abstraction backed by Vault Agent Injector, CSI Secret Store Driver, or External Secrets. See `docs/kubernetes-claw-runtime-design.md`.

- **Real Claw execution inside the runtime.** The runtime process now connects to Shclop, registers the agent, receives tasks, and streams demo events. It does not yet invoke NanoClaw, NemoClaw, or OpenClaw for real work. The next step is to translate `task.run` envelopes into the selected agent CLI invocation, stream stdout/stderr as structured events, enforce workspace/memory paths, and shut down cleanly on platform cancellation.

  Planned direction: add a runtime-side `ClawAdapter` boundary. Public docs found so far do not prove NanoClaw/OpenClaw are fully compatible at the task-execution contract level, so Shclop should verify runtime capabilities and prefer one `ClawCompatibleAdapter` only when the selected binaries expose the same contract. If no structured contract is available, a subprocess adapter may bridge stdout/stderr/exit codes into `message.started`, `message.delta`, `message.done`, and `message.error` events. See `docs/kubernetes-claw-runtime-design.md`.

- **Full Postgres platform schema.** Agents can use Postgres, but the durable platform model is still incomplete. Production needs tables for users, tenants, teams, sessions, messages, schedules, approvals, grants, lifecycle state, tool/action ledgers, usage, and audit records. Agent memory still belongs in workspace files; Postgres is for platform state and ordering.

- **Vault integration.** Secrets are not wired yet. The platform needs a SecretStore implementation where model credentials, OAuth refresh tokens, provider keys, connector material, and signing keys are stored by reference. Agent runtimes should never receive Vault tokens or provider credentials.

- **LLM Broker provider adapters.** Model calls currently have no real broker. This layer should route runtime model requests through platform policy, quotas, audit, and provider-specific adapters: OpenAI-compatible APIs, Anthropic, Vertex/Gemini, local Ollama/vLLM, or an internal corporate LLM gateway.

- **Integration Broker connectors.** The design calls for typed integration actions, but the connectors are not implemented. Examples: `github.create_pull_request`, `slack.post_message`, `notion.update_page`. Connectors should perform policy checks, request scoped secrets just in time, call provider APIs, and write an audit record.

- **Tenant/team RBAC.** The current auth path is local `admin/admin`. Production needs tenants, teams, roles, ownership, invitations or identity-provider mapping, and permission checks for agent creation, approvals, schedule ownership, integration grants, and admin guardrails.

- **Scheduler execution.** Schedule concepts are architectural only. The platform needs a durable schedule table, lease-based Go workers, timezone handling, retry policy, owner approval for agent-created schedules, and a path that wakes an idle agent before delivering the scheduled task.

- **Egress proxy enforcement.** The README describes deny-by-default egress, but no proxy or NetworkPolicy generator exists yet. This layer should block private ranges and arbitrary internet by default, allow explicit destinations per agent/tool/grant, log denies, and surface approval prompts when an agent requests new access.

- **Metrics endpoint and production dashboards.** The CLI has a metrics flag, but there is no useful metrics surface yet. The next step is to expose API, gateway, scheduler, broker, runtime lifecycle, secret-use, and egress metrics with labels that work for Prometheus without leaking tenant data.

- **Backups, upgrades, and restore workflows.** The platform needs operator procedures for Postgres PITR, workspace/PVC snapshots, object storage versioning, Vault backup/restore, Helm upgrades, schema migrations, and per-agent restore tests.

That separation is intentional. The foundation should be buildable and reviewable before the security-sensitive runtime, credential, and policy layers are added.
