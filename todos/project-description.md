# Project: Keystone Gateway
A lightweight, extensible reverse proxy and API gateway designed for SMBs and DevOps teams, currently migrating from dual-service to embedded Lua scripting through Chi routing APIs.

## Features
- Multi-tenant reverse proxy with host-based, path-based, and hybrid routing
- Health-based load balancing with automatic backend monitoring  
- Single binary deployment with YAML configuration
- **MIGRATING**: From dual-service (chi-stone + lua-stone) to embedded Lua engine
- Dynamic route registration via Lua scripts using Chi routing APIs
- High performance target: 300+ requests/second with <5ms latency
- Admin API endpoints for monitoring (/health, /tenants)
- Multi-client hosting on single infrastructure

## Tech Stack
**Languages:** Go (1.19+), Lua (via embedded gopher-lua)
**Frameworks:** Chi router v5 with embedded Lua engine integration
**Build Tools:** Go modules, comprehensive Makefile, Docker builds
**Testing:** Go testing, Testify, integration tests for Lua routing
**Migration Status:** Core embedded Lua implemented, Chi bindings need completion

## Structure
**Entry Points:** cmd/chi-stone/main.go (single binary with embedded Lua)
**Core Packages:** internal/config (YAML), internal/routing (gateway + lua_routes), internal/lua (embedded engine + chi_bindings)
**Migration Focus:** internal/lua/chi_bindings.go (Chi routing API integration)
**Configuration:** configs/ (lua_routing.enabled, scripts_dir)
**Scripts:** scripts/examples/ (Lua route definition examples)

## Architecture
**Current State:** Migrating from external lua-stone service to embedded gopher-lua
**New Approach:** Single chi-stone binary with embedded Lua engine for dynamic route registration
**Route Registry:** LuaRouteRegistry manages tenant-specific Chi submuxes
**Lua Integration:** Chi routing APIs exposed to Lua scripts (chi_route, chi_middleware, chi_group)
**Migration Status:** 95% complete, needs chi_bindings.go compilation fixes

## Commands
- Build: `make build` (chi-stone only - lua-stone being deprecated)
- Test: `make test` (includes Lua routing integration tests)
- Lint: `make fmt`, `make lint`, `make check`
- Dev/Run: `make run` (embedded Lua mode), `make dev` (full workflow)

## Testing
**Migration Testing:** Integration tests in test/integration/lua_routing_test.go
**Current Focus:** Testing embedded Lua route registration and Chi API integration
**Test Coverage:** Lua script execution, route mounting, tenant isolation
**Known Issues:** URL parameter extraction tests need chi_param() completion