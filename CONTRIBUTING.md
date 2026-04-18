# Contributing

Thanks for looking at `cfwarp-cli`.

This project is still early, but it already has two real audiences:

- **operators** who want a practical WARP-backed proxy lane on Linux / Docker
- **contributors** who want to improve the runtime, control plane, docs, and benchmark evidence

The best changes are small, explicit, and backed by tests, runtime evidence, or benchmark results.

## Before you start

Read these first:

- `README.md`
- `docs/README.md`
- `docs/status/2026-04-native-masque-status.md`
- `docs/warp-rotation-unlock.md`

If your change is architectural, also read:

- `docs/specs/002-unified-transport-dataplane/design.md`
- `docs/native-masque-vs-singbox-review.md`

If your change is deployment-oriented, also read:

- `docs/dogfood-debian13.md`

## Good contribution areas

Current high-value areas:

- native MASQUE startup and reconnect reliability
- endpoint family strategy and IPv4 / IPv6 behavior
- userspace dataplane profiling
- benchmark reporting and visualization
- packaging and reproducibility
- operator UX and documentation polish

## Ground rules

Keep tracked files free of:

- real host aliases
- real inventory entries
- local machine paths
- account identifiers tied to a user
- tokens, keys, or secrets

Use generic placeholders in examples:

- `proxy-host-1`
- `/path/to/project`
- `/path/to/state`
- `example-token`

For deployable artifacts, prefer **GitHub Actions / GHCR-built images** over local-only images.

## Development workflow

### 1. Make the smallest useful change

Prefer focused diffs over broad cleanup unless the task is explicitly a cleanup pass.

### 2. Run tests

```bash
go test ./...
```

If you touched a narrower area first, run the local package tests before the full suite.

### 3. Update docs when behavior changes

If your change affects behavior, config, operations, or terminology, update the relevant docs in the same change.

Most commonly:

- `README.md`
- `CONTRIBUTING.md`
- `docs/README.md`
- `docs/dogfood-debian13.md`
- `docs/warp-rotation-unlock.md`

### 4. Keep changes measurable when possible

For performance or runtime behavior changes, include at least one of:

- benchmark output
- real-target bench result
- profiling data
- runtime / log evidence
- before/after status output

## Repo map

### User / operator docs

- `README.md`
- `docs/README.md`
- `docs/dogfood-debian13.md`
- `docs/warp-rotation-unlock.md`

### Development / architecture docs

- `docs/specs/001-minimal-wireguard-proxy/*`
- `docs/specs/002-unified-transport-dataplane/*`
- `docs/status/2026-04-native-masque-status.md`
- `docs/native-masque-vs-singbox-review.md`
- `docs/tun-decision-note.md`

### Benchmark docs

- `docs/benchmark-package.md`
- `docs/benchmark-mechanism.md`
- `docs/benchmark-report-case.md`
- `docs/masque-performance-plan.md`
- `docs/masque-real-target-matrix-20260408.md`

### Deployment assets

- `deploy/docker-compose.dogfood.yml`
- `deploy/dogfood.env.example`
- `deploy/daemon-proxy.env.example`
- `ansible/dogfood-deploy.yml`
- `ansible/dogfood-status.yml`

## Benchmark entry points

### Quick protocol comparison

Use for fast proxy-path comparison on a remote host.

File:

- `ansible/protocol-quick-bench.yml`

Example:

```bash
ansible-playbook -i inventory.ini ansible/protocol-quick-bench.yml --limit warp
```

Example with tuning:

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

Example with tuning:

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

## What a good PR should answer

A good PR description answers:

1. What changed?
2. Why was it needed?
3. What evidence suggests it helps?
4. What tradeoff changed?
5. What is still unresolved?

Good PR shape:

- focused diff
- tests updated or added when appropriate
- docs updated when behavior changed
- benchmark or runtime evidence in the PR body for performance / reliability work

## If you are not sure where to start

A solid first contribution is often one of these:

- reproduce and isolate a native MASQUE startup failure
- compare IPv4 vs IPv6 endpoint behavior across more paths
- run a packet-size tuning sweep and summarize it clearly
- improve a confusing operator workflow in docs
- make a status / health output easier to understand
