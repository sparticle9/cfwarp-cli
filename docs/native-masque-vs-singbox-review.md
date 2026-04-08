# Native MASQUE vs `singbox-wireguard`: review assertions and open concerns

This document intentionally records **unresolved architectural skepticism** rather than a final conclusion.
It exists so we can review the direction later without losing the current concerns.

## Current assertion

The project currently treats these as distinct runtime approaches with a shared control plane:

- **Legacy transitional backend:** `singbox-wireguard`
- **Target native runtime path:** `native + masque`

The working assertion is:

> The native MASQUE path overlaps with `singbox-wireguard` mainly at the product surface
> (local proxy + WARP-backed egress), but not at the internal runtime architecture.

This assertion is useful for implementation progress, but it is **not yet accepted as settled architecture**.

## Why this remains under review

There is still reasonable skepticism about whether the new native layering is actually cleaner than continuing to lean more heavily on sing-box.

The core concerns are:

1. **Architectural cleanliness**
   - Is the native layering truly simpler and more coherent over time?
   - Or are we replacing a mature integrated engine with several homegrown layers that are individually understandable but collectively harder to reason about?

2. **Future leverage of sing-box capabilities**
   - sing-box already provides mature proxying, routing, DNS, protocol handling, observability, and operational behavior.
   - If future requirements need more of those capabilities, does a native path create a second platform we have to keep growing in parallel?

3. **Boundary correctness**
   - The intended unification boundary is the packet tunnel edge.
   - We should keep questioning whether this is the cleanest seam in practice, especially once richer runtime features appear.

4. **Operational duplication risk**
   - Even if transport implementations differ, both runtimes still expose similar user-facing behaviors.
   - The risk is not only code duplication, but duplicated operational semantics, debugging surfaces, and long-term maintenance burdens.

## Current technical picture

### `singbox-wireguard`

This path uses sing-box as the main runtime engine.

High-level flow:

```text
app -> sing-box local proxy frontend -> sing-box routing/dns/outbound -> WireGuard WARP -> Cloudflare
```

What our code mainly does:

- persist account/settings/state
- render sing-box config
- start/stop the sing-box process
- report runtime status

What sing-box owns:

- local proxy serving
- packet handling / routing internals
- DNS behavior
- WireGuard transport runtime
- most dataplane behavior

### `native + masque`

This path keeps the control plane in our CLI, but moves runtime behavior into our own process.

High-level flow:

```text
app -> local SOCKS/HTTP frontend -> shared userspace netstack -> packet engine -> MASQUE transport -> Cloudflare
```

What our code owns here:

- local HTTP/SOCKS frontends
- shared userspace dataplane engine
- userspace netstack integration
- MASQUE packet transport
- reconnect/error surfacing
- runtime orchestration

Important nuance:

- The native path currently uses `wireguard/tun/netstack` as a **local userspace IP stack implementation**.
- That does **not** mean the native path uses WireGuard as the network transport to Cloudflare.
- The transport to Cloudflare is MASQUE / CONNECT-IP over HTTP/3.

## Working claim that needs later review

The current implementation direction assumes:

- **shared dataplane abstractions** are worth owning directly,
- while **transport protocols** can vary underneath,
- and local **frontends** (SOCKS/HTTP/TUN) can remain transport-agnostic.

In short:

```text
frontend(s) -> shared dataplane -> transport plugin
```

This is the design that enabled native MASQUE without copying `usque`'s proxy shape and without hard-coupling the new architecture to sing-box.

## Skeptical counter-assertion

A serious alternative interpretation is:

> The native MASQUE runtime may be architecturally elegant in isolation, but still not justify itself if future product requirements would be better served by extending or reusing sing-box more deeply.

That concern remains valid.

## Review questions for later

When revisiting this decision, we should explicitly answer:

1. **Feature leverage:**
   - What future features would be nearly free with sing-box but costly in the native runtime?
   - Examples: advanced routing, policy controls, richer protocol support, more mature observability, TUN behavior, DNS control.

2. **Complexity accounting:**
   - Is the native path actually reducing complexity, or merely relocating it from a dependency into our own codebase?

3. **Maintenance cost:**
   - How much code do we need to keep adding before the native runtime becomes its own mini-networking platform?

4. **Operational reliability:**
   - Which path is easier to operate, debug, package, and recover under failure?
   - Early intermittent native MASQUE startup behavior is relevant here.

5. **Abstraction durability:**
   - Does the packet-boundary unification still look like the right seam after more transports, more modes, and more operator requirements arrive?

6. **Exit strategy:**
   - If the native path stops looking like the best long-term core, can it remain a transport experiment without forcing the whole product architecture around it?

## Provisional position

For now, the project can continue under this provisional stance:

- keep `singbox-wireguard` as the stable transitional runtime,
- keep native MASQUE as the pathfinder for the unified transport/dataplane architecture,
- do **not** yet declare the native architecture conclusively superior,
- and revisit the tradeoff once more runtime features and operational experience accumulate.

## Decision status

**Status:** explicitly unresolved, to be reviewed later.
