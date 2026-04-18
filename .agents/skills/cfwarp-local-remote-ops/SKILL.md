---
name: cfwarp-local-remote-ops
description: Help ordinary users get started quickly with cfwarp for local tmux-first operation and remote Linux (ansible) operations.
---

# cfwarp Local + Remote Ops

Use this skill for onboarding and operational guidance:
- start/use cfwarp locally in a tmux session (bounded history, attachable logs),
- run remote Linux deployments and validation via existing Ansible playbooks,
- execute common `cfwarp-cli` runtime/health/rotation commands.

## Quick use

For ordinary users, the practical steps are in the reference file:

- `references/cli-command-reference.md`

That file is the primary command checklist for local + remote usage.

Use the scripts in `scripts/` when you want short one-command entry points:

- `.agents/skills/cfwarp-local-remote-ops/scripts/local-tmux.sh start`
- `.agents/skills/cfwarp-local-remote-ops/scripts/remote-linux.sh deploy`

## Script behavior (high-level)

- `local-tmux.sh`
  - Start/attach/restart/stop cfwarp daemon under tmux with configurable session name and history limit.
  - Keeps operations explicit and recoverable with `status/logs`.
- `remote-linux.sh`
  - Thin wrapper around `ansible-playbook` and `ansible` for deploying, checking, benchmarking, and tailing logs.
  - Uses `CFWARP_ANSIBLE_*` env vars for host selection and compose path.

## Safety defaults

- Keep proxy bind ports local unless intentionally exposing service ports.
- Use bounded probes/timeouts in tests.
- Avoid committing inventories, `.env`, or host/network secrets.
