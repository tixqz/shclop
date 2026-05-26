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

# ── Helm upgrade ────────────────────────────────────────────────
echo "==> Deploying shclop tag=${TAG} from ${RELEASE_DIR}..."

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

# ── Rollout status ──────────────────────────────────────────────
echo ""
echo "==> Waiting for rollout of deploy/shclop-backend..."
kubectl rollout status deploy/shclop-backend --namespace default --timeout=300s

echo ""
echo "==> Pods:"
kubectl get pods --namespace default -l app.kubernetes.io/instance=shclop

echo ""
echo "==> Deploy of tag=${TAG} complete."
