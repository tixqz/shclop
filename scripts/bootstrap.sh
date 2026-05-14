#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: scripts/bootstrap.sh <check|install|reset|destroy> [flags]

Targets:
  local target is default
  --remote user@host    run action on remote Linux host over SSH

Flags:
  --dry-run
  --install-deps
  --yes
  --purge-data
  --remove-k3s
  --remove-kata
  --values PATH
USAGE
}

action="${1:-}"
if [[ -z "$action" ]]; then usage; exit 2; fi
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
    --remote)
      if [[ $# -lt 2 ]]; then usage; exit 2; fi
      remote="$2"
      shift 2
      ;;
    --dry-run) dry_run=true; shift ;;
    --install-deps) install_deps=true; shift ;;
    --yes) yes=true; shift ;;
    --purge-data) purge_data=true; shift ;;
    --remove-k3s) remove_k3s=true; shift ;;
    --remove-kata) remove_kata=true; shift ;;
    --values)
      if [[ $# -lt 2 ]]; then usage; exit 2; fi
      values="$2"
      shift 2
      ;;
    *) echo "unknown argument: $1" >&2; usage; exit 2 ;;
  esac
done

case "$action" in
  check|install|reset|destroy) ;;
  *) echo "unknown action: $action" >&2; usage; exit 2 ;;
esac

run_local() {
  echo "action=$action remote=local dry_run=$dry_run install_deps=$install_deps purge_data=$purge_data remove_k3s=$remove_k3s remove_kata=$remove_kata values=$values"
  if [[ "$action" == "destroy" && "$yes" != "true" ]]; then
    read -r -p "Type 'delete shclop' to continue: " confirm
    [[ "$confirm" == "delete shclop" ]] || { echo "aborted"; exit 1; }
  fi
  echo "bootstrap skeleton: implementation will add KVM/K3s/Kata/Helm operations"
}

if [[ -n "$remote" ]]; then
  remote_argv=("$action")
  if [[ "$dry_run" == "true" ]]; then remote_argv+=("--dry-run"); fi
  if [[ "$install_deps" == "true" ]]; then remote_argv+=("--install-deps"); fi
  if [[ "$yes" == "true" ]]; then remote_argv+=("--yes"); fi
  if [[ "$purge_data" == "true" ]]; then remote_argv+=("--purge-data"); fi
  if [[ "$remove_k3s" == "true" ]]; then remote_argv+=("--remove-k3s"); fi
  if [[ "$remove_kata" == "true" ]]; then remote_argv+=("--remove-kata"); fi
  if [[ -n "$values" ]]; then remote_argv+=("--values" "$values"); fi
  ssh "$remote" "bash -s -- $(printf '%q ' "${remote_argv[@]}")" < "$0"
else
  run_local
fi
