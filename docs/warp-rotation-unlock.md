# WARP address rotation and unlock checks

This document covers the minimal address-rotation and unlock-checking workflow built into `cfwarp-cli`.

## Goals

The implementation intentionally stays small:

- use the existing Cloudflare registration flow to obtain a new WARP address set
- expose the assigned IPv4 / IPv6 addresses directly from local state
- validate a candidate address through the already configured local proxy
- drive retries from a small set of service checks instead of a giant shell script

## What is implemented

### 1. Show allocated addresses

```bash
cfwarp-cli address show
cfwarp-cli address show --json
```

This reads the current account state and prints the assigned addresses:

- WireGuard IPv4 / IPv6
- WireGuard peer endpoint
- MASQUE IPv4 / IPv6 when MASQUE state exists

This is the address material that a local proxy backend already uses, and it is also the address material a future TUN-oriented backend would consume.

### 2. Lightweight unlock checks

```bash
cfwarp-cli unlock test --service gemini --service chatgpt
cfwarp-cli unlock test --service claude --json
```

Current checks:

- `gemini`
- `chatgpt` (alias: `openai`)
- `claude`

These checks are intentionally narrow and proxy-oriented:

- no huge shell dependency tree
- no giant all-services media sweep
- only a few HTTP requests through the configured local proxy

Current heuristics are adapted from the same broad ideas used by common shell scripts:

- **Gemini**: look for the known availability marker on `https://gemini.google.com`
- **ChatGPT / OpenAI**: combine the `cookie_requirements` endpoint with `ios.chat.openai.com`
- **Claude**: inspect the final redirected URL from `https://claude.ai/`

## 3. Rotate until checks pass

```bash
cfwarp-cli rotate --attempts 10 --service gemini --service chatgpt
```

Behavior:

1. stop the current backend if it is running
2. re-register the WARP account to obtain a new address set
3. bring the configured backend up again
4. run the requested unlock checks through the local proxy
5. stop and retry if checks fail
6. keep the new account when checks pass

If all attempts fail and a previous account existed, the command restores the previous account state and restarts the backend.

## Important scope note

This phase does **not** add a production TUN backend yet.

What it does give you is:

- direct visibility into the allocated IPv6 address
- deterministic address rotation
- proxy-driven unlock validation suitable for server-side dogfooding

That keeps the integration minimal while still covering the practical operator workflow.

## Suggested operator workflow

### Inspect current assigned addresses

```bash
cfwarp-cli registration show
cfwarp-cli address show
```

### Validate current proxy unlocks

```bash
cfwarp-cli unlock test --service gemini --service chatgpt
```

### Rotate until a target set passes

```bash
cfwarp-cli rotate --attempts 8 --service gemini --service chatgpt
```

### Keep MASQUE state while rotating

```bash
cfwarp-cli rotate --attempts 8 --service gemini --service chatgpt --masque
```

## Design notes

Compared with the large popular shell scripts, this integration deliberately avoids:

- dozens of unrelated streaming checks
- OS/package-manager orchestration
- giant mutable bash state machines
- hidden background retry loops

The goal here is:

- a small CLI surface
- predictable JSON/state files
- easy Docker / Ansible integration
- enough unlock logic to drive address rotation for the important targets
