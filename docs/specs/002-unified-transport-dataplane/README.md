# Specification: 002-unified-transport-dataplane

## Status

| Field | Value |
|-------|-------|
| **Created** | 2026-04-07 |
| **Current Phase** | Plan drafted |
| **Last Updated** | 2026-04-07 |

## Documents

| Document | Status | Notes |
|----------|--------|-------|
| `requirements.md` | skipped | Captured directly in discussion and constrained by existing MVP scope |
| `design.md` | completed | Native unified transport/data-plane direction with MASQUE-first transport work |
| `tasks.md` | completed | Incremental implementation plan targeting Alpine packaging first |

## Decisions Log

| Date | Decision | Rationale |
|------|----------|-----------|
| 2026-04-07 | Unify transports at the packet tunnel boundary | Keeps WireGuard and MASQUE under one high-control architecture without proxy-specific duplication |
| 2026-04-07 | Build one shared userspace data-plane engine for SOCKS5/HTTP/TUN | Preserves architectural clarity and avoids copying `usque`'s proxy structure |
| 2026-04-07 | Keep `singbox-wireguard` as a transitional backend while native runtime grows | Minimizes churn and preserves the current working WireGuard path |
| 2026-04-07 | Follow official `warp-cli` UX concepts more closely | Better fit for agent-driven control plane and explicit desired/current/runtime state management |
| 2026-04-07 | Target Alpine as the only packaging target in this phase | Reduces packaging surface area while the native runtime architecture is still settling |

## Context

This spec defines the next architecture step after the initial WireGuard MVP. The goal is to evolve `cfwarp-cli` from a backend-specific process runner into a control-plane-oriented CLI with a native runtime that can support multiple transports behind a shared packet-tunnel abstraction and a shared userspace data plane. MASQUE is the first native transport target, while the current `singbox-wireguard` path remains supported during migration. Alpine is the only build/package target in this phase.

## Start Here

1. Read `design.md` for the architecture split between control plane, transport, and data-plane frontends.
2. Read `tasks.md` for the incremental migration sequence.
3. Keep `docs/specs/001-minimal-wireguard-proxy/` as the baseline for currently shipped behavior.

---
*This file tracks the specification state for the unified native transport/data-plane plan.*
