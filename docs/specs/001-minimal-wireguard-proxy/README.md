# Specification: 001-minimal-wireguard-proxy

## Status

| Field | Value |
|-------|-------|
| **Created** | 2026-04-06 |
| **Current Phase** | Spec drafted |
| **Last Updated** | 2026-04-06 |

## Documents

| Document | Status | Notes |
|----------|--------|-------|
| `requirements.md` | completed | Linux-first, Docker-first explicit proxy MVP |
| `design.md` | completed | Uses direct Cloudflare registration + `sing-box` WireGuard outbound |
| `tasks.md` | completed | Incremental coding plan for initial implementation |

## Decisions Log

| Date | Decision | Rationale |
|------|----------|-----------|
| 2026-04-06 | Start with explicit proxy mode, not transparent routing | Lowest implementation risk and fastest validation path |
| 2026-04-06 | Use WireGuard-based userspace proxy backend first | Minimal Docker friction; avoids `NET_ADMIN`/kernel routing for MVP |
| 2026-04-06 | Keep MASQUE as a future backend seam, not an MVP target | Important for future parity, but unnecessary for first shipping version |
| 2026-04-06 | Make Docker deployment a first-class target | Fastest path for repeatable usage and distribution |

## Context

This spec narrows the initial scope of `cfwarp-cli` to a minimalist, Docker-friendly, WireGuard-based explicit proxy. The MVP is intended to own the registration/config UX and avoid dependence on `wgcf` or the official `warp-cli`, while still leaving room for future backends such as kernel WireGuard and MASQUE.

## Start Here

1. Read `docs/background.md`
2. Read `requirements.md`
3. Read `design.md` section **Minimal-effort implementation references**
4. Execute `tasks.md` from top to bottom

---
*This file tracks the current specification state for the first MVP.*
