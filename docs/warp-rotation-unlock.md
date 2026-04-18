# WARP address rotation, caps, and daemon control

This document covers the current minimal implementation for:

- inspecting assigned WARP addresses
- validating built-in capability probes
- rotating registrations manually
- keeping hashed memory of previously seen assigned addresses
- running a long-lived daemon that uses configured capability checks to decide when to rotate

## Model

The model is intentionally small and explicit:

- **transport** = upstream tunnel protocol (`wireguard` or `masque`)
- **access** = how local clients consume the tunnel (`socks5` or `http` today)
- **caps** = built-in capability probes the service should satisfy
- **rotation** = remediation used when a cap fails
- **daemon** = long-lived manager that owns checks, rotation, and live control

## Current support matrix

Access types currently implemented:

- `socks5`
- `http` (including HTTPS via CONNECT)

Reserved for later:

- `tun`

Current backend/access support:

| backend | socks5 | http | tun |
|---|---:|---:|---:|
| `singbox-wireguard` | yes | yes | no |
| `native-masque` | yes | yes | no |

## Commands

### Show assigned addresses

```bash
cfwarp-cli address show
cfwarp-cli address show --json
```

Shows the currently allocated address material from local account state:

- WireGuard IPv4 / IPv6
- WireGuard peer endpoint
- MASQUE IPv4 / IPv6 when MASQUE state exists

### Validate config integrity

```bash
cfwarp-cli validate
cfwarp-cli validate --json
```

This validates the resolved config after applying defaults, persisted settings, env vars, and CLI overrides.

### Manual unlock checks

```bash
cfwarp-cli unlock test --service gemini --service chatgpt
cfwarp-cli unlock test --json
```

Current manual unlock checks:

- `gemini`
- `chatgpt` (alias: `openai`)

### Manual rotation

```bash
cfwarp-cli rotate --attempts 8 --service gemini --service chatgpt
```

This is still available as an explicit operator tool.

### Run the daemon

```bash
cfwarp-cli daemon run
```

The daemon:

1. ensures the backend is running
2. evaluates configured cap probes on a schedule
3. rotates registration/account state when configured remediation rules say so
4. can be controlled live over a local Unix socket

### Control a running daemon

```bash
cfwarp-cli daemon ctl status
cfwarp-cli daemon ctl check
cfwarp-cli daemon ctl rotate
cfwarp-cli daemon ctl reload
```

Important behavior:

- `daemon ctl rotate` forces a rotation cycle **without restarting the daemon process**
- `daemon ctl reload` reloads settings and restarts the managed backend internally if needed

## Config shape

The config is carried by `settings.json` and the existing flag/env resolution layer.

You can provide configuration in three common ways:

- no file at all, relying on defaults
- a standalone settings file via `--settings-file` or `CFWARP_SETTINGS_FILE`
- a durable state dir containing `settings.json` plus account and rotation state

Current schema additions are:

```json
{
  "runtime_family": "legacy",
  "transport": "wireguard",
  "log_level": "info",
  "access": [
    {
      "type": "socks5",
      "listen_host": "0.0.0.0",
      "listen_port": 1080
    },
    {
      "type": "http",
      "listen_host": "0.0.0.0",
      "listen_port": 8080
    }
  ],
  "daemon": {
    "control_socket": "/run/cfwarp-cli/daemon.sock"
  },
  "caps": {
    "interval_seconds": 300,
    "checks": [
      {
        "probe": "internet",
        "required": true,
        "rotate_on_fail": true,
        "timeout_seconds": 10
      },
      {
        "probe": "gemini",
        "required": false,
        "rotate_on_fail": true,
        "timeout_seconds": 15
      },
      {
        "probe": "chatgpt",
        "required": false,
        "rotate_on_fail": true,
        "timeout_seconds": 15
      }
    ]
  },
  "rotation": {
    "enabled": true,
    "max_attempts_per_incident": 3,
    "settle_time_seconds": 12,
    "cooldown_seconds": 1800,
    "restore_last_good": true,
    "enroll_masque": false,
    "distinctness": "either",
    "history_size": 128
  },
  "masque_options": {
    "use_ipv6": false
  }
}
```

## Rotation memory and v4/v6 policy

Each observed assigned address set is stored in durable hashed memory at `rotation-history.json`.

The file stores only hashes and counters, not raw historical IPs.

`rotation.distinctness` controls what counts as a real rotation:

- `either` — accept a new IPv4 or a new IPv6
- `ipv4` — require a new IPv4
- `ipv6` — require a new IPv6
- `both` — require both IPv4 and IPv6 to be new

`rotation.history_size` bounds how many hashed assignments are retained.

For MASQUE endpoint family preference, continue to use:

- `masque_options.use_ipv6=false` for IPv4 endpoint selection
- `masque_options.use_ipv6=true` for IPv6 endpoint selection

## Built-in cap probes

Current built-in probe names:

- `internet`
- `warp`
- `gemini`
- `chatgpt`

These are **code-defined** probes, not arbitrary shell commands.

That keeps the system predictable and efficient while leaving room for later built-ins such as YouTube-specific capability checks.

## Required vs optional caps

Each cap check has:

- `required`
- `rotate_on_fail`

Behavior:

- if `rotate_on_fail=true`, the daemon may try rotation to satisfy that cap
- if `required=true` and rotation budget is exhausted, the daemon exits non-zero
- if `required=false` and rotation budget is exhausted, the daemon keeps running and remains degraded

## Notes on current implementation

- `access` is now the service-side exposure model
- the old single `mode` / `listen_host` / `listen_port` fields are still accepted for compatibility
- internally, the system derives those legacy fields from the first access entry when needed
- `http` access already supports HTTPS via CONNECT
- `tun` is not yet implemented and is rejected by config validation today

## Suggested operator workflow

### Inspect addresses

```bash
cfwarp-cli registration show
cfwarp-cli address show
```

### Validate config

```bash
cfwarp-cli validate --json
```

### Check current capabilities manually

```bash
cfwarp-cli unlock test --service gemini --service chatgpt
```

### Run long-lived management

```bash
cfwarp-cli daemon run
```

### Force a live rotation later

```bash
cfwarp-cli daemon ctl rotate
```

## Design goal

Compared with the large popular shell scripts, this implementation deliberately avoids:

- giant mutable bash control flow
- dozens of unrelated media checks
- package-manager orchestration mixed with runtime policy
- hidden always-on retry loops outside the managed daemon

The goal is:

- small CLI surface
- explicit JSON settings
- predictable daemon behavior
- live operator control without restarting the service
