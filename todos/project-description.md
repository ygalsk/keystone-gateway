# Project: Keystone Gateway
A lightweight, multi-tenant reverse proxy and API gateway designed for SMEs and DevOps teams who need efficient traffic routing without the complexity of enterprise solutions.

## Features
- Multi-tenant routing (host-based, path-based, hybrid)
- Health-aware load balancing with automatic failover
- YAML-based configuration with validation
- Modular architecture for easy extension
- Optional Lua scripting engine for advanced routing logic
- Docker containerization support
- Comprehensive monitoring and logging

## Tech Stack
- **Language**: Go 1.19+
- **HTTP Router**: Chi v5.2.2
- **Configuration**: YAML (gopkg.in/yaml.v3)
- **Testing**: Go testing + testify assertions
- **Build**: Makefile-based workflow
- **Containerization**: Docker with multi-stage builds
- **Scripting**: Lua engine (optional lua-stone service)

## Structure
- `cmd/chi-stone/` - Main gateway binary
- `internal/config/` - Configuration management and validation
- `internal/routing/` - Core gateway routing logic
- `test/` - Organized test structure (unit/integration/e2e)
- `configs/` - Example configurations
- `docs/` - Comprehensive documentation
- `lua-engine/` - Optional Lua scripting service
- `mock-backends/` - Test backend services

## Architecture
- **Gateway**: Main routing engine that matches requests to tenants
- **TenantRouter**: Manages backend pools for each tenant
- **GatewayBackend**: Individual backend server representations
- **Config**: Centralized configuration with validation
- **Lua Engine**: Optional service for advanced scripting features

## Commands
- Build: `make build`
- Test: `make test`
- Lint: `make lint`
- Dev/Run: `make run` or `go run cmd/chi-stone/main.go`
- Docker: `make docker-build`

## Testing
- Unit tests: Place alongside source files with `_test.go` suffix
- Integration tests: Use `test/integration/` directory
- Run specific tests: `go test ./internal/config -v`
- Test with coverage: `make test-coverage`
