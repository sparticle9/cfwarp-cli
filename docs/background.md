# Background Notes

## Purpose

This document captures the initial research gathered before implementation planning for `cfwarp-cli`.

The goal is to build our own Cloudflare-WARP-based CLI/tooling with a clear understanding of:

- what existing projects really do
- which parts are reusable ideas vs. marketing
- which components are likely worth owning ourselves
- which tradeoffs matter for production use

---

## 1. What MicroWARP actually is

Repository inspected:

- `https://github.com/ccbkkb/MicroWARP`

### Findings

MicroWARP is **not** a self-contained pure-C Cloudflare WARP implementation.

Its own repository is primarily:

- `Dockerfile`
- `entrypoint.sh`
- `docker-compose.yml`
- CI workflow files
- README

The repo itself contains no local C source tree implementing WARP.

### What it really assembles

MicroWARP is essentially a thin wrapper around three things:

1. **Linux kernel WireGuard**
   - brought up through `wg-quick up wg0`
2. **`wgcf`**
   - downloaded at runtime to register a device and generate a WireGuard profile
   - `wgcf` is an unofficial project written in **Go**
3. **`microsocks`**
   - built during Docker image build from the external repo `rofl0r/microsocks`
   - `microsocks` is written in **C**

### Practical conclusion

The useful part of MicroWARP is not “pure C”.
The useful part is its architecture:

- bootstrap WARP credentials/profile
- bring up WireGuard in the kernel
- expose a tiny SOCKS endpoint
- optionally override the peer endpoint
- optionally use the container as a sidecar/egress gateway

---

## 2. Is `wgcf` essential?

### Short answer

**Not fundamentally, but highly useful for bootstrap.**

### Why it matters

`wgcf` is currently the easiest public/off-the-shelf tool for:

- registering a WARP device
- generating a WireGuard profile
- working in a Linux/server environment without depending on Cloudflare's full desktop-oriented client stack

### Why it is not strictly essential

A future implementation could avoid `wgcf` if it can do one of these instead:

1. import an already-generated WireGuard profile
2. implement our own registration/config-generation client
3. interoperate with the official client where appropriate

### Working assumption for planning

For an MVP, `wgcf` is a reasonable bootstrap dependency.
Longer term, we should treat it as **replaceable**, not foundational.

---

## 3. What is good and bad about official `warp-cli` / `warp-svc`

### What the official client provides

Cloudflare docs describe the Cloudflare One Client as a richer system than a bare tunnel.
It includes:

- a background daemon/service (`warp-svc`)
- CLI management (`warp-cli`)
- secure tunnel transport using **WireGuard or MASQUE**
- DNS proxying / DNS handling
- split-tunnel and policy integration
- Cloudflare Zero Trust / team features
- broader client/device orchestration

This means the official client is not just “a WireGuard profile generator”.
It is a full client platform.

### Why people avoid it in simple server-side proxy scenarios

For a narrow use case like “provide WARP-backed outbound connectivity for apps/containers”, the official client may be heavier than necessary.

Observed during local validation:

- `caomingjun/warp:latest` image size pulled locally: **678MB**
- runtime memory sample after startup: about **148 MiB**
- running processes included `warp-svc`, `dbus-daemon`, `gost`, and shell glue

By contrast, a MicroWARP-style container built locally showed:

- image size: about **42MB** in our local build
- runtime memory sample: about **7 MiB** after startup in our environment
- only `microsocks` remained as PID 1 after setup

### Important nuance

This does **not** prove official `warp-cli` is bad.
It proves it is solving a broader problem with a larger runtime footprint.

### Current evidence against official client simplicity

There are community reports about:

- memory leaks in `warp-svc`
- high CPU in some versions/environments
- DNS-manager friction in containerized Linux setups
- extra complexity around DBus and service orchestration

These reports do not automatically disqualify `warp-cli`, but they do make it less appealing as the narrow data-plane for a lightweight server-side egress tool.

### Working conclusion

- If we want **maximum features and Cloudflare-suite compatibility**, official client integration is important to understand.
- If we want **minimal Linux/server-side egress plumbing**, a lighter architecture may be better.

---

## 4. Is `microsocks` reliable enough for production?

### What it is good at

`microsocks` is attractive because it is:

- tiny
- easy to compile
- operationally simple
- suitable for SOCKS5 exposure in front of an already-working tunnel

### What it is not

It is not a full proxy platform with a rich operational feature set.
Compared with more featureful proxies, it is more minimalist in areas such as:

- observability
- management APIs
- advanced auth/policy options
- traffic shaping / balancing
- richer protocol support

### Working conclusion

`microsocks` is likely acceptable as a **small explicit-proxy frontend** for narrow use cases.
But we should not assume it is the final answer for long-term production requirements without testing our own workloads.

For this project, it is better to separate concerns:

- **WARP control/data plane**
- **proxy exposure layer**

That way we can swap the proxy frontend later if needed.

---

## 5. `优选 IP` / endpoint override is a real and useful idea

MicroWARP supports an `ENDPOINT_IP` override that rewrites the WireGuard peer endpoint.

### What this means in practice

This is not a magical acceleration feature.
It is a practical way to:

- pin the tunnel bootstrap to a chosen Cloudflare endpoint
- try alternate ports such as `4500` instead of default UDP `2408`
- work around datacenter/provider-specific filtering, QoS, or poor paths

### Why it matters

Some VPS networks appear to treat default WARP/WireGuard paths poorly.
Allowing explicit endpoint selection can improve:

- handshake success rate
- stability
- latency/throughput from a given region/provider

### Design implication

`cfwarp-cli` should likely make endpoint control a first-class feature, including:

- manual endpoint override
- candidate testing
- health measurement
- future automatic selection/reselection

---

## 6. Sidecar / egress takeover mode is one of the best ideas to reuse

One of the most useful architectural patterns from MicroWARP-style projects is using the WARP container/service as the **network owner** for another application.

### Pattern

Instead of forcing each app to speak SOCKS/HTTP proxy explicitly, an app/container can:

- share the WARP container's network namespace, or
- otherwise route all outbound traffic through the WARP-managed interface

### Why this is valuable

This enables WARP-backed outbound traffic for:

- binaries without proxy support
- legacy applications
- containerized services where transparent egress control is preferable

### Design implication

This project should likely support two usage models:

1. **Explicit proxy mode**
   - SOCKS5 and/or HTTP proxy for proxy-aware clients
2. **Transparent/sidecar mode**
   - full-egress routing via shared namespace / route control

---

## 7. Preliminary architecture direction for `cfwarp-cli`

### Control plane options

Potential strategies:

1. **Bootstrap via `wgcf`**
   - simplest MVP path
2. **Import an existing profile**
   - useful for operators who already have credentials
3. **Implement our own registration flow**
   - longer-term ownership path
4. **Interop with official Cloudflare client**
   - possibly needed for paid/team/advanced-suite features

### Data plane options

Potential strategies:

1. **Kernel WireGuard (`wg`, `wg-quick`)**
   - minimal and attractive for Linux servers
2. **Official Cloudflare client (`warp-svc`)**
   - broader feature coverage, heavier runtime
3. **Hybrid**
   - use different backends depending on requested feature set

### Exposure options

Potential strategies:

1. **SOCKS5 frontend**
2. **HTTP proxy frontend**
3. **No frontend; route takeover only**
4. **Pluggable exposure layer**

---

## 8. Questions to answer in planning

### Product/feature questions

- What exact feature set counts as “leveraging all the goodies” of Cloudflare WARP free/paid?
- Which paid features are realistically accessible outside the official client?
- Which features require Cloudflare One / Zero Trust account integration?
- Do we target only Linux first, or design for cross-platform from day one?

### Technical questions

- Which parts should be owned natively vs. delegated to external binaries?
- Is the MVP centered on explicit proxy mode, transparent mode, or both?
- Should the project be a pure CLI, or CLI + daemon?
- What metrics should drive endpoint selection (`优选 IP`)?
- Do we want dynamic failover, or only static override first?
- Should proxy frontends be embedded or external/pluggable?

### Reliability questions

- How stable is `wgcf` registration over time?
- Can the official client be reused only for advanced/paid features while keeping a lightweight fast path for common cases?
- Which proxy frontend best balances minimalism and observability?

---

## 9. Recommended planning stance

### Near-term

Treat the project as a clean-sheet design with lessons learned from existing tools:

- reuse ideas, not marketing
- keep bootstrap/data-plane/exposure concerns separate
- plan for multiple backends where necessary
- make endpoint selection and sidecar usage first-class concepts

### Likely MVP sequence

1. **Linux-only MVP**
2. support importing or generating a WARP WireGuard profile
3. bring up/down tunnel cleanly
4. expose SOCKS5 or HTTP proxy
5. support endpoint override / candidate testing
6. add sidecar/transparent egress mode
7. evaluate official-client interop for advanced Cloudflare suite features

---

## 10. Summary

The strongest reusable conclusions so far are:

- MicroWARP's repo is wrapper/glue, not a pure-C implementation
- `wgcf` is useful for bootstrap but should remain replaceable
- official `warp-cli` / `warp-svc` is broader and heavier, not necessarily wrong
- endpoint override (`优选 IP`) is a valuable operational feature
- sidecar/full-egress mode is strategically important
- proxy frontend choice should be decoupled from WARP control/data plane

These notes are intended to support the next step: turning this research into concrete requirements and an implementation plan.
