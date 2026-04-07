# Internal Benchmark Package

This document serves as the entry point for sharing the benchmark work internally.

## Package contents

Read in this order:

1. **Project overview**
   - `README.md`
   - why the project exists, what is implemented, and where benchmarking fits

2. **Benchmark mechanism**
   - `docs/benchmark-mechanism.md`
   - host roles, phases, workload families, and resource measurement scope

3. **Case-oriented interpretation**
   - `docs/benchmark-report-case.md`
   - how to read the benchmark in the context of the intended workload mix

4. **Latest benchmark outputs**
   - `bench-results/report_<timestamp>.md`
   - `bench-results/summary_<timestamp>.json`
   - plus the phase CSVs for raw inspection

## Latest reference benchmark set

Latest completed reference run at time of writing:

- timestamp: `20260407T184320`

Produced artefacts:

- `report_20260407T184320.md`
- `summary_20260407T184320.json`
- `latency_20260407T184320.csv`
- `throughput_iperf3_20260407T184320.csv`
- `throughput_http_20260407T184320.csv`
- `k6_summary_20260407T184320.csv`
- `container_stats_20260407T184320.csv`

## What this package is intended to answer

This benchmark package is not a generic marketing comparison.
It is intended to answer practical engineering questions such as:

- which implementation is best for tiny metadata/control-plane traffic?
- which implementation is best for synchronous API-style requests?
- which implementation is best for upload-heavy traffic?
- which implementation is best for stream-heavy traffic?
- what is the tunnel-container resource cost under each workload class?

## Comparison matrix covered

Implementations compared:

- `cfwarp-cli` alpine image
- `cfwarp-cli` debian/distroless image
- original `MicroWARP`

Workload families covered:

- **metadata**
- **llm_sync**
- **upload**
- **stream**

Raw tunnel checks covered:

- RTT
- bidirectional `iperf3`
- large-file HTTP transfer

## Resource measurement scope

All CPU and memory numbers in the benchmark package refer to the **benchmarked tunnel container only**.

They do not include helper containers such as:

- `k6`
- `curl`
- `iperf3`
- remote benchmark target services

This is important when comparing lightweight and heavyweight tunnel frontends.

## How to present this package internally

Recommended presentation order:

1. Start with the problem framing in `README.md`
2. Show the measurement methodology in `docs/benchmark-mechanism.md`
3. Share the latest markdown report and its executive summary
4. Use `docs/benchmark-report-case.md` to discuss workload-specific interpretation
5. Keep final product decisions separate from benchmark facts unless the audience explicitly wants recommendations

## What is intentionally deferred

To keep the benchmark/reporting pipeline simple and maintainable, the following are intentionally deferred for now:

- chart generation
- heavy plotting dependencies in the Python analyzer
- polished decision memo language
- broader backend families beyond the current comparison set

The benchmark package is currently optimized for:

- reproducibility
- readable markdown
- machine-readable summaries
- operationally simple reruns
