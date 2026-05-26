# Shclop development

This file describes local development and verification. Production behavior is documented in [`README.md`](README.md): production uses PostgreSQL and the Kubernetes runtime provider with Kata runtime pods.

## Deployment workflow

The deploy workflow is triggered either manually (workflow_dispatch) or automatically on successful completion of the "Publish Images" workflow on the `prod` branch. It runs in the `production` GitHub environment.

### GitHub secrets required

| Secret | Purpose |
|---|---|
| `DEPLOY_HOST` | Hostname or IP of the target deployment server |
| `DEPLOY_USER` | SSH user for transfer and remote commands |
| `DEPLOY_SSH_KEY` | Private SSH key for authentication |

The deploy user on the server must have sudo access to run `/usr/local/sbin/shclop-deploy` (which is `scripts/deploy.sh` installed at that path). The deploy script requires:

- `helm` and `kubectl` binaries in PATH
- KUBECONFIG pointing to a valid k3s cluster config (defaults to `/etc/rancher/k3s/k3s.yaml`)
- The Helm chart at `charts/shclop` relative to the extracted release directory

### Flow

1. CI builds and pushes container images via "Publish Images" (or the workflow is triggered manually).
2. Deploy workflow computes the image tag (`sha-<commit>` for automated runs, or the manually supplied tag).
3. A tarball of the repository (excluding `.git`, ignored directories, `node_modules`, `web/dist`) is created and copied to the deploy host via SCP.
4. On the deploy host, the tarball is extracted and `sudo /usr/local/sbin/shclop-deploy <tag> <release-dir>` is executed.
5. The deploy script runs `helm upgrade --install` with production values, waits for rollout, and displays running pods.

## Local backend

For development and tests, use the in-memory store and mock provider:

```bash
go run ./cmd/shclop --dev --store inmemory --sandbox-provider mock
```

This mode is not production. It keeps state in memory and does not create Kubernetes resources.

The fallback local account is:

```text
admin / admin
```

For a more realistic local auth flow, set a bootstrap admin password:

```bash
SHCLOP_BOOTSTRAP_ADMIN_PASSWORD='replace-with-dev-password' \
go run ./cmd/shclop --dev --store inmemory --sandbox-provider mock
```

## UI development server

Run the Go backend:

```bash
go run ./cmd/shclop --dev --store inmemory --sandbox-provider mock
```

In another terminal, run Vite:

```bash
cd web
npm install
npm run dev
```

`web/vite.config.js` proxies `/api`, `/ws`, and `/runtime/ws` to `localhost:8080`.

## Serving the built UI from Go

The backend serves the compiled UI from `web/dist` by default.

```bash
cd web
npm install
npm run build
cd ..
go run ./cmd/shclop --dev --store inmemory --sandbox-provider mock
```

Override the static directory when needed:

```bash
go run ./cmd/shclop \
  --dev \
  --store inmemory \
  --sandbox-provider mock \
  --static-dir=/path/to/dist
```

## Local runtime options

### Mock provider

Use `--sandbox-provider mock` for most backend and UI work. It exercises API state transitions without starting a real runtime pod or container.

### Docker demo provider

The Docker demo provider is useful for local runtime wiring, but it is not a production isolation boundary.

Build runtime images:

```bash
make runtime-images
```

Run with Docker demo runtimes:

```bash
go run ./cmd/shclop \
  --dev \
  --store inmemory \
  --sandbox-provider docker-demo
```

Open `http://localhost:8080`, create an OpenClaw or NanoClaw agent, start it, and send a chat task.

### Manual runtime process

Keep the mock provider and start a runtime process manually with the token returned by `POST /api/agents/{agent_id}/start`:

```bash
go run ./cmd/shclop-runtime \
  --gateway ws://localhost:8080/runtime/ws \
  --agent-id <agent-id> \
  --token <runtime-token> \
  --runtime nanoclaw
```

Inside a runtime image, the same process is configured through environment variables:

```bash
SHCLOP_GATEWAY_URL=ws://shclop:8080/runtime/ws
SHCLOP_AGENT_ID=<agent-id>
SHCLOP_RUNTIME_TOKEN=<runtime-token>
SHCLOP_AGENT_FLAVOR=nanoclaw
```

## Production-like local configuration

Use PostgreSQL when testing persistence and migrations:

```bash
SHCLOP_BOOTSTRAP_ADMIN_PASSWORD='replace-with-dev-password' \
go run ./cmd/shclop \
  --dev \
  --store postgres \
  --postgres-dsn 'postgres://shclop:password@localhost:5432/shclop?sslmode=disable' \
  --sandbox-provider mock
```

Use `--sandbox-provider kubernetes` only against a Kubernetes cluster where the configured runtime namespace, RuntimeClass, storage, and permissions exist.

## LLM gateway development notes

Admin APIs store LLM gateway metadata and enabled model rows. The backend validates the selected model before agent create/start and requires gateway settings when starting an agent with a model.

Useful flags and environment variables:

```bash
--llm-gateway-base-url      # or SHCLOP_LLM_GATEWAY_BASE_URL
--llm-gateway-secret-name   # or SHCLOP_LLM_GATEWAY_SECRET_NAME
--llm-gateway-secret-key    # or SHCLOP_LLM_GATEWAY_SECRET_KEY
```

Shclop does not run a built-in LLM proxy in development or production.

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
