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
VICTORIA_METRICS_STACK_VERSION="${VICTORIA_METRICS_STACK_VERSION:-}"
VICTORIA_LOGS_VERSION="${VICTORIA_LOGS_VERSION:-}"

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
  --enable-observability  Install recommended observability stack (VictoriaMetrics k8s-stack, VictoriaLogs, Grafana)
  --observability-namespace NS  Namespace for observability components (default: monitoring)
  --grafana-host HOST     Explicit Grafana hostname (default: grafana.<public-ip>.nip.io)
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
    warn "[dry-run] installing kata-runtime from openSUSE repository"
    return 0
  fi
  info "Adding Kata repository..."
  local arch
  arch="$(uname -m)"

  local os_codename
  os_codename="$(. /etc/os-release && echo "$VERSION_CODENAME")"
  [[ -z "$os_codename" ]] && os_codename="noble"

  local repo_url="https://download.opensuse.org/repositories/home:/katacontainers:/releases:/$(arch):/${KATA_VERSION}/xUbuntu_$(lsb_release -rs 2>/dev/null || echo '24.04')"

  apt-get update -qq
  apt-get install -y -qq software-properties-common apt-transport-https ca-certificates

  echo "deb [signed-by=/usr/share/keyrings/kata-archive-keyring.gpg] ${repo_url}/ /" > /etc/apt/sources.list.d/kata.list

  curl -fsSL "${repo_url}/Release.key" 2>/dev/null | gpg --dearmor -o /usr/share/keyrings/kata-archive-keyring.gpg 2>/dev/null

  apt-get update -qq
  apt-get install -y -qq kata-runtime 2>/dev/null || {
    warn "Repository install failed, trying snap..."
    snap install kata-containers --classic 2>/dev/null && {
      ln -sf /snap/kata-containers/current/bin/kata-runtime /usr/local/bin/kata-runtime
    } || warn "Failed to install kata-runtime. Install manually: https://github.com/kata-containers/kata-containers"
  }

  if is_kata_installed; then
    step "kata-runtime installed: $(kata_version)"
  else
    warn "kata-runtime not found after install. Continuing, but nested virt may not work."
  fi
}

configure_containerd_kata() {
  header "Configuring containerd for Kata"
  if $dry_run; then
    warn "[dry-run] creating $K3S_CONTAINERD_DIR/config.toml.tmpl with kata runtime"
    return 0
  fi

  mkdir -p "$K3S_CONTAINERD_DIR"

  if [[ -f "$K3S_CONTAINERD_DIR/config.toml.tmpl" ]] && grep -q "kata" "$K3S_CONTAINERD_DIR/config.toml.tmpl" 2>/dev/null; then
    step "Kata already in containerd template"
    return 0
  fi

  local tmpl="$K3S_CONTAINERD_DIR/config.toml.tmpl"

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
{{ template "containerd" . }}

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
#  OBSERVABILITY
# ═══════════════════════════════════════════════════════════════
install_vm_k8s_stack() {
  local ns="$observability_namespace"
  local host
  host="$(resolve_grafana_host)" || host=""

  header "Installing VictoriaMetrics k8s-stack"

  if $dry_run; then
    warn "[dry-run] helm repo add vm https://victoriametrics.github.io/helm-charts/"
    warn "[dry-run] helm upgrade --install victoria-metrics-k8s-stack vm/victoria-metrics-k8s-stack -n $ns --values ..."
    return 0
  fi

  helm repo add vm https://victoriametrics.github.io/helm-charts/ 2>/dev/null || true
  helm repo update vm 2>/dev/null || true

  local tmp_values
  tmp_values="$(mktemp)"

  cat > "$tmp_values" <<YAML
grafana:
  enabled: true
  adminPassword: admin
  ingress:
    enabled: true
    ingressClassName: ${INGRESS_CLASS}
    hosts:
      - ${host}
YAML

  if [[ -n "$tls_email" ]]; then
    cat >> "$tmp_values" <<YAML
    annotations:
      cert-manager.io/cluster-issuer: ${TLS_CLUSTER_ISSUER}
    tls:
      - hosts:
          - ${host}
        secretName: grafana-${host//./-}-tls
YAML
  fi

  cat >> "$tmp_values" <<YAML
  additionalDataSources:
    - name: VictoriaLogs
      type: victorialogs
      url: http://victoria-logs:9428
      access: proxy
      isDefault: false

victoria-metrics:
  single:
    enabled: true

prometheus:
  enabled: false

defaultDashboards:
  enabled: true
YAML

  local ver_flag=()
  [[ -n "$VICTORIA_METRICS_STACK_VERSION" ]] && ver_flag=(--version "$VICTORIA_METRICS_STACK_VERSION")

  helm upgrade --install victoria-metrics-k8s-stack vm/victoria-metrics-k8s-stack \
    --namespace "$ns" --create-namespace \
    --values "$tmp_values" \
    "${ver_flag[@]}" \
    --wait

  rm -f "$tmp_values"
  step "VictoriaMetrics k8s-stack installed"
}

install_victoria_logs() {
  local ns="$observability_namespace"

  header "Installing VictoriaLogs"

  if $dry_run; then
    warn "[dry-run] helm upgrade --install victoria-logs vm/victoria-logs -n $ns"
    return 0
  fi

  helm repo add vm https://victoriametrics.github.io/helm-charts/ 2>/dev/null || true
  helm repo update vm 2>/dev/null || true

  local ver_flag=()
  [[ -n "$VICTORIA_LOGS_VERSION" ]] && ver_flag=(--version "$VICTORIA_LOGS_VERSION")

  helm upgrade --install victoria-logs vm/victoria-logs \
    --namespace "$ns" --create-namespace \
    "${ver_flag[@]}" \
    --wait

  step "VictoriaLogs installed"
}

install_observability_stack() {
  if [[ "$enable_observability" != "true" ]]; then
    return 0
  fi

  info "Installing observability stack (VictoriaMetrics, VictoriaLogs, Grafana)..."
  install_vm_k8s_stack
  install_victoria_logs
}

# ═══════════════════════════════════════════════════════════════
#  DEPLOY
# ═══════════════════════════════════════════════════════════════
deploy_shclop() {
  header "Деплой shclop"

  # Проверяем наличие Helm chart
  if [[ ! -d "$CHARTS_DIR/shclop" ]]; then
    fail "Helm chart не найден: $CHARTS_DIR/shclop (запусти скрипт из корня репозитория)"
  fi

  # Генерируем values, если не переданы
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

  if $dry_run; then
    warn "[dry-run] helm install $HELM_RELEASE_NAME $CHARTS_DIR/shclop -f $values_file"
    return 0
  fi

  # Проверяем helm
  if ! command -v helm &>/dev/null; then
    info "Helm не найден, устанавливаю..."
    curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
    step "Helm установлен"
  fi

  # Устанавливаем или обновляем
  if helm list -n "$SHCLOP_NAMESPACE" -q 2>/dev/null | grep -q "^${HELM_RELEASE_NAME}$"; then
    info "Релиз '$HELM_RELEASE_NAME' уже существует, обновляю..."
    helm upgrade "$HELM_RELEASE_NAME" "$CHARTS_DIR/shclop" \
      --namespace "$SHCLOP_NAMESPACE" \
      -f "$values_file" \
      --wait
    step "shclop обновлён"
  else
    info "Устанавливаю shclop..."
    helm install "$HELM_RELEASE_NAME" "$CHARTS_DIR/shclop" \
      --namespace "$SHCLOP_NAMESPACE" \
      --create-namespace \
      -f "$values_file" \
      --wait
    step "shclop установлен"
  fi

  # Показываем статус
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

  # Добавляем image repo, если указан
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
    # Default dev images
    cat >> "$out" <<VALUESYAML

agentRuntime:
  runtimeClassName: kata
  images:
    nanoclaw: shclop-runtime-nanoclaw:latest
    openclaw: shclop-runtime-openclaw:latest
VALUESYAML
  fi

  if [[ "$enable_observability" == "true" ]]; then
    local grafana_url
    grafana_url="https://$(resolve_grafana_host)"
    [[ -z "$tls_email" ]] && grafana_url="http://$(resolve_grafana_host)"
    cat >> "$out" <<VALUESYAML
monitoring:
  serviceMonitor:
    enabled: true

observability:
  retentionDays: 7
  victoriaMetrics:
    enabled: true
    releaseName: victoria-metrics-k8s-stack
  victoriaLogs:
    enabled: true
    releaseName: victoria-logs
  grafana:
    enabled: true
    releaseName: grafana
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

  # 1. Системные зависимости
  if $install_deps; then
    info "Фаза: установка системных зависимостей"
    install_k3s
    install_kata_ubuntu
    configure_containerd_kata
    wait_for_k3s
  elif $dry_run; then
    warn "[dry-run] без --install-deps не могу проверить K3s/Kata, показываю общий план"
    warn "[dry-run] установка K3s"
    warn "[dry-run] установка Kata Containers"
    warn "[dry-run] настройка containerd для Kata"
    warn "[dry-run] рестарт K3s"
  else
    info "Фаза: проверка существующих зависимостей"
    if ! is_k3s_installed || ! is_k3s_running; then
      fail "K3s не установлен/не запущен. Запусти с --install-deps или установи K3s вручную"
    fi
    if ! is_kata_installed; then
      fail "Kata Containers не установлены. Запусти с --install-deps или установи вручную"
    fi
    step "K3s и Kata в порядке"
  fi

  # 2. RuntimeClass
  create_runtimeclass

  # 3. Ingress TLS (optional)
  install_cert_manager
  create_clusterissuer

  # 4. Деплой shclop
  deploy_shclop

  # 5. Observability stack (optional)
  install_observability_stack

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
  header "Удаление Helm release"
  if ! command -v helm &>/dev/null; then
    warn "Helm не найден, пропускаю"
    return 0
  fi
  if ! helm list -n "$SHCLOP_NAMESPACE" -q 2>/dev/null | grep -q "^${HELM_RELEASE_NAME}$"; then
    warn "Релиз '$HELM_RELEASE_NAME' не найден"
    return 0
  fi
  if $dry_run; then
    warn "[dry-run] helm uninstall $HELM_RELEASE_NAME -n $SHCLOP_NAMESPACE"
    return 0
  fi
  if [[ "$yes" != "true" ]]; then
    read -r -p "Удалить Helm релиз '$HELM_RELEASE_NAME'? [y/N] " confirm
    [[ "$confirm" =~ ^[yY] ]] || { info "пропущено"; return 0; }
  fi
  helm uninstall "$HELM_RELEASE_NAME" -n "$SHCLOP_NAMESPACE" 2>/dev/null || true
  kubectl delete namespace "$SHCLOP_NAMESPACE" --ignore-not-found 2>/dev/null || true
  step "Helm релиз удалён"
}

destroy_k3s() {
  if ! $remove_k3s; then
    info "Пропускаю удаление K3s (используй --remove-k3s)"
    return 0
  fi
  header "Удаление K3s"
  if ! is_k3s_installed; then
    warn "K3s не установлен"
    return 0
  fi
  if $dry_run; then
    warn "[dry-run] /usr/local/bin/k3s-uninstall.sh"
    return 0
  fi
  if [[ "$yes" != "true" ]]; then
    read -r -p "Удалить K3s и все workloads? [y/N] " confirm
    [[ "$confirm" =~ ^[yY] ]] || { info "пропущено"; return 0; }
  fi
  if [[ -f /usr/local/bin/k3s-uninstall.sh ]]; then
    bash /usr/local/bin/k3s-uninstall.sh
  elif command -v k3s &>/dev/null; then
    /usr/local/bin/k3s-uninstall.sh 2>/dev/null || \
      curl -sfL https://get.k3s.io | sh -s - --uninstall
  fi
  step "K3s удалён"
}

destroy_kata() {
  if ! $remove_kata; then
    info "Пропускаю удаление Kata Containers (используй --remove-kata)"
    return 0
  fi
  header "Удаление Kata Containers"
  if ! is_kata_installed; then
    warn "Kata не установлен"
    return 0
  fi
  if $dry_run; then
    warn "[dry-run] удаление kata-runtime"
    return 0
  fi
  if [[ "$yes" != "true" ]]; then
    read -r -p "Удалить Kata Containers? [y/N] " confirm
    [[ "$confirm" =~ ^[yY] ]] || { info "пропущено"; return 0; }
  fi
  # Ubuntu/Debian
  if dpkg -l kata-runtime &>/dev/null 2>&1; then
    apt-get remove -y kata-runtime
    rm -f /etc/apt/sources.list.d/kata.list
  fi
  # Snap
  snap remove kata-containers 2>/dev/null || true
  step "Kata Containers удалён"
}

destroy_data() {
  if ! $purge_data; then
    info "Пропускаю удаление данных (используй --purge-data)"
    return 0
  fi
  header "Очистка данных"
  if $dry_run; then
    warn "[dry-run] удаление PVC, workspace данных"
    return 0
  fi
  if [[ "$yes" != "true" ]]; then
    read -r -p "Удалить все PVC и workspace данные? [y/N] " confirm
    [[ "$confirm" =~ ^[yY] ]] || { info "пропущено"; return 0; }
  fi
  kubectl delete pvc --all -n "$SHCLOP_NAMESPACE" 2>/dev/null || true
  kubectl delete pods --all -n "$SHCLOP_NAMESPACE" 2>/dev/null || true
  rm -rf /var/lib/rancher/k3s/storage/* 2>/dev/null || true
  step "Данные очищены"
}

action_destroy() {
  require_root
  if [[ "$yes" != "true" ]]; then
    echo ""
    warn "⚠️  Это удалит shclop и все связанные ресурсы."
    read -r -p "Введи 'delete shclop' для подтверждения: " confirm
    [[ "$confirm" == "delete shclop" ]] || { fail "Отменено"; }
  fi
  destroy_helm
  destroy_data
  destroy_kata
  destroy_k3s
  echo ""
  info "✅ Удаление завершено"
}

# ═══════════════════════════════════════════════════════════════
#  RESET
# ═══════════════════════════════════════════════════════════════
action_reset() {
  require_root
  info "Reset: удаляю Helm release и переустанавливаю..."
  # Быстрый destroy без подтверждений (только Helm)
  local old_yes="$yes"
  yes=true
  purge_data=false
  remove_k3s=false
  remove_kata=false
  destroy_helm
  yes="$old_yes"
  action_install
  echo ""
  info "✅ Reset завершён"
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

  info "Выполнение на $remote..."
  ssh "$remote" "bash -s -- $(printf '%q ' "${remote_argv[@]}")" < "$0"
else
  run_local
fi
