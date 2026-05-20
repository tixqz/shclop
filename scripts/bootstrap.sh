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
  --dry-run            Print actions without executing
  --yes                Skip confirmations (for destroy)
  --purge-data         Also remove PVCs, workspace data (for destroy)
  --remove-k3s         Remove K3s (for destroy)
  --remove-kata        Remove Kata (for destroy)
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
    *) echo "unknown argument: $1" >&2; usage; exit 2 ;;
  esac
done

case "$action" in check|install|reset|destroy) ;; *) echo "unknown action: $action" >&2; usage; exit 2 ;; esac

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
check_kvm() {
  info "KVM"
  if [[ -e /dev/kvm ]]; then
    step "/dev/kvm доступен"
  else
    warn "/dev/kvm не найден — Kata будет работать без аппаратной виртуализации (медленно)"
  fi
}

check_k3s() {
  header "K3s / Kubernetes"
  if is_k3s_installed; then
    step "k3s установлен: $(k3s_version)"
    if is_k3s_running; then
      step "k3s запущен"
      if kubectl get nodes &>/dev/null; then
        step "kubectl работает, node: $(kubectl get nodes -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo 'unknown')"
      else
        warn "kubectl не может подключиться к кластеру"
      fi
    else
      warn "k3s не запущен"
    fi
  else
    warn "k3s не установлен"
  fi
}

check_kata() {
  header "Kata Containers"
  if is_kata_installed; then
    step "kata-runtime установлен: $(kata_version)"
    if kata-runtime check 2>&1 | grep -qi "version"; then
      step "kata-runtime check пройден"
    else
      warn "kata-runtime check: смотри вывод выше"
      kata-runtime check 2>&1 | head -5
    fi
  else
    warn "kata-runtime не установлен"
  fi
}

check_containerd_runtime() {
  header "Containerd runtime (Kata)"
  local tmpl="$K3S_CONTAINERD_DIR/config.toml.tmpl"
  local cfg="$K3S_CONTAINERD_DIR/config.toml"

  if [[ -f "$tmpl" ]] && grep -q "kata" "$tmpl" 2>/dev/null; then
    step "Kata runtime прописан в containerd template"
  elif [[ -f "$cfg" ]] && grep -q "kata" "$cfg" 2>/dev/null; then
    step "Kata runtime есть в containerd config"
  else
    warn "Kata runtime не найден в конфиге containerd"
  fi
}

check_runtimeclass() {
  header "Kubernetes RuntimeClass"
  if kubectl get runtimeclass kata &>/dev/null 2>&1; then
    step "RuntimeClass 'kata' существует"
  else
    warn "RuntimeClass 'kata' не найден"
  fi
}

action_check() {
  info "Проверка предварительных условий..."
  check_kvm
  check_k3s
  check_kata
  check_containerd_runtime
  check_runtimeclass
  echo ""
  info "Готово. Если есть [!!] или [..] — запусти install --install-deps"
}

# ═══════════════════════════════════════════════════════════════
#  INSTALL
# ═══════════════════════════════════════════════════════════════
install_k3s() {
  header "Установка K3s"
  if is_k3s_installed; then
    warn "k3s уже установлен: $(k3s_version)"
    return 0
  fi
  if $dry_run; then
    warn "[dry-run] curl -sfL https://get.k3s.io | sh -s - --write-kubeconfig-mode 644"
    return 0
  fi
  info "Устанавливаю K3s..."
  curl -sfL https://get.k3s.io | sh -s - --write-kubeconfig-mode 644
  step "K3s установлен: $(k3s_version)"
}

install_kata_ubuntu() {
  header "Установка Kata Containers"
  if is_kata_installed; then
    warn "kata-runtime уже установлен: $(kata_version)"
    return 0
  fi
  if $dry_run; then
    warn "[dry-run] установка kata-runtime из репозитория openSUSE"
    return 0
  fi
  info "Добавляю репозиторий Kata..."
  local arch
  arch="$(uname -m)"

  local os_codename
  os_codename="$(. /etc/os-release && echo "$VERSION_CODENAME")"
  [[ -z "$os_codename" ]] && os_codename="noble"  # fallback

  # Используем репозиторий Kata для Ubuntu
  local repo_url="https://download.opensuse.org/repositories/home:/katacontainers:/releases:/$(arch):/${KATA_VERSION}/xUbuntu_$(lsb_release -rs 2>/dev/null || echo '24.04')"

  apt-get update -qq
  apt-get install -y -qq software-properties-common apt-transport-https ca-certificates

  echo "deb [signed-by=/usr/share/keyrings/kata-archive-keyring.gpg] ${repo_url}/ /" > /etc/apt/sources.list.d/kata.list

  curl -fsSL "${repo_url}/Release.key" 2>/dev/null | gpg --dearmor -o /usr/share/keyrings/kata-archive-keyring.gpg 2>/dev/null

  apt-get update -qq
  apt-get install -y -qq kata-runtime 2>/dev/null || {
    # fallback: try snap or direct binary
    warn "Установка из репозитория не удалась, пробую snap..."
    snap install kata-containers --classic 2>/dev/null && {
      ln -sf /snap/kata-containers/current/bin/kata-runtime /usr/local/bin/kata-runtime
    } || warn "Не удалось установить kata-runtime. Установи вручную: https://github.com/kata-containers/kata-containers"
  }

  if is_kata_installed; then
    step "kata-runtime установлен: $(kata_version)"
  else
    warn "kata-runtime не найден после установки. Продолжаю, но nested virt может не работать."
  fi
}

configure_containerd_kata() {
  header "Настройка containerd для Kata"
  if $dry_run; then
    warn "[dry-run] создание $K3S_CONTAINERD_DIR/config.toml.tmpl с kata runtime"
    return 0
  fi

  mkdir -p "$K3S_CONTAINERD_DIR"

  # Проверяем, есть ли уже kata в конфиге
  if [[ -f "$K3S_CONTAINERD_DIR/config.toml.tmpl" ]] && grep -q "kata" "$K3S_CONTAINERD_DIR/config.toml.tmpl" 2>/dev/null; then
    step "Kata уже есть в containerd template"
    return 0
  fi

  # Создаём .toml.tmpl для K3s
  local tmpl="$K3S_CONTAINERD_DIR/config.toml.tmpl"

  if [[ -f "$tmpl" ]]; then
    # Дописываем kata к существующему template
    cat >> "$tmpl" << 'KATAEOF'

# Kata Containers runtime
[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.kata]
  runtime_type = "io.containerd.kata.v2"
  privileged_without_host_devices = true
  [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.kata.options]
    ConfigPath = "/opt/kata/share/defaults/kata-containers/configuration.toml"
KATAEOF
  else
    # Свежий template: включаем дефолтный K3s конфиг + kata
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

  step "Containerd template обновлён"

  # Рестарт K3s чтобы перегенерировать конфиг
  info "Перезапускаю K3s..."
  systemctl restart k3s
  # Ждём готовности
  sleep 5
  local i=0
  while ! kubectl get nodes &>/dev/null; do
    sleep 2
    i=$((i+1))
    [[ $i -gt 60 ]] && { warn "K3s не восстановился после рестарта (60s)"; break; }
  done
  step "K3s перезапущен"
}

wait_for_k3s() {
  if $dry_run; then
    warn "[dry-run] ожидание готовности K3s (пропущено)"
    return 0
  fi
  info "Ожидание готовности K3s..."
  local i=0
  while ! kubectl get nodes &>/dev/null; do
    sleep 2
    i=$((i+1))
    [[ $i -gt 30 ]] && { warn "K3s не отвечает после 60s"; return 1; }
  done
  step "K3s готов: $(kubectl get nodes -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)"
}

create_runtimeclass() {
  header "Создание RuntimeClass"
  if kubectl get runtimeclass kata &>/dev/null 2>&1; then
    step "RuntimeClass kata уже существует"
    return 0
  fi
  if $dry_run; then
    warn "[dry-run] создание RuntimeClass 'kata'"
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
  step "RuntimeClass 'kata' создан"
}

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
    values_file="/tmp/shclop-bootstrap-values.yaml"
    if $dry_run; then
      warn "[dry-run] создание values.yaml с дефолтными настройками"
      return 0
    fi
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
  svc_port="$(kubectl get svc -n "$SHCLOP_NAMESPACE" "${HELM_RELEASE_NAME}-backend" -o jsonpath='{.spec.ports[0].nodePort}' 2>/dev/null || echo "8080")"
  info "shclop UI: http://localhost:${svc_port} (логин: admin/admin)"
}

generate_default_values() {
  local out="$1"
  cat > "$out" <<VALUESYAML
# Автоматически сгенерировано bootstrap.sh
config:
  store: inmemory
  logLevel: info

service:
  type: NodePort

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

  step "Сгенерирован values.yaml: $out"
}

action_install() {
  require_root

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

  # 3. Деплой shclop
  deploy_shclop

  if ! $dry_run; then
    echo ""
    info "✅ Установка завершена"
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
  [[ -n "$values" ]] && remote_argv+=("--values" "$values")
  [[ -n "$IMAGE_REPO" ]] && remote_argv+=("--image-repo" "$IMAGE_REPO")
  [[ -n "$IMAGE_TAG" ]] && remote_argv+=("--image-tag" "$IMAGE_TAG")

  info "Выполнение на $remote..."
  ssh "$remote" "bash -s -- $(printf '%q ' "${remote_argv[@]}")" < "$0"
else
  run_local
fi
