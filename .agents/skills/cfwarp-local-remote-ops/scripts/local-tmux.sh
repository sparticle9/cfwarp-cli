#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  local-tmux.sh start      Start cfwarp daemon in a dedicated tmux session
  local-tmux.sh stop       Stop the tmux session
  local-tmux.sh restart    Restart the tmux session
  local-tmux.sh attach     Attach to the session
  local-tmux.sh logs [N]   Show last N lines from tmux pane history (default 200)
  local-tmux.sh status      Print whether session exists

Environment:
  CFWARP_STATE_DIR           cfwarp state directory (default: ~/.local/state/cfwarp-cli)
  CFWARP_TMUX_SESSION        tmux session name (default: cfwarp-local)
  CFWARP_TMUX_WINDOW         tmux window name (default: daemon)
  CFWARP_TMUX_HISTORY_LIMIT  history-limit for window (default: 50000)
  CFWARP_DAEMON_ARGS         extra args appended to daemon run command
  CFWARP_SETTINGS_FILE        optional settings file for daemon
  CFWARP_TMUX_LOG_FILE       optional file for persistent pane logging
  CFWARP_TMUX_WORKDIR        working directory before start (default: cwd)
EOF
}

command_exists() {
  command -v "$1" >/dev/null 2>&1
}

ensure_dependencies() {
  if ! command_exists tmux; then
    echo "tmux is required for this helper" >&2
    exit 2
  fi

  if ! command_exists cfwarp-cli; then
    echo "cfwarp-cli must be available in PATH (or invoke a settings-driven command directly)" >&2
    exit 2
  fi
}

SESSION="${CFWARP_TMUX_SESSION:-cfwarp-local}"
WINDOW="${CFWARP_TMUX_WINDOW:-daemon}"
HISTORY_LIMIT="${CFWARP_TMUX_HISTORY_LIMIT:-50000}"
STATE_DIR="${CFWARP_STATE_DIR:-$HOME/.local/state/cfwarp-cli}"
SETTINGS_FILE="${CFWARP_SETTINGS_FILE:-}"
DAEMON_ARGS="${CFWARP_DAEMON_ARGS:-}"
WORKDIR="${CFWARP_TMUX_WORKDIR:-$PWD}"
LOG_FILE="${CFWARP_TMUX_LOG_FILE:-}"

build_daemon_cmd() {
  local cmd=(cfwarp-cli daemon run --state-dir "$STATE_DIR")
  if [[ -n "$SETTINGS_FILE" ]]; then
    cmd=("${cmd[@]}" --settings-file "$SETTINGS_FILE")
  fi

  if [[ -n "$DAEMON_ARGS" ]]; then
    # shellcheck disable=SC2206
    local args=( $DAEMON_ARGS )
    cmd=("${cmd[@]}" "${args[@]}")
  fi

  printf '%q ' "${cmd[@]}"
}

tmux_start() {
  ensure_dependencies
  mkdir -p "$STATE_DIR"
  if tmux has-session -t "$SESSION" 2>/dev/null; then
    echo "Session '$SESSION' already exists. Use stop/restart or attach first." >&2
    exit 2
  fi

  tmux new-session -d -s "$SESSION" -n "$WINDOW" -c "$WORKDIR" >/dev/null
  tmux set-window-option -t "${SESSION}:${WINDOW}" history-limit "$HISTORY_LIMIT" >/dev/null

  local cmd
  cmd="$(build_daemon_cmd)"
  tmux send-keys -t "${SESSION}:${WINDOW}" "${cmd}" C-m

  if [[ -n "$LOG_FILE" ]]; then
    mkdir -p "$(dirname "$LOG_FILE")"
    tmux pipe-pane -t "${SESSION}:${WINDOW}" "cat >> '$LOG_FILE'" >/dev/null
  fi

  echo "Started tmux session: $SESSION (window: $WINDOW)"
  echo "Attach with: tmux attach -t $SESSION"
}

tmux_stop() {
  if ! tmux has-session -t "$SESSION" 2>/dev/null; then
    echo "Session '$SESSION' not running." >&2
    exit 2
  fi
  tmux kill-session -t "$SESSION"
  echo "Stopped tmux session: $SESSION"
}

tmux_restart() {
  if tmux has-session -t "$SESSION" 2>/dev/null; then
    tmux kill-session -t "$SESSION"
  fi
  tmux_start
}

tmux_attach() {
  if ! tmux has-session -t "$SESSION" 2>/dev/null; then
    echo "Session '$SESSION' not running." >&2
    exit 2
  fi
  tmux attach -t "$SESSION"
}

tmux_logs() {
  if ! tmux has-session -t "$SESSION" 2>/dev/null; then
    echo "Session '$SESSION' not running." >&2
    exit 2
  fi
  local lines="${1:-200}"
  tmux capture-pane -t "${SESSION}:${WINDOW}" -p -S -"$lines"
}

tmux_status() {
  if tmux has-session -t "$SESSION" 2>/dev/null; then
    echo "Session '$SESSION' is running"
    tmux list-sessions -F '#{session_name}: #{session_created} | #{session_windows} windows'
  else
    echo "Session '$SESSION' is not running"
    exit 1
  fi
}

main() {
  local action="${1:-}"
  case "$action" in
    start) tmux_start ;;
    stop) tmux_stop ;;
    restart) tmux_restart ;;
    attach) tmux_attach ;;
    logs) tmux_logs "${2:-200}" ;;
    status) tmux_status ;;
    -h|--help|"" ) usage; exit 0 ;;
    *)
      echo "Unknown action: $action" >&2
      usage
      exit 2
      ;;
  esac
}

main "$@"
