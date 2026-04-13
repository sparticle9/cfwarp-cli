# MASQUE performance tuning plan

## Branch / worktree

- Branch: `perf/masque-tuning`
- Base: latest `main`
- Focus: **performance first, then reliability**

This plan is intentionally layered from the **transport core outward** so we do not accidentally optimize only the user-facing proxy path while leaving deeper MASQUE inefficiencies unresolved.

---

## Goals

Primary goal:

> Improve the steady-state performance of the native MASQUE path while preserving the architectural direction of the unified transport/data-plane design.

Secondary goal:

> Improve startup and operational reliability only when it does not materially penalize hot-path performance.

---

## Non-goals for this phase

- Native TUN implementation
- Broad feature expansion unrelated to MASQUE performance
- Replacing the current architecture before transport-core evidence is collected
- Prematurely optimizing only SOCKS / HTTP exposure before transport-core evidence exists

---

## Guiding principles

1. **Transport-first diagnosis**
   - The first question is whether the MASQUE / CONNECT-IP transport core is underperforming.
2. **Measure before changing**
   - Every optimization should be backed by a benchmark or instrumentation delta.
3. **Hot-path discipline**
   - Avoid adding overhead to the established packet-forwarding path unless measurement justifies it.
4. **Separate layers clearly**
   - Transport-core issues should be distinguished from netstack/engine issues and from SOCKS/HTTP exposure issues.
5. **Compare against the actual practical baseline**
   - Today the practical baseline is the existing WireGuard/WARP path.

---

## Working hypotheses

Current likely contributors to weaker MASQUE performance include some combination of:

1. QUIC / HTTP3 / CONNECT-IP transport overhead
2. Cloudflare MASQUE endpoint family/path selection issues
3. UDP socket buffer sizing limits on Linux hosts
4. packet copy / batching inefficiency in the shared engine
5. userspace netstack overhead vs the more mature sing-box WireGuard path
6. destination resolution / Happy Eyeballs deficiencies in the SOCKS/HTTP exposure layer

This plan investigates them in that order.

---

# Phase 0 — Benchmark and evidence discipline

## Objective

Ensure all later changes are compared against a stable, repeatable baseline.

## Tasks

1. Standardize three recurring benchmark shapes:
   - **transport/practical quick bench**
     - existing `ansible/protocol-quick-bench.yml`
     - fast local-proxy comparison, now with MASQUE tuning knobs
   - **protocol real-target bench**
     - existing `ansible/protocol-real-bench.yml`
     - compares `wireguard`, native `masque`, and vanilla `usque`
     - runs remote `iperf3` and remote HTTP transfer against a separate host
     - supports MASQUE tuning knobs and optional UDP buffer sysctl experiments
   - **runtime smoke validation**
     - existing `ansible/package-runtime-smoke.yml`
2. Fix benchmark result storage conventions:
   - record image revision
   - record host name / run timestamp
   - store wireguard vs masque comparison in a stable output file
3. Define a minimum recurring comparison set:
   - small-request latency
   - fixed-size HTTP download
   - remote `iperf3` throughput
   - startup success / retry count
4. Document exactly what each bench means:
   - quick bench = practical proxy-path comparison
   - real-target bench = practical proxy-path comparison with a separate remote target host and upstream implementation references

## Deliverables

- benchmark README section or note
- stable comparison artifacts from repeatable runs

## Exit criteria

- one command can reproduce a quick wireguard vs masque comparison
- results are archived per run with image revision metadata

---

# Phase 1 — Transport-core instrumentation

## Objective

Find out whether the MASQUE transport core itself is the primary bottleneck.

## Tasks

1. Add timing instrumentation inside `internal/transport/masque/transport.go` for:
   - endpoint resolution
   - UDP socket creation
   - QUIC dial
   - HTTP/3 client setup
   - CONNECT-IP handshake
   - time-to-first-successful-packet
2. Emit structured transport events for:
   - endpoint family selected (v4 vs v6)
   - retry count
   - handshake duration
   - reconnect reason
3. Add low-overhead counters for:
   - reconnects
   - startup attempts
   - packet bursts / activity timestamps
4. Ensure instrumentation can be turned on without changing behavior materially.

## Deliverables

- transport-core timing/events in logs or machine-readable state
- first evidence about where MASQUE startup and runtime time is spent

## Exit criteria

- for one benchmark run we can answer:
  - which endpoint family was used
  - how long QUIC + CONNECT-IP establishment took
  - whether first-packet readiness lagged after tunnel establishment

---

# Phase 2 — Transport-core experiments and fixes

## Objective

Optimize MASQUE transport behavior before touching user-facing proxy logic.

## Tasks

1. **Endpoint family experiments**
   - benchmark IPv4-only MASQUE endpoint selection
   - benchmark IPv6-only MASQUE endpoint selection
   - measure whether one family is consistently better on target hosts
2. **Handshake / startup tuning**
   - verify whether current retry behavior is masking a family/path selection issue
   - test whether startup is dominated by first-attempt endpoint choice
3. **UDP buffer and socket tuning**
   - quantify effect of larger `rmem_max` / `wmem_max`
   - decide whether to expose tuning guidance or runtime warnings only
4. **QUIC / transport knobs**
   - validate current `InitialPacketSize`, keepalive, reconnect delay choices
   - test whether any transport setting changes improve throughput or latency
5. **CONNECT-IP implementation review**
   - inspect whether current Go stack introduces avoidable setup or framing inefficiency
   - compare against community / upstream known issues

## Deliverables

- benchmark deltas for each transport-level experiment
- shortlist of transport-core changes that actually move the numbers

## Exit criteria

- at least one transport-core improvement or ruled-out hypothesis backed by measurements
- clear conclusion on whether endpoint family selection is materially affecting performance

---

# Phase 3 — Packet engine and netstack efficiency

## Objective

Evaluate the shared data-plane cost after transport-core issues are understood.

## Tasks

1. Measure copy/allocation behavior in:
   - `internal/dataplane/engine/engine.go`
   - netstack packet read/write path
2. Investigate batching opportunities:
   - packet read/write batching at the engine boundary
   - whether transport API shape should grow batch support later
3. Review pool sizing / reuse behavior:
   - MTU-sized buffer pool effectiveness
   - avoidable copies on ICMP / packet echo paths
4. Profile userspace netstack path under download-heavy load:
   - CPU
   - syscall cadence
   - packet throughput characteristics
5. Decide whether the current netstack remains the right tradeoff for proxy mode.

## Deliverables

- evidence on whether engine/netstack overhead is a major contributor
- prioritized list of engine-level optimizations

## Exit criteria

- we know whether transport-core or engine/netstack is the dominant performance limiter

---

# Phase 4 — Exposure-layer improvements (SOCKS / HTTP)

## Objective

Only after lower layers are understood, reduce overhead and latency in the user-facing proxy path.

## Tasks

1. Add better destination address selection:
   - avoid naive first-IP behavior
   - evaluate Happy Eyeballs-like strategy where appropriate
2. Benchmark SOCKS vs HTTP exposure on the same MASQUE transport core
3. Reduce frontend-specific avoidable overhead:
   - resolver path
   - auth path cost
   - dial behavior
   - HTTP CONNECT handling
4. Determine whether practical app-facing performance gaps are mostly transport-core or frontend-induced.

## Deliverables

- frontend comparison results
- exposure-layer improvements backed by measurements

## Exit criteria

- clear attribution between transport-core cost and frontend cost
- practical latency improvements visible in proxy-path benches

---

# Phase 5 — Host and operator tuning

## Objective

Turn benchmark findings into reproducible operational guidance.

## Tasks

1. Document recommended Linux sysctls for QUIC / MASQUE workloads if justified:
   - `net.core.rmem_max`
   - `net.core.wmem_max`
2. Decide whether container runtime should surface warnings or docs only
3. Add performance notes for:
   - host networking environment
   - IPv4 vs IPv6 endpoint choice
   - benchmark interpretation caveats
4. Extend Ansible playbooks to capture relevant environment hints when useful.

## Deliverables

- operator guidance for MASQUE performance-sensitive deployments
- benchmark caveat documentation

## Exit criteria

- repeatable deployment guidance exists for getting the best-known MASQUE performance

---

# Phase 6 — Review and architectural decision checkpoint

## Objective

Use evidence to decide how far to continue investing in the current native MASQUE path.

## Questions to answer

1. Is the MASQUE transport core itself competitive enough?
2. Are the remaining gaps mostly implementation debt or fundamental architectural tradeoffs?
3. Does the shared packet-boundary design still look justified after measurements?
4. Is the next best step:
   - more native MASQUE optimization,
   - better frontend behavior,
   - or renewed investment in the sing-box path for certain modes?

## Deliverables

- short review memo with benchmark-backed conclusions
- recommendation for next implementation phase

---

## Suggested execution order

1. Phase 0 — benchmark discipline
2. Phase 1 — transport-core instrumentation
3. Phase 2 — transport-core experiments and fixes
4. Phase 3 — engine/netstack efficiency
5. Phase 4 — SOCKS/HTTP exposure improvements
6. Phase 5 — operator tuning
7. Phase 6 — review checkpoint

---

## Immediate next tasks

1. Add transport-core timing / event instrumentation to native MASQUE startup and reconnect paths.
2. Add explicit endpoint-family visibility to runtime logs/state.
3. Run repeated v4 vs v6 MASQUE transport comparisons on the remote validation host.
4. Capture whether UDP buffer tuning changes throughput or only removes warnings.
5. Compare HTTP and SOCKS exposure only after those transport-core measurements exist.
