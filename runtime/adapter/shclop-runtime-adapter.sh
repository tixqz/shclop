#!/usr/bin/env sh
set -eu

mkdir -p /workspace /memory

echo "shclop runtime flavor: ${SHCLOP_AGENT_FLAVOR:-unknown}"
echo "workspace: /workspace"
echo "memory: /memory"

case "${SHCLOP_AGENT_FLAVOR:-}" in
  nanoclaw)
    exec nano-claw --help
    ;;
  nemoclaw)
    exec nemoclaw --help
    ;;
  openclaw)
    exec openclaw --help
    ;;
  *)
    echo "set SHCLOP_RUNTIME_COMMAND or SHCLOP_AGENT_FLAVOR" >&2
    exit 64
    ;;
esac
