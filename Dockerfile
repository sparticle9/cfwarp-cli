# Global ARGs (must be re-declared in each stage that needs them)
ARG SINGBOX_VERSION=1.13.5

# Stage 1: Build the Go binary
FROM golang:1.24-alpine AS builder
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build \
    -ldflags="-s -w \
      -X github.com/nexus/cfwarp-cli/internal/version.Version=${VERSION} \
      -X github.com/nexus/cfwarp-cli/internal/version.Commit=${COMMIT} \
      -X github.com/nexus/cfwarp-cli/internal/version.Date=${BUILD_DATE}" \
    -o cfwarp-cli .

# Stage 2: Download sing-box
FROM alpine:3.21 AS singbox
# Re-declare ARGs consumed in this stage
ARG SINGBOX_VERSION=1.13.5
ARG TARGETARCH=amd64
RUN apk add --no-cache curl ca-certificates && \
    mkdir /out && \
    curl -fsSL \
      "https://github.com/SagerNet/sing-box/releases/download/v${SINGBOX_VERSION}/sing-box-${SINGBOX_VERSION}-linux-${TARGETARCH}.tar.gz" \
      | tar -xz --strip-components=1 -C /tmp && \
    install -m 0755 /tmp/sing-box /out/sing-box

# Stage 3: Final image
FROM alpine:3.21
RUN apk add --no-cache ca-certificates && \
    adduser -D -u 1000 cfwarp
COPY --from=builder /build/cfwarp-cli /usr/local/bin/cfwarp-cli
COPY --from=singbox /out/sing-box /usr/local/bin/sing-box
COPY docker/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
VOLUME /var/lib/cfwarp-cli
EXPOSE 1080
USER cfwarp
ENTRYPOINT ["/entrypoint.sh"]
