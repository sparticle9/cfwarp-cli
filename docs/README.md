# Documentation map

This repository contains both **operator-facing** and **development-facing** documentation.

Current support is aimed at Linux and macOS on Apple Silicon. The broadest runtime and deployment coverage is still Linux-host oriented, while macOS on Apple Silicon currently supports local CLI and local Docker workflows for the legacy WireGuard control-plane lane.

If you are new here, start with the section that matches your goal.

## I want to use the project

Start here:

1. `../README.md`
2. `dogfood-debian13.md`
3. `warp-rotation-unlock.md`

### Operator docs

- `dogfood-debian13.md`
  - remote Docker deployment
  - dual WireGuard / MASQUE localhost proxies
  - sing-box fragment integration
  - on-demand status workflow
- `../.pi/agent/skills/cfwarp-local-remote-ops/SKILL.md`
  - local tmux-based deploy/ops workflow
  - local vs remote Linux runbook
  - common status/rotation/bench checks

- `warp-rotation-unlock.md`
  - transport / access / caps / rotation model
  - daemon control
  - hashed rotation memory
  - IPv4 / IPv6 distinctness policy

## I want to understand the current project state

- `status/2026-04-native-masque-status.md`
- `native-masque-vs-singbox-review.md`
- `tun-decision-note.md`

## I want to understand the architecture

Historical / design docs:

- `specs/001-minimal-wireguard-proxy/README.md`
- `specs/001-minimal-wireguard-proxy/requirements.md`
- `specs/001-minimal-wireguard-proxy/design.md`
- `specs/002-unified-transport-dataplane/README.md`
- `specs/002-unified-transport-dataplane/design.md`
- `specs/002-unified-transport-dataplane/tasks.md`

## I want to contribute

Start here:

1. `../CONTRIBUTING.md`
2. `status/2026-04-native-masque-status.md`
3. `specs/002-unified-transport-dataplane/design.md`

Depending on the area, continue with:

- `native-masque-vs-singbox-review.md`
- `masque-performance-plan.md`
- benchmark docs below

## I want to understand the benchmark system

- `benchmark-package.md`
- `benchmark-mechanism.md`
- `benchmark-report-case.md`
- `masque-performance-plan.md`
- `masque-real-target-matrix-20260408.md`

## Background / research

- `background.md`

## Notes on freshness

Some docs are:

- **operator-current** — meant to reflect current usage
- **status snapshots** — point-in-time checkpoints
- **historical specs** — useful for understanding intent and evolution

When behavior changes, the highest-priority docs to keep current are:

- `../README.md`
- `../CONTRIBUTING.md`
- `dogfood-debian13.md`
- `warp-rotation-unlock.md`
