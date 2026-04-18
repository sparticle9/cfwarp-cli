---
name: cfwarp-local-remote-ops
description: Deploy cfwarp-cli locally in a tmux session with bounded history, run remote Linux deployments via existing Ansible playbooks, and perform common operational checks.
---

# cfwarp Local + Remote Ops

Use this skill when the request is about:
- launching cfwarp locally for validation with tmux session history control,
- deploying or maintaining the remote Linux dogfood stack,
- looking up common runtime/ops checks (register/import, status, rotation, logs, etc.).

## 1) Local deploy / run with tmux session history

This project keeps localhost-first testing handy. For local execution and easier log inspection,
run the daemon through a dedicated tmux session.

Defaults:

- `CFWARP_STATE_DIR` (default: `~/.local/state/cfwarp-cli`)
- `CFWARP_TMUX_SESSION` (default: `cfwarp-local`)
- `CFWARP_TMUX_WINDOW` (default: `daemon`)
- `CFWARP_TMUX_HISTORY_LIMIT` (default: `50000`)
- `CFWARP_TMUX_LOG_FILE` (optional, if set, tmux pane is logged to file)
- `CFWARP_DAEMON_ARGS` (optional, extra args passed to `cfwarp-cli daemon run`)
- `CFWARP_SETTINGS_FILE` (optional, pass `--settings-file` instead of settings discovery)

### Start / attach / stop

```bash
# start cfwarp daemon in tmux
./.pi/agent/skills/cfwarp-local-remote-ops/scripts/local-tmux.sh start

# attach for live interaction/logs
tmux attach -t "${CFWARP_TMUX_SESSION:-cfwarp-local}"

# quick status
tmux has-session -t "${CFWARP_TMUX_SESSION:-cfwarp-local}" && echo "running"

# stop session
./.pi/agent/skills/cfwarp-local-remote-ops/scripts/local-tmux.sh stop
```

Additional controls:

```bash
# tail current pane history (default 200 lines)
./.pi/agent/skills/cfwarp-local-remote-ops/scripts/local-tmux.sh logs
./.pi/agent/skills/cfwarp-local-remote-ops/scripts/local-tmux.sh logs 400

# restart with new args (stops + starts)
./.pi/agent/skills/cfwarp-local-remote-ops/scripts/local-tmux.sh restart
```

If your environment rewrites DNS answers (complex NAT / fake-IP scenarios), prefer explicit
HTTPS upstream DNS for this process:

```bash
export CFWARP_DNS_MODE=https
export CFWARP_DNS_SERVER=1.1.1.1
export CFWARP_DNS_SERVER_PORT=443
export CFWARP_DNS_PATH=/dns-query
```

Then confirm with:

```bash
cfwarp-cli status --json --state-dir "$CFWARP_STATE_DIR" --trace
```

> The session helper is non-destructive: it only wraps execution/attach/stop commands and does not alter runtime behavior.

## 2) Remote Linux server operations (ansible)

This project ships Ansible playbooks for managed remote Linux hosts.

Inventory conventions (never commit real hostnames):

```ini
[warp]
proxy-host-1
```

Use `ansible/inventory.ini.example` as template.

### Core tasks

```bash
# Deploy/refresh dual-path (WireGuard + MASQUE) dogfood stack
ansible-playbook -i ansible/inventory.ini ansible/dogfood-deploy.yml --limit proxy-host-1

# Check runtime status, container state, and sing-box integration
ansible-playbook -i ansible/inventory.ini ansible/dogfood-status.yml --limit proxy-host-1

# Teardown this managed stack
ansible -i ansible/inventory.ini proxy-host-1 -b -m shell -a 'cd /opt/cfwarp-dogfood && docker compose down'
```

Or use the project helper wrapper:

```bash
CFWARP_ANSIBLE_LIMIT=proxy-host-1 .pi/agent/skills/cfwarp-local-remote-ops/scripts/remote-linux.sh deploy
CFWARP_ANSIBLE_LIMIT=proxy-host-1 .pi/agent/skills/cfwarp-local-remote-ops/scripts/remote-linux.sh status
CFWARP_ANSIBLE_LIMIT=proxy-host-1 .pi/agent/skills/cfwarp-local-remote-ops/scripts/remote-linux.sh logs 80
CFWARP_ANSIBLE_LIMIT=proxy-host-1 .pi/agent/skills/cfwarp-local-remote-ops/scripts/remote-linux.sh down
CFWARP_ANSIBLE_LIMIT=proxy-host-1 .pi/agent/skills/cfwarp-local-remote-ops/scripts/remote-linux.sh quick-bench
CFWARP_ANSIBLE_LIMIT=proxy-host-1 .pi/agent/skills/cfwarp-local-remote-ops/scripts/remote-linux.sh exec proxy-host-1 -- docker exec cfwarp-warp cfwarp-cli status --json --state-dir /home/cfwarp/.local/state/cfwarp-cli
```

### Benchmark / validation helpers

```bash
# Fast proxy-path comparison
ansible-playbook -i ansible/inventory.ini ansible/protocol-quick-bench.yml --limit proxy-host-1

# Real target benchmark (iperf3 / HTTP)
ansible-playbook -i ansible/inventory.ini ansible/protocol-real-bench.yml --limit proxy-host-1

# Local packaged runtime smoke
ansible-playbook -i ansible/inventory.ini ansible/package-runtime-smoke.yml --limit proxy-host-1
```

## 3) Common ops checks (run locally or via remote shell)

- Register/import workflow:
  - `cfwarp-cli register --state-dir "$CFWARP_STATE_DIR"`
  - `cfwarp-cli import --state-dir "$CFWARP_STATE_DIR" --file /path/to/account.json`
- Runtime health:
  - `cfwarp-cli status --json --state-dir "$CFWARP_STATE_DIR"`
  - `cfwarp-cli status --json --trace --state-dir "$CFWARP_STATE_DIR"`
  - `cfwarp-cli daemon ctl status --state-dir "$CFWARP_STATE_DIR"`
- Rotate / unlock checks:
  - `cfwarp-cli unlock test --service gemini --state-dir "$CFWARP_STATE_DIR"`
  - `cfwarp-cli rotate --attempts 3 --service gemini --state-dir "$CFWARP_STATE_DIR"`
- On remote, replace `CFWARP_STATE_DIR` with each container mount path and run the same command via:

```bash
ansible -i ansible/inventory.ini proxy-host-1 -b -m shell -a 'docker exec cfwarp-warp cfwarp-cli status --json --state-dir /home/cfwarp/.local/state/cfwarp-cli'
```

# 4) Safety checks / guardrails

- Prefer bounded probes and explicit timeouts in automation to avoid blocked local tests.
- For remote playbooks, keep proxy bind ports localhost-only unless you intentionally expose them.
- Use durable artifacts from CI and avoid committing local `.env`, inventory, tokens, or host-specific secrets.