# Shclop development

This file keeps development and test workflow out of the README. The README should stay focused on what Shclop is, how it is deployed, and how to run the functional demo.

## Local backend

Run the backend with the local in-memory store:

```bash
go run ./cmd/shclop --dev --store inmemory
```

This local mode only enables the fallback `admin/admin` account. Use the mock YAML identity adapter below when testing the Bob/Alice demo flows.

The backend serves the compiled UI from `web/dist` by default. Build the UI first when using the backend as the frontend server:

```bash
cd web
npm install
npm run build
cd ..
go run ./cmd/shclop --dev --store inmemory
```

Override the static directory when needed:

```bash
go run ./cmd/shclop --dev --store inmemory --static-dir=/path/to/dist
```

## UI development server

Run Vite against the Go backend:

```bash
go run ./cmd/shclop --dev --store inmemory
```

In another terminal:

```bash
cd web
npm install
npm run dev
```

`web/vite.config.js` proxies `/api`, `/ws`, and `/runtime/ws` to `localhost:8080`.

## Mock YAML identity adapter

Run the backend with the mock identity provider:

```bash
go run ./cmd/shclop \
  --dev \
  --store inmemory \
  --identity-provider mock-yaml \
  --identity-mock-yaml config/identity.mock.yaml
```

Demo users:

- `alice@acme.test` / `alice`
- `bob@acme.test` / `bob`
- `eve@other.test` / `eve`

The adapter maps YAML fields into the Shclop identity model: subject, email, display name, tenant, teams, roles, and groups. OIDC, LDAP, and header-auth adapters should return the same identity shape and reuse the same organization mapping path.

## Local Docker demo runtime

Build the runtime images:

```bash
make runtime-images
```

Run the backend with the Docker demo sandbox provider:

```bash
go run ./cmd/shclop \
  --dev \
  --store inmemory \
  --sandbox-provider docker-demo \
  --identity-provider mock-yaml \
  --identity-mock-yaml config/identity.mock.yaml
```

Open `http://localhost:8080`, log in as `bob@acme.test/bob`, create an agent, start it, and send a chat task.

`docker-demo` launches local containers with the Docker CLI. It is a demo launcher, not a production isolation boundary.

## Manual runtime debugging

Keep the default mock sandbox provider and start a runtime process manually with the token returned by `POST /api/agents/{agent_id}/start`:

```bash
go run ./cmd/shclop-runtime \
  --gateway ws://localhost:8080/runtime/ws \
  --agent-id <agent-id> \
  --token <runtime-token> \
  --runtime nanoclaw
```

Inside a runtime image the same process is configured through environment variables:

```bash
SHCLOP_GATEWAY_URL=ws://shclop-backend:8080/runtime/ws
SHCLOP_AGENT_ID=<agent-id>
SHCLOP_RUNTIME_TOKEN=<runtime-token>
SHCLOP_AGENT_FLAVOR=nanoclaw
```

## Tests and checks

Run Go tests:

```bash
go test ./...
```

Build the frontend:

```bash
cd web
npm install
npm run build
```

Render Helm manifests:

```bash
helm template shclop charts/shclop
```

Run the Makefile verification bundle:

```bash
make verify
```

Useful targets:

```bash
make test
make web-build
make runtime-images
make helm-template
make bootstrap-check
make clean
```

## Troubleshooting

If the backend fails with `listen tcp :8080: bind: address already in use`, find the process:

```bash
lsof -nP -iTCP:8080 -sTCP:LISTEN
```

Stop it with `kill <PID>`, or run Shclop on another port:

```bash
go run ./cmd/shclop --dev --store inmemory --addr :18080
```
