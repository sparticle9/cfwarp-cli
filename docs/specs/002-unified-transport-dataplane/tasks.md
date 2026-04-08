# Implementation Plan — Unified Transport and Shared Data Plane

- [x] 1. Refactor the persisted state and settings model for transport-aware runtime selection.
  - Replace the current WireGuard-shaped account model with a schema-versioned structure that can store both WireGuard and MASQUE material without overloading field meanings.
  - Split operator settings into runtime family, transport, and mode, while keeping listen/auth/log settings shared where possible.
  - Add migration helpers and tests covering current `001` state files, invalid runtime combinations, and JSON round-trips for nested MASQUE options.
  - Requirements/design refs: transport-aware account state, desired settings split, migration summary steps 1–2.

- [x] 2. Introduce a backend/runtime registry and remove hardcoded `sing-box` assumptions from the CLI control path.
  - Replace direct `singbox.*` calls in `cmd/up.go`, `cmd/down.go`, and `cmd/status.go` with a runtime/backend lookup flow.
  - Keep `singbox-wireguard` working as the first registered implementation while making room for a native runtime family.
  - Add tests for registry lookup, unsupported runtime selection, and backwards-compatible behavior of existing commands.
  - Requirements/design refs: control-plane service model, legacy backend coexistence, migration summary step 1.

- [x] 3. Add a richer runtime-state model and service-oriented orchestration seam.
  - Extend runtime state beyond PID tracking to capture desired transport/mode, lifecycle phase, listener info, selected endpoint, reconnect metadata, and last transport error.
  - Introduce an internal runtime orchestrator package and a hidden/internal service command or equivalent entrypoint for native runtimes.
  - Add tests for status transitions (`idle`, `connecting`, `connected`, `degraded`, `stopped`) and stale runtime cleanup across external and native runtimes.
  - Requirements/design refs: runtime state, control-plane service model, error handling.

- [x] 4. Define the packet transport abstraction and test doubles for native transports.
  - Create the `PacketTransport` / `PacketTunnel` interfaces and any shared event/stats types needed by the orchestrator and data plane.
  - Add fake transports/tunnels for deterministic unit tests without real network dependency.
  - Add tests that verify the abstraction supports MTU/address reporting, packet read/write, close behavior, and event delivery.
  - Requirements/design refs: packet transport abstraction, transport layer boundaries.

- [x] 5. Build the shared native data-plane engine on top of the packet transport abstraction.
  - Create a reusable engine package that owns userspace stack setup, packet loop wiring, DNS path ownership, and shared counters/errors.
  - Add pooled-buffer utilities and packet-loop tests to validate start/stop behavior, error propagation, and reduced allocation churn.
  - Keep the engine transport-agnostic so both MASQUE now and native WireGuard later can plug into it.
  - Requirements/design refs: shared data-plane engine, performance decisions, testing strategy for data-plane tests.

- [x] 6. Implement native SOCKS5 and HTTP proxy frontends against the shared engine.
  - Add frontend packages that expose the engine as SOCKS5 and HTTP/CONNECT listeners without embedding transport-specific setup logic.
  - Support shared auth, listen address, and health reporting behavior driven from the common settings model.
  - Add automated tests for SOCKS TCP/UDP behavior, HTTP CONNECT flows, plain HTTP proxying, auth/no-auth variants, and listener failure paths using fake packet transports.
  - Requirements/design refs: frontends/service modes, data-plane tests, official `warp-cli`-inspired mode separation.

- [x] 7. Port the Cloudflare MASQUE registration and enrollment control-plane flow.
  - Add Cloudflare API code that performs consumer registration, MASQUE key enrollment, and persistence into the new MASQUE-specific account state fields.
  - Parse endpoint and key material robustly instead of reusing WireGuard-specific field assumptions.
  - Add tests for success, malformed responses, token errors, overwrite/import behavior, and state persistence.
  - Requirements/design refs: native MASQUE transport responsibilities, `usque` reference inputs, error handling.

- [x] 8. Implement the native MASQUE transport package.
  - Port the MASQUE-specific TLS client auth, endpoint public-key pinning, QUIC/H3 session setup, Cloudflare-specific CONNECT-IP quirks, reconnect logic, and packet tunnel implementation into `internal/transport/masque`.
  - Reuse shared packet/event/stats types from the transport abstraction rather than embedding CLI or proxy concerns.
  - Add tests for TLS setup, endpoint pin mismatch, reconnect loop behavior, transport close semantics, and packet read/write behavior with deterministic fakes where possible.
  - Requirements/design refs: native MASQUE transport, transport tests, migration summary step 4.

- [x] 9. Integrate the native runtime family into the control plane and add MASQUE-first mode selection.
  - Wire the orchestrator so `runtime_family=native` + `transport=masque` + `mode=socks5|http` starts the shared engine and frontend listeners through the local runtime service.
  - Add validation and capability checks so unsupported combinations fail clearly before runtime startup.
  - Add command and integration tests for `connect`, `disconnect`, `status --json`, and compatibility wrappers from `up/down` when native MASQUE is selected.
  - Requirements/design refs: control-plane runtime model, error handling, migration summary steps 4–5.

- [x] 10. Redesign the CLI surface toward transport/mode-oriented control while preserving compatibility.
  - Add new command groups for `registration`, `transport`, and `mode`, plus JSON-friendly `status`/`stats` paths aligned with agent usage.
  - Keep the current command set working during transition, but route shared logic through the new control-plane services and state model.
  - Add command tests covering help text, JSON output, idempotent updates, and backwards-compatible aliases or wrappers.
  - Requirements/design refs: control-plane runtime model, official `warp-cli` concepts, testing strategy.

- [x] 11. Keep `singbox-wireguard` working as a transitional runtime under the new architecture.
  - Adapt the existing `singbox-wireguard` path to the new registry, settings, and runtime-state model without forcing it into native data-plane code.
  - Ensure legacy runtime reporting still surfaces through the richer status/stats model where meaningful.
  - Add regression tests so the current WireGuard MVP behavior remains stable while native MASQUE work lands incrementally.
  - Requirements/design refs: legacy backend coexistence, migration summary step 6.

- [ ] 12. Update Alpine-first packaging, build, and integration coverage for the new runtime layout.
  - Consolidate packaging around Alpine as the only target in this phase, including the native runtime path and any retained legacy backend dependencies.
  - Update Dockerfile and compose examples so Alpine remains the canonical deployment target while native MASQUE support is introduced.
  - Add opt-in integration tests for Alpine image build, native MASQUE startup smoke coverage, and preserved `singbox-wireguard` behavior.
  - Requirements/design refs: packaging and build target, integration tests, migration summary step 7.
