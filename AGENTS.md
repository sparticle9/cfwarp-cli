# AGENTS.md

## Project rules

- Never write, embed, or commit any user-specific environment setup, machine details, host aliases, inventory entries, local paths, account details, tokens, keys, secrets, or credentials into tracked files.
- This prohibition applies to all committable artifacts, including code, docs, examples, comments, playbooks, inventories, scripts, tests, fixtures, templates, and generated files.
- When examples are needed, use clearly generic placeholders such as `proxy-host-1`, `/path/to/project`, `example-token`, and similar non-user-specific values.
- If environment-specific information is needed for local execution, keep it out of tracked files and provide it only ephemerally at runtime or via untracked local overrides.
