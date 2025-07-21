# Project: Keystone Gateway

A high-performance, programmable reverse proxy and API gateway written in Go with embedded Lua scripting capabilities. Designed for multi-tenant environments where different applications or services need dynamic routing with programmable behavior.

## Features

- **Multi-tenant routing**: Host-based, path-based, and hybrid routing strategies
- **Embedded Lua scripting**: Define HTTP routes and middleware dynamically without recompilation  
- **Load balancing & health monitoring**: Round-robin load balancing with automatic backend health checking
- **Thread-safe Lua execution**: State pooling prevents segfaults in concurrent environments
- **Admin API**: Health endpoints and tenant management for monitoring and debugging
- **Configuration-driven**: YAML-based configuration with hot-reloadable Lua scripts

## Tech Stack

- **Language**: Go 1.19+ (Go 1.21+ recommended, current dev: 1.23.5)
- **Web Framework**: Chi Router v5.2.2 with built-in middleware
- **Scripting**: Embedded Lua via gopher-lua v1.1.1 with custom Chi bindings
- **Configuration**: YAML v3.0.1 for declarative setup
- **Code Quality**: golangci-lint, gosec, Trivy security scanning
- **CI/CD**: GitHub Actions with cross-platform builds and container support

## Structure

- `/cmd/main.go` - Main application entry point
- `/internal/config/` - YAML configuration parsing and validation
- `/internal/routing/` - Gateway routing logic, load balancing, tenant management  
- `/internal/lua/` - Embedded Lua scripting engine with Chi bindings
- `/configs/` - Configuration files and examples
- `/scripts/` - Lua scripts for dynamic route definitions
- `/test/` - Test organization (unit, integration, e2e, fixtures, mocks)

## Architecture

**Request Flow**: HTTP Requests → Chi Router → Lua Route Registry → Tenant Routers → Backend Services

**Core Components**:
- **Gateway**: Reverse proxy with multi-tenant routing and load balancing
- **Lua Engine**: Dynamic route/middleware definition via embedded scripting
- **Route Registry**: Per-tenant route isolation using Chi submuxes
- **Health Monitor**: Backend health checking with automatic failover

## Commands

- **Build**: `go build -o keystone-gateway ./cmd/`
- **Test**: `go test ./...` (test structure exists, files need implementation)
- **Lint**: `golangci-lint run`
- **Run**: `./keystone-gateway -config config.yaml`

## Testing

Test directory structure is organized with `/test/{unit,integration,e2e,fixtures,mocks}/` but no actual `*_test.go` files exist yet. Tests need to be implemented using Go's standard testing package with race detection and coverage reporting.

## Editor

- Open folder: [PENDING USER INPUT]