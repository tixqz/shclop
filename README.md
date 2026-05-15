# shclop

Shclop is an open-source platform for running organization-controlled AI agents.

The point is not to make another chat UI. The point is to give teams a place where agent access to models, tools, secrets, browsers, shells, schedules, and networks can be configured once, audited, and operated without every team inventing its own unsafe wrapper.

## Motivation

Useful agents stop being harmless as soon as they can do real work.

They need model credentials, browser access, repository access, files, schedulers, webhooks, shell commands, and sometimes package installs. If every user connects these pieces by hand, the organization loses the things it normally expects from infrastructure: reviewable configuration, isolation, revocation, logs, and a way to answer “who allowed this action?” after an incident.

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
  --set image.tag=0.1.0
```

Check the rendered manifests before applying them:

```bash
helm template shclop charts/shclop \
  --set image.repository=registry.example.com/shclop/shclop \
  --set image.tag=0.1.0
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

Local build without Docker:

```bash
make build
```

Full local verification:

```bash
make verify
```

Useful targets:

```bash
make test
make web-build
make helm-template
make bootstrap-check
make clean
```

The backend serves the UI from `web/dist` by default. Override it when needed:

```bash
shclop --static-dir=/path/to/dist
```

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

## Development

Run the backend in dev/mock mode:

```bash
go run ./cmd/shclop --dev --mock-runtime --mock-llm --mock-secrets --store inmemory
```

Run the frontend against the backend during UI work:

```bash
cd web
npm install
npm run dev
```

Run Go tests directly:

```bash
go test ./...
```

## Status

This repository currently contains the foundation slice:

- Go backend entrypoint, config, logging, REST API, local auth, in-memory store, and mock WebSocket runtime.
- React/Vite/TypeScript UI served separately in dev or embedded in the built container image.
- Dockerfile for a single backend+UI image.
- Helm chart skeleton for the backend service.
- Bootstrap script skeleton with local default and explicit `--remote user@host` execution.
- Design and implementation plan documents under `docs/superpowers/`.

Not implemented yet:

- real Kubernetes sandbox provider;
- real OpenClaw runtime image;
- Postgres persistence;
- Vault integration;
- LLM Broker provider adapters;
- Integration Broker connectors;
- tenant/team RBAC;
- real scheduler execution;
- egress proxy enforcement;
- metrics endpoint and production dashboards;
- backups, upgrades, and restore workflows.

That separation is intentional. The foundation should be buildable and reviewable before the security-sensitive runtime, credential, and policy layers are added.
