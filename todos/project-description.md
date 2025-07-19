# Project: Keystone Gateway
A lightweight, extensible reverse proxy specifically designed for SMEs and DevOps teams, combining simplicity with flexibility through Lua scripting architecture.

## Features
- Fast HTTP routing and reverse proxy (300+ req/sec)
- Multi-tenant architecture with host-based and path-based routing
- Health-based load balancing with automatic backend failover
- Optional Lua scripting for advanced features (CI/CD, canary deployments, custom business logic)
- Single binary deployment with no dependencies
- YAML-based configuration
- Admin API for health monitoring and tenant management
- Community-driven extensibility through Lua scripts

## Tech Stack
- **Language**: Go 1.19+
- **Router**: Chi v5 (github.com/go-chi/chi/v5)
- **Scripting**: Lua via gopher-lua
- **Config**: YAML (gopkg.in/yaml.v3)
- **Testing**: Go testing + testify
- **Deployment**: Docker, systemd, single binary

## Structure
- `cmd/chi-stone/` - Main gateway binary entry point
- `cmd/lua-stone/` - Lua scripting engine binary
- `internal/config/` - Configuration management
- `internal/routing/` - Core routing and load balancing logic
- `internal/health/` - Health checking functionality 
- `internal/proxy/` - Proxy implementation
- `configs/` - Configuration files and examples
- `test/` - Unit, integration, and e2e tests
- `deployments/` - Docker and systemd deployment configs

## Architecture
Two-binary architecture: 
1. **chi-stone**: Core gateway with Chi router handling HTTP routing, load balancing, and health checks
2. **lua-stone**: Optional Lua scripting engine for advanced routing decisions and middleware

Components communicate via HTTP API, with chi-stone making requests to lua-stone for script-based routing decisions.

## Commands
- Build: `make build` or `go build ./cmd/chi-stone` & `go build ./cmd/lua-stone`
- Test: `make test` or `go test ./...`
- Lint: `make lint` 
- Dev/Run: `make dev` (complete workflow) or `./chi-stone -config configs/examples/config.yaml`

## Testing
- Unit tests: `internal/*/..._test.go` using Go testing + testify
- Integration tests: `test/integration/`
- E2E tests: `test/e2e/`
- Test structure follows Go conventions with `_test.go` suffix
