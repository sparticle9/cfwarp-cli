# Native MASQUE status — 2026-04

## Status

Native MASQUE support reached solid public checkpoint.

Project now has:

- native MASQUE control-plane support
- native MASQUE runtime path
- runtime diagnostics and transport timing
- tuning knobs for protocol experiments
- quick and real-target benchmark coverage
- green `go test ./...`

This checkpoint is good enough to publish, discuss, and invite contributors.

This checkpoint is **not** final proof that native MASQUE work is complete.

---

## What exists today

### Runtime and protocol support

Implemented:

- native MASQUE runtime family
- MASQUE transport selection and persisted state support
- HTTP/SOCKS local proxy runtime path for native MASQUE
- startup and retry diagnostics
- transport timing events and runtime debug capture
- MASQUE environment tuning knobs

Key files and areas:

- `internal/cloudflare/client.go`
- `internal/state/models.go`
- `internal/transport/masque/`
- `internal/backend/native/`
- `ansible/protocol-quick-bench.yml`
- `ansible/protocol-real-bench.yml`
- `docs/masque-performance-plan.md`
- `docs/masque-real-target-matrix-20260408.md`

### Validation and comparison

Current branch state at time of memo:

- clean working tree
- tests passing
- real benchmark evidence captured
- review notes written down

Project also has comparison coverage against:

- stable WireGuard path
- upstream `usque`

---

## What evidence says now

Current evidence supports these claims:

1. Native MASQUE moved beyond prototype stage.
2. Native MASQUE is real enough for continued development and outside review.
3. Native MASQUE can produce credible results against upstream `usque` on some paths.
4. Performance and behavior still vary by workload and endpoint family.
5. Startup retry behavior remains unresolved.
6. Architecture review remains open.

This means native MASQUE support is **real**, **usable for continued work**, and **worth public collaboration**.

This does **not** mean native MASQUE is finished or default-ready for every use case.

---

## Current conclusions

### Safe conclusions

Safe to say:

- project supports native MASQUE in meaningful way
- project can benchmark native MASQUE against practical alternatives
- current docs and tooling are enough for contributors to reproduce and extend work
- current codebase is strong enough for public review and targeted help

### Unsafe conclusions

Not safe to say:

- native MASQUE support is feature-complete
- native MASQUE should replace stable WireGuard path by default
- startup path is clean
- one tuning profile should become universal default
- architecture question is settled

---

## Known open problems

### 1. Startup retry tax

Observed in current real-target matrix:

- successful MASQUE runs still show `retry_count=1`

Need:

- isolate first-attempt failure cause
- determine family/path/handshake interaction
- separate startup cost from steady-state dataplane tuning

### 2. IPv4 vs IPv6 behavior

Current evidence suggests:

- IPv4 may help some `iperf3` results
- IPv6 may help bulk HTTP on some paths

Need:

- broader repeated runs
- longer test windows
- decision on default family strategy vs adaptive strategy

### 3. Workload-sensitive tuning

Current evidence suggests:

- one profile may improve one traffic shape while hurting another

Need:

- packet size sweeps
- longer real-target runs
- stronger noise control
- workload-specific interpretation

### 4. Dataplane vs transport attribution

Still not fully settled:

- how much cost belongs to transport core
- how much cost belongs to userspace dataplane / netstack
- how much cost belongs to frontend proxy exposure

Need:

- deeper profiling on download-heavy and request-heavy paths

### 5. Architecture review still open

Open question remains:

- keep pushing native runtime as long-term core
- or keep native MASQUE mainly as transport pathfinder while stable runtime stays elsewhere

See:

- `docs/native-masque-vs-singbox-review.md`

### 6. Packaging and public usage polish

Still thin:

- broader packaging story beyond current branch/build flow
- docs for public try-out path
- contributor-ready issue map for MASQUE work

---

## Contributor entry points

Good areas for contribution:

### Protocol and runtime

- startup retry root cause
- endpoint family selection strategy
- packet size / QUIC tuning experiments
- userspace dataplane profiling
- HTTP vs SOCKS exposure cost comparison

### Benchmarking and reporting

- more repeatable public benchmark recipes
- result visualization
- benchmark fixture cleanup
- noise reduction and confidence reporting

### Runtime and ops

- packaging and reproducibility
- operator guidance for Linux UDP buffer tuning
- safer runtime debug surfaces
- docs for public evaluation flow

---

## Recommended next steps

### Short term

1. Publish this status memo.
2. Keep README status block current.
3. Use `CONTRIBUTING.md` as contributor landing page.
4. Open focused issues for:
   - startup retry root cause
   - family selection strategy
   - packet size sweep
   - dataplane profiling
   - packaging/docs improvements

### Medium term

1. Run longer real-target matrix.
2. Separate startup-path fixes from steady-state tuning claims.
3. Decide whether next optimization phase targets:
   - transport core
   - dataplane efficiency
   - exposure-layer behavior
4. Revisit whether native MASQUE stays experimental or becomes stronger default candidate.

---

## Bottom line

Native MASQUE support reached strong public checkpoint.

Enough code. Enough eval. Enough docs to make repo public and ask for targeted help.

Support is real. Work is not done.
