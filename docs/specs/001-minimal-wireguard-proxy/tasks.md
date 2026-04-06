# Implementation Plan — Minimal WireGuard Proxy MVP

- [ ] 1. Initialize the Go CLI workspace and command skeleton for `cfwarp-cli`.
  - Create the Go module, main entrypoint, command package, and version wiring.
  - Implement stub commands for `register`, `import`, `render`, `up`, `down`, `status`, and `endpoint test`.
  - Add unit tests for command parsing and top-level validation.
  - Requirements: 1.1, 1.2, 1.3, 8.1.

- [ ] 2. Implement config/state path management and persistent data models.
  - Add code for host and container state path resolution.
  - Create JSON-backed models for `AccountState`, `Settings`, `RuntimeState`, and `EndpointCandidate`.
  - Add secure file-writing helpers and tests for overwrite protection and permission handling.
  - Requirements: 2.3, 4.1, 7.1, 7.4, 8.2.

- [ ] 3. Implement the Cloudflare consumer registration client owned by this project.
  - Add X25519 key generation and registration request/response handling.
  - Implement `register` and `import` command logic using the persistent state layer.
  - Add tests using mocked HTTP responses for success, timeout, malformed response, and overwrite cases.
  - Requirements: 2.1, 2.2, 2.3, 2.4.

- [ ] 4. Implement settings loading from flags, env vars, and persisted defaults.
  - Support listen host, listen port, auth, log level, backend choice, and endpoint override.
  - Add validation for malformed endpoint values and inconsistent auth settings.
  - Add tests for precedence and validation behavior.
  - Requirements: 3.2, 5.1, 5.4, 6.4, 7.2.

- [ ] 5. Implement the backend abstraction and the first `singbox-wireguard` backend.
  - Define the backend interface described in the design.
  - Use `sing-box` as the **first backend for explicit proxy mode** because it can run the WireGuard transport and proxy listener in userspace, which keeps the first Docker deployment lower-friction and typically avoids `NET_ADMIN` / `wg0` setup for the MVP path.
  - Make the implementation boundary explicit so a later `kernel-wg-microsocks` backend can be added for a leaner but higher-privilege Linux path.
  - Add prerequisite checks for the `sing-box` binary.
  - Render backend config from registration state and settings.
  - Add golden tests for rendered config variants including no-auth, auth-enabled, and endpoint-override cases.
  - Requirements: 3.1, 3.2, 3.3, 3.4, 5.1, 5.2, 6.1, 8.1, 8.2, 8.3.

- [ ] 6. Implement process supervision for `up`, `down`, and runtime cleanup.
  - Launch `sing-box` with the rendered config, capture PID/log paths, and persist runtime metadata.
  - Implement clean shutdown and stale-runtime detection.
  - Add tests with a fake backend process to cover start, stop, crash, and stale PID handling.
  - Requirements: 4.1, 4.2, 4.4, 7.3.

- [ ] 7. Implement `status` and lightweight health probing.
  - Report configuration presence, process liveness, local reachability, and last known error.
  - Add optional JSON output and an opt-in remote `cdn-cgi/trace` probe.
  - Add tests for status output in healthy, missing-config, crashed-process, and probe-failure states.
  - Requirements: 4.3, 4.4, 7.2, 7.3.

- [ ] 8. Implement `endpoint test` for manual candidate validation.
  - Validate endpoint syntax and render candidate-specific backend config.
  - Add optional lightweight backend preflight checks without mutating stored runtime state.
  - Add tests for valid, invalid, default, and override candidate flows.
  - Requirements: 5.1, 5.2, 5.3, 5.4.

- [ ] 9. Add Docker packaging for the MVP deployment.
  - Create a multi-stage Dockerfile that builds `cfwarp-cli`, installs `sing-box`, and runs as a non-root user.
  - Implement an entrypoint or foreground command path that auto-registers on first start and reuses persisted state afterward.
  - Add a container smoke test that verifies the image starts and the proxy port opens with a mounted volume.
  - Requirements: 6.1, 6.2, 6.4, 7.4.

- [ ] 10. Add example compose-backed integration tests and proxy-consumer tests.
  - Create automated integration coverage for the documented single-container deployment.
  - Add a test fixture or scripted check for an app container consuming the proxy through `ALL_PROXY`.
  - Keep live external trace checks opt-in so default CI remains deterministic.
  - Requirements: 6.2, 6.3, 7.2.
