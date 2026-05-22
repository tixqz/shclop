#!/usr/bin/env bash
# shclop-mock-runtime-launcher — auto-starts mock runtimes for agents
set -euo pipefail

GATEWAY="${SHCLOP_GATEWAY_URL:-http://localhost:8080}"
RUNTIME_BIN="${SHCLOP_RUNTIME_BIN:-/usr/local/bin/mock-runtime}"
FLAVOR="${SHCLOP_AGENT_FLAVOR:-openclaw}"
POLL_INTERVAL="${SHCLOP_POLL_INTERVAL:-5}"

declare -A RUNNING_AGENTS

log() { echo "[$(date +%H:%M:%S)] $*"; }

get_token() {
  local agent_id="$1"
  local token
  token=$(curl -s -X POST "${GATEWAY}/api/agents/${agent_id}/start" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}" \
    -H 'Content-Type: application/json' \
    -d "{\"runtime\":\"${FLAVOR}\"}" 2>/dev/null | grep -o '"runtime_token":"[^"]*"' | cut -d'"' -f4)
  echo "$token"
}

list_starting_agents() {
  curl -s "${GATEWAY}/api/agents" \
    -H "Authorization: Bearer ${ADMIN_TOKEN}" 2>/dev/null | \
    grep -o '"id":"[^"]*","owner_id":"[^"]*","tenant_id":"[^"]*","name":"[^"]*","model":"[^"]*","state":"starting"' | \
    grep -o '"id":"[^"]*"' | cut -d'"' -f4
}

log "Mock runtime launcher started"
log "Gateway: ${GATEWAY}, Flavor: ${FLAVOR}"

while true; do
  # Get token (use bob - owns the agents)
  if [[ -z "${ADMIN_TOKEN:-}" ]]; then
    ADMIN_TOKEN=$(curl -s -X POST "${GATEWAY}/api/auth/login" \
      -d '{"username":"bob@acme.test","password":"72jUddj1%$PA"}' \
      -H 'Content-Type: application/json' 2>/dev/null | \
      grep -o '"token":"[^"]*"' | cut -d'"' -f4)
    if [[ -n "$ADMIN_TOKEN" ]]; then
      log "Admin token acquired"
    fi
  fi

  if [[ -z "${ADMIN_TOKEN:-}" ]]; then
    sleep "$POLL_INTERVAL"
    continue
  fi

  # Find agents that need runtime
  for agent_id in $(list_starting_agents); do
    if [[ -n "${RUNNING_AGENTS[$agent_id]:-}" ]]; then
      continue
    fi

    log "Starting runtime for agent: ${agent_id}"
    token=$(get_token "$agent_id")
    if [[ -z "$token" ]]; then
      log "Failed to get token for agent ${agent_id}"
      continue
    fi

    # Launch mock runtime in background
    nohup "$RUNTIME_BIN" \
      --gateway "ws://localhost:8080/runtime/ws" \
      --agent-id "$agent_id" \
      --token "$token" \
      --runtime "$FLAVOR" \
      > "/var/log/shclop-runtime-${agent_id}.log" 2>&1 &

    RUNNING_AGENTS[$agent_id]=1
    log "Runtime launched for agent ${agent_id} (PID: $!)"
  done

  # Clean up dead processes
  for agent_id in "${!RUNNING_AGENTS[@]}"; do
    if ! pgrep -f "mock-runtime.*${agent_id}" > /dev/null 2>&1; then
      log "Runtime for agent ${agent_id} died, removing from tracking"
      unset "RUNNING_AGENTS[$agent_id]"
    fi
  done

  sleep "$POLL_INTERVAL"
done
