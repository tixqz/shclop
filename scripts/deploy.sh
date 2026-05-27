#!/usr/bin/env bash
# shclop deploy — run on the server by a restricted deploy user via sudo
#
# Usage:
#   sudo ./scripts/deploy.sh <image-tag> [release-dir]
#
# Arguments:
#   image-tag   Required. Docker image tag to deploy (e.g. sha-abc123 or v0.1.0).
#   release-dir Optional. Path to the unpacked release directory containing
#               charts/shclop. Defaults to the current working directory.
#
# Environment:
#   KUBECONFIG  Kubernetes config path. Defaults to /etc/rancher/k3s/k3s.yaml.
set -euo pipefail

# ── Argument parsing ────────────────────────────────────────────
if [[ $# -lt 1 ]]; then
  echo "Usage: $0 <image-tag> [release-dir]" >&2
  exit 1
fi

TAG="$1"
RELEASE_DIR="${2:-$(pwd)}"

# ── Kubeconfig ──────────────────────────────────────────────────
KUBECONFIG="${KUBECONFIG:-/etc/rancher/k3s/k3s.yaml}"
export KUBECONFIG

# ── Prerequisite checks ─────────────────────────────────────────
if ! command -v helm &>/dev/null; then
  echo "ERROR: helm not found in PATH" >&2
  exit 1
fi

if ! command -v kubectl &>/dev/null; then
  echo "ERROR: kubectl not found in PATH" >&2
  exit 1
fi

echo "==> Checking kubectl access..."
if ! kubectl cluster-info &>/dev/null; then
  echo "ERROR: kubectl cannot connect to the cluster (KUBECONFIG=${KUBECONFIG})" >&2
  exit 1
fi

echo "==> Checking helm access..."
if ! helm version &>/dev/null; then
  echo "ERROR: helm cannot connect to the cluster" >&2
  exit 1
fi

# ── Locate chart ────────────────────────────────────────────────
CHART_DIR="${RELEASE_DIR}/charts/shclop"
if [[ ! -d "$CHART_DIR" ]]; then
  echo "ERROR: chart not found at ${CHART_DIR}" >&2
  exit 1
fi

MIGRATIONS_DIR="${RELEASE_DIR}/migrations"
DASHBOARDS_DIR="${RELEASE_DIR}/monitoring/grafana-dashboards"

apply_database_migrations() {
  if [[ ! -d "$MIGRATIONS_DIR" ]]; then
    echo "==> No migrations directory found at ${MIGRATIONS_DIR}; skipping."
    return 0
  fi

  shopt -s nullglob
  local migrations=("${MIGRATIONS_DIR}"/*.sql)
  shopt -u nullglob
  if [[ ${#migrations[@]} -eq 0 ]]; then
    echo "==> No SQL migrations found; skipping."
    return 0
  fi

  echo "==> Applying database migrations..."
  kubectl rollout status deploy/shclop-postgres --namespace default --timeout=300s
  for migration in "${migrations[@]}"; do
    echo "    applying $(basename "$migration")"
    kubectl exec -i deploy/shclop-postgres --namespace default -- \
      psql -U shclop -d shclop -v ON_ERROR_STOP=1 < "$migration"
  done
}

provision_grafana_dashboards() {
  if [[ ! -d "$DASHBOARDS_DIR" ]]; then
    echo "==> No Grafana dashboards directory found at ${DASHBOARDS_DIR}; skipping."
    return 0
  fi

  shopt -s nullglob
  local dashboards=("${DASHBOARDS_DIR}"/*.json)
  shopt -u nullglob
  if [[ ${#dashboards[@]} -eq 0 ]]; then
    echo "==> No Grafana dashboard JSON files found; skipping."
    return 0
  fi

  echo "==> Provisioning Grafana dashboards..."
  kubectl rollout status deploy/grafana --namespace monitoring --timeout=300s
  local grafana_password
  grafana_password="$(kubectl get secret grafana --namespace monitoring -o jsonpath='{.data.admin-password}' | base64 -d)"
  for dashboard in "${dashboards[@]}"; do
    echo "    importing $(basename "$dashboard")"
    python3 - "$dashboard" <<'PY' | curl -fsS -k -u "admin:${grafana_password}" \
      -H 'Content-Type: application/json' \
      -X POST https://grafana.178.62.240.51.nip.io/api/dashboards/db \
      --data-binary @- >/dev/null
import json
import sys

with open(sys.argv[1], encoding="utf-8") as fh:
    dashboard = json.load(fh)

print(json.dumps({"dashboard": dashboard, "overwrite": True, "folderId": 0}))
PY
  done
}

# ── Helm upgrade ────────────────────────────────────────────────
echo "==> Deploying shclop tag=${TAG} from ${RELEASE_DIR}..."

echo "==> Ensuring node-exporter is installed..."
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts/ 2>/dev/null || true
helm repo update prometheus-community 2>/dev/null || true
helm upgrade --install node-exporter prometheus-community/prometheus-node-exporter \
  --namespace monitoring --create-namespace \
  --set fullnameOverride=node-exporter \
  --set-string 'service.annotations.prometheus\.io/scrape=true' \
  --set-string 'service.annotations.prometheus\.io/port=9100' \
  --wait

echo "==> Ensuring LiteLLM Prometheus metrics are enabled..."
helm upgrade --install litellm oci://ghcr.io/berriai/litellm-helm \
  --namespace default --create-namespace \
  --reuse-values \
  --set 'proxy_config.litellm_settings.callbacks[0]=prometheus' \
  --set 'proxy_config.litellm_settings.require_auth_for_metrics_endpoint=false' \
  --set-string 'service.annotations.prometheus\.io/scrape=true' \
  --set-string 'service.annotations.prometheus\.io/port=4000' \
  --set-string 'service.annotations.prometheus\.io/path=/metrics/' \
  --wait

helm upgrade --install shclop "$CHART_DIR" \
  --namespace default --create-namespace \
  --set "image.repository=ghcr.io/tixqz/shclop" \
  --set "image.tag=${TAG}" \
  --set "image.pullPolicy=Always" \
  --set "agentRuntime.images.openclaw=ghcr.io/tixqz/shclop-runtime-openclaw:${TAG}" \
  --set "agentRuntime.images.nanoclaw=ghcr.io/tixqz/shclop-runtime-nanoclaw:${TAG}" \
  --set "sandbox.kubernetes.namespace=default" \
  --set "ingress.enabled=true" \
  --set "ingress.className=traefik" \
  --set "ingress.host=shclop.178.62.240.51.nip.io" \
  --set "ingress.tls.enabled=true" \
  --set "ingress.tls.clusterIssuer=letsencrypt-http" \
  --set "llmGateway.litellm.enabled=true" \
  --set "llmGateway.litellm.serviceName=litellm" \
  --set "llmGateway.litellm.namespace=default" \
  --set "llmGateway.litellm.port=4000" \
  --set "llmGateway.existingSecret.name=litellm-master" \
  --set "llmGateway.existingSecret.key=api-key" \
  --set "observability.grafana.enabled=true" \
  --set "observability.grafana.url=https://grafana.178.62.240.51.nip.io" \
  --wait

apply_database_migrations
provision_grafana_dashboards

# ── Rollout status ──────────────────────────────────────────────
echo ""
echo "==> Waiting for rollout of deploy/shclop-backend..."
kubectl rollout status deploy/shclop-backend --namespace default --timeout=300s

echo ""
echo "==> Pods:"
kubectl get pods --namespace default -l app.kubernetes.io/instance=shclop

echo ""
echo "==> Deploy of tag=${TAG} complete."
