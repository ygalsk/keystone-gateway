# Multi-stage hardened build for chi-stone (main gateway)
FROM golang:1.22-alpine3.19 AS builder

# Install build dependencies and security updates
RUN apk add --no-cache \
    git \
    ca-certificates \
    tzdata \
    upx \
    && apk upgrade --no-cache

# Create non-root user for build
RUN adduser -D -g '' appuser

WORKDIR /build

# Copy go mod files for dependency caching
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build with security hardening flags
RUN CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    go build \
    -a \
    -installsuffix cgo \
    -ldflags '-w -s -extldflags "-static"' \
    -o chi-stone \
    ./cmd/chi-stone

# Compress binary
RUN upx --best --lzma chi-stone

# Final stage - distroless for maximum security
FROM gcr.io/distroless/static-debian12:nonroot

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy CA certificates
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy binary
COPY --from=builder /build/chi-stone /chi-stone

# Copy default config
COPY --from=builder /build/configs/examples/config.yaml /config/config.yaml

# Use non-root user
USER nonroot:nonroot

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/chi-stone", "-health-check"]

# Expose gateway port
EXPOSE 8080

# Default command
ENTRYPOINT ["/chi-stone"]
CMD ["-config", "/config/config.yaml", "-addr", ":8080"]
