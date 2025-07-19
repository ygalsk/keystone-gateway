# Project: Keystone Gateway

A lightweight, extensible reverse proxy designed for KMUs (Small and Medium Enterprises) and DevOps teams. Provides intelligent HTTP request routing to backend services based on hostnames and URL paths with health-aware load balancing.

## Features

- **Multi-tenant Routing**: Host-based, path-based, and hybrid routing strategies
- **Health-based Load Balancing**: Round-robin with automatic health checks and failover
- **Simple YAML Configuration**: Zero-code setup for basic use cases
- **Built-in Monitoring**: `/health` and `/tenants` endpoints for observability
- **Configurable Admin Endpoints**: Custom base paths for management APIs
- **Docker-ready**: Complete containerization with Docker Compose
- **Single Binary Deployment**: Statically compiled Go binary with minimal dependencies

## Tech Stack

- **Language**: Go 1.19+
- **HTTP Router**: Chi Router v5
- **Configuration**: YAML (gopkg.in/yaml.v3)
- **Containerization**: Docker with multi-stage builds and Alpine Linux
- **Orchestration**: Docker Compose + Docker Swarm support
- **Testing**: Go built-in testing framework
- **Build Tools**: Makefile

## Structure

- **Entry Point**: `main.go` (555 lines) - Complete gateway implementation
- **Configuration**: `configs/config.yaml` - Production configuration
- **Infrastructure**: `Dockerfile`, `docker-compose.yml`, `Makefile`
- **Documentation**: `README.md`, `docs/` directory
- **Testing**: `main_test.go` - Unit and integration tests
- **Mock Services**: `mock-backends/` - Testing backend services

## Architecture

Clean layered architecture with Chi router integration:
- **HTTP Layer**: Chi Router with middleware stack
- **Gateway Core**: Routing logic with three router types (path/host/hybrid)
- **Backend Management**: Health checks and round-robin load balancing
- **Reverse Proxy**: httputil.ReverseProxy for request forwarding

## Commands

- **Build**: `make build` or `go build -o keystone-gateway main.go`
- **Test**: `make test` or `go test ./...`
- **Lint**: `go fmt ./...` and `go vet ./...`
- **Dev/Run**: `go run main.go` or `./keystone-gateway -config configs/config.yaml -addr :8080`

## Testing

Uses Go's standard testing framework with table-driven tests. Create new tests following existing patterns:
- Unit tests for core functionality
- HTTP integration tests using `httptest.NewServer`
- Benchmark tests for performance-critical operations
- Mock backend servers for testing routing and load balancing