#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  remote-linux.sh deploy [ansible args...]   Deploy remote dogfood stack
  remote-linux.sh status [ansible args...]   Show remote status
  remote-linux.sh quick-bench [ansible args...]    Run protocol-quick-bench.yml
  remote-linux.sh real-bench [ansible args...]     Run protocol-real-bench.yml
  remote-linux.sh smoke [ansible args...]     Run package-runtime-smoke.yml
  remote-linux.sh logs [lines]               Show compose/log tail from /opt/cfwarp-dogfood (default 120)
  remote-linux.sh down                       Stop managed compose stack
  remote-linux.sh exec <target-pattern> -- <command...>   Run shell command on target hosts

Env:
  CFWARP_ANSIBLE_INVENTORY  path to ansible inventory (default: ansible/inventory.ini)
  CFWARP_ANSIBLE_LIMIT      host pattern/group (default: warp)
  CFWARP_REMOTE_COMPOSE_DIR  compose path on host (default: /opt/cfwarp-dogfood)

Examples:
  CFWARP_ANSIBLE_LIMIT=proxy-host-1 ./remote-linux.sh deploy
  ./remote-linux.sh logs 120
  ./remote-linux.sh exec proxy-host-1 -- docker exec cfwarp-warp cfwarp-cli status --json --state-dir /home/cfwarp/.local/state/cfwarp-cli
EOF
}

command_exists() {
  command -v "$1" >/dev/null 2>&1
}

require() {
  local bin=$1
  if ! command_exists "$bin"; then
    echo "$bin is required for this helper" >&2
    exit 2
  fi
}

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../../../.." && pwd)"
INVENTORY="${CFWARP_ANSIBLE_INVENTORY:-$REPO_ROOT/ansible/inventory.ini}"
LIMIT="${CFWARP_ANSIBLE_LIMIT:-warp}"
COMPOSE_DIR="${CFWARP_REMOTE_COMPOSE_DIR:-/opt/cfwarp-dogfood}"

PLAYBOOK_DIR="$REPO_ROOT/ansible"

run_playbook() {
  local playbook=$1
  shift
  require ansible-playbook

  local ansible_args=(
    -i "$INVENTORY"
    "$playbook"
  )

  if [[ -n "$LIMIT" ]]; then
    ansible_args+=(--limit "$LIMIT")
  fi

  ansible-playbook "${ansible_args[@]}" "$@"
}

run_shell() {
  local cmd="$*"
  require ansible

  local target="${LIMIT:-all}"
  if [[ -z "$cmd" ]]; then
    echo "remote shell command required" >&2
    exit 2
  fi

  ansible -i "$INVENTORY" "$target" -b -m shell -a "$cmd"
}

main() {
  local action="${1:-}"
  shift || true

  case "$action" in
    deploy)
      run_playbook "$PLAYBOOK_DIR/dogfood-deploy.yml" "$@"
      ;;
    status)
      run_playbook "$PLAYBOOK_DIR/dogfood-status.yml" "$@"
      ;;
    quick-bench)
      run_playbook "$PLAYBOOK_DIR/protocol-quick-bench.yml" "$@"
      ;;
    real-bench)
      run_playbook "$PLAYBOOK_DIR/protocol-real-bench.yml" "$@"
      ;;
    smoke)
      run_playbook "$PLAYBOOK_DIR/package-runtime-smoke.yml" "$@"
      ;;
    logs)
      local lines=120
      if [[ "$#" -gt 0 ]]; then
        if [[ "$1" =~ ^[0-9]+$ ]]; then
          lines="$1"
          shift
        fi
      fi
      run_shell "cd ${COMPOSE_DIR} && docker compose ps && docker compose logs --tail ${lines}"
      ;;
    down)
      run_shell "cd ${COMPOSE_DIR} && docker compose down"
      ;;
    exec)
      if [[ "$#" -lt 1 ]]; then
        echo "exec requires a target host/pattern and optional -- and command" >&2
        exit 2
      fi
      # syntax: exec <target-pattern> -- <command...>
      local target_pattern=$1
      shift
      if [[ "$#" -gt 0 && "$1" == "--" ]]; then
        shift
      fi
      if [[ $# -lt 1 ]]; then
        echo "exec command required after --" >&2
        exit 2
      fi
      ansible -i "$INVENTORY" "$target_pattern" -b -m shell -a "$*"
      ;;
    -h|--help|"")
      usage
      ;;
    *)
      echo "Unknown action: $action" >&2
      usage
      exit 2
      ;;
  esac
}

main "$@"
