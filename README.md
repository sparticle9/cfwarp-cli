# cfwarp-cli

A CLI-first toolkit for building on top of Cloudflare WARP capabilities — free or paid — with a focus on practical Linux/server use cases.

## Goals

- Bootstrap and manage Cloudflare WARP connectivity
- Support both explicit proxy mode and full-egress/sidecar mode over time
- Make endpoint selection / `优选 IP` experimentation easy
- Keep the architecture transparent and composable
- Avoid overclaiming what is implemented vs. what is wrapped

## Current implementation direction

The first spec is intentionally narrow:

- **Docker-first**
- **Linux-first**
- **explicit proxy mode first**
- **minimal WireGuard-based backend first**
- **future MASQUE path reserved, not yet implemented**

## Documents

- `docs/background.md` — research notes and technical background gathered so far
- `docs/specs/001-minimal-wireguard-proxy/requirements.md` — MVP requirements
- `docs/specs/001-minimal-wireguard-proxy/design.md` — MVP design and Docker deployment
- `docs/specs/001-minimal-wireguard-proxy/tasks.md` — incremental implementation plan

## Status

Spec drafted for the first MVP: a minimalist Docker-friendly WireGuard-based explicit proxy.
