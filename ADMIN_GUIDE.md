# Shclop admin guide

This guide describes the current admin UI/API for a self-hosted Shclop installation. Admins manage platform access and runtime configuration; they do not manage workspaces, skills, MCP servers, third-party integrations, or approval policy workflows in the current product scope.

## Sign in

Shclop uses local authentication only.

On first startup, the backend bootstraps one admin user from configuration:

- username: `bootstrapAdmin.username` / `SHCLOP_BOOTSTRAP_ADMIN_USERNAME`;
- password: `bootstrapAdmin.existingSecret` in Helm, or `SHCLOP_BOOTSTRAP_ADMIN_PASSWORD` outside Helm.

After signing in, use the admin area to create named user accounts and disable the bootstrap password workflow operationally by keeping the Secret protected and rotating it when needed.

## Users

Admins can:

- list local users;
- create users;
- assign `admin` or `user` role;
- disable or re-enable users.

Disabled users cannot log in. Existing runtime pods should still be stopped through normal agent controls if access is revoked during an active session.

API shape:

```http
GET /api/admin/users
POST /api/admin/users
PATCH /api/admin/users/{user_id}
```

Example create request:

```json
{
  "username": "ada",
  "password": "replace-with-temporary-password",
  "role": "user"
}
```

Example disable request:

```json
{
  "disabled": true
}
```

## LLM models

Admins decide which model identifiers users can select for agents.

Each model has:

- display name shown in the UI;
- provider model identifier passed to runtime pods;
- enabled/disabled state.

Only enabled models can be used when creating or starting an agent. If a model is disabled after an agent is created, the next start attempt is rejected.

API shape:

```http
GET /api/admin/models
POST /api/admin/models
PATCH /api/admin/models/{model_id}
```

Example:

```json
{
  "display_name": "Gateway GPT-4.1 Mini",
  "provider_model": "gpt-4.1-mini",
  "enabled": true
}
```

## LLM gateway

Shclop does not proxy model traffic. Admins configure metadata that lets runtime pods call an operator-provided gateway:

- `base_url`: gateway URL;
- `secret_name`: Kubernetes Secret containing the API key;
- `secret_key`: Secret key containing the API key;
- `enabled`: whether the gateway is usable.

The backend stores the base URL and Secret reference metadata. It passes those values to Kubernetes runtime pods when agents start.

API shape:

```http
GET /api/admin/llm-gateway
PATCH /api/admin/llm-gateway
```

Example:

```json
{
  "enabled": true,
  "base_url": "https://llm-gateway.example.com/v1",
  "secret_name": "shclop-llm-gateway",
  "secret_key": "api-key"
}
```

## Runtime status

The admin overview shows the configured runtime provider and runtime pod settings, including:

- sandbox provider, expected to be `kubernetes` in production;
- Kubernetes namespace for runtime pods;
- Kata `runtimeClassName`;
- OpenClaw and NanoClaw runtime images.

Production runtime pods are created by the backend through the Kubernetes provider. For each start, Shclop creates hardened runtime resources such as a Pod, PVC, Secret, and NetworkPolicy.

## Observability status

The admin overview shows:

- metrics enabled/disabled;
- logging enabled status;
- configured Grafana URL;
- `/healthz` and `/readyz` status.

Operational endpoints:

- `GET /healthz` for liveness;
- `GET /readyz` for store-backed readiness;
- `GET /metrics` for Prometheus-compatible metrics.

Logs are JSON on stdout/stderr. The Helm values document VictoriaMetrics k8s-stack, VictoriaLogs, and Grafana as the recommended monitoring/logging stack, with 7-day retention defaults.

## Current limitations

The current admin path does not include:

- LDAP, OIDC, header auth, or SCIM;
- workspaces or tenant/team mapping;
- skill/catalog management;
- MCP server management;
- third-party integration connectors;
- security policy approvals;
- built-in LLM proxying.
