FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o lua-engine .

# Final stage
FROM scratch

# Copy the binary
COPY --from=builder /app/lua-engine /lua-engine

# Create non-root user (using nobody user ID)
USER 65534:65534

# Expose port
EXPOSE 8081

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD ["/lua-engine", "-addr", ":8081"]

ENTRYPOINT ["/lua-engine"]
