# Shclop user guide

This guide describes the workspace user flow implemented for the current demo. Workspace users manage their own agents and sessions; they do not manage the platform environment.

## Demo account

Run Shclop with the mock YAML identity provider:

```bash
go run ./cmd/shclop \
  --dev \
  --store inmemory \
  --identity-provider mock-yaml \
  --identity-mock-yaml config/identity.mock.yaml
```

Workspace user login:

```text
bob@acme.test / bob
```

Bob has the workspace user role in tenant `acme` and team `engineering`.

## Home

After login, Bob lands on Home. Home is an event-first feed, not a workspace directory or runtime debug panel. It shows:

- a compact Continue strip for the last workspace and recent/next chat;
- a compact Workspace progress section with per-workspace task bars;
- a pinned Needs attention subset for drafts, pending integrations, and policy blocks;
- a recent meaningful event timeline with workspace, subject, action, severity, and time;
- filtered backend activity when it adds user-facing context.

Workspace details, counts, and browsing belong in the Workspaces tab.

Top-level tabs now include global **Agents** and **Skills** catalogs alongside Home, Workspaces, and the admin-only Admin area. Agents and Skills are user-wide UI catalogs in this prototype; they are not workspace-scoped by default.

Both catalogs are list-first: the Add button opens the create modal, and clicking a row opens a detail modal with edit/delete actions.

The top-right user menu shows the signed-in user, avatar initials, organization metadata (`tenant_id` and `team_ids`), and roles from the identity provider. Settings is visible but disabled for now; Log out clears the current UI session.

## Agent flow

1. Log in as Bob.
2. Open Home to continue recent workspace work or jump into Workspaces.
3. Open a workspace.
4. Create or select a workspace chat.
5. Choose the primary agent, allowed workspace context, and safety preset for the chat.
6. Use workspace activity and backend signals to understand what happened next.

## Global catalogs

- **Agents** holds reusable global agents with name, model, tags, purpose, state, and linked workspaces.
- **Skills** holds reusable instruction packs created in the UI or imported from an external URL.
- Workspace assignment for global agents is intentionally not exposed in this prototype UI yet.
- Skills are cataloged now and can be attached to agents later.

When the backend runs with `--sandbox-provider docker-demo`, starting the agent launches a local runtime container through Docker. With the default mock sandbox provider, the platform issues a runtime lease but does not start a real container.

## Activity log

The activity log shows Bob’s own actions, including:

- login;
- agent creation;
- agent start request;
- sandbox start result;
- routed browser task.

Admins can see system-wide activity. Workspace users see only their own activity.

## Current limitations

- Agent and skill changes are UI-only in this prototype; durable backend persistence is not implemented yet.
- Previous session history is represented by workspace chat examples and in-memory activity only.
- Activity is not durable yet.
- Runtime output is demo structured events; real OpenClaw/NanoClaw/NemoClaw task execution is not wired yet.
