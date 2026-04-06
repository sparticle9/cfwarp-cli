# cfwarp-cli

A CLI-first toolkit for building on top of Cloudflare WARP capabilities — free or paid — with a focus on practical Linux/server use cases.

## Goals

- Bootstrap and manage Cloudflare WARP connectivity
- Support both explicit proxy mode and full-egress/sidecar mode
- Make endpoint selection / `优选 IP` experimentation easy
- Keep the architecture transparent and composable
- Avoid overclaiming what is implemented vs. what is wrapped

## Initial scope

This repository starts as a research and planning workspace.

Near-term focus:

1. document the problem space
2. choose a control-plane strategy for WARP registration/config
3. define MVPs for proxy mode and sidecar mode
4. evaluate implementation language and runtime tradeoffs

## Documents

- `docs/background.md` — research notes and technical background gathered so far

## Status

Planning / discovery.
