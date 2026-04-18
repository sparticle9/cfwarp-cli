# CLI Command Reference

This is a practical command reference for Linux usage. It covers command names, common flags, and copy/paste workflows.

> For quick start workflows (local tmux + remote Linux), also see:
> - `../.agents/skills/cfwarp-local-remote-ops/SKILL.md`
> - `../.agents/skills/cfwarp-local-remote-ops/references/cli-command-reference.md`

## Common form

```bash
cfwarp-cli [global-flags] <command> [command-flags]
```

Global flag:

- `--state-dir <path>`: override state root directory (default platform path)

## Core command groups

### Account

- `cfwarp-cli register [--force] [--masque]`
  - Generate a new WARP device/account locally.

- `cfwarp-cli import --file <path> [--force]`
  - Import existing account JSON.

- `cfwarp-cli registration new [--force] [--masque]`
  - Same as `register`.

- `cfwarp-cli registration import --file <path> [--force]`
  - Same as `import`.

- `cfwarp-cli registration show [--json]`
  - Show current account metadata.

- `cfwarp-cli address show [--json]`
  - Show assigned WireGuard / MASQUE addresses.

### Transport + mode + runtime config

- `cfwarp-cli transport show [--json]`
  - Show selected transport/backend/runtime_family.

- `cfwarp-cli transport set <transport> [--runtime-family legacy|native]`
  - Persist transport and runtime family override.

- `cfwarp-cli mode show`
  - Show service mode/listen settings.

- `cfwarp-cli mode set <mode>`
  - Persist service mode (`socks5` / `http` etc.).

- `cfwarp-cli stats`
  - Show compact runtime snapshot.

- `cfwarp-cli render [--output <file>]`
  - Render backend config from current account/settings.

- `cfwarp-cli validate [--json]`
  - Validate resolved settings integrity.

### Service lifecycle

- `cfwarp-cli up [--foreground]`
  - Start proxy backend.

- `cfwarp-cli down`
  - Stop backend and clear runtime state.

- `cfwarp-cli connect`
  - Compatibility wrapper for `up`.

- `cfwarp-cli disconnect`
  - Compatibility wrapper for `down`.

- `cfwarp-cli daemon run`
  - Long-lived runtime with periodic checks and local socket control.

- `cfwarp-cli daemon ctl status|check|rotate|reload`
  - Send command to running daemon.

### Diagnostics and verification

- `cfwarp-cli status [--json] [--trace] [--require-account] [--require-running] [--require-reachable] [--require-warp]`
  - Health/reporting command.

- `cfwarp-cli endpoint test [--json] <host:port>...`
  - Probe endpoint candidate syntax/reachability.

- `cfwarp-cli unlock test [--service <svc>] [--json] [--timeout <dur>]`
  - Probe unlock targets through configured proxy.

- `cfwarp-cli rotate [--attempts N] [--service <svc>] [--timeout <dur>] [--settle-time <dur>] [--masque]`
  - Re-register and optionally validate unlock targets.

### Utility

- `cfwarp-cli completion bash|zsh [--init-file ~/.zshrc.local]`
  - Generate shell completion script.

- `cfwarp-cli version`
  - Print version and environment info.

## Linux copy/paste patterns

### Quick health check

```bash
cfwarp-cli status --json --state-dir /path/to/state
cfwarp-cli status --json --trace --state-dir /path/to/state
cfwarp-cli daemon ctl status --state-dir /path/to/state
```

### Common daily workflow

```bash
# Start/reload
cfwarp-cli register --state-dir /path/to/state
cfwarp-cli up --state-dir /path/to/state
cfwarp-cli status --state-dir /path/to/state --require-running

# Verify
cfwarp-cli unlock test --service gemini --state-dir /path/to/state

# Rotate on failure
cfwarp-cli rotate --attempts 3 --service gemini --state-dir /path/to/state

# Stop
cfwarp-cli down --state-dir /path/to/state
```

## Completion notes

For custom shell init files (common on macOS/Linux setups):

```bash
cfwarp-cli completion zsh --init-file ~/.zshrc.local
cfwarp-cli completion bash --init-file ~/.bashrc.local
```

## What to keep in docs vs in scripts

- **Command reference doc (this file):** stable command list, flags, and examples.
- **Skill docs (`.agents/...`):** onboarding flow, tmux wrapper scripts, and environment-specific operations.
- **docs/README.md:** discovery index to both of the above.
