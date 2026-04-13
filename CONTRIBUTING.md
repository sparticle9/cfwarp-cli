# Contributing

Thanks for looking at `cfwarp-cli`.

Project is early, narrow, and benchmark-driven. Best contributions come from changes with clear before/after evidence.

## Good first contribution areas

- native MASQUE startup retry root cause
- endpoint family selection strategy
- packet size / QUIC tuning experiments
- userspace dataplane profiling
- benchmark reporting and visualization
- packaging and reproducibility

See:

- `docs/status/2026-04-native-masque-status.md`
- `docs/masque-performance-plan.md`
- `docs/native-masque-vs-singbox-review.md`
- `docs/benchmark-mechanism.md`

## Ground rules

Keep tracked files free of:

- host aliases
- inventory entries tied to real machines
- local paths
- account details
- tokens, keys, or secrets

Use generic placeholders in docs and examples.

## Dev workflow

### 1. Run tests

```bash
go test ./...
```

### 2. Read status and benchmark docs before changing MASQUE code

Start here:

- `docs/status/2026-04-native-masque-status.md`
- `docs/benchmark-mechanism.md`

### 3. Keep changes measurable

For perf or runtime behavior changes, include at least one of:

- quick bench result
- real-target bench result
- profiling data
- log/runtime evidence for startup behavior

## Benchmark entry points

### Quick protocol comparison

Use for fast proxy-path comparison on remote host.

File:

- `ansible/protocol-quick-bench.yml`

Example:

```bash
ansible-playbook -i inventory.ini ansible/protocol-quick-bench.yml --limit warp
```

With tuning:

```bash
ansible-playbook -i inventory.ini ansible/protocol-quick-bench.yml --limit warp \
  -e cfwarp_image=ghcr.io/sparticle9/cfwarp-cli:your-tag \
  -e latency_runs=30 \
  -e download_runs=5 \
  -e udp_buffer_bytes=7500000 \
  -e masque_connect_port=443 \
  -e masque_initial_packet_size=1242
```

### Real-target protocol comparison

Use for remote `iperf3` + HTTP comparison across implementations.

File:

- `ansible/protocol-real-bench.yml`

Example:

```bash
ansible-playbook -i inventory.ini ansible/protocol-real-bench.yml --limit warp \
  -e cfwarp_image=ghcr.io/sparticle9/cfwarp-cli:your-tag \
  -e bench_id=protocol-real-test
```

Tuned example:

```bash
ansible-playbook -i inventory.ini ansible/protocol-real-bench.yml --limit warp \
  -e cfwarp_image=ghcr.io/sparticle9/cfwarp-cli:your-tag \
  -e bench_id=protocol-real-tuned \
  -e udp_buffer_bytes=7500000 \
  -e masque_connect_port=443 \
  -e masque_mtu=1280 \
  -e masque_initial_packet_size=1242 \
  -e masque_keepalive_period_seconds=25 \
  -e masque_reconnect_delay_millis=1200
```

## Benchmark notes

- quick bench is practical proxy-path comparison, not transport-pure microbenchmark
- real-target bench compares `wireguard`, native `masque`, and upstream `usque`
- longer runs beat one-off wins
- startup success and retry count matter, not only throughput

## Pull request guidance

PR should answer:

1. What changed?
2. Why?
3. What evidence says change helps?
4. What tradeoff changed?
5. What remains unresolved?

Good PR shape:

- small focused diff
- updated doc or note when behavior changes
- benchmark or test output in PR body

## If you want to help but do not know where to start

Open issue or PR draft around one of these:

- reproduce startup retry cause
- compare IPv4 vs IPv6 endpoint selection across more paths
- run packet size sweep and summarize results
- profile userspace dataplane under download-heavy load
- improve public benchmark result presentation
