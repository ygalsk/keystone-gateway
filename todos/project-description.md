# Project: Keystone Gateway
A high-performance, programmable reverse proxy and API gateway written in Go with embedded Lua scripting for dynamic routing in multi-tenant environments.

## Features
- **Multi-tenant routing**: Host-based, path-based, and hybrid routing strategies
- **Embedded Lua scripting**: Dynamic route definition without recompilation
- **Load balancing**: Round-robin load balancing with health checking
- **Admin API**: Health monitoring, tenant management, and real-time status
- **High-performance architecture**: Built on Chi router with thread-safe design
- **Flexible configuration**: YAML-based configuration with example templates

## Tech Stack
- **Go 1.19+**: Primary backend language with Chi router
- **Lua**: Embedded scripting via gopher-lua for dynamic routing
- **Libraries**: go-chi/chi/v5, yuin/gopher-lua, gopkg.in/yaml.v3
- **Build**: Go modules, Makefile automation
- **Testing**: Built-in Go testing with unit/integration/e2e tests
- **CI/CD**: GitHub Actions with security scanning and multi-platform builds

## Structure
- **cmd/**: Application entry point (main.go)
- **internal/**: Core packages (config, lua, routing)
- **configs/**: Configuration files and examples
- **scripts/**: Lua route definition scripts
- **tests/**: Test suites (unit/integration/e2e)
- **testdata/**: Test fixtures and sample data
- **bin/**: Built binaries

## Architecture
- **HTTP Layer**: Chi router with middleware stack
- **Gateway Layer**: Multi-tenant routing and load balancing
- **Lua Engine**: Thread-safe state pools for dynamic scripting
- **Configuration**: YAML-based tenant and service management

## Commands
- Build: `make build` or `go build -o bin/keystone-gateway cmd/main.go`
- Test: `make test` or `go test ./tests/...`
- Lint: `make lint` or `golangci-lint run`
- Dev/Run: `make run` or `./bin/keystone-gateway -config config.yaml`

## Testing
- **Framework**: Go's built-in testing package with table-driven tests
- **Organization**: Separate unit, integration, and e2e test directories
- **Naming**: `*_test.go` files with `TestFunctionName` pattern
- **Test data**: Located in `testdata/` with configs and Lua scripts
- **Running**: Use `make test-unit`, `make test-integration`, `make test-e2e`