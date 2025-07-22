# Project: Keystone Gateway
High-performance, programmable reverse proxy and API gateway with embedded Lua scripting for dynamic multi-tenant routing.

## Features
- Multi-tenant routing (host-based, path-based, hybrid strategies)
- Embedded Lua scripting engine for dynamic route definition
- Load balancing with round-robin and health checking
- Admin API for health monitoring and tenant management
- Thread-safe Lua state pools for concurrent execution
- TLS/HTTPS support and comprehensive request handling

## Tech Stack
- **Languages**: Go (1.19+), Lua (embedded via gopher-lua)
- **Frameworks**: Chi Router v5.2.2, net/http reverse proxy
- **Build**: Makefile, Go modules, golangci-lint
- **Testing**: Go testing package with 3-tier structure (unit/integration/e2e)
- **CI/CD**: GitHub Actions with multi-platform builds, security scanning

## Structure
- `/cmd/main.go` - Application entry point and HTTP handlers
- `/internal/config/` - YAML configuration management
- `/internal/lua/` - Embedded Lua engine with Chi bindings
- `/internal/routing/` - Gateway routing and load balancing
- `/scripts/` - Lua routing scripts (*.lua files)
- `/configs/examples/` - Sample configurations
- `/tests/` - Three-tier test suite (unit, integration, e2e)

## Architecture
Layered architecture with HTTP layer (Chi), Application layer (Gateway), Multi-tenant routing, and embedded Lua scripting. Components interact through request flow: Chi router → middleware → tenant matching → Lua execution → backend selection → reverse proxy.

## Commands
- Build: `make build`
- Test: `make test` (also `test-unit`, `test-integration`, `test-e2e`)
- Lint: `make lint`
- Dev/Run: `make run`

## Testing
Uses Go's built-in testing package with table-driven tests, httptest for HTTP mocking, temporary directories for isolation, and comprehensive coverage reporting. Test data stored in `/testdata/` with sample configs and Lua scripts.