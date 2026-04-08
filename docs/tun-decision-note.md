# TUN decision note: sing-box first vs native MASQUE later

## Status

Provisional decision, recorded for implementation sequencing.

## Question

How should `cfwarp-cli` approach TUN mode and Docker-stack gateway behavior?

Two candidate directions exist:

1. **Use the existing sing-box powered WireGuard/WARP path first**
2. **Build native MASQUE TUN mode directly**

## Decision

For the **first practical TUN milestone**, prefer:

> **sing-box powered WARP first**

For the **long-term architecture target**, keep open:

> **native packet-boundary TUN later, after native MASQUE startup is more reliable**

## Why this is the current decision

### 1. sing-box is the easier first TUN milestone

The sing-box path is currently the fastest route to a working TUN-based gateway because it already brings more of the difficult system behavior:

- TUN device handling
- routing behavior
- DNS behavior
- interface lifecycle
- operational edge-case handling

In contrast, native MASQUE TUN would still require new implementation work in the repo for:

- TUN frontend integration
- route setup / endpoint bypass handling
- container-gateway ergonomics
- additional recovery and operational hardening

### 2. Native MASQUE is not stable enough yet for TUN-first

Current live testing has shown intermittent native MASQUE startup failure during CONNECT-IP establishment, with errors such as:

```text
http3: parsing frame failed: PROTOCOL_VIOLATION (remote)
```

That makes proxy mode tolerable with retries, but it is not a good foundation yet for a namespace-wide or stack-wide TUN gateway.

### 3. The architecture still points toward shared frontends later

The unified transport/data-plane design still makes sense as a long-term direction:

```text
frontend(s) -> shared dataplane -> transport plugin
```

A native TUN frontend would fit that architecture well. But the sequencing matters:

- stabilize native MASQUE first
- prove TUN/gateway behavior with the easier path first
- revisit whether native MASQUE TUN is worth the complexity once the transport is operationally trustworthy

## Practical interpretation

### Near-term goal: get TUN-based gateway behavior working

Use the sing-box powered WireGuard/WARP path first.

This is the best route if the immediate outcome needed is:

- TUN-based outbound path
- sidecar or shared-network-namespace usage
- earlier validation of container gateway patterns

### Later goal: preserve architectural cleanliness

Once native MASQUE startup/reliability improves, revisit a native TUN frontend.

That later work should be evaluated against:

- whether the packet-boundary abstraction still looks like the right seam
- whether sing-box remains the better practical runtime for richer gateway features
- whether native MASQUE provides enough value to justify the extra ownership surface

## Recommended sequence

1. **Fix native MASQUE startup flakiness**
2. **Prototype/validate TUN using sing-box powered WARP first**
3. **Validate Docker gateway patterns using that easier path**
4. **Only then decide whether to implement native MASQUE TUN**

## What this note is not saying

This note does **not** conclude that native MASQUE TUN is a bad idea.

It only concludes that:

- it is **not** the easiest first TUN milestone
- sing-box powered WARP is the more practical first step
- native MASQUE TUN should be reconsidered after transport reliability improves

## Decision summary

- **First TUN milestone:** sing-box powered WARP
- **Long-term option under review:** native MASQUE TUN
- **Current blocker for native-first TUN:** startup flakiness and higher implementation/operational cost
