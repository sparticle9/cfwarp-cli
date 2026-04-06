# Integration Tests

These tests require a Docker environment and are opt-in.

## Prerequisites

- Docker with Compose plugin
- A built cfwarp-cli image

## Running

### Build the image first

```bash
docker build -t cfwarp-cli:test .
```

### Run the smoke test

```bash
go test -v -tags integration -timeout 120s ./test/integration/
```

### Run with live WARP verification (requires real network)

```bash
CFWARP_TEST_LIVE=1 go test -v -tags integration -timeout 180s ./test/integration/
```

### Use a custom image

```bash
CFWARP_TEST_IMAGE=ghcr.io/owner/cfwarp-cli:latest go test -v -tags integration ./test/integration/
```
