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
