# shclop

Shclop is an open-source platform for running organization-controlled AI agents.

The point is to give teams a place where agent access to models, tools, secrets, browsers, shells, schedules, and networks can be configured once, audited, and operated without every team inventing its own unsafe wrapper.

## Motivation

Shclop is designed around a stricter model:

- agents run in isolated runtime environments;
- model and provider credentials stay behind platform brokers;
- tool and integration calls are typed operations, not arbitrary secret-bearing HTTP;
- network egress is denied by default and opened deliberately;
- tenant, team, user, agent, schedule, and action boundaries are first-class platform concepts;
- operators can deploy the same control plane on a single node or in a Kubernetes cluster.

## High-level architecture

```text
Browser / UI
    |
    | HTTPS / WebSocket
    v
Shclop API + Agent Gateway
    |
    | lifecycle requests
    v
Orchestrator / Sandbox Provider
    |
    | Kubernetes RuntimeClass
    v
Kata-isolated Agent Runtime Pods

Agent Runtime --> LLM Broker ---------> model provider or internal LLM gateway
Agent Runtime --> Integration Broker -> typed provider connectors -> GitHub/Slack/etc.
Agent Runtime --> Egress Proxy -------> explicitly allowed network destinations

SecretStore / Vault keeps provider credentials out of the runtime.
Postgres keeps platform state, audit records, sessions, schedules, and ownership.
Object storage keeps artifacts, archives, and workspace exports.
Observability stack collects metrics, logs, traces, and security events.
```

Docker images are packaging. They are not the security boundary for untrusted agents. In production, agent runtimes are expected to run as Kubernetes pods with a Kata RuntimeClass so that the workload gets a microVM boundary while still using normal OCI images and Kubernetes scheduling.

## Installation: Kubernetes

Kubernetes is the primary deployment shape. Build one backend+UI image, push it to a registry reachable by the cluster, then install the Helm chart.

```bash
make docker-build IMAGE=registry.example.com/shclop/shclop:0.1.0
docker push registry.example.com/shclop/shclop:0.1.0

helm install shclop charts/shclop \
  --set image.repository=registry.example.com/shclop/shclop \
  --set image.tag=0.1.0 \
  --set sandbox.provider=kubernetes
```

Check the rendered manifests before applying them:

```bash
helm template shclop charts/shclop \
  --set image.repository=registry.example.com/shclop/shclop \
  --set image.tag=0.1.0 \
  --set sandbox.provider=kubernetes
```

For a real cluster, plan these pieces before exposing users:

- Ingress or gateway with public TLS.
- Upstream TLS or mTLS from ingress to Shclop services.
- Postgres for platform state.
- Vault or another SecretStore implementation for credentials.
- S3-compatible object storage for artifacts and archives.
- A Kata-enabled node pool for agent runtimes.
- NetworkPolicy support from the cluster CNI.
- Persistent storage classes for workspaces.
- A registry policy for platform and agent runtime images.

The current chart is intentionally small: it installs the Shclop backend service and deployment. External Postgres, Vault, object storage, ingress, RuntimeClass, and observability wiring belong in environment-specific values or adjacent platform charts.

## Installation: single node

Single-node mode is for evaluation and small self-hosted installs. It still targets Linux and KVM because the production runtime model depends on Kata/microVM isolation.

Minimum host expectations:

- Linux host or VM with `/dev/kvm` available;
- container runtime and Kubernetes distribution, usually K3s for this mode;
- Kata Containers installed and registered as a Kubernetes RuntimeClass;
- enough CPU/RAM/disk for both control plane services and agent runtimes.

Run the bootstrap checks locally:

```bash
scripts/bootstrap.sh check
```

Run them against a remote Linux host:

```bash
scripts/bootstrap.sh check --remote root@example.com
```

Install path shape:

```bash
scripts/bootstrap.sh install --install-deps
```

The bootstrap command is conservative by design: local is the default target, remote execution is explicit through `--remote`, and destructive actions require explicit commands and confirmation flags.

## Build and image workflow

The main artifact is a single OCI image containing the Go backend and the compiled React UI.

```bash
make docker-build IMAGE=shclop:latest
```

Build runtime images for local/demo agents:

```bash
make runtime-images
```

**Runtime flavor for tests:** `nanoclaw` is the default. All test commands, demos, and debugging examples that accept `--runtime` should use `nanoclaw` unless explicitly testing another flavor.

`nemoclaw` is removed for now — it will be added back later when its NVIDIA-based installer and runtime dependencies are reviewed.

Detailed development, testing, and UI workflow notes live in [`DEVELOPMENT.md`](DEVELOPMENT.md).

## Local functional demo

The local demo uses Docker only as a convenience launcher for runtime containers. It is not the production isolation model.

```bash
make runtime-images

go run ./cmd/shclop \
  --dev \
  --store inmemory \
  --sandbox-provider docker-demo \
  --identity-provider mock-yaml \
  --identity-mock-yaml config/identity.mock.yaml
```

Open `http://localhost:8080`, log in as `bob@acme.test/bob`, create an agent, start it, and send a chat task. The backend starts a local runtime container, the runtime connects back to `/runtime/ws`, and browser messages stream through the Agent Gateway.

The UI also exposes global **Agents** and **Skills** tabs as simple list-first catalogs. The Add button opens the create form, and clicking a row opens a detail modal with edit/delete actions. The top-right user menu shows avatar initials, organization metadata, roles, disabled Settings, and Log out.

For identity-provider mapping demos:

```bash
go run ./cmd/shclop \
  --dev \
  --store inmemory \
  --sandbox-provider docker-demo \
  --identity-provider mock-yaml \
  --identity-mock-yaml config/identity.mock.yaml
```

Demo users are `alice@acme.test/alice`, `bob@acme.test/bob`, and `eve@other.test/eve`. If Shclop is started without `--identity-provider mock-yaml`, only the local fallback account `admin/admin` is available.

## Multi-tenancy

Shclop treats multi-tenancy as an operational boundary, not just a UI filter.

The intended hierarchy is:

```text
tenant / organization
  team / workspace
    user
      agent
        sessions, schedules, grants, workspace, audit events
```

The practical rules:

- A user can create agents they own.
- Agent actions are evaluated against platform guardrails and owner-approved grants.
- Teams can share integrations and policies without exposing raw credentials to agents.
- Admins define upper bounds: allowed tools, model providers, network scopes, runtime profiles, retention, and budget limits.
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

- **Kubernetes sandbox controller.** The repository now has an embedded Kubernetes provider MVP and the backend does talk to the Kubernetes API for sandbox lifecycle. The current implementation creates Pod, Secret, PVC, and NetworkPolicy resources for `sandbox.provider=kubernetes`, but this remains an MVP. The standalone controller extraction, production Vault-backed secret delivery, and full egress proxy enforcement are still future work.

  Planned direction: continue extracting the controller behind stable interfaces so a future standalone controller can reuse the same resource builders and lifecycle logic. NetworkPolicy should stay configured through sandbox config/Helm values (`disabled`, `restricted`, or `custom`), not by requiring users to write raw per-agent policy YAML. Kubernetes Secret delivery is currently the MVP fallback; production secret delivery should go through a `RuntimeSecretStore`/`SecretRef` abstraction backed by Vault Agent Injector, CSI Secret Store Driver, or External Secrets. See `docs/kubernetes-claw-runtime-design.md`.

- **Real Claw execution inside the runtime.** The runtime process now connects to Shclop, registers the agent, receives tasks, and streams demo events. It uses an adapter boundary, but it does not yet invoke NanoClaw/OpenClaw for real work. The next step is to translate `task.run` envelopes into the selected agent CLI invocation, stream stdout/stderr as structured events, enforce workspace/memory paths, and shut down cleanly on platform cancellation.

  Planned direction: add a runtime-side `ClawAdapter` boundary. Public docs found so far do not prove NanoClaw/OpenClaw are fully compatible at the task-execution contract level, so Shclop should verify runtime capabilities and prefer one `ClawCompatibleAdapter` only when the selected binaries expose the same contract. If no structured contract is available, a subprocess adapter may bridge stdout/stderr/exit codes into `message.started`, `message.delta`, `message.done`, and `message.error` events. See `docs/kubernetes-claw-runtime-design.md`.

- **Full Postgres platform schema.** Agents can use Postgres, but the durable platform model is still incomplete. Production needs tables for users, tenants, teams, sessions, messages, schedules, approvals, grants, lifecycle state, tool/action ledgers, usage, and audit records. Agent memory still belongs in workspace files; Postgres is for platform state and ordering.

- **Vault integration.** Secrets are not wired yet. The platform needs a SecretStore implementation where model credentials, OAuth refresh tokens, provider keys, connector material, and signing keys are stored by reference. Agent runtimes should never receive Vault tokens or provider credentials.

- **LLM Broker provider adapters.** Model calls currently have no real broker. This layer should route runtime model requests through platform policy, quotas, audit, and provider-specific adapters: OpenAI-compatible APIs, Anthropic, Vertex/Gemini, local Ollama/vLLM, or an internal corporate LLM gateway.

- **Integration Broker connectors.** The design calls for typed integration actions, but the connectors are not implemented. Examples: `github.create_pull_request`, `slack.post_message`, `notion.update_page`. Connectors should perform policy checks, request scoped secrets just in time, call provider APIs, and write an audit record.

- **Tenant/team RBAC.** The current auth path is local `admin/admin`. Production needs tenants, teams, roles, ownership, invitations or identity-provider mapping, and permission checks for agent creation, approvals, schedule ownership, integration grants, and admin guardrails.

- **Scheduler execution.** Schedule concepts are architectural only. The platform needs a durable schedule table, lease-based Go workers, timezone handling, retry policy, owner approval for agent-created schedules, and a path that wakes an idle agent before delivering the scheduled task.

- **Egress proxy enforcement.** Generated MVP NetworkPolicies already exist for Kubernetes sandboxes, but the full egress proxy is still missing. This layer should add proxy enforcement, per-agent/tool/grant allowlists, deny logging, and approval prompts when an agent requests new access.

- **Metrics endpoint and production dashboards.** The CLI has a metrics flag, but there is no useful metrics surface yet. The next step is to expose API, gateway, scheduler, broker, runtime lifecycle, secret-use, and egress metrics with labels that work for Prometheus without leaking tenant data.

- **Backups, upgrades, and restore workflows.** The platform needs operator procedures for Postgres PITR, workspace/PVC snapshots, object storage versioning, Vault backup/restore, Helm upgrades, schema migrations, and per-agent restore tests.

That separation is intentional. The foundation should be buildable and reviewable before the security-sensitive runtime, credential, and policy layers are added.
