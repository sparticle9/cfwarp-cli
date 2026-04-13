# MASQUE Real-Target Tuning Matrix — 2026-04-08

This note captures the first real-target tuning matrix run using:

- warp host: `tyo`
- remote benchmark target: `laxhd`
- bench harness: `ansible/protocol-real-bench.yml`
- image under test: `ghcr.io/sparticle9/cfwarp-cli:perf-masque-tuning`
- image revision: `2ffa41d4c9c10653db9683357301ceeb25aeeb4a`

It is intentionally a **fast directional matrix**, not a decision-grade benchmark set.

## Run shape

Shared run parameters:

- `startup_wait_seconds=8`
- `latency_runs=2`
- `iperf3_runs=1`
- `iperf3_duration=3`
- `iperf3_parallel=1`
- `http_runs=1`

Compared implementations in every run:

- `wireguard` — `cfwarp-cli` legacy WireGuard path
- `masque` — `cfwarp-cli` native MASQUE path
- `usque` — vanilla upstream release binary

## MASQUE experiment matrix

| Bench ID | MASQUE tuning | Endpoint family | Median latency | iperf up/down | HTTP mean |
|---|---|---:|---:|---:|---:|
| `perf-matrix-default` | default | ipv4 | `39.1 ms` | `74.09 / 29.93 Mbps` | `20.47 MiB/s` |
| `perf-matrix-udpbuf` | `udp_buffer_bytes=7500000` | ipv4 | `32.2 ms` | `60.81 / 27.88 Mbps` | `18.96 MiB/s` |
| `perf-matrix-tuned` | UDP buf + `connect_port=443` + `initial_packet_size=1242` + `keepalive=25s` + `reconnect_delay=1200ms` | ipv4 | `36.4 ms` | `106.23 / 47.32 Mbps` | `15.82 MiB/s` |
| `perf-matrix-ipv6` | tuned + `use_ipv6=true` | ipv6 | `36.6 ms` | `76.87 / 32.42 Mbps` | `25.04 MiB/s` |

All four MASQUE runs still recorded:

- `startup_attempt=1`
- `retry_count=1`

So the current startup retry mitigation is still being exercised even when the run later succeeds.

## WireGuard and usque context from the same runs

### `perf-matrix-default`
- wireguard: `37.8 ms`, `175.74 / 128.88 Mbps`, `25.52 MiB/s`
- usque: `43.0 ms`, `38.78 / 22.03 Mbps`, `19.97 MiB/s`

### `perf-matrix-udpbuf`
- wireguard: `26.9 ms`, `165.26 / 126.3 Mbps`, `25.24 MiB/s`
- usque: `35.3 ms`, `52.07 / 33.28 Mbps`, `19.98 MiB/s`

### `perf-matrix-tuned`
- wireguard: `27.1 ms`, `45.07 / 0.0 Mbps`, `22.77 MiB/s`
- usque: `40.3 ms`, `52.77 / 32.65 Mbps`, `17.67 MiB/s`

### `perf-matrix-ipv6`
- wireguard: `25.4 ms`, `144.67 / 89.53 Mbps`, `22.09 MiB/s`
- usque: `41.2 ms`, `40.53 / 17.94 Mbps`, `19.52 MiB/s`

Because this was a very small matrix, the WireGuard `iperf3` variability here should be treated as noise-sensitive.

## Direct observations

### 1. UDP buffer increase helped latency, but not everything
`perf-matrix-udpbuf` improved MASQUE latency versus the untuned default:

- median latency: `39.1 -> 32.2 ms`
- p95 latency: `43.8 -> 32.8 ms`

But throughput and HTTP were not clearly better in the same run.

### 2. The tuned IPv4 profile improved iperf3 throughput the most
The tuned IPv4 run gave the best MASQUE `iperf3` numbers in this matrix:

- upload: `106.23 Mbps`
- download: `47.32 Mbps`

But it also gave the weakest HTTP result among the MASQUE cases:

- HTTP mean: `15.82 MiB/s`

That suggests the current tuning set may help one practical traffic shape while hurting another.

### 3. IPv6 MASQUE is viable and may help bulk HTTP on this path
The IPv6-tuned run selected:

- endpoint: `[2606:4700:103::1]:443`
- family: `ipv6`

It delivered the best MASQUE HTTP result in this matrix:

- HTTP mean: `25.04 MiB/s`

That was better than:

- default IPv4 MASQUE: `20.47 MiB/s`
- tuned IPv4 MASQUE: `15.82 MiB/s`
- upstream `usque`: `19.52 MiB/s` in the same run

But it did **not** beat the tuned IPv4 run on `iperf3` throughput.

### 4. Native MASQUE now beats vanilla usque on several points in this matrix
Compared with upstream `usque`, native MASQUE showed stronger results in multiple runs:

- better latency in the UDP-buffer and tuned runs
- higher `iperf3` throughput in tuned IPv4
- much higher HTTP throughput in tuned IPv6

That does not prove MASQUE is solved, but it does suggest the native runtime now has credible room to outperform the reference implementation in some shapes.

### 5. Retry-on-start remains a standing tax
Every successful MASQUE run still had `retry_count=1`.

So even while tuning steady-state performance, startup behavior is still not truly clean. This should stay a separate line of investigation, because it can otherwise distort short benchmark runs.

## Current working hypothesis

At this point the evidence suggests:

1. **No single MASQUE tuning profile dominates all traffic shapes**.
2. **IPv4 tuned profile** currently looks more promising for `iperf3` throughput.
3. **IPv6 tuned profile** currently looks more promising for bulk HTTP transfer.
4. The most likely next wins are still in:
   - endpoint-family strategy
   - packet-size tuning
   - userspace dataplane / netstack efficiency
   - startup retry root cause isolation

## Recommended next matrix

Run a slightly longer, still practical matrix with:

1. `default`
2. `udpbuf`
3. `udpbuf + ipv4 tuned`
4. `udpbuf + ipv6 tuned`
5. `udpbuf + ipv4 tuned + initial_packet_size=1200`
6. `udpbuf + ipv4 tuned + initial_packet_size=1280` if stable

And increase run shape to something less noisy, e.g.:

- `latency_runs=5`
- `iperf3_duration=8` or `10`
- `http_runs=2`

## Suggested decision rule for the next round

- If **IPv6 keeps winning HTTP** while **IPv4 keeps winning iperf3**, treat family selection as workload-sensitive rather than globally fixed.
- If a packet-size sweep can improve both `iperf3` and HTTP on the same family, prioritize that next.
- If steady-state wins remain inconsistent while retry count stays fixed at `1`, separate startup-path investigation from dataplane tuning conclusions.
