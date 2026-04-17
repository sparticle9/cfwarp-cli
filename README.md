# cfwarp-cli

`cfwarp-cli` is a CLI-first toolkit for building lightweight, Linux-friendly outbound connectivity on top of Cloudflare WARP.

The project started from a practical gap: existing WARP-based container solutions tend to optimize for convenience or feature breadth, while server-side workloads often need something narrower and more explicit:

- deterministic Docker deployment
- low operational footprint
- explicit proxy and sidecar-style egress patterns
- controllable endpoint selection / experimentation
- honest, benchmarkable tradeoffs between implementations

## Why this project exists

The main motivation is to own the parts that matter for server-side egress:

- bootstrap and manage WARP connectivity without hiding the moving parts
- compare multiple tunnel/proxy approaches under the same benchmark harness
- keep deployment simple enough for small VPS or sidecar scenarios
- leave room for future backends such as kernel WireGuard and MASQUE without locking the CLI contract too early

## Current implementation direction

The current implementation remains intentionally narrow, but is no longer WireGuard-only:

- **Docker-first**
- **Linux-first**
- **explicit proxy mode first**
- **stable `singbox-wireguard` backend**
- **experimental native MASQUE backend**

## Deployment / usage scenarios

The codebase is currently oriented around three practical deployment patterns:

1. **Explicit proxy container**
   - expose a SOCKS5 endpoint for proxy-aware clients
   - useful for crawlers, API clients, and tools that can already speak SOCKS

2. **Sidecar / shared-network egress**
   - run the WARP-backed container next to another container and share its network namespace
   - useful when the application itself does not support proxy configuration well

3. **Implementation comparison / battle testing**
   - compare multiple published images and upstream implementations under one repeatable harness
   - currently includes `cfwarp-cli` variants, the original `MicroWARP`, and vanilla `usque` in the protocol-focused real-target bench

## Performance comparison philosophy

This project does **not** treat one benchmark as sufficient.

It separates:

- **raw tunnel behavior**
  - RTT
  - bidirectional iperf3 throughput
  - large-file HTTP transfer

- **API-like workload behavior**
  - metadata-style small requests
  - LLM-style synchronous POSTs
  - upload-heavy requests
  - long-lived streaming responses

This matters because the fastest raw tunnel is not always the best fit for real workloads such as:

- LLM API calls
- TTS / ASR calls
- metadata retrieval
- many concurrent small/medium requests

## Documents

- `docs/background.md` — research notes and implementation background
- `docs/benchmark-package.md` — entry point for sharing the benchmark work internally
- `docs/benchmark-mechanism.md` — benchmark harness design, phases, and result semantics
- `docs/benchmark-report-case.md` — interpretation of the latest benchmark set for the intended workload mix
- `docs/dogfood-debian13.md` — Debian 13 dogfood runbook for dual localhost-bound WireGuard and MASQUE proxies
- `docs/warp-rotation-unlock.md` — WARP address rotation, access/caps config, and daemon control workflow
- `docs/status/2026-04-native-masque-status.md` — public status memo for native MASQUE support
- `docs/specs/001-minimal-wireguard-proxy/requirements.md` — MVP requirements
- `docs/specs/001-minimal-wireguard-proxy/design.md` — MVP design and Docker deployment
- `docs/specs/001-minimal-wireguard-proxy/tasks.md` — incremental implementation plan

## Current status

Implemented today:

- `cfwarp-cli` Docker-oriented WireGuard backend flow
- experimental native MASQUE HTTP/SOCKS runtime path
- built-in WARP address inspection, manual rotation, daemon-managed caps checks, lightweight unlock probes, and hashed rotation memory with IPv4/IPv6 distinctness policies
- multi-arch GHCR container publishing workflow for Alpine and Debian variants
- published-image benchmark harness
- comparison against original `MicroWARP`
- protocol-focused real-target bench with vanilla `usque`, remote `iperf3`, and remote HTTP transfer
- raw tunnel + API-like workload benchmarking
- markdown/JSON report generation with per-container resource summaries
- dogfood-oriented deploy and status playbooks for remote Docker hosts

Not implemented yet:

- full CLI/control-plane polish for the new transport-oriented UX
- broader native runtime packaging and docs beyond the current Alpine-first path
- chart generation in the reporting pipeline
- broader provider/backend families beyond the current WARP-focused scope

## Project status

Current stable path:

- `singbox-wireguard`

Current experimental path:

- native MASQUE

Current dogfood posture:

- use `singbox-wireguard` as the default lane for real daemon traffic
- run native MASQUE alongside it on a second localhost-bound port for verification and comparison

Recent MASQUE work completed:

- control-plane and runtime support for native MASQUE
- runtime instrumentation and diagnostics
- tuning knobs and protocol bench coverage
- comparative evaluation against WireGuard and upstream `usque`

Current state:

- native MASQUE support is real and usable for continued development
- branch stable
- tests green
- benchmark evidence directional, not final
- contributor help wanted on startup retry, endpoint family strategy, dataplane profiling, and packaging/docs

See:

- `docs/status/2026-04-native-masque-status.md`
- `docs/native-masque-vs-singbox-review.md`
- `CONTRIBUTING.md`

## Contributing

See `CONTRIBUTING.md` for test, benchmark, and review workflow.
