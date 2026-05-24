#!/usr/bin/env sh
set -eu

mkdir -p /workspace /memory

echo "shclop runtime flavor: ${SHCLOP_AGENT_FLAVOR:-unknown}"
echo "workspace: /workspace"
echo "memory: /memory"

if [ "${SHCLOP_RUNTIME_TOKEN:-}" = "" ] && [ "${SHCLOP_RUNTIME_TOKEN_FILE:-}" != "" ] && [ -r "$SHCLOP_RUNTIME_TOKEN_FILE" ]; then
  SHCLOP_RUNTIME_TOKEN="$(cat "$SHCLOP_RUNTIME_TOKEN_FILE")"
fi

if [ "${SHCLOP_GATEWAY_URL:-}" != "" ] && [ "${SHCLOP_AGENT_ID:-}" != "" ] && [ "${SHCLOP_RUNTIME_TOKEN:-}" != "" ]; then
  exec shclop-runtime \
    --gateway "$SHCLOP_GATEWAY_URL" \
    --agent-id "$SHCLOP_AGENT_ID" \
    --token "$SHCLOP_RUNTIME_TOKEN" \
    --runtime "${SHCLOP_AGENT_FLAVOR:-demo}"
fi

case "${SHCLOP_AGENT_FLAVOR:-}" in
  nanoclaw)
    exec nano-claw --help
    ;;
  openclaw)
    exec openclaw --help
    ;;
  *)
    echo "set SHCLOP_RUNTIME_COMMAND or SHCLOP_AGENT_FLAVOR" >&2
    exit 64
    ;;
esac
