# Project: Keystone Gateway
A high-performance, programmable reverse proxy and API gateway written in Go with embedded Lua scripting for dynamic routing in multi-tenant environments.

## Features
- **Multi-tenant routing**: Host-based, path-based, and hybrid routing strategies
- **Embedded Lua scripting**: Dynamic route definition and middleware without recompilation
- **Load balancing**: Round-robin load balancing with health checking
- **Advanced middleware**: Authentication, rate limiting, request/response transformation
- **Admin API**: Health monitoring and tenant management endpoints
- **Thread-safe architecture**: Lua state pools and atomic operations for concurrent safety

## Tech Stack
- **Language**: Go 1.21+
- **Router**: Chi v5.2.2
- **Scripting**: Gopher-Lua v1.1.1 for embedded Lua interpreter
- **Configuration**: YAML v3
- **Build**: Go modules (no build system currently configured)
- **Database**: PostgreSQL with JSONB support

## Structure
- **`cmd/`**: Main application entry point
- **`internal/config/`**: Configuration management and YAML parsing
- **`internal/routing/`**: Core routing logic and load balancing
- **`internal/lua/`**: Lua scripting engine and Chi bindings
- **`configs/`**: YAML configuration files
- **`scripts/`**: Lua routing scripts and examples

## Architecture
**Layered + Plugin Architecture**: HTTP layer (Chi) → Application layer (Gateway + Lua Engine) → Business logic (Routing + Configuration). Components interact through thread-safe Lua state pools, atomic operations for health tracking, and dynamic route registration via Lua scripts.

## Commands
- Build: `go build -o keystone-gateway ./cmd/`
- Test: **No tests currently implemented**
- Lint: **No linting configured**
- Dev/Run: `go run ./cmd/`

## Testing
**No testing framework currently set up** - needs to be implemented