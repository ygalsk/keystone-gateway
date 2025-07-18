# Multi-stage build with hardened Alpine Linux
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
    -ldflags='-w -s -extldflags "-static"' \
    -o keystone-gateway \
    main.go

# Compress binary (optional, saves ~30% size)
RUN upx --best --lzma keystone-gateway

# Final stage - Hardened Alpine
FROM alpine:3.19

# Install security updates and minimal runtime dependencies
RUN apk --no-cache upgrade && \
    apk --no-cache add \
    ca-certificates \
    tzdata \
    dumb-init \
    wget \
    && rm -rf /var/cache/apk/*

# Create non-root user
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Create app directory with proper permissions
RUN mkdir -p /app/configs && \
    chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

WORKDIR /app

# Copy binary and configs from builder
COPY --from=builder --chown=appuser:appgroup /build/keystone-gateway .
COPY --from=builder --chown=appuser:appgroup /build/configs ./configs

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Security: Run as non-root, use dumb-init for signal handling
ENTRYPOINT ["/usr/bin/dumb-init", "--"]
CMD ["./keystone-gateway", "-config", "configs/config.yaml"]

# Expose port
EXPOSE 8080

# Labels for better maintainability
LABEL maintainer="kontakt@keystone-gateway.dev" \
      org.opencontainers.image.title="Keystone Gateway" \
      org.opencontainers.image.description="Smart Reverse Proxy Gateway for SMBs" \
      org.opencontainers.image.url="https://github.com/ygalsk/keystone-gateway" \
      org.opencontainers.image.source="https://github.com/ygalsk/keystone-gateway" \
      org.opencontainers.image.vendor="Keystone Gateway Team" \
      org.opencontainers.image.licenses="MIT"
