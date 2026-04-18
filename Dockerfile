# syntax=docker/dockerfile:1

# Global ARGs (must be re-declared in each stage that needs them)
ARG SINGBOX_VERSION=1.13.5
ARG ALPINE_VERSION=3.23

# ── Stage 1: Build cfwarp-cli ────────────────────────────────────────────────
FROM golang:1.26-alpine${ALPINE_VERSION} AS builder
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags="-s -w \
      -X github.com/nexus/cfwarp-cli/internal/version.Version=${VERSION} \
      -X github.com/nexus/cfwarp-cli/internal/version.Commit=${COMMIT} \
      -X github.com/nexus/cfwarp-cli/internal/version.Date=${BUILD_DATE}" \
    -o /out/cfwarp-cli .

# ── Stage 2: Fetch sing-box for target arch ──────────────────────────────────
FROM alpine:${ALPINE_VERSION} AS singbox
ARG SINGBOX_VERSION=1.13.5
# TARGETARCH is injected by BuildKit (amd64 / arm64)
ARG TARGETARCH=amd64

RUN apk add --no-cache curl ca-certificates && \
    mkdir -p /out && \
    curl -fsSL \
      "https://github.com/SagerNet/sing-box/releases/download/v${SINGBOX_VERSION}/sing-box-${SINGBOX_VERSION}-linux-${TARGETARCH}.tar.gz" \
      | tar -xz --strip-components=1 -C /tmp && \
    install -m 0755 /tmp/sing-box /out/sing-box

# ── Stage 3: Final minimal image ─────────────────────────────────────────────
FROM alpine:${ALPINE_VERSION}
ARG ALPINE_VERSION

RUN apk add --no-cache ca-certificates gcompat && \
    adduser -D -u 1000 -g cfwarp cfwarp && \
    mkdir -p /home/cfwarp/.local/state/cfwarp-cli && \
    chown -R cfwarp:cfwarp /home/cfwarp

ENV HOME=/home/cfwarp \
    CFWARP_STATE_DIR=/home/cfwarp/.local/state/cfwarp-cli \
    CFWARP_REGISTER_ON_START=1

COPY --from=builder  /out/cfwarp-cli      /usr/local/bin/cfwarp-cli
COPY --from=singbox  /out/sing-box        /usr/local/bin/sing-box
COPY docker/entrypoint.sh                 /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 1080
HEALTHCHECK --interval=30s --timeout=10s --start-period=45s --retries=3 \
  CMD ["cfwarp-cli", "status", "--state-dir", "/home/cfwarp/.local/state/cfwarp-cli", "--require-account", "--require-running", "--require-reachable"]
USER cfwarp
ENTRYPOINT ["/entrypoint.sh"]
