# Requirements — Minimal WireGuard Proxy MVP

## Introduction

`cfwarp-cli` starts as a CLI for bringing up a Cloudflare-WARP-backed explicit proxy using a WireGuard-compatible backend, with current support aimed at Linux and macOS on Apple Silicon and broader platform support expected later. The initial MVP prioritizes minimal effort, reproducible deployment, and clear ownership of the registration and configuration path, with Docker as the primary deployment path.

The MVP deliberately excludes transparent routing, kernel-level route takeover, and MASQUE implementation work. Those remain future phases.

### Scope for this spec

- direct Cloudflare consumer registration or import of existing credentials
- generation of runtime configuration for a WireGuard-based proxy backend
- lifecycle management for a local explicit proxy
- endpoint override (`优选 IP`) support
- specific Docker deployment guidance for real usage

### Non-goals for this MVP

- transparent TUN/sidecar traffic takeover
- Zero Trust enrollment flows
- MASQUE implementation
- full feature parity with the official Cloudflare One client
- performance auto-tuning beyond basic endpoint selection helpers

## Requirements

### 1. CLI foundation and current runtime support

**User story:** As an operator, I want a single CLI that manages the proxy lifecycle, so that I can use Cloudflare WARP from Linux hosts or Docker without glue scripts.

#### Acceptance criteria
1.1. **When** the operator runs a supported command such as `register`, `import`, `up`, `down`, `status`, or `render`, **the system shall** expose it through a single `cfwarp-cli` binary with help text and machine-readable exit codes.
1.2. **When** the system starts on an unsupported platform or missing prerequisite runtime, **the system shall** fail with a clear error explaining what is unsupported and how to proceed.
1.3. **When** the operator requests version or diagnostics output, **the system shall** print the application version, detected backend, and required external binary paths.

### 2. Registration and credential ownership

**User story:** As an operator, I want the tool to own the bootstrap path for consumer WARP credentials, so that I do not depend on `wgcf`, shortlinks, or the official client for the MVP.

#### Acceptance criteria
2.1. **When** the operator runs `cfwarp-cli register`, **the system shall** generate a local keypair, call the Cloudflare consumer registration API, and persist the returned account data in local state.
2.2. **When** the operator already has valid WARP registration data, **the system shall** allow import from a file or flags without forcing re-registration.
2.3. **If** persisted registration data already exists, **the system shall** require explicit confirmation or a `--force` flag before overwriting it.
2.4. **When** registration fails due to network, API, or invalid response conditions, **the system shall** preserve existing state and return a structured error.

### 3. WireGuard proxy backend configuration

**User story:** As an operator, I want the CLI to render a working backend configuration automatically, so that I can start a WARP-backed proxy with minimal manual editing.

#### Acceptance criteria
3.1. **When** registration data is available, **the system shall** render a backend configuration containing the WireGuard addresses, peer public key, local private key, and proxy listener settings.
3.2. **When** the operator specifies listen host, listen port, or proxy authentication settings, **the system shall** include them in the rendered backend configuration.
3.3. **When** the operator requests a dry run or render-only mode, **the system shall** output the generated backend configuration without starting the proxy.
3.4. **If** required backend fields are missing or malformed, **the system shall** refuse to render a runnable configuration and explain which fields are invalid.

### 4. Proxy lifecycle management

**User story:** As an operator, I want simple commands to start, stop, and inspect the proxy, so that I can run it locally or in Docker with minimal ceremony.

#### Acceptance criteria
4.1. **When** the operator runs `cfwarp-cli up`, **the system shall** validate prerequisites, render configuration, launch the configured backend, and persist runtime metadata.
4.2. **When** the operator runs `cfwarp-cli down`, **the system shall** stop the managed backend process and clean up transient runtime files.
4.3. **When** the operator runs `cfwarp-cli status`, **the system shall** report whether the backend process is configured, running, and locally reachable.
4.4. **If** the backend exits unexpectedly, **the system shall** surface the last known error and non-zero process status to the operator.

### 5. Endpoint override (`优选 IP`) support

**User story:** As an operator, I want to override the default Cloudflare peer endpoint, so that I can try datacenter-specific IP/port combinations that work better for my network.

#### Acceptance criteria
5.1. **When** the operator supplies an endpoint override such as `host:port`, **the system shall** use it in the rendered WireGuard peer configuration.
5.2. **When** no endpoint override is supplied, **the system shall** use the default Cloudflare endpoint settings appropriate for the selected backend.
5.3. **When** the operator runs an endpoint validation or test command, **the system shall** attempt a basic probe or config render sanity check and report success or failure per candidate.
5.4. **If** an endpoint value is malformed, **the system shall** reject it before launch with a validation error.

### 6. Primary Docker deployment support

**User story:** As an operator, I want a specific Docker deployment path, so that I can run the proxy reproducibly without hand-writing bespoke shell entrypoints.

#### Acceptance criteria
6.1. **When** the operator builds the project container image, **the system shall** be runnable as a single-container explicit proxy without `NET_ADMIN` or kernel WireGuard requirements for the MVP backend.
6.2. **When** the operator uses the documented `docker run` or `docker compose` examples, **the system shall** persist registration state in a mounted volume and expose the configured proxy port.
6.3. **When** another container needs to consume the proxy, **the system shall** provide a documented Docker pattern using environment variables such as `ALL_PROXY` or service-to-service DNS.
6.4. **If** container startup fails due to missing configuration, **the system shall** log the failure cause and exit non-zero instead of hanging silently.

### 7. Local state, diagnostics, and minimal observability

**User story:** As an operator, I want predictable state files and diagnostics, so that I can troubleshoot and automate the tool.

#### Acceptance criteria
7.1. **When** the CLI persists registration or runtime state, **the system shall** store it in deterministic paths under a documented config/state directory.
7.2. **When** the operator requests status in JSON form, **the system shall** emit structured fields suitable for scripting.
7.3. **When** the backend writes logs or health errors, **the system shall** make the latest actionable failure reason available through the CLI.
7.4. **If** sensitive credentials are written to disk, **the system shall** restrict file permissions to the current user in host mode and to the application user in container mode.

### 8. Future backend extensibility

**User story:** As a maintainer, I want the MVP to leave room for kernel WireGuard and MASQUE backends later, so that we do not lock the project into one implementation forever.

#### Acceptance criteria
8.1. **When** the initial backend is implemented, **the system shall** isolate backend-specific rendering and process control behind a backend interface.
8.2. **When** future work adds a new backend such as kernel WireGuard or MASQUE, **the system shall** be able to reuse the existing CLI state and command model without breaking current users.
8.3. **If** a future backend is unavailable on the current system, **the system shall** report that limitation without affecting the configured MVP backend.
