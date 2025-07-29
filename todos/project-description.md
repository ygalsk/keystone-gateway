# Project: Keystone Gateway
A high-performance, programmable reverse proxy and API gateway with embedded Lua scripting for dynamic multi-tenant routing.

## Features
- Multi-tenant routing strategies (host-based, path-based, hybrid)
- Embedded Lua scripting engine for dynamic route definition without recompilation
- Round-robin load balancing with automatic failover and health checking
- HTTP compression with configurable gzip for improved performance
- Admin API for monitoring gateway and tenant health status
- Thread-safe architecture with Lua state pools and atomic operations
- TLS support with optional HTTPS termination
- Graceful shutdown with proper cleanup and connection draining

## Tech Stack
- **Languages**: Go 1.19+, Lua (embedded), Shell/Bash, YAML
- **Frameworks**: Chi Router v5.2.2, Standard Library HTTP, Gopher-Lua v1.1.1
- **Build Tools**: Go Modules, Docker Multi-stage Build, GNU Make
- **Infrastructure**: Docker Compose, Alpine Linux, Nginx, Prometheus

## Structure
- `/cmd/main.go` - Main application entry point
- `/internal/config/` - Configuration management and YAML parsing
- `/internal/routing/` - Core routing, load balancing, and proxy logic
- `/internal/lua/` - Embedded Lua scripting engine for dynamic routing
- `/configs/` - Configuration files with environment examples
- `/scripts/` - Lua scripts for dynamic route definitions
- `/tests/` - Comprehensive test suite (unit, integration, e2e, performance)

## Architecture
Layered architecture with HTTP layer (Chi router), Application layer (Gateway with Lua engine), Business logic (Multi-tenant routing and load balancing), and Backend integration (Health checking and proxy management). Uses embedded Lua scripting for dynamic routing without restarts.

## Commands
- Build: `make build` (local) or `make build` (Docker)
- Test: `make test` (core tests) or `make test-all` (including performance)
- Lint: `make lint` (requires golangci-lint)
- Dev/Run: `make run` or `./bin/keystone-gateway -config configs/production.yaml`

## Testing
Comprehensive testing infrastructure with Go's standard testing package:
- **Unit tests** (`/tests/unit/`) - Fast, isolated component testing
- **Integration tests** (`/tests/integration/`) - Component interaction testing
- **E2E tests** (`/tests/e2e/`) - Full system workflow testing
- **Performance tests** - Benchmarks, load tests, regression tracking
- **Fixtures** - Extensive utilities for backends, configs, and HTTP clients