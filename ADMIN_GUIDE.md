# Shclop admin guide

This guide describes the read-only admin area implemented for the current demo. Admins are separate from normal members: they inspect the platform environment and guardrails, while members manage their own agents.

## Demo account

Run Shclop with the mock YAML identity provider:

```bash
go run ./cmd/shclop \
  --dev \
  --store inmemory \
  --identity-provider mock-yaml \
  --identity-mock-yaml config/identity.mock.yaml
```

Admin login:

```text
alice@acme.test / alice
```

Alice has role `admin` in tenant `acme` and team `platform`.

## What the admin area shows now

The current admin area is read-only. It is meant to document and demo the control-plane boundary before editable admin operations are added.

It shows:

- active identity provider;
- configured sandbox provider;
- runtime image catalog for OpenClaw, NanoClaw, and NemoClaw;
- users, tenants, teams, roles, and groups from `config/identity.mock.yaml`;
- application activity log: login, agent creation, agent start, sandbox start, runtime connection, and routed task events.

## What admins are allowed to manage later

The admin role should eventually manage platform-level guardrails, not individual user work unless required for support or incident response.

Planned admin controls:

- identity providers: local, mock YAML, OIDC, LDAP, header auth;
- tenant and team mapping rules;
- sandbox providers: mock, local Docker demo, Kubernetes/Kata;
- runtime image catalog and enabled runtime flavors;
- model provider configuration through the LLM Broker;
- global resource limits: max agents per user, runtime CPU/RAM/disk profiles, workspace size;
- network and egress policies;
- integration connector availability;
- audit retention, backup policy, and operational health.

## What admins should not do

Admins should not bypass runtime isolation, inject provider secrets into agent runtimes, or turn the Integration Broker into a generic HTTP proxy. Admin controls should set policy and guardrails; agents should still operate through typed, auditable interfaces.

## Current limitations

- Settings are not editable from the admin area yet.
- Users and roles come from mock YAML for demo purposes.
- Activity is in-memory and disappears when the backend restarts.
- There is no durable audit schema yet.
