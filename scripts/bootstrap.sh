#!/usr/bin/env bash
# shclop bootstrap — single-node installer for K3s + Kata + shclop Helm chart
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
CHARTS_DIR="$REPO_DIR/charts"

# ── Terminal helpers ──────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'

info()  { echo -e "${CYAN}==>${NC} $*"; }
step()  { echo -e "  ${GREEN}[ok]${NC} $*"; }
warn()  { echo -e "  ${YELLOW}[..]${NC} $*"; }
error() { echo -e "  ${RED}[!!]${NC} $*"; }
fail()  { error "$*"; exit 1; }
header(){ echo -e "\n${BOLD}── $* ──${NC}"; }

# ── Defaults ──────────────────────────────────────────────────
K3S_VERSION="${K3S_VERSION:-latest}"
KATA_VERSION="${KATA_VERSION:-stable-3.x}"
KATA_STATIC_VERSION="${KATA_STATIC_VERSION:-3.31.0}"
KATA_DEFAULT_MEMORY_MIB="${KATA_DEFAULT_MEMORY_MIB:-768}"
HELM_RELEASE_NAME="${HELM_RELEASE_NAME:-shclop}"
SHCLOP_NAMESPACE="${SHCLOP_NAMESPACE:-default}"
IMAGE_REPO="${IMAGE_REPO:-}"
IMAGE_TAG="${IMAGE_TAG:-latest}"
CERT_MANAGER_VERSION="${CERT_MANAGER_VERSION:-v1.16.2}"
INGRESS_CLASS="${INGRESS_CLASS:-traefik}"
TLS_CLUSTER_ISSUER="${TLS_CLUSTER_ISSUER:-letsencrypt-http}"
MIN_CPU_CORES="${MIN_CPU_CORES:-2}"
MIN_MEMORY_MIB="${MIN_MEMORY_MIB:-4096}"
MIN_DISK_GIB="${MIN_DISK_GIB:-30}"
OBSERVABILITY_NAMESPACE="${OBSERVABILITY_NAMESPACE:-monitoring}"
# (removed VictoriaMetrics/VictoriaLogs — replaced by Prometheus + Loki + Fluent Bit)
LITELLM_RELEASE_NAME="${LITELLM_RELEASE_NAME:-litellm}"
LITELLM_NAMESPACE="${LITELLM_NAMESPACE:-$SHCLOP_NAMESPACE}"
LITELLM_IMAGE_TAG="${LITELLM_IMAGE_TAG:-v1.85.0}"
LITELLM_MODEL_ALIAS="${LITELLM_MODEL_ALIAS:-deepseek-v4-flash}"
LITELLM_OPENROUTER_MODEL="${LITELLM_OPENROUTER_MODEL:-deepseek/deepseek-v4-flash:free}"
LITELLM_OPENROUTER_SECRET="${LITELLM_OPENROUTER_SECRET:-litellm-openrouter}"
LITELLM_MASTER_SECRET="${LITELLM_MASTER_SECRET:-litellm-master}"

# ── Flag parsing ──────────────────────────────────────────────
usage() {
  cat <<USAGE
Usage: scripts/bootstrap.sh <check|install|reset|destroy> [flags]

Actions:
  check           Verify prerequisites (KVM, K3s, Kata, RuntimeClass)
  install         Install K3s, Kata, RuntimeClass, deploy shclop
  reset           Destroy + install (same flags apply)
  destroy         Tear down shclop, optionally K3s/Kata

Targets:
  (no flag)       Run locally
  --remote USER@HOST   Run action on remote host over SSH

Flags:
  --install-deps       Install K3s and Kata (required for first install)
  --values PATH        Helm values file (required for Helm deploy)
  --image-repo REPO    Container registry for shclop/runtime images
  --image-tag TAG      Image tag (default: latest)
  --enable-ingress     Expose shclop through Ingress (K3s Traefik by default)
  --public-ip IP       Build nip.io hostname: shclop.<IP>.nip.io
  --host HOST          Explicit Ingress hostname (overrides --public-ip)
  --tls-email EMAIL    Enable Let's Encrypt TLS via cert-manager ACME account
  --ingress-class NAME IngressClass name (default: traefik)
  --cluster-issuer NAME cert-manager ClusterIssuer name (default: letsencrypt-http)
  --dry-run            Print actions without executing
  --yes                Skip confirmations (for destroy)
  --purge-data         Also remove PVCs, workspace data (for destroy)
  --remove-k3s         Remove K3s (for destroy)
  --remove-kata        Remove Kata (for destroy)
  --enable-observability  Install recommended observability stack (Prometheus + Grafana + Loki + Fluent Bit)
  --observability-namespace NS  Namespace for observability components (default: monitoring)
  --grafana-host HOST     Explicit Grafana hostname (default: grafana.<public-ip>.nip.io)

  --enable-litellm        Install internal LiteLLM gateway (ClusterIP only) for LLM traffic
  --litellm-namespace NS  Namespace for LiteLLM (default: shclop namespace)
  --litellm-release NAME  Helm release name for LiteLLM (default: litellm)
  --litellm-model MODEL   OpenRouter model id exposed through LiteLLM (default: deepseek/deepseek-v4-flash:free)
USAGE
}

action="${1:-}"
[[ -z "$action" ]] && { usage; exit 2; }
shift

remote=""
dry_run=false
install_deps=false
yes=false
purge_data=false
remove_k3s=false
remove_kata=false
values=""
enable_ingress=false
public_ip=""
ingress_host=""
tls_email=""
enable_observability=false
observability_namespace="$OBSERVABILITY_NAMESPACE"
grafana_host=""
enable_litellm=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --remote) [[ $# -lt 2 ]] && { usage; exit 2; }; remote="$2"; shift 2 ;;
    --dry-run) dry_run=true; shift ;;
    --install-deps) install_deps=true; shift ;;
    --yes) yes=true; shift ;;
    --purge-data) purge_data=true; shift ;;
    --remove-k3s) remove_k3s=true; shift ;;
    --remove-kata) remove_kata=true; shift ;;
    --values) [[ $# -lt 2 ]] && { usage; exit 2; }; values="$2"; shift 2 ;;
    --image-repo) [[ $# -lt 2 ]] && { usage; exit 2; }; IMAGE_REPO="$2"; shift 2 ;;
    --image-tag) [[ $# -lt 2 ]] && { usage; exit 2; }; IMAGE_TAG="$2"; shift 2 ;;
    --enable-ingress) enable_ingress=true; shift ;;
    --public-ip) [[ $# -lt 2 ]] && { usage; exit 2; }; public_ip="$2"; shift 2 ;;
    --host) [[ $# -lt 2 ]] && { usage; exit 2; }; ingress_host="$2"; shift 2 ;;
    --tls-email) [[ $# -lt 2 ]] && { usage; exit 2; }; tls_email="$2"; shift 2 ;;
    --ingress-class) [[ $# -lt 2 ]] && { usage; exit 2; }; INGRESS_CLASS="$2"; shift 2 ;;
    --cluster-issuer) [[ $# -lt 2 ]] && { usage; exit 2; }; TLS_CLUSTER_ISSUER="$2"; shift 2 ;;
    --enable-observability) enable_observability=true; shift ;;
    --observability-namespace) [[ $# -lt 2 ]] && { usage; exit 2; }; observability_namespace="$2"; shift 2 ;;
    --grafana-host) [[ $# -lt 2 ]] && { usage; exit 2; }; grafana_host="$2"; shift 2 ;;

    --enable-litellm) enable_litellm=true; shift ;;
    --litellm-namespace) [[ $# -lt 2 ]] && { usage; exit 2; }; LITELLM_NAMESPACE="$2"; shift 2 ;;
    --litellm-release) [[ $# -lt 2 ]] && { usage; exit 2; }; LITELLM_RELEASE_NAME="$2"; shift 2 ;;
    --litellm-model) [[ $# -lt 2 ]] && { usage; exit 2; }; LITELLM_OPENROUTER_MODEL="$2"; shift 2 ;;
    *) echo "unknown argument: $1" >&2; usage; exit 2 ;;
  esac
done

case "$action" in check|install|reset|destroy) ;; *) echo "unknown action: $action" >&2; usage; exit 2 ;; esac

resolve_ingress_host() {
  if [[ -n "$ingress_host" ]]; then
    echo "$ingress_host"
    return 0
  fi
  if [[ -n "$public_ip" ]]; then
    echo "shclop.${public_ip}.nip.io"
    return 0
  fi
  return 1
}

validate_ingress_config() {
  if [[ -n "$tls_email" && "$enable_ingress" != "true" ]]; then
    fail "--tls-email requires --enable-ingress"
  fi
  if [[ "$enable_ingress" == "true" ]] && ! resolve_ingress_host >/dev/null; then
    fail "--enable-ingress requires --public-ip IP or --host HOST"
  fi
  if [[ "$enable_observability" == "true" && "$enable_ingress" != "true" ]]; then
    fail "--enable-observability requires --enable-ingress (Grafana needs a hostname)"
  fi
}

resolve_grafana_host() {
  if [[ -n "$grafana_host" ]]; then
    echo "$grafana_host"
    return 0
  fi
  if [[ -n "$public_ip" ]]; then
    echo "grafana.${public_ip}.nip.io"
    return 0
  fi
  if [[ -n "$ingress_host" ]]; then
    echo "$ingress_host" | sed 's/^shclop\./grafana./'
    return 0
  fi
  return 1
}

# ── Root check ────────────────────────────────────────────────
require_root() {
  if $dry_run; then return 0; fi
  [[ $EUID -eq 0 ]] || fail "this action requires root (run with sudo or as root)"
}

# ── K3s helpers ───────────────────────────────────────────────
K3S_CONTAINERD_DIR="/var/lib/rancher/k3s/agent/etc/containerd"
K3S_KUBECONFIG="/etc/rancher/k3s/k3s.yaml"

is_k3s_installed() { command -v k3s &>/dev/null; }
is_k3s_running()   { systemctl is-active --quiet k3s 2>/dev/null; }
k3s_version()      { k3s --version 2>/dev/null | head -1; }
kubectl()          { k3s kubectl "$@"; }
helm()             { KUBECONFIG="$K3S_KUBECONFIG" command helm "$@"; }

# ── Kata helpers ──────────────────────────────────────────────
is_kata_installed()  { command -v kata-runtime &>/dev/null; }
kata_version()       { kata-runtime --version 2>/dev/null | head -1; }
kata_config_path()   { echo "/opt/kata/share/defaults/kata-containers/configuration.toml"; }

# ═══════════════════════════════════════════════════════════════
#  CHECK
# ═══════════════════════════════════════════════════════════════
cpu_cores() {
  getconf _NPROCESSORS_ONLN 2>/dev/null || nproc 2>/dev/null || echo 0
}

memory_mib() {
  if [[ -r /proc/meminfo ]]; then
    awk '/MemTotal/ { printf "%d", $2 / 1024 }' /proc/meminfo
  elif command -v sysctl >/dev/null 2>&1; then
    local bytes
    bytes="$(sysctl -n hw.memsize 2>/dev/null || true)"
    if [[ "$bytes" =~ ^[0-9]+$ ]]; then
      echo $((bytes / 1024 / 1024))
    else
      echo 0
    fi
  else
    echo 0
  fi
}

disk_available_gib() {
  local path="/var/lib/rancher"
  [[ -d "$path" ]] || path="/"
  df -Pk "$path" 2>/dev/null | awk 'NR == 2 { printf "%d", $4 / 1024 / 1024 }'
}

check_hardware() {
  header "Hardware sizing"

  local cores mem disk
  cores="$(cpu_cores)"
  mem="$(memory_mib)"
  disk="$(disk_available_gib)"

  if [[ "$cores" =~ ^[0-9]+$ ]] && (( cores >= MIN_CPU_CORES )); then
    step "CPU cores: ${cores} (minimum: ${MIN_CPU_CORES})"
  else
    warn "CPU cores: ${cores:-unknown}; minimum recommended for single-node install: ${MIN_CPU_CORES}"
  fi

  if [[ "$mem" =~ ^[0-9]+$ ]] && (( mem >= MIN_MEMORY_MIB )); then
    step "Memory: ${mem} MiB (minimum: ${MIN_MEMORY_MIB} MiB)"
  else
    warn "Memory: ${mem:-unknown} MiB; minimum recommended for single-node install: ${MIN_MEMORY_MIB} MiB"
  fi

  if [[ "$disk" =~ ^[0-9]+$ ]] && (( disk >= MIN_DISK_GIB )); then
    step "Free disk: ${disk} GiB (minimum: ${MIN_DISK_GIB} GiB)"
  else
    warn "Free disk: ${disk:-unknown} GiB; minimum recommended for single-node install: ${MIN_DISK_GIB} GiB"
  fi
}

check_kvm() {
  info "KVM"
  if [[ -e /dev/kvm ]]; then
    step "/dev/kvm is available"
  else
    warn "/dev/kvm was not found; Kata may run without hardware virtualization and is not recommended for production isolation"
  fi
}

check_k3s() {
  header "K3s / Kubernetes"
  if is_k3s_installed; then
    step "k3s installed: $(k3s_version)"
    if is_k3s_running; then
      step "k3s is running"
      if kubectl get nodes &>/dev/null; then
        step "kubectl works, node: $(kubectl get nodes -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo 'unknown')"
      else
        warn "kubectl cannot connect to the cluster"
      fi
    else
      warn "k3s is not running"
    fi
  else
    warn "k3s is not installed"
  fi
}

check_kata() {
  header "Kata Containers"
  if is_kata_installed; then
    step "kata-runtime installed: $(kata_version)"
    if kata-runtime check 2>&1 | grep -qi "version"; then
      step "kata-runtime check passed"
    else
      warn "kata-runtime check: see output above"
      kata-runtime check 2>&1 | head -5
    fi
  else
    warn "kata-runtime not installed"
  fi
}

check_containerd_runtime() {
  header "Containerd runtime (Kata)"
  local tmpl="$K3S_CONTAINERD_DIR/config.toml.tmpl"
  local cfg="$K3S_CONTAINERD_DIR/config.toml"

  if [[ -f "$tmpl" ]] && grep -q "kata" "$tmpl" 2>/dev/null; then
    step "Kata runtime is in containerd template"
  elif [[ -f "$cfg" ]] && grep -q "kata" "$cfg" 2>/dev/null; then
    step "Kata runtime is in containerd config"
  else
    warn "Kata runtime not found in containerd config"
  fi
}

check_runtimeclass() {
  header "Kubernetes RuntimeClass"
  if kubectl get runtimeclass kata &>/dev/null 2>&1; then
    step "RuntimeClass 'kata' exists"
  else
    warn "RuntimeClass 'kata' not found"
  fi
}

action_check() {
  info "Checking prerequisites..."
  check_hardware
  check_kvm
  check_k3s
  check_kata
  check_containerd_runtime
  check_runtimeclass
  echo ""
  info "Done. If there are [..] warnings, rerun with --install-deps"
}

# ═══════════════════════════════════════════════════════════════
#  INSTALL — system dependencies
# ═══════════════════════════════════════════════════════════════
install_k3s() {
  header "Installing K3s"
  if is_k3s_installed; then
    warn "k3s already installed: $(k3s_version)"
    return 0
  fi
  if $dry_run; then
    warn "[dry-run] curl -sfL https://get.k3s.io | sh -s - --write-kubeconfig-mode 644"
    return 0
  fi
  info "Installing K3s..."
  curl -sfL https://get.k3s.io | sh -s - --write-kubeconfig-mode 644
  step "K3s installed: $(k3s_version)"
}

install_kata_ubuntu() {
  header "Installing Kata Containers"
  if is_kata_installed; then
    warn "kata-runtime already installed: $(kata_version)"
    return 0
  fi
  if $dry_run; then
    warn "[dry-run] installing kata-static ${KATA_STATIC_VERSION} from GitHub release"
    return 0
  fi

  info "Installing prerequisites..."
  apt-get update -qq
  apt-get install -y -qq curl ca-certificates zstd

  local arch
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64) kata_arch="amd64" ;;
    aarch64|arm64) kata_arch="arm64" ;;
    *) fail "unsupported architecture: $arch (only x86_64/amd64 and aarch64/arm64 are supported)" ;;
  esac

  local bootstrap_dir="$REPO_DIR/.bootstrap"
  mkdir -p "$bootstrap_dir"

  local tarball="kata-static-${KATA_STATIC_VERSION}-${kata_arch}.tar.zst"
  local url="https://github.com/kata-containers/kata-containers/releases/download/${KATA_STATIC_VERSION}/${tarball}"

  info "Downloading ${tarball}..."
  curl -fsSL "$url" -o "$bootstrap_dir/$tarball" || fail "failed to download kata-static tarball from $url"

  info "Extracting to / ..."
  tar --use-compress-program=unzstd -xf "$bootstrap_dir/$tarball" -C / || {
    rm -f "$bootstrap_dir/$tarball"
    fail "failed to extract kata-static tarball"
  }
  rm -f "$bootstrap_dir/$tarball"

  # Symlink binaries into PATH
  local kata_bin_src="/opt/kata/bin"
  if [[ -f "$kata_bin_src/kata-runtime" ]]; then
    ln -sf "$kata_bin_src/kata-runtime" /usr/local/bin/kata-runtime
  fi
  if [[ -f "$kata_bin_src/containerd-shim-kata-v2" ]]; then
    ln -sf "$kata_bin_src/containerd-shim-kata-v2" /usr/local/bin/containerd-shim-kata-v2
  fi

  if is_kata_installed; then
    step "kata-runtime installed: $(kata_version)"
  else
    fail "kata-runtime is still missing after static install. The release tarball may be incomplete or architecture mismatch."
  fi
}

install_helm() {
  header "Installing Helm"
  if command -v helm &>/dev/null; then
    warn "Helm already installed: $(helm version --short 2>/dev/null || true)"
    return 0
  fi
  if $dry_run; then
    warn "[dry-run] curl -sfL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash"
    return 0
  fi
  info "Installing Helm..."
  curl -sfL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
  step "Helm installed: $(helm version --short)"
}

tune_kata_configuration() {
  header "Tuning Kata configuration"
  local config="/opt/kata/share/defaults/kata-containers/configuration.toml"
  if $dry_run; then
    warn "[dry-run] set Kata default_memory to ${KATA_DEFAULT_MEMORY_MIB} MiB in ${config}"
    return 0
  fi
  if [[ ! -f "$config" ]]; then
    warn "Kata configuration not found at ${config}; skipping memory tuning"
    return 0
  fi

  python3 - "$config" "$KATA_DEFAULT_MEMORY_MIB" <<'PY'
from pathlib import Path
import re
import sys

path = Path(sys.argv[1])
memory = sys.argv[2]
content = path.read_text()
updated, count = re.subn(r'(?m)^default_memory\s*=\s*\d+', f'default_memory = {memory}', content, count=1)
if count == 0:
    raise SystemExit('default_memory was not found in Kata configuration')
if updated != content:
    path.write_text(updated)
PY
  step "Kata default_memory set to ${KATA_DEFAULT_MEMORY_MIB} MiB"
}

configure_containerd_kata() {
  header "Configuring containerd for Kata"
  if $dry_run; then
    warn "[dry-run] creating $K3S_CONTAINERD_DIR/config.toml.tmpl with kata runtime"
    return 0
  fi

  # Validate that the Kata containerd shim binary is present before writing config
  if [[ ! -x /opt/kata/bin/containerd-shim-kata-v2 ]] && [[ ! -x /usr/local/bin/containerd-shim-kata-v2 ]]; then
    fail "Kata containerd shim not found at /opt/kata/bin/containerd-shim-kata-v2. Install Kata first (--install-deps)."
  fi

  mkdir -p "$K3S_CONTAINERD_DIR"

  local tmpl="$K3S_CONTAINERD_DIR/config.toml.tmpl"

  if [[ -f "$tmpl" ]] && grep -q '{{[[:space:]]*template "containerd"' "$tmpl" 2>/dev/null; then
    info "Updating legacy K3s containerd template include..."
    python3 - "$tmpl" <<'PY'
from pathlib import Path
import sys

path = Path(sys.argv[1])
content = path.read_text()
content = content.replace('{{ template "containerd" . }}', '{{ template "base" . }}')
path.write_text(content)
PY
  fi

  if [[ -f "$K3S_CONTAINERD_DIR/config.toml.tmpl" ]] && grep -q "kata" "$K3S_CONTAINERD_DIR/config.toml.tmpl" 2>/dev/null; then
    step "Kata already in containerd template"
    return 0
  fi

  if [[ -f "$tmpl" ]]; then
    cat >> "$tmpl" << 'KATAEOF'

# Kata Containers runtime
[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.kata]
  runtime_type = "io.containerd.kata.v2"
  privileged_without_host_devices = true
  [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.kata.options]
    ConfigPath = "/opt/kata/share/defaults/kata-containers/configuration.toml"
KATAEOF
  else
    cat > "$tmpl" << 'KATAEOF'
{{ template "base" . }}

# Kata Containers runtime
[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.kata]
  runtime_type = "io.containerd.kata.v2"
  privileged_without_host_devices = true
  [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.kata.options]
    ConfigPath = "/opt/kata/share/defaults/kata-containers/configuration.toml"
KATAEOF
  fi

  step "Containerd template updated"

  info "Restarting K3s..."
  systemctl restart k3s
  sleep 5
  local i=0
  while ! kubectl get nodes &>/dev/null; do
    sleep 2
    i=$((i+1))
    [[ $i -gt 60 ]] && { warn "K3s did not recover after restart (60s)"; break; }
  done
  step "K3s restarted"
}

wait_for_k3s() {
  if $dry_run; then
    warn "[dry-run] waiting for K3s readiness (skipped)"
    return 0
  fi
  info "Waiting for K3s readiness..."
  local i=0
  while ! kubectl get nodes &>/dev/null; do
    sleep 2
    i=$((i+1))
    [[ $i -gt 30 ]] && { warn "K3s not responding after 60s"; return 1; }
  done
  step "K3s ready: $(kubectl get nodes -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)"
}

create_runtimeclass() {
  header "Creating RuntimeClass"
  if kubectl get runtimeclass kata &>/dev/null 2>&1; then
    step "RuntimeClass kata already exists"
    return 0
  fi
  if $dry_run; then
    warn "[dry-run] creating RuntimeClass 'kata'"
    return 0
  fi

  kubectl apply -f - <<'RUNTIMECLASS'
apiVersion: node.k8s.io/v1
kind: RuntimeClass
metadata:
  name: kata
handler: kata
overhead:
  podFixed:
    memory: "160Mi"
    cpu: "250m"
RUNTIMECLASS
  step "RuntimeClass 'kata' created"
}

validate_kata_runtimeclass() {
  header "Validating Kata RuntimeClass"
  if $dry_run; then
    warn "[dry-run] creating smoke test pod with runtimeClassName: kata and waiting for completion"
    return 0
  fi

  # Ensure namespace exists
  local ns="${SHCLOP_NAMESPACE:-default}"
  kubectl get namespace "$ns" &>/dev/null || kubectl create namespace "$ns"

  local pod_name="kata-smoke-test-$(date +%s)"

  kubectl run "$pod_name" --image=busybox:1.36 --restart=Never \
    --namespace "$ns" \
    --overrides='{"spec":{"runtimeClassName":"kata"}}' \
    -- true || {
    fail "Failed to create smoke test pod for Kata validation"
  }

  info "Waiting up to 120s for pod $pod_name to complete..."

  # Check initial phase — the pod may have already completed
  local phase
  phase="$(kubectl get pod "$pod_name" --namespace "$ns" -o jsonpath='{.status.phase}' 2>/dev/null || true)"

  if [[ "$phase" != "Succeeded" && "$phase" != "Failed" ]]; then
    # Pod still running/pending: wait for Ready first
    if ! kubectl wait --for=condition=Ready "pod/$pod_name" --namespace "$ns" --timeout=30s 2>/dev/null; then
      # May have completed without being Ready; refresh phase
      phase="$(kubectl get pod "$pod_name" --namespace "$ns" -o jsonpath='{.status.phase}' 2>/dev/null || true)"
    fi
  fi

  if [[ "$phase" != "Succeeded" && "$phase" != "Failed" ]]; then
    # Still not done: wait for final completion
    kubectl wait --for=jsonpath='{.status.phase}'=Succeeded "pod/$pod_name" --namespace "$ns" --timeout=90s 2>/dev/null || true
    phase="$(kubectl get pod "$pod_name" --namespace "$ns" -o jsonpath='{.status.phase}' 2>/dev/null || true)"
  fi

  if [[ "$phase" != "Succeeded" ]]; then
    error "Kata smoke test pod failed (phase: ${phase:-unknown})"
    echo ""
    error "Pod description:"
    kubectl describe pod "$pod_name" --namespace "$ns" 2>/dev/null || true
    echo ""
    error "Recent events for pod:"
    kubectl get events --namespace "$ns" --field-selector "involvedObject.name=$pod_name" 2>/dev/null || true
    echo ""
    kubectl delete pod "$pod_name" --namespace "$ns" --ignore-not-found 2>/dev/null || true
    fail "Kata RuntimeClass validation failed. Kata may not be functional. Fix the issue and re-run."
  fi

  # Clean up on success
  kubectl delete pod "$pod_name" --namespace "$ns" --ignore-not-found 2>/dev/null || true
  step "Kata RuntimeClass validated (pod $pod_name completed successfully)"
}

install_cert_manager() {
  if [[ -z "$tls_email" ]]; then
    return 0
  fi
  header "Installing cert-manager"
  if kubectl get deployment cert-manager -n cert-manager &>/dev/null 2>&1; then
    step "cert-manager already installed"
    return 0
  fi
  if $dry_run; then
    warn "[dry-run] kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/${CERT_MANAGER_VERSION}/cert-manager.yaml"
    warn "[dry-run] wait for cert-manager readiness"
    return 0
  fi
  kubectl apply -f "https://github.com/cert-manager/cert-manager/releases/download/${CERT_MANAGER_VERSION}/cert-manager.yaml"
  info "Waiting for cert-manager deployments..."
  kubectl wait --for=condition=Available deployment/cert-manager -n cert-manager --timeout=120s
  kubectl wait --for=condition=Available deployment/cert-manager-webhook -n cert-manager --timeout=120s
  kubectl wait --for=condition=Available deployment/cert-manager-cainjector -n cert-manager --timeout=120s
  step "cert-manager installed"
}

create_clusterissuer() {
  if [[ -z "$tls_email" ]]; then
    return 0
  fi
  header "Creating ClusterIssuer"
  if kubectl get clusterissuer "${TLS_CLUSTER_ISSUER}" &>/dev/null 2>&1; then
    step "ClusterIssuer '${TLS_CLUSTER_ISSUER}' already exists"
    return 0
  fi
  if $dry_run; then
    warn "[dry-run] create ClusterIssuer '${TLS_CLUSTER_ISSUER}' for Let's Encrypt HTTP-01 (${INGRESS_CLASS})"
    return 0
  fi
  kubectl apply -f - <<CLUSTERISSUER
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: ${TLS_CLUSTER_ISSUER}
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: ${tls_email}
    privateKeySecretRef:
      name: ${TLS_CLUSTER_ISSUER}-account-key
    solvers:
      - http01:
          ingress:
            ingressClassName: ${INGRESS_CLASS}
CLUSTERISSUER
  step "ClusterIssuer '${TLS_CLUSTER_ISSUER}' created"
}

# ═══════════════════════════════════════════════════════════════
#  OBSERVABILITY — Prometheus + Grafana + Loki + Fluent Bit
# ═══════════════════════════════════════════════════════════════
# Release names for the stack
PROMETHEUS_SERVER_RELEASE="${PROMETHEUS_SERVER_RELEASE:-prometheus-server}"
LOKI_RELEASE="${LOKI_RELEASE:-loki}"
FLUENT_BIT_RELEASE="${FLUENT_BIT_RELEASE:-fluent-bit}"
GRAFANA_RELEASE="${GRAFANA_RELEASE:-grafana}"
# The actual Prometheus server Kubernetes Service name (chart creates <release>-server).
# Default: prometheus-server-server matches the prometheus-community/prometheus chart
# with release name prometheus-server (the default PROMETHEUS_SERVER_RELEASE).
PROMETHEUS_SERVER_SERVICE="${PROMETHEUS_SERVER_SERVICE:-prometheus-server-server}"
# Grafana admin password (default matches current QA environment)
# Override with GRAFANA_ADMIN_PASSWORD for shared environments. Do not commit
# real shared credentials into this script.
GRAFANA_ADMIN_PASSWORD="${GRAFANA_ADMIN_PASSWORD:-admin}"

cleanup_old_observability() {
  local ns="$observability_namespace"

  if $dry_run; then
    warn "[dry-run] would clean up old observability releases (victoria-metrics, victoria-logs, prometheus-stack) if present"
    return 0
  fi

  # Only check/remove when helm is available and can connect
  if ! command -v helm &>/dev/null; then
    return 0
  fi

  local removed=false

  # Old VictoriaMetrics stack
  if helm list -n "$ns" -q 2>/dev/null | grep -q "^victoria-metrics-k8s-stack$"; then
    warn "Removing previous victoria-metrics-k8s-stack release..."
    helm uninstall victoria-metrics-k8s-stack -n "$ns" 2>/dev/null || true
    # Clean up leftover CRDs and resources from the old kube-prometheus-style stack
    kubectl delete crd -l app.kubernetes.io/instance=victoria-metrics-k8s-stack --ignore-not-found 2>/dev/null || true
    kubectl delete crd -l app.kubernetes.io/managed-by=victoria-metrics-operator --ignore-not-found 2>/dev/null || true
    removed=true
  fi

  # Previous interrupted test runs may leave Victoria resources even after the
  # Helm release record is gone. Delete only resources carrying the old release
  # label so unrelated monitoring resources are left intact.
  kubectl delete all -A -l app.kubernetes.io/instance=victoria-metrics-k8s-stack --ignore-not-found 2>/dev/null || true
  kubectl delete svc,ingress -A -l app.kubernetes.io/instance=victoria-metrics-k8s-stack --ignore-not-found 2>/dev/null || true
  kubectl delete all -A -l app.kubernetes.io/instance=prometheus-stack --ignore-not-found 2>/dev/null || true
  kubectl delete svc,ingress -A -l app.kubernetes.io/instance=prometheus-stack --ignore-not-found 2>/dev/null || true
  kubectl delete svc -n kube-system prometheus-stack-kube-prom-kubelet --ignore-not-found 2>/dev/null || true

  # Old VictoriaLogs
  if helm list -n "$ns" -q 2>/dev/null | grep -q "^victoria-logs$"; then
    warn "Removing previous victoria-logs release..."
    helm uninstall victoria-logs -n "$ns" 2>/dev/null || true
    removed=true
  fi

  # Old kube-prometheus-stack (if previously installed under prometheus-stack name)
  if helm list -n "$ns" -q 2>/dev/null | grep -q "^prometheus-stack$"; then
    warn "Removing previous prometheus-stack (kube-prometheus-stack) release..."
    helm uninstall prometheus-stack -n "$ns" 2>/dev/null || true
    # Clean up leftover ServiceMonitor CRDs from the operator
    kubectl delete crd servicemonitors.monitoring.coreos.com --ignore-not-found 2>/dev/null || true
    kubectl delete crd prometheusrules.monitoring.coreos.com --ignore-not-found 2>/dev/null || true
    kubectl delete crd podmonitors.monitoring.coreos.com --ignore-not-found 2>/dev/null || true
    kubectl delete crd probes.monitoring.coreos.com --ignore-not-found 2>/dev/null || true
    kubectl delete crd alertmanagers.monitoring.coreos.com --ignore-not-found 2>/dev/null || true
    kubectl delete crd thanosrulers.monitoring.coreos.com --ignore-not-found 2>/dev/null || true
    removed=true
  fi

  if $removed; then
    step "Cleaned up previous observability releases"
  fi
  return 0
}

install_prometheus_server() {
  local ns="$observability_namespace"

  header "Installing Prometheus server (standalone)"

  if $dry_run; then
    warn "[dry-run] helm repo add prometheus-community https://prometheus-community.github.io/helm-charts/"
    warn "[dry-run] helm upgrade --install ${PROMETHEUS_SERVER_RELEASE} prometheus-community/prometheus -n $ns --values ..."
    return 0
  fi

  helm repo add prometheus-community https://prometheus-community.github.io/helm-charts/ 2>/dev/null || true
  helm repo update prometheus-community 2>/dev/null || true

  local tmp_values
  tmp_values="$REPO_DIR/.bootstrap/prometheus-server-values.yaml"
  mkdir -p "$REPO_DIR/.bootstrap"

  cat > "$tmp_values" <<YAML
# Lightweight standalone Prometheus for single-node deployments
alertmanager:
  enabled: false

kube-state-metrics:
  enabled: false

prometheus-node-exporter:
  enabled: false

prometheus-pushgateway:
  enabled: false

server:
  enabled: true
  resources:
    requests:
      cpu: 100m
      memory: 256Mi
    limits:
      cpu: 300m
      memory: 512Mi
  retention: 7d
  global:
    scrape_interval: 30s
    evaluation_interval: 30s

# The chart's default Prometheus config already scrapes pods/services annotated
# with prometheus.io/scrape=true, so shclop uses Service annotations instead of
# ServiceMonitor CRDs.
YAML

  helm upgrade --install "${PROMETHEUS_SERVER_RELEASE}" prometheus-community/prometheus \
    --namespace "$ns" --create-namespace \
    --values "$tmp_values" \
    --wait

  rm -f "$tmp_values"
  step "Prometheus server installed"
}

install_grafana() {
  local ns="$observability_namespace"
  local host
  host="$(resolve_grafana_host)" || host=""

  header "Installing Grafana (standalone)"

  if $dry_run; then
    warn "[dry-run] helm repo add grafana https://grafana.github.io/helm-charts/"
    warn "[dry-run] helm upgrade --install ${GRAFANA_RELEASE} grafana/grafana -n $ns --values ..."
    return 0
  fi

  helm repo add grafana https://grafana.github.io/helm-charts/ 2>/dev/null || true
  helm repo update grafana 2>/dev/null || true

  local tmp_values
  tmp_values="$REPO_DIR/.bootstrap/grafana-values.yaml"
  mkdir -p "$REPO_DIR/.bootstrap"

  # Build Grafana ingress annotation/tls block
  local grafana_ingress_tls=""
  if [[ -n "$tls_email" ]]; then
    grafana_ingress_tls=$(cat <<YAML
ingress:
  enabled: true
  ingressClassName: ${INGRESS_CLASS}
  annotations:
    cert-manager.io/cluster-issuer: ${TLS_CLUSTER_ISSUER}
  hosts:
    - ${host}
  tls:
    - hosts:
        - ${host}
      secretName: grafana-${host//./-}-tls
YAML
)
  else
    grafana_ingress_tls=$(cat <<YAML
ingress:
  enabled: true
  ingressClassName: ${INGRESS_CLASS}
  hosts:
    - ${host}
YAML
)
  fi

  # Provision a basic home dashboard for QA validation. When sidecar is enabled,
  # Grafana discovers ConfigMaps with label grafana_dashboard: "1".
  local grafana_dashboard_provision=""
  grafana_dashboard_provision=$(cat <<YAML
sidecar:
  dashboards:
    enabled: true
    label: grafana_dashboard
dashboards:
  default:
    shclop-qa-overview:
      json: |
        {
          "title": "Shclop QA Overview",
          "uid": "shclop-qa-overview",
          "tags": ["shclop", "qa"],
          "timezone": "browser",
          "panels": [
            {
              "title": "Backend Status",
              "type": "stat",
              "datasource": "Prometheus",
              "gridPos": {"h": 6, "w": 8, "x": 0, "y": 0},
              "targets": [{"expr": "up{service=\"shclop-backend\"}", "refId": "A"}],
              "fieldConfig": {
                "defaults": {
                  "thresholds": {
                    "mode": "absolute",
                    "steps": [
                      {"color": "green", "value": null},
                      {"color": "red", "value": 0.5}
                    ]
                  }
                }
              }
            },
            {
              "title": "Container Memory",
              "type": "timeseries",
              "datasource": "Prometheus",
              "gridPos": {"h": 6, "w": 8, "x": 8, "y": 0},
              "targets": [{"expr": "container_memory_usage_bytes{container=\"shclop\"}", "refId": "A"}],
              "fieldConfig": {"defaults": {"unit": "bytes"}}
            },
            {
              "title": "Container CPU",
              "type": "timeseries",
              "datasource": "Prometheus",
              "gridPos": {"h": 6, "w": 8, "x": 16, "y": 0},
              "targets": [{"expr": "rate(container_cpu_usage_seconds_total{container=\"shclop\"}[1m]) * 100", "refId": "A"}],
              "fieldConfig": {"defaults": {"unit": "percent"}}
            },
            {
              "title": "Recent Shclop Logs",
              "type": "logs",
              "datasource": "Loki",
              "gridPos": {"h": 6, "w": 24, "x": 0, "y": 6},
              "targets": [{"expr": "{job=\"fluentbit\"} |= \"shclop\"", "refId": "A"}]
            }
          ],
          "schemaVersion": 39,
          "time": {"from": "now-1h", "to": "now"}
        }
YAML
)

  cat > "$tmp_values" <<YAML
# Lightweight standalone Grafana for single-node deployments
adminPassword: ${GRAFANA_ADMIN_PASSWORD:-admin}
defaultDashboardsEnabled: false
${grafana_ingress_tls}
${grafana_dashboard_provision}
datasources:
  datasources.yaml:
    apiVersion: 1
    datasources:
      - name: Prometheus
        type: prometheus
        url: http://${PROMETHEUS_SERVER_SERVICE:-prometheus-server-server}.${ns}.svc.cluster.local
        access: proxy
        isDefault: true
      - name: Loki
        type: loki
        url: http://${LOKI_RELEASE}.${ns}.svc.cluster.local:3100
        access: proxy
        isDefault: false
resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 200m
    memory: 256Mi
YAML

  helm upgrade --install "${GRAFANA_RELEASE}" grafana/grafana \
    --namespace "$ns" --create-namespace \
    --values "$tmp_values" \
    --wait

  rm -f "$tmp_values"
  step "Grafana installed"
}

install_loki() {
  local ns="$observability_namespace"

  header "Installing Loki (monolithic mode, filesystem storage)"

  if $dry_run; then
    warn "[dry-run] helm repo add grafana-community https://grafana-community.github.io/helm-charts"
    warn "[dry-run] helm upgrade --install ${LOKI_RELEASE} grafana-community/loki -n $ns --values ..."
    return 0
  fi

  helm repo add grafana-community https://grafana-community.github.io/helm-charts 2>/dev/null || true
  helm repo update grafana-community 2>/dev/null || true

  local tmp_values
  tmp_values="$REPO_DIR/.bootstrap/loki-values.yaml"
  mkdir -p "$REPO_DIR/.bootstrap"

  cat > "$tmp_values" <<YAML
# Monolithic Loki with filesystem storage — no MinIO, no gateway, no caches, no canary
deploymentMode: Monolithic

loki:
  commonConfig:
    replication_factor: 1
  storage:
    type: filesystem
  schemaConfig:
    configs:
      - from: 2024-01-01
        store: tsdb
        object_store: filesystem
        schema: v13
        index:
          prefix: index_
          period: 24h
  auth_enabled: false

monitoring:
  selfMonitoring:
    enabled: false
    grafanaAgent:
      enabled: false

lokiCanary:
  enabled: false

gateway:
  enabled: false

minio:
  enabled: false

chunksCache:
  enabled: false

resultsCache:
  enabled: false

singleBinary:
  replicas: 1
  resources:
    requests:
      cpu: 100m
      memory: 256Mi
    limits:
      cpu: 500m
      memory: 1Gi

# No extra persistence for demo/lightweight; ephemeral is fine with 7d retention.
persistence:
  enabled: false

backend:
  replicas: 0
read:
  replicas: 0
write:
  replicas: 0
ingester:
  replicas: 0
querier:
  replicas: 0
queryFrontend:
  replicas: 0
queryScheduler:
  replicas: 0
distributor:
  replicas: 0
compactor:
  replicas: 0
indexGateway:
  replicas: 0
bloomPlanner:
  replicas: 0
bloomBuilder:
  replicas: 0
bloomGateway:
  replicas: 0

test:
  enabled: false
YAML

  helm upgrade --install "${LOKI_RELEASE}" grafana-community/loki \
    --namespace "$ns" --create-namespace \
    --values "$tmp_values" \
    --wait

  rm -f "$tmp_values"
  step "Loki installed"
}

install_fluent_bit() {
  local ns="$observability_namespace"

  header "Installing Fluent Bit (DaemonSet, tailing container logs to Loki)"

  if $dry_run; then
    warn "[dry-run] helm repo add fluent https://fluent.github.io/helm-charts/"
    warn "[dry-run] helm upgrade --install ${FLUENT_BIT_RELEASE} fluent/fluent-bit -n $ns --values ..."
    return 0
  fi

  helm repo add fluent https://fluent.github.io/helm-charts/ 2>/dev/null || true
  helm repo update fluent 2>/dev/null || true

  local tmp_values
  tmp_values="$REPO_DIR/.bootstrap/fluent-bit-values.yaml"
  mkdir -p "$REPO_DIR/.bootstrap"

  # Fluent Bit DaemonSet — tails all container logs and forwards to Loki
  cat > "$tmp_values" <<YAML
daemonSet:
  enabled: true

config:
  service: |
    [SERVICE]
      Daemon Off
      Flush 1
      Log_Level info
      Parsers_File parsers.conf
      Parsers_File custom_parsers.conf
      HTTP_Server On
      HTTP_Listen 0.0.0.0
      HTTP_Port 2020

  inputs: |
    [INPUT]
      Name tail
      Path /var/log/containers/*.log
      multiline.parser docker, cri
      Tag kube.*
      Mem_Buf_Limit 5MB
      Skip_Long_Lines On

  filters: |
    [FILTER]
      Name kubernetes
      Match kube.*
      Merge_Log On
      Keep_Log Off
      K8S-Logging.Parser On
      K8S-Logging.Exclude On

  outputs: |
    [OUTPUT]
      Name loki
      Match *
      Host ${LOKI_RELEASE}.${ns}.svc.cluster.local
      Port 3100
      labels job=fluentbit
      auto_kubernetes_labels on

resources:
  requests:
    cpu: 50m
    memory: 64Mi
  limits:
    cpu: 200m
    memory: 256Mi

tolerations:
  - operator: Exists

# No runtimeClassName — observability pods run on standard containerd/runc
YAML

  helm upgrade --install "${FLUENT_BIT_RELEASE}" fluent/fluent-bit \
    --namespace "$ns" --create-namespace \
    --values "$tmp_values" \
    --wait

  rm -f "$tmp_values"
  step "Fluent Bit installed"
}

install_observability_stack() {
  if [[ "$enable_observability" != "true" ]]; then
    return 0
  fi

  info "Installing observability stack (Prometheus + Loki + Fluent Bit + Grafana)..."

  # Clean up old observability releases (VictoriaMetrics, VictoriaLogs, old kube-prometheus-stack)
  cleanup_old_observability

  # 1. Prometheus server (standalone — no CRD/operator dependency)
  install_prometheus_server

  # 2. Loki (log storage)
  install_loki

  # 3. Fluent Bit (log collection DaemonSet)
  install_fluent_bit

  # 4. Grafana (standalone — datasources configured for Prometheus and Loki)
  install_grafana

  step "Observability stack installed"
}

# ═══════════════════════════════════════════════════════════════
#  LITELLM GATEWAY
# ═══════════════════════════════════════════════════════════════
litellm_service_url() {
  echo "http://${LITELLM_RELEASE_NAME}.${LITELLM_NAMESPACE}.svc.cluster.local:4000/v1"
}

ensure_litellm_secrets() {
  local openrouter_key="${OPENROUTER_API_KEY:-${LITELLM_OPENROUTER_API_KEY:-}}"
  local master_key="${LITELLM_MASTER_KEY:-}"

  if kubectl get secret -n "$LITELLM_NAMESPACE" "$LITELLM_OPENROUTER_SECRET" >/dev/null 2>&1; then
    step "LiteLLM OpenRouter Secret exists: ${LITELLM_OPENROUTER_SECRET}"
  else
    [[ -n "$openrouter_key" ]] || fail "OPENROUTER_API_KEY or LITELLM_OPENROUTER_API_KEY is required to create LiteLLM Secret '${LITELLM_OPENROUTER_SECRET}'"
    kubectl create secret generic "$LITELLM_OPENROUTER_SECRET" \
      --namespace "$LITELLM_NAMESPACE" \
      --from-literal=OPENROUTER_API_KEY="$openrouter_key" \
      --dry-run=client -o yaml | kubectl apply -f - >/dev/null
    step "LiteLLM OpenRouter Secret created: ${LITELLM_OPENROUTER_SECRET}"
  fi

  if kubectl get secret -n "$LITELLM_NAMESPACE" "$LITELLM_MASTER_SECRET" >/dev/null 2>&1; then
    step "LiteLLM master Secret exists: ${LITELLM_MASTER_SECRET}"
  else
    if [[ -z "$master_key" ]]; then
      master_key="sk-$(openssl rand -hex 24)"
    fi
    kubectl create secret generic "$LITELLM_MASTER_SECRET" \
      --namespace "$LITELLM_NAMESPACE" \
      --from-literal=api-key="$master_key" \
      --dry-run=client -o yaml | kubectl apply -f - >/dev/null
    step "LiteLLM master Secret created: ${LITELLM_MASTER_SECRET}"
  fi
}

install_litellm_gateway() {
  if [[ "$enable_litellm" != "true" ]]; then
    return 0
  fi

  header "Installing LiteLLM gateway"

  if $dry_run; then
    warn "[dry-run] create LiteLLM OpenRouter and master Secrets in namespace ${LITELLM_NAMESPACE}"
    warn "[dry-run] helm upgrade --install ${LITELLM_RELEASE_NAME} oci://ghcr.io/berriai/litellm-helm -n ${LITELLM_NAMESPACE} --values ..."
    return 0
  fi

  mkdir -p "$REPO_DIR/.bootstrap"
  kubectl create namespace "$LITELLM_NAMESPACE" --dry-run=client -o yaml | kubectl apply -f - >/dev/null
  ensure_litellm_secrets

  local master_key values_file
  master_key="$(kubectl get secret -n "$LITELLM_NAMESPACE" "$LITELLM_MASTER_SECRET" -o jsonpath='{.data.api-key}' | base64 -d)"
  values_file="$REPO_DIR/.bootstrap/litellm-values.yaml"

  cat > "$values_file" <<YAML
image:
  repository: ghcr.io/berriai/litellm
  tag: "${LITELLM_IMAGE_TAG}"
  pullPolicy: IfNotPresent

service:
  type: ClusterIP
  port: 4000
  annotations:
    prometheus.io/scrape: "true"
    prometheus.io/port: "4000"
    prometheus.io/path: "/metrics/"

ingress:
  enabled: false

db:
  deployStandalone: false
  useExisting: false

redis:
  enabled: false

migrationJob:
  enabled: false

# Conservative resources for a 2 vCPU / 4 GB node.
# Requests move the pod to Burstable QoS, while limits allow LiteLLM's Python
# startup burst without starving K3s system or shclop workloads.
resources:
  requests:
    cpu: 150m
    memory: 512Mi
  limits:
    cpu: 750m
    memory: 2Gi

# More forgiving probes so cold starts and transient resource pressure do not
# trigger unnecessary restarts.
livenessProbe:
  path: /health/liveliness
  initialDelaySeconds: 10
  periodSeconds: 20
  timeoutSeconds: 5
  successThreshold: 1
  failureThreshold: 6

readinessProbe:
  path: /health/readiness
  initialDelaySeconds: 5
  periodSeconds: 15
  timeoutSeconds: 5
  successThreshold: 1
  failureThreshold: 10

startupProbe:
  path: /health/readiness
  initialDelaySeconds: 0
  periodSeconds: 10
  timeoutSeconds: 5
  successThreshold: 1
  failureThreshold: 60

masterkey: "${master_key}"

environmentSecrets:
  - ${LITELLM_OPENROUTER_SECRET}

proxy_config:
  model_list:
    - model_name: ${LITELLM_MODEL_ALIAS}
      litellm_params:
        model: openrouter/${LITELLM_OPENROUTER_MODEL}
        api_key: os.environ/OPENROUTER_API_KEY
  general_settings:
    master_key: os.environ/PROXY_MASTER_KEY
  litellm_settings:
    require_auth_for_metrics_endpoint: false
    callbacks:
      - prometheus
YAML

  helm upgrade --install "$LITELLM_RELEASE_NAME" oci://ghcr.io/berriai/litellm-helm \
    --namespace "$LITELLM_NAMESPACE" --create-namespace \
    --values "$values_file" \
    --wait

  step "LiteLLM gateway installed: $(litellm_service_url)"
}


# ═══════════════════════════════════════════════════════════════
#  DEPLOY
# ═══════════════════════════════════════════════════════════════
deploy_shclop() {
  header "Deploy shclop"

  # Check Helm chart presence
  if [[ ! -d "$CHARTS_DIR/shclop" ]]; then
    fail "Helm chart not found: $CHARTS_DIR/shclop (run the script from the repository root)"
  fi

  # Generate values when none are provided
  local values_file=""
  if [[ -n "$values" ]]; then
    values_file="$values"
  else
    values_file="$REPO_DIR/.bootstrap/shclop-bootstrap-values.yaml"
    if $dry_run; then
      warn "[dry-run] create values.yaml with default settings"
      return 0
    fi
    mkdir -p "$REPO_DIR/.bootstrap"
    generate_default_values "$values_file"
  fi

  local helm_args=("$HELM_RELEASE_NAME" "$CHARTS_DIR/shclop" --namespace "$SHCLOP_NAMESPACE" -f "$values_file")
  if [[ "$enable_litellm" == "true" ]]; then
    helm_args+=(
      --set "llmGateway.litellm.enabled=true"
      --set "llmGateway.litellm.serviceName=${LITELLM_RELEASE_NAME}"
      --set "llmGateway.litellm.namespace=${LITELLM_NAMESPACE}"
      --set "llmGateway.baseURL=$(litellm_service_url)"
      --set "llmGateway.existingSecret.name=${LITELLM_MASTER_SECRET}"
      --set "llmGateway.existingSecret.key=api-key"
    )
  fi

  if [[ "$enable_observability" == "true" ]]; then
    # Standalone Prometheus scrapes Service annotations; no ServiceMonitor CRD is installed.
    helm_args+=(--set "monitoring.serviceMonitor.enabled=false")
  fi

  if [[ -n "$IMAGE_REPO" ]]; then
    # Explicit flags must override any stale values file so runtime pods use the
    # same prebuilt registry tag as the backend image.
    helm_args+=(
      --set "image.repository=${IMAGE_REPO}"
      --set "image.tag=${IMAGE_TAG}"
      --set "agentRuntime.images.openclaw=${IMAGE_REPO}-runtime-openclaw:${IMAGE_TAG}"
      --set "agentRuntime.images.nanoclaw=${IMAGE_REPO}-runtime-nanoclaw:${IMAGE_TAG}"
    )
  else
    helm_args+=(
      --set "image.tag=${IMAGE_TAG}"
      --set "agentRuntime.images.openclaw=shclop-runtime-openclaw:${IMAGE_TAG}"
      --set "agentRuntime.images.nanoclaw=shclop-runtime-nanoclaw:${IMAGE_TAG}"
    )
  fi

  if $dry_run; then
    warn "[dry-run] helm upgrade --install ${helm_args[*]} --create-namespace --wait"
    return 0
  fi

  # Check Helm
  if ! command -v helm &>/dev/null; then
    info "Helm not found, installing..."
    curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
    step "Helm installed"
  fi

  # Install or upgrade
  info "Installing or upgrading shclop..."
  helm upgrade --install "${helm_args[@]}" --create-namespace --wait
  step "shclop deployed"

  # Show status
  echo ""
  kubectl get pods -n "$SHCLOP_NAMESPACE" -l app.kubernetes.io/instance="$HELM_RELEASE_NAME"
  echo ""

  local svc_port
  if [[ "$enable_ingress" == "true" ]]; then
    local host scheme
    host="$(resolve_ingress_host)"
    scheme="http"
    [[ -n "$tls_email" ]] && scheme="https"
    info "shclop UI: ${scheme}://${host} (login: admin/admin)"
  else
    svc_port="$(kubectl get svc -n "$SHCLOP_NAMESPACE" "${HELM_RELEASE_NAME}-backend" -o jsonpath='{.spec.ports[0].nodePort}' 2>/dev/null || echo "8080")"
    info "shclop UI: http://localhost:${svc_port} (login: admin/admin)"
  fi
}

generate_default_values() {
  local out="$1"
  local service_type="NodePort"
  if [[ "$enable_ingress" == "true" ]]; then
    service_type="ClusterIP"
  fi

  cat > "$out" <<VALUESYAML
# Automatically generated by bootstrap.sh
config:
  store: inmemory
  logLevel: info

service:
  type: ${service_type}

VALUESYAML

  if [[ "$enable_ingress" == "true" ]]; then
    local host tls_enabled
    host="$(resolve_ingress_host)"
    tls_enabled="false"
    [[ -n "$tls_email" ]] && tls_enabled="true"
    cat >> "$out" <<VALUESYAML
ingress:
  enabled: true
  className: ${INGRESS_CLASS}
  host: ${host}
  tls:
    enabled: ${tls_enabled}
    clusterIssuer: ${TLS_CLUSTER_ISSUER}

VALUESYAML
  fi

  cat >> "$out" <<VALUESYAML

sandbox:
  provider: kubernetes
  kubernetes:
    namespace: $SHCLOP_NAMESPACE
    gatewayURL: ws://shclop-backend:8080/runtime/ws
    networkPolicy:
      enabled: true
      mode: restricted

agentRuntime:
  runtimeClassName: kata
VALUESYAML

  if [[ "$enable_litellm" == "true" ]]; then
    cat >> "$out" <<VALUESYAML

llmGateway:
  baseURL: $(litellm_service_url)
  litellm:
    enabled: true
    serviceName: ${LITELLM_RELEASE_NAME}
    namespace: ${LITELLM_NAMESPACE}
    port: 4000
  existingSecret:
    name: ${LITELLM_MASTER_SECRET}
    key: api-key
VALUESYAML
  fi

  # Add image repo when provided
  if [[ -n "$IMAGE_REPO" ]]; then
    cat >> "$out" <<VALUESYAML

image:
  repository: $IMAGE_REPO
  tag: $IMAGE_TAG

agentRuntime:
  runtimeClassName: kata
  images:
    nanoclaw: ${IMAGE_REPO}-runtime-nanoclaw:${IMAGE_TAG}
    openclaw: ${IMAGE_REPO}-runtime-openclaw:${IMAGE_TAG}
VALUESYAML
  else
    # No image repository provided; use bare image names (tag overridable via IMAGE_TAG)
    cat >> "$out" <<VALUESYAML

image:
  tag: ${IMAGE_TAG}

agentRuntime:
  runtimeClassName: kata
  images:
    nanoclaw: shclop-runtime-nanoclaw:${IMAGE_TAG}
    openclaw: shclop-runtime-openclaw:${IMAGE_TAG}
VALUESYAML
  fi

  if [[ "$enable_observability" == "true" ]]; then
    local grafana_url
    grafana_url="https://$(resolve_grafana_host)"
    [[ -z "$tls_email" ]] && grafana_url="http://$(resolve_grafana_host)"
    cat >> "$out" <<VALUESYAML
monitoring:
  serviceMonitor:
    enabled: false

observability:
  retentionDays: 7
  prometheus:
    enabled: true
    releaseName: ${PROMETHEUS_SERVER_RELEASE:-prometheus-server}
  loki:
    enabled: true
    releaseName: ${LOKI_RELEASE:-loki}
  fluentBit:
    enabled: true
    releaseName: ${FLUENT_BIT_RELEASE:-fluent-bit}
  grafana:
    enabled: true
    releaseName: ${GRAFANA_RELEASE:-grafana}
    url: ${grafana_url}

VALUESYAML
  fi

  step "Generated values.yaml: $out"
}

action_install() {
  require_root
  validate_ingress_config
  check_hardware
  check_kvm

  # 1. System dependencies
  if $install_deps; then
    info "Phase: installing system dependencies"
    install_k3s
    install_kata_ubuntu
    tune_kata_configuration
    configure_containerd_kata
    wait_for_k3s
    install_helm
  elif $dry_run; then
    warn "[dry-run] --install-deps not set, showing general plan"
    warn "[dry-run] install K3s"
    warn "[dry-run] install Kata Containers"
    warn "[dry-run] configure containerd for Kata"
    warn "[dry-run] restart K3s"
    warn "[dry-run] install Helm"
  else
    info "Phase: checking existing dependencies"
    if ! is_k3s_installed || ! is_k3s_running; then
      fail "K3s is not installed or not running. Run with --install-deps or install K3s manually."
    fi
    if ! is_kata_installed; then
      fail "Kata Containers are not installed. Run with --install-deps or install manually."
    fi
    step "K3s and Kata are ready"
  fi

  # 2. RuntimeClass
  create_runtimeclass

  # 2b. Validate Kata RuntimeClass with smoke test
  validate_kata_runtimeclass

  # 3. Ingress TLS (optional)
  install_cert_manager
  create_clusterissuer

  # 4. Internal LLM gateway (optional)
  install_litellm_gateway

  # 5. Observability stack (optional)
  # Standalone Prometheus + Grafana; no CRD/operator dependency so order relative to shclop doesn't matter.
  install_observability_stack

  # 6. Deploy shclop
  deploy_shclop

  if ! $dry_run; then
    echo ""
    info "Installation complete"

    # Show Grafana URL if observability was installed
    if [[ "$enable_observability" == "true" ]]; then
      local grafana_url
      grafana_url="https://$(resolve_grafana_host)"
      [[ -z "$tls_email" ]] && grafana_url="http://$(resolve_grafana_host)"
      step "Grafana: ${grafana_url} (login: admin/admin)"
    fi
  fi
}

# ═══════════════════════════════════════════════════════════════
#  DESTROY
# ═══════════════════════════════════════════════════════════════
destroy_helm() {
  header "Remove Helm release"
  if ! command -v helm &>/dev/null; then
    warn "Helm not found, skipping"
    return 0
  fi
  if ! helm list -n "$SHCLOP_NAMESPACE" -q 2>/dev/null | grep -q "^${HELM_RELEASE_NAME}$"; then
    warn "Release '$HELM_RELEASE_NAME' not found"
    return 0
  fi
  if $dry_run; then
    warn "[dry-run] helm uninstall $HELM_RELEASE_NAME -n $SHCLOP_NAMESPACE"
    return 0
  fi
  if [[ "$yes" != "true" ]]; then
    read -r -p "Delete Helm release '$HELM_RELEASE_NAME'? [y/N] " confirm
    [[ "$confirm" =~ ^[yY] ]] || { info "skipped"; return 0; }
  fi
  helm uninstall "$HELM_RELEASE_NAME" -n "$SHCLOP_NAMESPACE" 2>/dev/null || true
  kubectl delete namespace "$SHCLOP_NAMESPACE" --ignore-not-found 2>/dev/null || true
  step "Helm release deleted"
}

destroy_k3s() {
  if ! $remove_k3s; then
    info "Skipping K3s removal (use --remove-k3s)"
    return 0
  fi
  header "Remove K3s"
  if ! is_k3s_installed; then
    warn "K3s is not installed"
    return 0
  fi
  if $dry_run; then
    warn "[dry-run] /usr/local/bin/k3s-uninstall.sh"
    return 0
  fi
  if [[ "$yes" != "true" ]]; then
    read -r -p "Delete K3s and all workloads? [y/N] " confirm
    [[ "$confirm" =~ ^[yY] ]] || { info "skipped"; return 0; }
  fi
  if [[ -f /usr/local/bin/k3s-uninstall.sh ]]; then
    bash /usr/local/bin/k3s-uninstall.sh
  elif command -v k3s &>/dev/null; then
    /usr/local/bin/k3s-uninstall.sh 2>/dev/null || \
      curl -sfL https://get.k3s.io | sh -s - --uninstall
  fi
  step "K3s deleted"
}

destroy_kata() {
  if ! $remove_kata; then
    info "Skipping Kata Containers removal (use --remove-kata)"
    return 0
  fi
  header "Remove Kata Containers"
  if ! is_kata_installed; then
    warn "Kata is not installed"
    return 0
  fi
  if $dry_run; then
    warn "[dry-run] remove kata-runtime"
    return 0
  fi
  if [[ "$yes" != "true" ]]; then
    read -r -p "Delete Kata Containers? [y/N] " confirm
    [[ "$confirm" =~ ^[yY] ]] || { info "skipped"; return 0; }
  fi
  # Ubuntu/Debian
  if dpkg -l kata-runtime &>/dev/null 2>&1; then
    apt-get remove -y kata-runtime
    rm -f /etc/apt/sources.list.d/kata.list
  fi
  # Snap
  snap remove kata-containers 2>/dev/null || true
  step "Kata Containers deleted"
}

destroy_data() {
  if ! $purge_data; then
    info "Skipping data removal (use --purge-data)"
    return 0
  fi
  header "Clean data"
  if $dry_run; then
    warn "[dry-run] delete PVCs and workspace data"
    return 0
  fi
  if [[ "$yes" != "true" ]]; then
    read -r -p "Delete all PVCs and workspace data? [y/N] " confirm
    [[ "$confirm" =~ ^[yY] ]] || { info "skipped"; return 0; }
  fi
  kubectl delete pvc --all -n "$SHCLOP_NAMESPACE" 2>/dev/null || true
  kubectl delete pods --all -n "$SHCLOP_NAMESPACE" 2>/dev/null || true
  rm -rf /var/lib/rancher/k3s/storage/* 2>/dev/null || true
  step "Data cleaned"
}

action_destroy() {
  require_root
  if [[ "$yes" != "true" ]]; then
    echo ""
    warn "⚠️  This will delete shclop and all related resources."
    read -r -p "Type 'delete shclop' to confirm: " confirm
    [[ "$confirm" == "delete shclop" ]] || { fail "Cancelled"; }
  fi
  destroy_helm
  destroy_data
  destroy_kata
  destroy_k3s
  echo ""
  info "✅ Removal completed"
}

# ═══════════════════════════════════════════════════════════════
#  RESET
# ═══════════════════════════════════════════════════════════════
action_reset() {
  require_root
  info "Reset: deleting Helm release and reinstalling..."
  # Fast destroy without confirmations (Helm only)
  local old_yes="$yes"
  yes=true
  purge_data=false
  remove_k3s=false
  remove_kata=false
  destroy_helm
  yes="$old_yes"
  action_install
  echo ""
  info "✅ Reset completed"
}

# ═══════════════════════════════════════════════════════════════
#  MAIN DISPATCH
# ═══════════════════════════════════════════════════════════════
run_local() {
  case "$action" in
    check)   action_check ;;
    install) action_install ;;
    reset)   action_reset ;;
    destroy) action_destroy ;;
  esac
}

# Remote mode: send this script to the remote host and re-run
if [[ -n "$remote" ]]; then
  remote_argv=("$action")
  $dry_run      && remote_argv+=("--dry-run")
  $install_deps && remote_argv+=("--install-deps")
  $yes          && remote_argv+=("--yes")
  $purge_data   && remote_argv+=("--purge-data")
  $remove_k3s   && remote_argv+=("--remove-k3s")
  $remove_kata  && remote_argv+=("--remove-kata")
  $enable_ingress && remote_argv+=("--enable-ingress")
  $enable_observability && remote_argv+=("--enable-observability")
  $enable_litellm && remote_argv+=("--enable-litellm")
  [[ -n "$values" ]] && remote_argv+=("--values" "$values")
  [[ -n "$IMAGE_REPO" ]] && remote_argv+=("--image-repo" "$IMAGE_REPO")
  [[ -n "$IMAGE_TAG" ]] && remote_argv+=("--image-tag" "$IMAGE_TAG")
  [[ -n "$public_ip" ]] && remote_argv+=("--public-ip" "$public_ip")
  [[ -n "$ingress_host" ]] && remote_argv+=("--host" "$ingress_host")
  [[ -n "$tls_email" ]] && remote_argv+=("--tls-email" "$tls_email")
  [[ -n "$INGRESS_CLASS" ]] && remote_argv+=("--ingress-class" "$INGRESS_CLASS")
  [[ -n "$TLS_CLUSTER_ISSUER" ]] && remote_argv+=("--cluster-issuer" "$TLS_CLUSTER_ISSUER")
  [[ -n "$observability_namespace" && "$observability_namespace" != "$OBSERVABILITY_NAMESPACE" ]] && remote_argv+=("--observability-namespace" "$observability_namespace")
  [[ -n "$grafana_host" ]] && remote_argv+=("--grafana-host" "$grafana_host")
  [[ -n "$LITELLM_NAMESPACE" && "$LITELLM_NAMESPACE" != "$SHCLOP_NAMESPACE" ]] && remote_argv+=("--litellm-namespace" "$LITELLM_NAMESPACE")
  [[ -n "$LITELLM_RELEASE_NAME" && "$LITELLM_RELEASE_NAME" != "litellm" ]] && remote_argv+=("--litellm-release" "$LITELLM_RELEASE_NAME")
  [[ -n "$LITELLM_OPENROUTER_MODEL" && "$LITELLM_OPENROUTER_MODEL" != "deepseek/deepseek-v4-flash:free" ]] && remote_argv+=("--litellm-model" "$LITELLM_OPENROUTER_MODEL")

  info "Running on $remote..."
  ssh "$remote" "bash -s -- $(printf '%q ' "${remote_argv[@]}")" < "$0"
else
  run_local
fi
