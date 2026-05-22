# Kubernetes OpenClaw/NanoClaw Runtime Design

## Purpose

This document describes the current Kubernetes runtime path for self-hosted Shclop production installs.

The backend is the production control plane. It creates hardened Kubernetes resources for each OpenClaw/NanoClaw agent start and uses the runtime WebSocket as the readiness path back to the UI.

## Scope

Current runtime scope:

- Kubernetes sandbox provider selected with `sandbox.provider=kubernetes`.
- Kata RuntimeClass configured through `agentRuntime.runtimeClassName` / `--agent-runtime-class`.
- Per-agent runtime Pod.
- Per-agent workspace PVC.
- Per-agent runtime token Secret.
- Per-agent NetworkPolicy.
- OpenClaw and NanoClaw runtime images.
- LLM gateway base URL, model, and API key SecretKeyRef passed to runtime pods.

Out of current scope:

- standalone Kubernetes controller deployment;
- workspace/team abstractions;
- skills/catalogs;
- MCP and third-party integration management;
- policy approval workflows;
- built-in LLM proxy;
- LDAP/OIDC/header auth/SCIM.

## Runtime start flow

1. User creates an agent with runtime `openclaw` or `nanoclaw` and an enabled model.
2. User starts the agent.
3. Backend verifies that the model is still enabled.
4. Backend loads LLM gateway settings.
5. Backend rejects the start if the agent has a model but the gateway is not enabled or has no base URL.
6. Backend generates a runtime token.
7. Kubernetes provider creates runtime resources:
   - workspace PVC;
   - runtime token Secret;
   - NetworkPolicy;
   - hardened runtime Pod using the configured Kata RuntimeClass.
8. Runtime pod starts `shclop-runtime`.
9. Runtime connects to `/runtime/ws` with its agent ID and token.
10. Browser chat sends tasks through the backend to the registered runtime.

Pod `Running` is useful diagnostic state. Runtime WebSocket registration is the application readiness signal for chat routing.

## Runtime pod security posture

Runtime pods should keep the hardened baseline:

- Kata RuntimeClass, for example `runtimeClassName: kata`;
- no host network, host PID, or host IPC;
- no privileged mode;
- no hostPath mounts;
- no host devices;
- no mounted Kubernetes service account token;
- dropped Linux capabilities;
- read-only root filesystem where practical;
- workspace mounted from the per-agent PVC.

Docker/container images are packaging. Kata provides the production isolation boundary expected for agent workloads.

## Runtime configuration

The backend passes runtime configuration as pod environment and Secret references, including:

- `SHCLOP_GATEWAY_URL` for `/runtime/ws`;
- `SHCLOP_AGENT_ID`;
- `SHCLOP_AGENT_FLAVOR` (`openclaw` or `nanoclaw`);
- runtime token reference;
- LLM gateway base URL;
- selected LLM model;
- LLM gateway API key SecretKeyRef.

The backend stores only LLM gateway metadata: base URL, Secret name, and Secret key. Shclop does not proxy LLM calls.

## Workspace PVC

Each runtime gets a workspace PVC mounted into the runtime pod.

Important settings:

```yaml
sandbox:
  kubernetes:
    workspace:
      size: 10Gi
      storageClassName: ""
      retention: delete # delete | retain
```

Use `retain` when workspaces must survive runtime cleanup. Use `delete` for development and disposable agents.

## NetworkPolicy

The provider creates NetworkPolicy resources for runtime pods when enabled:

```yaml
sandbox:
  kubernetes:
    networkPolicy:
      enabled: true
      allowedCIDRs: "10.0.0.0/8"
```

The current policy layer is intentionally simple. It is not a full egress proxy or approval system. Operators should keep allowed CIDRs narrow and route model access through the configured LLM gateway.

## PostgreSQL and runtime state

Production requires PostgreSQL for platform state. The Helm chart deploys bundled single-node PostgreSQL by default and can instead read an existing DSN Secret.

Runtime workspace data belongs on the per-agent PVC. PostgreSQL stores platform state such as users, agents, model configuration, gateway settings, and activity.

## Observability

The backend exposes:

- JSON stdout/stderr logs;
- `GET /healthz`;
- `GET /readyz`;
- `GET /metrics` in Prometheus format.

Helm values document an external observability stack:

- VictoriaMetrics k8s-stack;
- VictoriaLogs;
- Grafana;
- 7-day retention default.

## Failure cases to surface

Runtime start failures should be visible in logs, metrics, and agent state where possible:

- invalid or missing RuntimeClass;
- runtime namespace missing;
- insufficient RBAC for Pod/PVC/Secret/NetworkPolicy creation;
- PVC create or bind failure;
- image pull failure;
- pod scheduling failure;
- runtime token Secret failure;
- NetworkPolicy creation failure;
- LLM gateway not configured;
- selected model disabled;
- runtime WebSocket authentication failure.

## Design direction

The embedded Kubernetes provider is the current production path. A future standalone controller could adopt the same resource labels and lifecycle concepts, but it is not part of the current product scope.
