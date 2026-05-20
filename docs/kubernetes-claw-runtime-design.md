# Kubernetes Sandbox and Claw Runtime Execution Design

## Purpose

This design turns the current demo/runtime foundation into the first real Kubernetes-based execution path:

- start agent runtimes as Kubernetes/Kata sandboxes;
- attach runtime credentials through a secret-store abstraction;
- provision per-agent workspace storage;
- enforce baseline NetworkPolicy isolation;
- execute real NanoClaw/OpenClaw work through a runtime-side adapter boundary;
- document that this is an embedded MVP, not the final standalone production controller.

## Current State

The repository already has:

- backend REST and WebSocket wiring for browser chat and runtime registration;
- in-memory runtime registry and `task.run` routing;
- demo runtime process that connects to `/runtime/ws` and streams demo events;
- Docker demo sandbox provider for local demos;
- hardened Kubernetes/Kata pod spec builder foundation;
- embedded Kubernetes provider MVP for sandbox lifecycle;
- Kubernetes Secret store MVP fallback;
- NetworkPolicy generation with broad backend/Vault allow rules unless custom CIDRs are configured;
- runtime image skeletons for NanoClaw and OpenClaw.

Remaining work includes Kubernetes pod status/watch/log collection, production Vault-backed secret delivery, full egress proxy enforcement, standalone controller extraction, and real NanoClaw/OpenClaw invocation beyond the adapter/subprocess boundary.

## Architecture Decision

Use option 3: **embedded Kubernetes controller MVP with extraction-ready interfaces**.

For the first implementation, the backend creates and deletes Kubernetes resources directly. The code must still be structured as if the controller could later move into a separate worker/controller:

- public API stays backend-owned;
- sandbox lifecycle goes through `SandboxProvider`;
- runtime credentials go through `RuntimeSecretStore`;
- Kubernetes resource building, applying, status mapping, and cleanup stay in small isolated components;
- Kubernetes resources carry stable labels so a future standalone reconciler can adopt them.

This avoids the full complexity of a distributed reconciliation system while preserving a migration path to one.

## Runtime Start Flow

1. `POST /api/agents/{id}/start` generates a runtime token and sandbox spec.
2. Backend calls the Kubernetes `SandboxProvider`.
3. Provider asks `RuntimeSecretStore` to store the token and returns a `SecretRef`.
4. Provider creates or updates sandbox resources:
   - workspace PVC;
   - NetworkPolicy;
   - runtime Pod using the configured Kata `runtimeClassName`;
   - secret delivery wiring based on `SecretRef`.
5. Runtime Pod starts `shclop-runtime`.
6. `shclop-runtime` reads its credential, connects to backend `/runtime/ws`, sends `runtime.hello`, and is registered by the backend registry.
7. Agent becomes ready for tasks only after runtime WebSocket registration succeeds.

Pod `Running` is diagnostic state. Runtime WebSocket registration is the source of truth for task readiness.

## Kubernetes Sandbox Resources

The MVP lifecycle includes:

- Pod;
- runtime credential reference;
- NetworkPolicy;
- PVC-backed workspace;
- cleanup for all created resources.

The Pod must use the existing hardened security posture:

- configured Kata RuntimeClass;
- no host network, PID, IPC, hostPath, privileged mode, host devices, or Kubernetes service account token;
- read-only root filesystem where practical;
- dropped Linux capabilities;
- workspace mounted at `/workspace`;
- runtime env such as `SHCLOP_GATEWAY_URL`, `SHCLOP_AGENT_ID`, and `SHCLOP_AGENT_FLAVOR`.

All resources should be labelled with at least:

- `app.kubernetes.io/name=shclop`;
- `shclop.io/agent-id`;
- `shclop.io/runtime-flavor`;
- `shclop.io/sandbox-id`.

These labels support cleanup, diagnostics, and future standalone reconciliation.

## Runtime Credentials and Vault

The backend must not hard-code Kubernetes Secret as the production source of truth for runtime tokens. Introduce a credential abstraction:

```go
type RuntimeSecretStore interface {
    PutRuntimeToken(agentID, token string, ttl time.Duration) (SecretRef, error)
    DeleteRuntimeToken(agentID string) error
}
```

`SecretRef` describes how the Pod should receive the credential without forcing the Kubernetes provider to keep handling raw token values.

Initial implementations:

1. **KubernetesSecretStore**
   - development and single-node MVP fallback;
   - creates a Kubernetes Secret;
   - mounts the token as a file or env var.

2. **VaultSecretStore**
   - production-oriented path;
   - backend writes the token to Vault, for example under `secret/shclop/runtimes/{agent_id}/token`;
   - Pod receives the token through Vault Agent Injector, CSI Secret Store Driver, or External Secrets Operator;
   - runtime reads `SHCLOP_RUNTIME_TOKEN_FILE` first and falls back to `SHCLOP_RUNTIME_TOKEN`.

Documentation must state that Kubernetes Secret mode is an MVP/dev fallback, while Vault-backed delivery is the production recommendation. Future standalone controllers should operate on `SecretRef`, not raw token values.

## NetworkPolicy

The MVP should create a deny-by-default NetworkPolicy for runtime Pods and allow only the endpoints needed for first operation:

- backend gateway/WebSocket endpoint;
- secret delivery path if required by the chosen SecretStore integration;
- explicit future egress/proxy exceptions.

This MVP does not implement the full egress proxy. It lays down the per-runtime policy generator boundary and should document remaining production work: proxy enforcement, per-agent allowlists, private-range blocking, deny logs, and approval flows.

## NetworkPolicy Configuration Interface

Operators should not configure runtime egress by writing raw per-agent NetworkPolicy YAML. Shclop should expose a small sandbox configuration surface and generate Kubernetes NetworkPolicies from it.

Example Helm/config shape:

```yaml
sandbox:
  kubernetes:
    networkPolicy:
      enabled: true
      mode: restricted # disabled | restricted | custom
      allowedCIDRs: "10.20.30.40/32,10.20.30.41/32"
```

Supported modes:

- `disabled`: do not create NetworkPolicy; development/debug only.
- `restricted`: default mode; creates the MVP NetworkPolicy with broad backend/Vault allow rules and no custom egress CIDRs. Stronger deny-by-default controls for Kubernetes API, cloud metadata endpoints, Postgres, and private ranges belong to the future egress proxy/policy layer.
- `custom`: future extension for explicit operator-defined egress allow rules for corporate proxies, registries, or approved internal services.

Internal config shape:

```go
type NetworkPolicySpec struct {
    Enabled       bool
    Mode          NetworkPolicyMode
    AllowBackend  bool
    AllowVault    bool
    AllowedEgress []EgressRule
}
```

The Kubernetes provider should delegate policy generation to a focused builder:

```go
BuildRuntimeNetworkPolicy(agentID, sandboxID string, spec NetworkPolicySpec) (*networkingv1.NetworkPolicy, error)
```

MVP configuration is operator/admin-owned through Helm values or backend config and can apply globally or by runtime flavor. Agent-requested egress, approval prompts, dynamic policy updates, and richer per-egress objects belong to the later egress proxy/policy system, not this MVP.

## Workspace PVC

Each sandbox gets a workspace PVC mounted at `/workspace`.

PVC lifecycle must be configurable:

- `delete` for ephemeral agents and tests;
- `retain` for persistent workspaces.

The default for early development can be `delete`; production configuration should make workspace retention explicit.

## Stop and Cleanup

`SandboxProvider.Stop(ctx, agentID)` deletes or releases resources in a deterministic order:

1. runtime Pod;
2. NetworkPolicy;
3. runtime credential through `RuntimeSecretStore.DeleteRuntimeToken`;
4. PVC according to the configured workspace retention policy.

Cleanup should be idempotent. Missing resources are not fatal. Failures must be surfaced in logs/status and should leave enough labels for manual cleanup.

## Claw Execution Contract

The term “API” here means the **execution contract between `shclop-runtime` and the selected Claw runtime**, not a public HTTP API.

When backend sends a `task.run` envelope to `shclop-runtime`, the runtime needs a defined way to pass that task into NanoClaw/OpenClaw and stream results back. That contract may be a CLI command, stdio protocol, local socket, or subprocess fallback.

Public documentation found so far does not prove that NanoClaw and OpenClaw are fully CLI/API-compatible for runtime execution. NanoClaw is described as a lightweight OpenClaw alternative with similar core functionality, and it includes migration tooling from OpenClaw state, but that does not establish a shared task execution protocol.

Therefore shclop should not assume compatibility as a fact. It should verify capabilities at runtime and hide the details behind an adapter interface.

## Claw Adapter Layer

Add a runtime-side adapter boundary:

```go
type ClawAdapter interface {
    Run(ctx context.Context, task Task) (<-chan ClawEvent, error)
}
```

Preferred implementation:

- `ClawCompatibleAdapter`, parameterized by binary path/image/flavor, used when NanoClaw/OpenClaw expose the same verified execution contract.

Fallback structure:

- if capability checks show different contracts, add flavor-specific adapters behind the same `ClawAdapter` interface;
- if no structured contract is available, use a subprocess fallback that maps stdout/stderr/exit code into shclop events.

Runtime startup should perform a capability/protocol check. If the selected binary does not support the expected contract, runtime should fail clearly and emit a `message.error`/diagnostic event rather than silently switching behavior.

## Runtime Task Flow

`shclop-runtime` task handling changes from demo events to real adapter execution:

1. receive `task.run` from backend;
2. normalize payload into internal `Task`;
3. select/configure `ClawAdapter` from `SHCLOP_AGENT_FLAVOR` and runtime config;
4. call `adapter.Run(ctx, task)`;
5. convert `ClawEvent` stream into existing shclop WebSocket envelope types:
   - `message.started`;
   - `message.delta`;
   - `message.done`;
   - `message.error`.

The adapter interface keeps runtime tests independent from real NanoClaw/OpenClaw binaries.

## Error Handling

Sandbox start errors should map to actionable agent states/logs:

- invalid RuntimeClass;
- PVC create/bind failure;
- image pull failure;
- Pod scheduling failure;
- SecretStore/Vault delivery failure;
- runtime WebSocket auth failure;
- runtime startup timeout.

Execution errors should preserve:

- task ID;
- flavor;
- adapter/capability failure;
- process exit code or structured error code;
- timeout/cancellation reason.

## Testing Strategy

Unit tests:

- Kubernetes resource builders produce hardened Pod/PVC/NetworkPolicy specs;
- `RuntimeSecretStore` implementations return expected `SecretRef` shapes;
- cleanup is idempotent;
- Claw adapter mapping converts stdout/stderr/exit codes or structured events into shclop events.

Integration-style tests:

- fake Kubernetes client verifies start/stop resource lifecycle;
- fake runtime WebSocket registration remains the readiness signal;
- fake Claw binary/protocol verifies real task flow without upstream dependencies.

Existing demo flow tests should continue to pass.

## Documentation Updates Required

Update project documentation to make these boundaries explicit:

- Kubernetes sandbox controller becomes an embedded MVP first, not a standalone production controller;
- production extraction requires persistent desired state, reconciliation loop, finalizers/retries, standalone RBAC/deployment, and durable status handling;
- runtime token delivery goes through `RuntimeSecretStore` and `SecretRef`;
- Kubernetes Secret mode is development/MVP fallback;
- Vault-backed secret delivery is the production recommendation;
- real Claw execution uses a `ClawAdapter` boundary because NanoClaw/OpenClaw runtime-execution compatibility is not confirmed by current public docs;
- subprocess fallback is a compatibility bridge, not the desired long-term protocol.

## Out of Scope for This MVP

- standalone Kubernetes controller deployment;
- durable desired-state database and full reconciliation loop;
- finalizers and advanced orphan recovery;
- full egress proxy implementation;
- full Postgres platform schema;
- complete Vault operator setup;
- signed/pinned reproducible runtime images;
- proving NanoClaw/OpenClaw protocol compatibility beyond runtime capability checks.
