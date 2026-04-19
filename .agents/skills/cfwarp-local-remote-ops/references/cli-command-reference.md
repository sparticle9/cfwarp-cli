# cfwarp Local + Remote CLI Reference

This is the command-first reference for ordinary users who want to get started quickly.
Use this file as a copy/paste checklist.

---

## 0) Prerequisites

- `cfwarp-cli` built or installed locally
- `tmux` (for local background session workflow)
- `ansible` + `ansible-playbook` + Docker on remote host (for Linux remote operations)
- On remote hosts: SSH access via your SSH config/inventory

---

## 1) Local: start and keep daemon logs under control (tmux)

### Start

```bash
export CFWARP_STATE_DIR=${CFWARP_STATE_DIR:-$HOME/.local/state/cfwarp-cli}
mkdir -p "$CFWARP_STATE_DIR"

# If DNS interception/fake-IP is present in your network:
export CFWARP_DNS_MODE=https
export CFWARP_DNS_SERVER=1.1.1.1
export CFWARP_DNS_SERVER_PORT=443
export CFWARP_DNS_PATH=/dns-query

./.agents/skills/cfwarp-local-remote-ops/scripts/local-tmux.sh start
```

### Attach / inspect

```bash
tmux attach -t "${CFWARP_TMUX_SESSION:-cfwarp-local}"
./.agents/skills/cfwarp-local-remote-ops/scripts/local-tmux.sh logs 200
./.agents/skills/cfwarp-local-remote-ops/scripts/local-tmux.sh status
```

### Health check

```bash
cfwarp-cli status --json --state-dir "$CFWARP_STATE_DIR"
cfwarp-cli status --json --trace --state-dir "$CFWARP_STATE_DIR"
cfwarp-cli daemon ctl status --state-dir "$CFWARP_STATE_DIR"
```

### Shell completion (bash / zsh)

```bash
# One-off completion for current session
cfwarp-cli completion bash | source
cfwarp-cli completion zsh | source

# Persist for custom init file (for example ~/.zshrc.local)
cfwarp-cli completion zsh --init-file ~/.zshrc.local
cfwarp-cli completion bash --init-file ~/.bashrc.local
```

### Common maintenance

```bash
cfwarp-cli unlock test --service gemini --service netflix --service youtube --state-dir "$CFWARP_STATE_DIR"
cfwarp-cli rotate --attempts 3 --service gemini --service netflix --state-dir "$CFWARP_STATE_DIR"
cfwarp-cli down --state-dir "$CFWARP_STATE_DIR"
```

### Stop/restart

```bash
./.agents/skills/cfwarp-local-remote-ops/scripts/local-tmux.sh stop
./.agents/skills/cfwarp-local-remote-ops/scripts/local-tmux.sh restart
```

---

## 2) Remote Linux: copy/paste ansible workflows

### One-time setup

```bash
cp ansible/inventory.ini.example ansible/inventory.ini
# edit ansible/inventory.ini and fill hosts like:
# [warp]
# proxy-host-1
```

### Deploy / status / teardown

```bash
export CFWARP_ANSIBLE_INVENTORY=ansible/inventory.ini
export CFWARP_ANSIBLE_LIMIT=laxhd   # or proxy-host-1, if your target is in [warp]

# The deploy and status playbooks use `dogfood_hosts` to select where sing-box routing is
# managed. Explicitly set it when the target is outside the [warp] group.
ansible-playbook -i "$CFWARP_ANSIBLE_INVENTORY" \
  ansible/dogfood-deploy.yml --limit "$CFWARP_ANSIBLE_LIMIT" \
  -e "dogfood_hosts=$CFWARP_ANSIBLE_LIMIT"

ansible-playbook -i "$CFWARP_ANSIBLE_INVENTORY" \
  ansible/dogfood-status.yml --limit "$CFWARP_ANSIBLE_LIMIT" \
  -e "dogfood_hosts=$CFWARP_ANSIBLE_LIMIT"

ansible -i "$CFWARP_ANSIBLE_INVENTORY" "$CFWARP_ANSIBLE_LIMIT" -b -m shell \
  -a "cd /opt/cfwarp-dogfood && docker compose down"
```

### Remote validation and smoke checks

```bash
# fast proxy-path benchmark
ansible-playbook -i "$CFWARP_ANSIBLE_INVENTORY" \
  ansible/protocol-quick-bench.yml --limit "$CFWARP_ANSIBLE_LIMIT"

# real target benchmark
ansible-playbook -i "$CFWARP_ANSIBLE_INVENTORY" \
  ansible/protocol-real-bench.yml --limit "$CFWARP_ANSIBLE_LIMIT"

# packaged runtime smoke
ansible-playbook -i "$CFWARP_ANSIBLE_INVENTORY" \
  ansible/package-runtime-smoke.yml --limit "$CFWARP_ANSIBLE_LIMIT"
```

### Remote status checks inside containers

```bash
ansible -i "$CFWARP_ANSIBLE_INVENTORY" "$CFWARP_ANSIBLE_LIMIT" -b -m shell -a \
  'docker exec cfwarp-warp cfwarp-cli status --json --state-dir /home/cfwarp/.local/state/cfwarp-cli'

ansible -i "$CFWARP_ANSIBLE_INVENTORY" "$CFWARP_ANSIBLE_LIMIT" -b -m shell -a \
  'docker exec cfwarp-masque cfwarp-cli status --json --state-dir /home/cfwarp/.local/state/cfwarp-cli'
```

---

## 3) CLI wrappers (optional)

If you prefer short commands, the scripts below call the same ansible and tmux operations:

```bash
./.agents/skills/cfwarp-local-remote-ops/scripts/remote-linux.sh deploy
./.agents/skills/cfwarp-local-remote-ops/scripts/remote-linux.sh status
./.agents/skills/cfwarp-local-remote-ops/scripts/remote-linux.sh quick-bench
./.agents/skills/cfwarp-local-remote-ops/scripts/remote-linux.sh real-bench
./.agents/skills/cfwarp-local-remote-ops/scripts/remote-linux.sh smoke
./.agents/skills/cfwarp-local-remote-ops/scripts/remote-linux.sh logs 120
./.agents/skills/cfwarp-local-remote-ops/scripts/remote-linux.sh exec proxy-host-1 -- docker exec cfwarp-warp cfwarp-cli status --json --state-dir /home/cfwarp/.local/state/cfwarp-cli
```

---

## 4) Why this path exists

- keep first-run copy/paste commands in one file
- allow low-friction onboarding for users unfamiliar with all ansible flags
- reuse the same commands locally and on remote hosts without router edits
