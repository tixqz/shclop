# shclop

Self-hosted *Claw orchestration platform.

## Development

Run the backend in dev/mock mode:

```bash
go run ./cmd/shclop --dev --mock-runtime --mock-llm --mock-secrets --store inmemory
```

Run tests:

```bash
go test ./...
```

Run the frontend:

```bash
cd web
npm install
npm run dev
```

## Single-node evaluation

Linux with KVM is required for full runtime evaluation.

```bash
scripts/bootstrap.sh check
scripts/bootstrap.sh install --install-deps
scripts/bootstrap.sh check --remote root@example.com
```

Docker Compose and macOS-native runtime are out of scope.
