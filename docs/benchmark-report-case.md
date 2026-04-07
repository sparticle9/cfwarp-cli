# Benchmark Report for the Intended Workload Mix

This note interprets the current benchmark suite for the primary target workload style:

- many concurrent requests
- mostly small to medium payloads
- some upload-heavy requests
- some long-lived streaming responses
- server-side deployment, not desktop browsing

## Important framing

The benchmark suite is intentionally split into two layers:

1. **Raw tunnel metrics**
   - useful for sanity checks and upper bounds
   - not sufficient on their own

2. **API-like workload metrics**
   - much closer to real usage
   - should drive most implementation choices

For this workload mix, the most important metrics are:

- p95 latency
- p99 latency
- error rate under concurrency
- memory footprint of the tunnel container itself
- behavior under upload-heavy and streaming scenarios

## Practical reading of the current results

### `cfwarp-cli` alpine variant

Strengths:
- strongest raw throughput in the current comparison
- best p95 on most synchronous API-style profiles
- strong general-purpose choice if throughput and balanced latency both matter

Weaknesses:
- larger memory footprint than `MicroWARP`
- not always best on the longest streaming profile

Best fit:
- general default choice for mixed workloads
- synchronous API traffic
- higher-throughput egress scenarios

### `cfwarp-cli` debian/distroless variant

Strengths:
- broadly similar behavior to alpine
- sometimes competitive or better on selected heavier request patterns
- useful as a native-glibc comparison point

Weaknesses:
- currently does not show a clear performance win over alpine in the present dataset
- API-suite memory can spike higher than alpine on some streaming cases

Best fit:
- comparison / hardening baseline
- further validation if native-glibc packaging is preferred for operational reasons

### `MicroWARP`

Strengths:
- by far the smallest memory footprint
- best RTT in the current test set
- very attractive for tiny-request / metadata-style traffic
- competitive on large HTTP download in this environment

Weaknesses:
- much lower raw bidirectional throughput
- much weaker on medium synchronous POST-style API workloads
- less suitable when request bodies grow or workloads become more compute/transport intensive

Best fit:
- highly memory-sensitive environments
- metadata/control-plane style traffic
- lightweight egress where raw throughput is not the priority

## What the current benchmark means for real decisions

### If the workload is mostly metadata / control-plane

Look at:
- `metadata` family p95
- memory peak

Interpretation:
- `MicroWARP` is very attractive here because its latency is competitive and its footprint is tiny.

### If the workload is mostly synchronous LLM API traffic

Look at:
- `llm_sync` family p95
- error rate
- memory peak

Interpretation:
- `cfwarp-cli` alpine is currently the strongest default.
- `MicroWARP` is meaningfully weaker once request size and concurrency rise together.

### If the workload is upload-heavy (ASR-like)

Look at:
- `upload` family p95
- tail latency
- memory stability

Interpretation:
- `cfwarp-cli` variants are currently the safer bet.
- `MicroWARP` remains viable for lighter upload scenarios, but is less convincing as request size increases.

### If the workload is stream-heavy

Look at:
- `stream` family p95
- long-tail latency
- memory peak during stream profiles

Interpretation:
- all implementations become slower here, as expected.
- the `cfwarp-cli` variants are currently more balanced than `MicroWARP` for sustained streaming-like traffic.

## Recommended default choice today

If one implementation must be chosen as the broad default today:

- choose **`cfwarp-cli` alpine** for general mixed workloads

If a separate lightweight deployment class is needed:

- keep **`MicroWARP`** as a memory-first option for metadata/light traffic

If native-glibc packaging must be evaluated further:

- keep **`cfwarp-cli` debian/distroless** in the benchmark matrix, but treat it as a validation branch rather than the current default winner

## What still needs further iteration

The current benchmark suite is already useful, but future iterations should refine:

1. more workload-size sweeps in each family
2. stronger request-normalized CPU metrics
3. more streaming cases with longer duration / lower concurrency mixes
4. repeated-run variance checks across different times of day

## Bottom line

For the intended workload mix, this project should not be judged by raw throughput alone.

The current evidence suggests:
- **`cfwarp-cli` alpine** is the best all-rounder
- **`MicroWARP`** is the best memory-first niche option
- **`cfwarp-cli` debian/distroless** remains valuable to keep in the matrix, but has not yet earned default status
