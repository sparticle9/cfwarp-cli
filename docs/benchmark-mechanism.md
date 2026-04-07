# Benchmark Mechanism

This document explains how the benchmark harness in `ansible/bench.yml` works, what it measures, and how to interpret the outputs.

## Goals

The benchmark harness is designed to compare multiple published tunnel/proxy implementations under the same conditions.

It deliberately measures two different classes of behavior:

1. **Raw tunnel behavior**
   - RTT
   - bidirectional throughput
   - large-object transfer

2. **API-like workload behavior**
   - many concurrent small or medium requests
   - upload-heavy requests
   - long-lived streaming-style responses

This split exists because a tunnel that looks good under `iperf3` can still behave poorly for real application traffic.

## Host roles

The benchmark setup uses three logical roles:

- **controller**
  - where Ansible is invoked
  - only orchestrates and fetches results

- **proxy host**
  - runs the implementation under test
  - also runs helper load-generator containers in the tested container's network namespace

- **benchmark target host**
  - runs remote services used by the benchmark suite
  - currently:
    - `iperf3` server
    - static HTTP server for large object transfer
    - API-style HTTP server (`go-httpbin`) for `k6` workloads

## Implementations compared

The harness currently compares:

- `cfwarp-cli` alpine-based published image
- `cfwarp-cli` debian/distroless published image
- original `MicroWARP` published image

All benchmark runs use **published images only**.

## Benchmark phases

### 1. Raw latency

Tool:
- `ping`

Purpose:
- basic round-trip cost through the active tunnel

Output:
- p50 / p95 / p99 / mean / stddev RTT

### 2. Raw throughput

Tool:
- `iperf3 --bidir`

Purpose:
- simultaneous upload and download capacity through the tunnel

Output:
- upload median / p95 / mean
- download median / p95 / mean
- retransmit counts

### 3. Large object HTTP transfer

Tool:
- `curl`

Purpose:
- compare long-lived, application-level bulk transfer behavior

Output:
- speed in bytes/sec
- total request time
- HTTP status code

## API-like workload suite (`k6`)

The `k6` phase models the main application families we care about.

### Metadata family

Profiles:
- `meta-4k`
- `meta-8k`

Shape:
- tiny GET requests
- high concurrency
- low response size

Use case approximation:
- metadata retrieval
- control-plane lookups
- model/provider discovery

### LLM sync family

Profiles:
- `llm-8k`
- `llm-32k`

Shape:
- synchronous POST requests
- small-to-medium bodies
- moderate concurrency

Use case approximation:
- non-streaming LLM inference
- JSON API requests with moderate prompt size

### Upload family

Profiles:
- `asr-256k`
- `asr-1m`

Shape:
- upload-heavy POST requests
- lower concurrency than metadata

Use case approximation:
- ASR-style uploads
- medium-large request bodies with smaller responses

### Stream family

Profiles:
- `sse-2s-32k`
- `sse-5s-64k`

Shape:
- long-lived GET requests using drip-style streaming
- lower concurrency, longer duration

Use case approximation:
- SSE-like streaming responses
- LLM/TTS-style long-lived requests

## Resource measurement model

The benchmark reports CPU and memory for the **benchmarked tunnel container only**.

This is done by sampling the container's Linux cgroup directly:

- CPU:
  - from `cpu.stat` deltas
- memory:
  - from `memory.current`
- network I/O:
  - from per-container `docker stats` network counters

This means the reported resource values do **not** include:

- `k6` helper container resource usage
- `curl` helper container resource usage
- `iperf3` helper container resource usage
- benchmark target host services

## Why CPU percentages often look very small

These tunnel containers are relatively lightweight in this setup, and the host has more than one vCPU available. As a result, CPU percentages can remain near zero even when the tunnel is actively forwarding traffic.

That is why the report also emphasizes:

- memory peak
- per-phase resource breakdown
- workload-specific latency and throughput

## Report outputs

The harness produces:

- raw CSV files for each phase
- a markdown report
- a machine-readable JSON summary

Main output files:

- `latency_<ts>.csv`
- `throughput_iperf3_<ts>.csv`
- `throughput_http_<ts>.csv`
- `k6_summary_<ts>.csv`
- `container_stats_<ts>.csv`
- `report_<ts>.md`
- `summary_<ts>.json`

## Operational workflow

The playbook is designed for iterative reruns.

Recommended pattern:

1. start/reuse benchmark target services once
2. prepare proxy host once
3. rerun `bench,report` repeatedly while tuning

In other words, use:

- `servers,prepare` for environment setup
- `bench,report` for repeated experiment loops

The harness also attempts to clean stale sampler processes from interrupted runs before each variant starts.

## Future reporting improvements

Charts are intentionally not generated yet, to keep the Python reporting logic simple.

When charting is added later, the most useful plots will likely be:

- p95 latency by workload family
- throughput by variant
- memory peak by phase
- per-profile CPU summary

For now, the markdown report is intended to remain readable, explicit, and sufficient for direct decision-making without a charting dependency.
