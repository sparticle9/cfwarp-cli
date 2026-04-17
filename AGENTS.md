# AGENTS.md

## Project rules

- Never write, embed, or commit any user-specific environment setup, machine details, host aliases, inventory entries, local paths, account details, tokens, keys, secrets, or credentials into tracked files.
- This prohibition applies to all committable artifacts, including code, docs, examples, comments, playbooks, inventories, scripts, tests, fixtures, templates, and generated files.
- When examples are needed, use clearly generic placeholders such as `proxy-host-1`, `/path/to/project`, `example-token`, and similar non-user-specific values.
- If environment-specific information is needed for local execution, keep it out of tracked files and provide it only ephemerally at runtime or via untracked local overrides.
- For Dockerized dogfood or deployment flows, prefer a **single published image/package** for both WireGuard and MASQUE setups, and switch protocol behavior through runtime configuration/env vars instead of per-protocol images.
- For image builds that are meant to be deployed, shared, or referenced by Ansible/docs, prefer **GitHub Actions / GHCR-built artifacts**. Use local `docker build` only for quick smoke tests, and do not treat local-only images as release or dogfood artifacts.
