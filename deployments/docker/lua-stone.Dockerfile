# Multi-stage hardened build for lua-stone (scripting engine)
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
    -o lua-stone \
    ./cmd/lua-stone

# Compress binary
RUN upx --best --lzma lua-stone

# Final stage - distroless for maximum security
FROM gcr.io/distroless/static-debian12:nonroot

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy CA certificates
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy binary
COPY --from=builder /build/lua-stone /lua-stone

# Copy default Lua scripts
COPY --from=builder /build/scripts/lua/ /scripts/

# Use non-root user
USER nonroot:nonroot

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/lua-stone", "-health-check"]

# Expose lua engine port
EXPOSE 8081

# Default command
ENTRYPOINT ["/lua-stone"]
CMD ["-addr", ":8081", "-scripts", "/scripts"]
