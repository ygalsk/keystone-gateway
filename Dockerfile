# ============================================
# Stage 1: Builder
# ============================================
FROM golang:1.24-bookworm AS builder

# Install LuaJIT development dependencies for CGO
RUN apt-get update && apt-get install -y --no-install-recommends \
    libluajit-5.1-dev \
    pkg-config \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build

# Copy dependency files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build with LuaJIT support using CGO
# The -tags luajit enables LuaJIT-specific code paths
# pkg-config provides correct compiler and linker flags
#
# DEFAULT BUILD (RECOMMENDED): Portable binary that works on any x86_64 CPU
RUN CGO_ENABLED=1 \
    CGO_CFLAGS="$(pkg-config --cflags luajit)" \
    CGO_LDFLAGS="$(pkg-config --libs luajit)" \
    go build -tags luajit \
    -ldflags="-w -s" \
    -o keystone-gateway \
    ./cmd

# OPTIMIZED BUILD (OPTIONAL): +10-15% performance, CPU-specific (NOT portable)
# Uncomment the lines below and comment out the DEFAULT BUILD above to enable
# ⚠️ WARNING: Binary will only work on CPUs matching the build machine architecture
#
# RUN CGO_ENABLED=1 \
#     CGO_CFLAGS="-O3 -march=native -flto $(pkg-config --cflags luajit)" \
#     CGO_LDFLAGS="-O3 -flto $(pkg-config --libs luajit)" \
#     go build -tags luajit \
#     -ldflags="-w -s" \
#     -o keystone-gateway \
#     ./cmd

# ============================================
# Stage 2: Runtime
# ============================================
FROM debian:bookworm-slim

# Build arguments for user/group ID (allows matching host user)
ARG USER_ID=1000
ARG GROUP_ID=1000

# Install runtime dependencies only (no dev headers)
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    libluajit-5.1-2 \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user for security with configurable UID/GID
RUN groupadd -r gateway -g ${GROUP_ID} && \
    useradd -r -g gateway -u ${USER_ID} -m -d /app gateway

WORKDIR /app

# Copy binary from builder with proper ownership
COPY --from=builder --chown=gateway:gateway /build/keystone-gateway /app/keystone-gateway

# Create volume mount points with correct permissions
RUN mkdir -p /app/scripts && \
    chown -R gateway:gateway /app

# Switch to non-root user
USER gateway

# Environment variables for runtime configuration
# Users can override these via docker-compose or command-line
ENV CONFIG_PATH=/app/config.yaml \
    LISTEN_ADDR=:8080

# Expose HTTP port
EXPOSE 8080

# Health check using the built-in /health endpoint
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Use ENTRYPOINT for the binary, CMD for default arguments
# This allows users to override flags easily
ENTRYPOINT ["/app/keystone-gateway"]
CMD ["-config", "/app/config.yaml", "-addr", ":8080"]
