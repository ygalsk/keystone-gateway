# Chi Routing API Redesign for Better Lua Integration
**Status:** InProgress
**Agent PID:** 533341

## Original Todo
do a code and architecture review of the current code base of chi0stone and lua-stone, then createe a suggestion stTING REWRTIGNT HE API OF CHI TO BE MORE LIKE A CUSTOM ROUTE register for chi routing and how we could leverge that for better lua script execution

## Description
Create a new Chi Router API design that enables lua-stone scripts to register custom routes directly with the Chi router, replacing the current static configuration-based routing with a dynamic, lua-scriptable route registration system. This will transform lua-stone from an external backend-selection service into an embedded route-definition engine that can register custom Chi routes, middleware, and matchers at runtime.

## Implementation Plan
- [x] **Create embedded Lua engine interface** (`internal/lua/engine.go`): Replace external lua-stone service with embedded gopher-lua engine that can register routes directly with Chi router
- [x] **Design Chi route registration API** (`internal/routing/lua_routes.go`): Create new API allowing Lua scripts to call `chi_route(method, pattern, handler)` and `chi_middleware(handler)` functions  
- [x] **Refactor Gateway to use dynamic routing** (`internal/routing/gateway.go:50-120`): Replace static route maps with dynamic route registration system that accepts lua-defined routes
- [x] **Create Lua-Chi bridge functions** (`internal/lua/chi_bindings.go`): Implement Go functions callable from Lua that register routes with Chi router (route, middleware, group, mount)
- [x] **Update configuration schema** (`internal/config/config.go:25-35`): Add `lua_routes` field to tenant config for specifying route definition scripts vs backend selection scripts
- [ ] **Replace setupTenantRouting with script execution** (`cmd/chi-stone/main.go:215-257`): Remove manual route configuration, replace with lua script execution that registers routes dynamically
- [ ] **Create lua script route examples** (`scripts/examples/`): Provide example scripts showing custom route registration patterns (canary routes, A/B testing routes, auth routes)
- [ ] **Automated test**: Create integration tests verifying lua scripts can register custom Chi routes and middleware (`test/integration/lua_routing_test.go`)
- [ ] **User test**: Verify lua scripts can define custom routing patterns like `/api/v1/{version}/users` with version-based backend selection without core code changes

## Notes
Analysis complete - see analysis.md. Current architecture has static route registration and external lua-stone service. New design will embed lua engine and allow dynamic route registration through Chi API bindings.
