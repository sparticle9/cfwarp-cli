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

The first implementation remains intentionally narrow:

- **Docker-first**
- **Linux-first**
- **explicit proxy mode first**
- **minimal WireGuard-based backend first**
- **future MASQUE path reserved, not yet implemented**

## Deployment / usage scenarios

The codebase is currently oriented around three practical deployment patterns:

1. **Explicit proxy container**
   - expose a SOCKS5 endpoint for proxy-aware clients
   - useful for crawlers, API clients, and tools that can already speak SOCKS

2. **Sidecar / shared-network egress**
   - run the WARP-backed container next to another container and share its network namespace
   - useful when the application itself does not support proxy configuration well

3. **Implementation comparison / battle testing**
   - compare multiple published images under one repeatable harness
   - currently includes `cfwarp-cli` variants and the original `MicroWARP`

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
- `docs/benchmark-mechanism.md` — benchmark harness design, phases, and result semantics
- `docs/benchmark-report-case.md` — interpretation of the latest benchmark set for the intended workload mix
- `docs/specs/001-minimal-wireguard-proxy/requirements.md` — MVP requirements
- `docs/specs/001-minimal-wireguard-proxy/design.md` — MVP design and Docker deployment
- `docs/specs/001-minimal-wireguard-proxy/tasks.md` — incremental implementation plan

## Current status

Implemented today:

- `cfwarp-cli` Docker-oriented WireGuard backend flow
- published-image benchmark harness
- comparison against original `MicroWARP`
- raw tunnel + API-like workload benchmarking
- markdown/JSON report generation with per-container resource summaries

Not implemented yet:

- MASQUE backend
- chart generation in the reporting pipeline
- broader provider/backend families beyond the current WARP-focused scope
