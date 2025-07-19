# Code and Architecture Analysis: Chi-Stone and Lua-Stone

## Current Architecture Overview

### Chi-Stone (Main Gateway)
- **File**: `cmd/chi-stone/main.go` (277 lines)
- **Router**: Uses Chi v5 as HTTP router
- **Structure**: Monolithic approach with tight coupling between routing logic and proxy logic
- **Key Components**:
  - Application struct with gateway instance
  - SetupRouter() creates chi.Mux with middleware stack
  - setupTenantRouting() manually configures routes per tenant
  - ProxyHandler() and ProxyMiddleware() handle backend selection
  - Health/Admin endpoints for monitoring

### Lua-Stone (Scripting Engine)
- **File**: `cmd/lua-stone/main.go` (442 lines)
- **Purpose**: Separate service for Lua script execution
- **API**: REST endpoints `/route/{tenant}`, `/health`, `/reload`
- **Communication**: HTTP-based with chi-stone

### Current Routing Implementation Issues

#### 1. **Manual Route Configuration**
```go
// Current approach in setupTenantRouting()
for _, tenant := range cfg.Tenants {
    if len(tenant.Domains) > 0 && tenant.PathPrefix != "" {
        // Hybrid routing - manually configured
        r.Route(tenant.PathPrefix, func(r chi.Router) {
            r.Use(app.HostMiddleware(tenant.Domains))
            r.Use(app.ProxyMiddleware(router, tenant.PathPrefix))
            r.HandleFunc("/*", func(w http.ResponseWriter, r *http.Request) {
                // Middleware handles everything
            })
        })
    } else if len(tenant.Domains) > 0 {
        // Host-only routing - manually configured
        // ... more manual configuration
    }
}
```

**Problems**:
- Repetitive code for each routing type
- No extensibility without core changes
- Difficult to add new routing patterns
- Lua integration is an afterthought, not integrated into routing

#### 2. **Lua Integration Issues**
- Lua-stone is separate service, not integrated into routing flow
- HTTP overhead for each routing decision
- No lua-based route registration - only backend selection
- Lua scripts can't define custom routing patterns
- Current lua-client in `pkg/client/lua-client.go` only handles POST requests to external service

#### 3. **Architectural Coupling**
```go
// Gateway struct has tight coupling
type Gateway struct {
    config        *config.Config
    pathRouters   map[string]*TenantRouter  // Static routing maps
    hostRouters   map[string]*TenantRouter
    hybridRouters map[string]map[string]*TenantRouter
    luaClient     *LuaClient  // External HTTP client
}
```

**Problems**:
- Static routing maps prevent dynamic route registration
- Lua integration is external HTTP call, not embedded
- No plugin architecture for extending routing logic

### Route Matching Logic Analysis
```go
// Current MatchRoute in internal/routing/gateway.go
func (gw *Gateway) MatchRoute(host, path string) (*TenantRouter, string) {
    // Priority 1: Hybrid routing (host + path)
    // Priority 2: Host-only routing  
    // Priority 3: Path-only routing
    // Fixed priority system, no customization
}
```

**Limitations**:
- Fixed matching priorities
- No way to add custom matching logic
- Lua scripts can't influence route matching, only backend selection

### Middleware Architecture Analysis
```go
// Current middleware approach
func (app *Application) ProxyMiddleware(tr *TenantRouter, stripPrefix string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            backend := tr.NextBackend()  // Simple round-robin
            // ... proxy logic
        })
    }
}
```

**Issues**:
- Middleware is not composable
- No way to inject lua-based routing logic into middleware chain
- Backend selection is hard-coded to round-robin

## Lua-Stone Deep Dive

### Current Lua Script Interface
```lua
-- Current lua script pattern
function on_route_request(request, backends)
    -- Can only select backends, not define routes
    return {
        selected_backend = "backend-name",
        modified_headers = {...},
        modified_path = "...",
        reject = false
    }
end
```

**Limitations**:
- Scripts can only choose from pre-defined backends
- No route definition capabilities
- No integration with Chi router registration
- Execution happens after route matching, not during

### Communication Overhead
- Each routing decision requires HTTP call to lua-stone
- Serialization/deserialization overhead
- Network latency for local decisions
- No caching of lua routing decisions

## Analysis Summary

### Current Strengths
1. Clean separation of concerns (routing vs scripting)
2. Chi router provides solid HTTP routing foundation
3. Configuration-driven tenant setup
4. Health checking and admin endpoints

### Major Architectural Problems
1. **Static Route Registration**: Routes are configured at startup, no dynamic registration
2. **External Lua Integration**: Lua is external service, not embedded in routing flow
3. **Limited Lua Capabilities**: Scripts can only select backends, not define routing patterns
4. **Performance Overhead**: HTTP calls for every routing decision
5. **No Plugin Architecture**: Core changes required for new routing patterns
6. **Tight Coupling**: Gateway directly manages route maps and middleware

### Missing Capabilities for Better Lua Integration
1. **Dynamic Route Registration**: Lua scripts should be able to register custom routes
2. **Embedded Lua Engine**: Lua should be embedded, not external service
3. **Route-Level Lua Hooks**: Lua scripts should execute during route matching
4. **Composable Middleware**: Lua scripts should be able to create custom middleware
5. **Chi Router Extensions**: Custom route matchers and handlers
