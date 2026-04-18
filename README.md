# cfwarp-cli

`cfwarp-cli` is a Docker-friendly toolkit for running Cloudflare WARP-backed outbound proxy lanes with an explicit, inspectable control plane.

Current support is aimed at:

- **Linux** — full runtime focus
- **macOS on Apple Silicon** — supported for local CLI/config workflows and local Docker usage on the legacy WireGuard lane; native MASQUE and remote deployment tooling remain Linux-first

with broader platform support expected over time.

Today the project supports two main runtime paths:

- **stable:** `singbox-wireguard`
- **experimental:** native **MASQUE**

It is designed for server-side use cases where you want a small, scriptable tool rather than a desktop-style VPN client:

- localhost SOCKS5 / HTTP proxy exposure
- Docker sidecar or shared-host egress
- local CLI workflows on Linux and macOS (Apple Silicon)
- optional explicit config in `settings.json`
- CLI-friendly health, status, and rotation control
- repeatable benchmarking across implementations

## Start in ~60 seconds

If this is your first run, use one command:

```bash
# Linux/macOS quick smoke (no local files needed)
docker run --rm -d --name cfwarp-quickstart -p 127.0.0.1:1080:1080 ghcr.io/sparticle9/cfwarp-cli:latest

# Wait a few seconds, then verify proxy + route
curl -fsSL --proxy socks5h://127.0.0.1:1080 https://www.cloudflare.com/cdn-cgi/trace | head -n 6

# Optional: CLI-visible status check from inside container
docker exec cfwarp-quickstart cfwarp-cli status --json --state-dir /home/cfwarp/.local/state/cfwarp-cli
```

Stop it when done:

```bash
docker stop cfwarp-quickstart
```

> This is intentionally minimal and works for first-time onboarding; if this works, the project is set up correctly.

## Current status

### Stable

- WireGuard via `singbox-wireguard`
- explicit proxy mode (`socks5`, `http`)
- Docker / GHCR deployment flow
- registration, import, render, start, stop, status
- dogfood Ansible playbooks for remote hosts

### Practical support matrix

| capability | Linux amd64 | Linux arm64 | macOS (Apple Silicon) |
|---|---:|---:|---:|
| config / validation / docs-driven workflows | yes | yes | yes |
| local CLI control-plane (`register`, `import`, `render`, `up`, `down`) | yes | ✅ validated on a Linux arm64 VPS (`orabr`) | yes |
| local Docker (`docker run` / `docker-compose`) for legacy WireGuard lane | yes (`linux/amd64` images) | ✅ (published images; container tested) | yes |
| local Docker for experimental MASQUE lane | yes | ✅ | no |
| native remote dogfood / Ansible deployment path | yes (Linux only) | yes (Linux only) | no |
| native MASQUE runtime (`runtime_family=native`, `transport=masque`) | experimental | experimental | no |

*Linux arm64 host verification is now validated for local CLI + container workflows from `orabr`; continue to report any regressions with exact host + image tag.

If you can validate on a real Linux arm64 VPS, add this to your checklist:

```bash
# replace <host> and optional user
ssh <host> 'uname -m && docker run --rm -d --name cfwarp-arm64-check -p 127.0.0.1:1080:1080 ghcr.io/sparticle9/cfwarp-cli:latest'
sleep 5
ssh <host> 'curl -fsSL --proxy socks5h://127.0.0.1:1080 https://www.cloudflare.com/cdn-cgi/trace | head -n 6'
ssh <host> 'docker stop cfwarp-arm64-check'
```

If you're a power user doing hardening/comparisons, use this as the baseline for your own matrix and issue reports.

For production-style traffic, use the WireGuard lane by default.

If you want to evaluate MASQUE, run it side-by-side with WireGuard and compare in your workload and region.


### Experimental

- native MASQUE runtime
- daemon-managed capability checks
- live rotation without restarting the daemon process
- hashed rotation memory with IPv4 / IPv6 distinctness policies

If you need the safest current lane for real traffic, use **WireGuard**.
If you want to evaluate the direction of the native runtime, run **MASQUE** alongside it and compare behavior.

## WireGuard vs MASQUE (practical guidance)

Use this for issue reports and internal comparisons:

| dimension | WireGuard (legacy) | MASQUE (native) |
|---|---|---|
| deployment maturity | stable | experimental |
| default profile for beginners | ✅ yes | ⚠️ for evaluation |
| startup behavior | predictable | currently improving; occasional reconnect/startup nuances |
| Linux arm64 support | ✅ image + container path; host-native check pending | ⚠️ host/native check pending |
| when to use | default path for production-like traffic; stable first-pass profile. | evaluation path and migration rehearsal; pair with WireGuard for comparison |

For side-by-side performance checks, use separate runtimes/states and the same target workload.
Good starting references are:

- `docs/dogfood-debian13.md` (dual-stack deployment)
- `docs/masque-real-target-matrix-20260408.md` (observability + measured tradeoffs)
- `docs/native-masque-vs-singbox-review.md` (design risks and caveats)

### Issue report minimum (helps collaborators close gaps fast)

If something fails, include:

- exact platform and architecture (e.g. `linux/arm64`, `linux/amd64`, `darwin/arm64`)
- command path used (`docker run`, `cfwarp-cli up`, `daemon run`, etc.)
- full `settings.json` or env overlay
- `cfwarp-cli status --json --require-warp`
- `curl -fsSL --proxy socks5h://127.0.0.1:1080 https://www.cloudflare.com/cdn-cgi/trace`
- if used, one line from speed check (5MB `__down`)
- latest 40 lines from `backend.stderr.log` (container or `state/logs`)
- whether this is native Linux arm64 host vs docker image

## What the tool can do

### Registration and state

- register a new consumer WARP device
- import existing WARP credentials
- inspect current registration state and assigned addresses

### Runtime control

- render backend config
- connect / disconnect the configured runtime
- run a long-lived daemon
- inspect runtime and backend status
- force a live rotation with `daemon ctl rotate`

### Exposure model

Current access types:

- `socks5`
- `http` (including HTTPS via CONNECT)

Reserved for later:

- `tun`

### Rotation and health policy

The current config model uses these terms:

- **transport** — upstream tunnel protocol (`wireguard` or `masque`)
- **access** — how local clients consume the tunnel (`socks5` or `http`)
- **caps** — built-in capability probes
- **rotation** — remediation policy when caps fail
- **daemon** — the long-running manager process

## Quick start

### 1. Build or use a published image

For deployable or shared usage, prefer the GHCR images built by GitHub Actions (multi-arch; Linux `amd64` + `arm64`).

Examples:

- `ghcr.io/sparticle9/cfwarp-cli:latest`
- `ghcr.io/sparticle9/cfwarp-cli:latest-debian`
- `ghcr.io/sparticle9/cfwarp-cli:sha-<commit>`

### 2. Pick the usage mode you want

#### A. Out-of-the-box, no persistence

For many quick scenarios, no config mount and no durable data mount should be needed:

```bash
docker run --rm -p 127.0.0.1:1080:1080 ghcr.io/sparticle9/cfwarp-cli:latest
```

This uses built-in defaults and ephemeral in-container state.

#### B. Optional config file only

If you want to control behavior without opting into full durable state, mount a standalone settings file:

```bash
docker run --rm \
  -p 127.0.0.1:1080:1080 \
  -v "$PWD/settings.json:/etc/cfwarp/settings.json:ro" \
  -e CFWARP_SETTINGS_FILE=/etc/cfwarp/settings.json \
  ghcr.io/sparticle9/cfwarp-cli:latest
```

#### C. Durable data directory

If you want registration/account persistence, rotation memory, and reusable daemon state, mount a data directory too:

```bash
docker run --rm \
  -p 127.0.0.1:1080:1080 \
  -v "$PWD/settings.json:/etc/cfwarp/settings.json:ro" \
  -v "$PWD/data:/home/cfwarp/.local/state/cfwarp-cli" \
  -e CFWARP_SETTINGS_FILE=/etc/cfwarp/settings.json \
  ghcr.io/sparticle9/cfwarp-cli:latest
```

For the Debian image, use `/home/nonroot/.local/state/cfwarp-cli` as the data mount path.

### 3. Example `settings.json`

Example `settings.json` for the stable WireGuard lane:

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
    }
  ],
  "caps": {
    "interval_seconds": 300,
    "checks": [
      {
        "probe": "internet",
        "required": true,
        "rotate_on_fail": true,
        "timeout_seconds": 10
      }
    ]
  },
  "rotation": {
    "enabled": true,
    "max_attempts_per_incident": 3,
    "settle_time_seconds": 12,
    "cooldown_seconds": 1800,
    "restore_last_good": true,
    "distinctness": "either",
    "history_size": 128
  }
}
```

Optional explicit DNS policy for environments where system DNS is intercepted or fake-IP translated:

```json
{
  "runtime_family": "legacy",
  "transport": "wireguard",
  "access": [
    {
      "type": "socks5",
      "listen_host": "127.0.0.1",
      "listen_port": 1080
    }
  ],
  "dns": {
    "mode": "https",
    "server": "1.1.1.1",
    "strategy": "ipv4_only"
  }
}
```

Supported `dns.mode` values today:

- `local`
- `udp`
- `https`

For `https`, the default port/path are `443` and `/dns-query`.
Useful env vars for container workflows:

- `CFWARP_DNS_MODE`
- `CFWARP_DNS_SERVER`
- `CFWARP_DNS_SERVER_PORT`
- `CFWARP_DNS_PATH`
- `CFWARP_DNS_STRATEGY`

### 4. Validate the resolved config

With a standalone settings file:

```bash
cfwarp-cli validate --json --settings-file /path/to/settings.json
```

With a durable data dir:

```bash
cfwarp-cli validate --json --state-dir /path/to/state
```

### 5. Optional Cloudflare throughput smoke test

If status is healthy, you can run a lightweight throughput check through the proxy via Cloudflare's speedtest APIs:

```bash
# Download-only (5,000,000-byte sample; bump via bytes=... if needed for heavier checks)
curl -o /dev/null \
  --proxy socks5h://127.0.0.1:1080 \
  -sS -w 'speed.download.bytes=%{size_download} time=%{time_total}s bps=%{speed_download}\n' \
  'https://speed.cloudflare.com/__down?bytes=5000000'

# Upload-only (optional, 2 MB sample)
curl -o /dev/null \
  --proxy socks5h://127.0.0.1:1080 \
  -sS -w 'speed.upload.bytes=%{size_upload} time=%{time_total}s bps=%{speed_upload}\n' \
  -X POST 'https://speed.cloudflare.com/__up' \
  --data-binary @<(head -c 2000000 </dev/zero)
```

`time_total` is in seconds, `size_*` is bytes, and `bps` is bytes/sec (multiply by 8/1_000_000 for Mb/s).

Use this only as a rough sanity check and avoid very large samples for routine checks.

### 6. Run the daemon

```bash
cfwarp-cli daemon run --settings-file /path/to/settings.json
```

or

```bash
cfwarp-cli daemon run --state-dir /path/to/state
```

### 7. Inspect and control it

```bash
cfwarp-cli daemon ctl status --settings-file /path/to/settings.json
cfwarp-cli daemon ctl check --settings-file /path/to/settings.json
cfwarp-cli daemon ctl rotate --state-dir /path/to/state
cfwarp-cli address show --json --state-dir /path/to/state
```

## Docker / remote-host usage

The most fully documented deployment path today is still a Linux remote host.

For a practical remote deployment with both WireGuard and MASQUE lanes side by side, start here:

- `docs/dogfood-debian13.md`

That runbook covers:

- Docker Compose deployment
- per-service bind-mounted state directories
- localhost-only proxy ports
- sing-box fragment integration
- on-demand status inspection

Relevant assets:

- `deploy/docker-compose.dogfood.yml`
- `deploy/dogfood.env.example`
- `deploy/daemon-proxy.env.example`
- `ansible/dogfood-deploy.yml`
- `ansible/dogfood-status.yml`

## Common commands

### Registration

```bash
cfwarp-cli register --state-dir /path/to/state
cfwarp-cli import --state-dir /path/to/state --file /path/to/account.json
cfwarp-cli registration show --state-dir /path/to/state
```

### Runtime selection and startup

```bash
cfwarp-cli transport show --state-dir /path/to/state
cfwarp-cli mode show --state-dir /path/to/state
cfwarp-cli up --state-dir /path/to/state
cfwarp-cli status --json --state-dir /path/to/state
cfwarp-cli down --state-dir /path/to/state
```

### Rotation and unlock checks

```bash
cfwarp-cli unlock test --service gemini --service chatgpt --state-dir /path/to/state
cfwarp-cli rotate --attempts 5 --service gemini --state-dir /path/to/state
cfwarp-cli address show --json --state-dir /path/to/state
```

## User documentation map

### Operator / user docs

- `docs/README.md` — documentation map
- `docs/dogfood-debian13.md` — remote host deployment and sing-box integration
- `docs/warp-rotation-unlock.md` — caps, rotation, daemon, hashed IP memory

### Project background and evaluation docs

- `docs/background.md`
- `docs/benchmark-mechanism.md`
- `docs/benchmark-package.md`
- `docs/benchmark-report-case.md`
- `docs/status/2026-04-native-masque-status.md`
- `docs/native-masque-vs-singbox-review.md`

## Contributor guide

If you want to work on the project, read:

- `CONTRIBUTING.md`
- `docs/README.md`
- `docs/status/2026-04-native-masque-status.md`
- `docs/specs/002-unified-transport-dataplane/design.md`

Good contribution areas right now:

- native MASQUE startup reliability
- endpoint family strategy and IPv4 / IPv6 behavior
- userspace dataplane profiling
- benchmark reporting and visualization
- packaging and reproducible deployment
- docs and usability polish

## Development notes

### Run tests

```bash
go test ./...
```

### Keep docs aligned with behavior

If a change affects user-visible behavior, update the relevant docs in the same change set:

- `README.md`
- `CONTRIBUTING.md`
- `docs/dogfood-debian13.md`
- `docs/warp-rotation-unlock.md`
- spec or status docs when architecture or scope changed

### Security / hygiene rules

Do not commit:

- real host aliases or inventory details
- local machine paths
- tokens, keys, or secrets
- user-specific environment details

Use placeholders such as:

- `proxy-host-1`
- `/path/to/state`
- `example-token`

## Roadmap themes

Near-term themes:

- improve native MASQUE stability
- keep WireGuard deployment simple and dependable
- continue unifying runtime control around the transport/access/caps model
- improve health reporting so container health matches real data-plane state more closely
- make docs and operator workflows easier to follow

## License / repository context

This repository is currently best read as an actively developed, benchmark-driven engineering project rather than a polished end-user VPN product.

If you want a production-oriented starting point today:

- use the WireGuard lane first
- treat native MASQUE as an evaluation lane
- prefer GHCR-built images for real deployment
