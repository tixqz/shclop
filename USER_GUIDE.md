# Shclop user guide

This guide describes the member flow implemented for the current demo. Members manage their own agents and sessions; they do not manage the platform environment.

## Demo account

Run Shclop with the mock YAML identity provider:

```bash
go run ./cmd/shclop \
  --dev \
  --store inmemory \
  --identity-provider mock-yaml \
  --identity-mock-yaml config/identity.mock.yaml
```

Member login:

```text
bob@acme.test / bob
```

Bob has role `member` in tenant `acme` and team `engineering`.

## Member dashboard

After login, Bob lands in the member dashboard. It shows:

- profile summary: user, tenant, teams, roles;
- “My agents”;
- runtime controls for the selected agent;
- chat task box;
- current session events;
- user activity log.

## Agent flow

1. Log in as Bob.
2. Create an agent from “My agents”.
3. Select the agent.
4. Pick a runtime flavor: `openclaw`, `nanoclaw`, or `nemoclaw`.
5. Start the selected agent.
6. Send a chat task.
7. Watch runtime events stream back into the current session view.

When the backend runs with `--sandbox-provider docker-demo`, starting the agent launches a local runtime container through Docker. With the default mock sandbox provider, the platform issues a runtime lease but does not start a real container.

## Activity log

The activity log shows Bob’s own actions, including:

- login;
- agent creation;
- agent start request;
- sandbox start result;
- routed browser task.

Admins can see system-wide activity. Members see only their own activity.

## Current limitations

- Agent delete/archive is not implemented yet.
- Previous session history is represented by current runtime events and in-memory activity only.
- Activity is not durable yet.
- Runtime output is demo structured events; real OpenClaw/NanoClaw/NemoClaw task execution is not wired yet.
