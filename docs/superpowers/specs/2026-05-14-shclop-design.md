# Shclop Design Specification

Date: 2026-05-14

Project: **shclop** — **self-hosted *Claw orchestration platform**.

## 1. Goal

Shclop is a self-hosted platform where users create isolated OpenClaw agents, chat with them in a browser, persist their workspace and markdown memory, run scheduled tasks, and connect model/integration providers without exposing secrets to agent runtimes.

The design optimizes for:

- production-oriented architecture;
- strong runtime isolation;
- secure secret handling;
- scalable Kubernetes deployment;
- practical single-node evaluation;
- backend/control-plane correctness over UI polish.

## 2. Chosen approach

Shclop uses a **production-oriented modular monolith** for v1.

The backend is one Go binary with clear internal module boundaries:

- API Server;
- Agent Gateway;
- Orchestrator;
- Scheduler;
- LLM Broker;
- Integration Broker;
- SecretStore adapter;
- Storage adapters;
- Auth module;
- dev/mock providers.

This avoids early distributed-system complexity while preserving boundaries that can later become separate services if needed.

Rejected alternatives:

- **Minimal chat platform**: too small; does not address schedules, multi-agent lifecycle, integrations, or sandbox security.
- **Microservices from day one**: premature; adds deployment, TLS, RPC, observability, and failure-mode complexity before the core platform is proven.

## 3. Deployment model

### 3.1 Production deployment

Production deployment target:

```text
Kubernetes
+ Kata Containers RuntimeClass
+ dedicated agent node pool
+ Helm chart
```

Control-plane services run as regular Kubernetes workloads.

Untrusted OpenClaw agent runtimes run only as Kata-backed pods:

```text
1 agent = 1 Kubernetes Pod = 1 Kata microVM
```

Production deployments should use:

- dedicated KVM/Kata node pools for agent workloads;
- taints/tolerations to keep platform/control-plane workloads off agent nodes;
- Pod Security Admission, admission policies, and NetworkPolicy;
- external or in-cluster Postgres, Vault, and S3-compatible storage depending on deployment profile.

### 3.2 Single-node evaluation

Single-node mode is for evaluation and testing, not the primary production claim.

The bootstrap entrypoint is one script with action subcommands. The target is local by default. Remote execution is selected with `--remote user@host`.

```bash
./scripts/bootstrap.sh check
./scripts/bootstrap.sh install
./scripts/bootstrap.sh reset
./scripts/bootstrap.sh destroy

./scripts/bootstrap.sh check --remote user@host
./scripts/bootstrap.sh install --remote user@host
./scripts/bootstrap.sh reset --remote user@host
./scripts/bootstrap.sh destroy --remote user@host
```

Flags modify actions; they do not define the main action:

```bash
--dry-run
--install-deps
--yes
--purge-data
--remove-k3s
--remove-kata
--remote user@host
--values path/to/values.yaml
```

Examples:

```bash
./scripts/bootstrap.sh check
./scripts/bootstrap.sh install --install-deps
./scripts/bootstrap.sh reset --remote root@example.com --install-deps
./scripts/bootstrap.sh destroy --remote root@example.com --purge-data --remove-k3s --remove-kata
```

The script must be conservative by default:

- no destructive action without explicit action and confirmation;
- `destroy` requires typed confirmation unless `--yes` is provided;
- destructive operations remove only resources created by Shclop/bootstrap and selected by Shclop labels/namespaces;
- unknown distributions or existing incompatible Kubernetes/Kata installs fall back to check/manual instructions.

The script installs or verifies, on the target Linux host:

- KVM availability (`/dev/kvm`);
- required host packages;
- K3s;
- Kata Containers;
- `RuntimeClass: kata`;
- cert-manager;
- Postgres;
- Vault;
- MinIO/S3-compatible storage;
- Shclop Helm release;
- a validation Kata pod.

Supported target is Linux with KVM. macOS and Docker Compose are out of scope. A macOS machine may invoke `bootstrap.sh <action> --remote user@host` as an SSH client, but it is not a native runtime host.

### 3.3 Helm dependencies

The Helm chart supports both dependency modes:

1. **Bundled dependencies** for single-node evaluation:
   - Postgres;
   - Vault;
   - MinIO.

2. **External dependencies** for production/multi-node/enterprise:
   - external Postgres;
   - external Vault or compatible SecretStore adapter;
   - external S3-compatible object storage.

## 4. Runtime and component architecture

```text
Browser UI
  ↓ REST + WebSocket
Shclop Go Backend
  ├── API Server
  ├── Agent Gateway
  ├── Orchestrator
  ├── Scheduler
  ├── LLM Broker
  ├── Integration Broker
  ├── SecretStore adapter
  ├── Storage adapters
  └── Auth module
        ↓
Kubernetes/Kata Agent Runtime
```

### 4.1 API Server

REST JSON API for:

- authentication and sessions;
- users;
- agents;
- schedules;
- approvals;
- model providers;
- credential metadata;
- admin settings;
- integration metadata.

### 4.2 Agent Gateway

The Agent Gateway is a transport/control component, not an intelligent agent.

Responsibilities:

- receive browser WebSocket connections;
- receive outbound runtime WebSocket connections;
- route user messages to the correct runtime;
- stream runtime responses/events back to the browser;
- handle reconnect/replay using sequence numbers and persisted messages;
- present approval prompts;
- track minimal live status: queued, starting, running, streaming, done, error.

It does not execute tools and does not hold provider secrets.

For v1, realtime transport is WebSocket with typed JSON envelopes. gRPC/Connect is out of scope.

### 4.3 Orchestrator

The Orchestrator manages agent lifecycle:

```text
Created → Starting → Hot → Idle → Archived → Deleted
```

Responsibilities:

- create/delete Kata runtime pods;
- create/manage per-agent workspace PVCs;
- apply NetworkPolicies;
- assign runtime identity/certificates;
- track leases and runtime state;
- stop idle runtimes;
- restore archived or idle agents by starting a new pod over durable state.

The microVM is disposable. Durable state lives outside it.

### 4.4 Scheduler

Scheduled tasks use a Go platform scheduler with DB leases.

Flow:

```text
schedules table
  → scheduler worker acquires DB lease
  → Orchestrator wakes agent if needed
  → Agent Gateway sends scheduled task
  → Runtime executes task
  → result appears in chat/history
```

Agents may propose schedules, but creation requires a user approval card. Agents cannot silently create background schedules.

## 5. Runtime isolation and hardening

Shclop assumes Agent Runtime compromise, including root inside the guest/container.

Security goal:

```text
root inside Agent Runtime ≠ root on Kubernetes node/host
```

The security boundary is layered:

```text
Kubernetes Restricted Pod
+ Kata microVM isolation
+ no host namespace/device/mount access
+ deny-by-default network
+ no runtime secrets
+ hardened KVM nodes
```

Agent runtime pods must use:

- `runtimeClassName: kata`;
- `privileged: false`;
- `hostNetwork: false`;
- `hostPID: false`;
- `hostIPC: false`;
- no `hostPath`;
- no host device mounts;
- `automountServiceAccountToken: false`;
- `allowPrivilegeEscalation: false`;
- all Linux capabilities dropped;
- read-only root filesystem;
- writable paths limited to workspace, memory, and bounded temporary storage;
- CPU, memory, disk, and PID limits.

Kata provides the VM boundary. Kubernetes security context, Pod Security Admission, admission policies, CNI, and NetworkPolicy enforce workload policy. Prompt instructions are not security controls.

Kata creates a VM per pod sandbox in this design. Containers in one pod may share that Kata VM. Shclop therefore keeps the model simple: one agent runtime per pod.

## 6. Network model

Agent runtime egress is deny-by-default.

Runtime may talk only to:

- Agent Gateway over mTLS;
- controlled egress proxy when access is explicitly allowed;
- LLM Broker and Integration Broker endpoints as required by platform design.

Runtime must not directly access:

- Vault;
- Kubernetes API;
- Postgres;
- MinIO/object storage;
- provider APIs;
- cloud metadata endpoints;
- internal CIDRs;
- arbitrary internet destinations.

External access flow:

```text
Agent requests access/action
  → policy check
  → approval if required
  → scoped allow rule or brokered action
```

DNS and egress must be controlled to prevent direct DNS or network exfiltration. Integrations add explicit allow rules or use brokered connectors.

## 7. TLS and identity

All production control/chat traffic is encrypted.

Required channel model:

```text
Browser --HTTPS/WSS--> Ingress/Edge Proxy
Ingress/Edge Proxy --TLS or mTLS--> Shclop Backend/Agent Gateway
Agent Runtime --mTLS--> Agent Gateway
```

No plain HTTP is allowed for production control/chat traffic inside the cluster.

Runtime identity uses an Identity Provider abstraction.

v1 default:

- cert-manager + internal CA;
- short-lived per-agent/per-pod certificates;
- Gateway validates certificate chain, agent identity, and runtime lease.

Future providers:

- SPIFFE/SPIRE;
- Vault PKI;
- other workload identity systems.

## 8. Secrets and integrations

Rule:

```text
Agent Runtime never receives provider secrets, Vault tokens, LLM keys,
Kubernetes tokens, cloud credentials, or integration tokens.
```

Secrets live in a SecretStore. Vault is the recommended production SecretStore.

Postgres stores metadata and `secret_ref` only.

### 8.1 Integration Broker

Integration flow:

```text
Agent Runtime
  → Integration Broker typed action
  → Broker Facade authorization/policy/audit
  → Provider Connector
  → per-request scoped Vault token
  → provider API
```

The Broker is not a generic HTTP proxy and not a token vending machine for agents.

Forbidden API shape:

```text
fetch(url, secretRef, arbitraryHeaders)
```

Allowed API shape:

```text
github.create_pull_request(...)
slack.post_message(...)
notion.update_page(...)
google.create_calendar_event(...)
```

Provider connectors are stateless pools by provider. Connectors must not have persistent broad Vault permissions. For each provider call, the connector receives or obtains a short-lived, request-scoped, tenant/integration/action-scoped Vault token. Secret paths are derived server-side from verified grants, never supplied by the agent.

High-risk tenants may use dedicated connector pools or Vault namespaces, but unbounded per-tenant connector deployments are not the default.

## 9. LLM/model providers

All model calls go through Shclop LLM Broker.

```text
Agent Runtime
  → LLM Broker
  → provider / local endpoint / corporate gateway
```

Agent Runtime never receives provider or gateway credentials.

Supported provider classes:

- OpenAI;
- Anthropic;
- Gemini;
- OpenRouter;
- OpenAI-compatible endpoints;
- Ollama;
- vLLM;
- LiteLLM;
- corporate LLM gateways;
- custom adapters.

### 9.1 Credentials and ownership

Credential ownership supports both:

- admin-managed providers/gateways;
- optional user BYO credentials if admin policy allows it.

User-provided secrets are submitted only through Shclop UI/API over HTTPS and written immediately to SecretStore. They are never shown again.

Postgres stores only metadata:

```text
model_credentials:
  id
  owner_user_id / tenant_id
  provider_type
  display_name
  secret_ref
  scopes / allowed_models metadata
  created_at
  last_used_at
```

Vault path pattern:

```text
secret/data/tenants/{tenant_id}/users/{user_id}/model-providers/{credential_id}
```

### 9.2 Model catalog

Shclop does not require mandatory per-model admin approval by default.

Model catalog is derived from credentials/gateways available to the user:

```text
approved provider/gateway credential
  → discovery
  → user-visible model catalog
```

Provider discovery answers what exists, not what is inherently trusted. Policy still applies at provider/credential/gateway level and may optionally deny specific models.

Controls focus on:

- provider/gateway trust;
- credential ownership;
- budgets;
- rate limits;
- audit;
- optional deny/allow policies;
- SSRF prevention for custom base URLs.

For v1, model selection is user-level only. Users can choose personal defaults:

- default model;
- cheap model;
- strong/reasoning model;
- coding model.

Agents inherit user model settings. Per-agent/per-task routing is out of scope for v1.

## 10. Memory and storage

Agent memory is markdown/wiki-first on the persistent workspace.

Suggested layout:

```text
/workspace/*
/memory/profile.md
/memory/facts.md
/memory/preferences.md
/memory/decisions.md
/memory/tasks.md
/memory/summaries/*.md
```

Durable storage:

- workspace and markdown memory: Kubernetes PVC;
- artifacts/archive/backups: S3-compatible storage, e.g. MinIO;
- platform state: Postgres;
- secrets: Vault or SecretStore adapter.

Postgres is not treated as agent memory. It stores platform state:

- users;
- agents;
- sessions;
- messages;
- approvals;
- schedules;
- minimal tool/action ledger;
- model credential metadata;
- lifecycle state.

Vector search is out of scope for v1. If added later, it indexes markdown/files while source of truth remains files and platform state.

## 11. Tool and package execution

Runtime image is immutable.

Allowed:

- user-space dependencies installed into workspace;
- Python virtualenvs under `/workspace/.venv`;
- npm dependencies under `/workspace/node_modules`;
- user-local binaries under `/workspace/.local/bin`;
- downloads through controlled egress;
- audited package installs;
- quotas and timeouts.

Forbidden:

- mutating the base runtime image;
- installing system packages into the runtime layer;
- privileged daemons;
- Docker socket access;
- VPNs or network-stack changes;
- kernel modules;
- host devices;
- direct provider/API/Vault/Kubernetes access.

Risky actions such as downloading unknown binaries, `curl | sh`, or executable modifications from untrusted sources require approval or are blocked by policy.

## 12. Auth and authorization

v1 auth:

- local username/password;
- secure sessions;
- owner-only approvals for user-owned agents.

Future-ready model:

- `AuthProvider` interface;
- OIDC later;
- LDAP later.

OIDC is moderate complexity if identity/session abstractions are clean. LDAP is more complex because of bind/search/group mapping and sync behavior.

RBAC/team hierarchy is future-ready but not a v1 priority. Minimal future roles:

- owner;
- admin;
- member;
- viewer.

For v1, the user who creates an agent owns and approves actions for that agent, bounded by platform guardrails.

## 13. UI

Frontend stack:

```text
React + Vite + TypeScript
```

No SSR.

Primary screens:

- login;
- agent list;
- agent creation wizard;
- chat;
- approval cards;
- schedules;
- model providers/settings;
- integrations stub;
- admin/system settings.

Agent creation wizard fields:

- name;
- template/role;
- model defaults;
- resource profile;
- first task.

Advanced tab:

- enabled tools;
- egress policy;
- schedule permission;
- max runtime/idle timeout;
- workspace size;
- secret/integration scopes;
- autonomy mode.

Autonomy modes:

- Conservative;
- Balanced default;
- Autonomous within configured scopes and hard platform guardrails.

## 14. API contract

v1 uses REST JSON and WebSocket only.

REST is used for CRUD/config:

- auth/session;
- users;
- agents;
- schedules;
- model providers;
- credential metadata;
- approvals;
- admin settings.

WebSocket with typed JSON envelopes is used for realtime:

- chat streaming;
- runtime events;
- tool progress;
- approval prompts;
- schedule execution status.

gRPC/Connect is out of scope for v1. Internal boundaries should be Go interfaces so internal RPC can be introduced later without changing core concepts.

## 15. Dev/mock mode

The same Go backend binary supports dev/mock mode.

Example:

```bash
shclop server --dev --mock-runtime --mock-llm --mock-secrets --store inmemory
```

Mock providers:

- in-memory store;
- mock SecretStore;
- mock SandboxProvider;
- mock Agent Runtime;
- mock LLM Broker.

Purpose:

- UI development;
- API contract testing;
- chat streaming simulation;
- approval UI;
- schedules UI;
- runtime lifecycle states.

Mock mode is not a deployment mode and provides no sandbox/security guarantees.

## 16. Observability

Shclop exposes:

- structured logs;
- configurable log level;
- metrics endpoint;
- metrics enable/disable flag.

Collection, storage, dashboards, and alerting stacks are out of scope.

## 17. Out of scope for v1

- Docker Compose;
- macOS native runtime;
- gRPC/Connect;
- full backup/upgrade automation;
- billing;
- complete RBAC/team hierarchy;
- production-grade bundled observability stack;
- VM RAM snapshots;
- direct runtime secrets;
- arbitrary integration HTTP proxy;
- per-agent/per-task model router;
- vector database;
- full integration catalog implementation.

## 18. Repository layout

Recommended layout:

```text
cmd/shclop/
internal/api/
internal/auth/
internal/gateway/
internal/orchestrator/
internal/scheduler/
internal/llmbroker/
internal/integrations/
internal/secrets/
internal/storage/
internal/runtime/
internal/models/
internal/approvals/
internal/config/
web/
charts/shclop/
scripts/bootstrap.sh
docs/
```

## 19. Spec self-review

- No placeholders remain.
- Scope is focused on v1 architecture and deployment design.
- Docker Compose and macOS-native runtime are explicitly out of scope.
- Bootstrap CLI uses consistent action subcommands with local execution by default and `--remote user@host` for SSH execution.
- Security model consistently assumes runtime compromise and forbids runtime secrets.
- Model provider policy allows users to use discovered models through approved credentials/gateways without mandatory per-model approval.
