# Keystone Gateway — V1: Modern Go Implementation Plan (2025)

## Summary

Ship a single Go binary (`keystone-gateway`) that reads human YAML config, routes/multiplexes tenants (host + base_path), proxies to HTTP upstreams with round-robin + active health checks, supports optional per-tenant Lua scripts — **and** exposes a safe Chi-router facade to tenant Lua so scripts can add/delete routes/groups/middleware at runtime (dynamic routing is opt-in & rate limited).

**Built with 2025 Go best practices: atomic operations, context cancellation, structured logging, and production-grade security.**

---

# Core Architecture (Modern Design)

* **Public listener**: RequestID → RealIP → Logger → Recoverer → TenantResolver → Lua pre-hook → Chi Router (atomic snapshot) → HealthyUpstream → ReverseProxy → Lua post-hook → StructuredLog
* **Admin listener**: Protected admin endpoints with JWT/Bearer auth
* **Control plane**: fsnotify watcher → ValidationPipeline → DynamicManager → AtomicSnapshotBuilder → `atomic.Pointer[Snapshot]` swap
* **Lua engine**: SecureLuaPool with context cancellation, sandboxing, bytecode caching

---

# Modern Tech Stack (2025)

## Core Dependencies
```go
// Core routing and middleware
github.com/go-chi/chi/v5
github.com/go-chi/chi/v5/middleware
github.com/go-chi/cors
github.com/go-chi/httprate

// Lua scripting with security
github.com/yuin/gopher-lua

// Configuration and observability  
gopkg.in/yaml.v3
log/slog  // Go 1.21+ structured logging
```

## Modern Go Features Used
- **Atomic Operations**: `atomic.Pointer[T]` for lock-free snapshot swapping
- **Context Cancellation**: Request timeouts, graceful shutdown, Lua script limits
- **Structured Logging**: `slog` with request correlation and tenant context
- **Type Safety**: Generics for atomic operations and better API design
- **Security**: TLS 1.2+, sandbox Lua VMs, rate limiting, CORS

---

# Human-Friendly YAML Configuration

```yaml
version: 1
server:
  public_addr: ":8080"
  admin_addr: ":9000"
  read_header_timeout: "5s"
  idle_timeout: "120s"
  shutdown_timeout: "30s"

tls:
  enabled: true
  cert_file: "/etc/ssl/certs/gateway.crt"
  key_file: "/etc/ssl/private/gateway.key"
  min_version: "1.2"

security:
  cors:
    allowed_origins: ["https://*", "http://localhost:*"]
    allowed_methods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
    allowed_headers: ["Accept", "Authorization", "Content-Type", "X-Request-ID"]
    max_age: 300
  rate_limiting:
    requests_per_minute: 1000
    burst_size: 100
  
dynamic_routing:
  enabled: true
  debounce_ms: 200
  max_changes_per_minute: 60
  lua_timeout: "50ms"
  lua_memory_limit: "10MB"

observability:
  log_level: "info"
  log_format: "json"  # or "text"
  metrics_enabled: true
  tracing_enabled: true

tenants:
  - name: "prod-api"
    domains: ["api.production.com", "api.company.com"]
    base_path: "/"
    lua_script: "production-auth-routes.lua"
    dynamic_routing_enabled: true
    lua_limits:
      max_routes: 200
      max_middlewares: 20
      cpu_timeout: "30ms"
    upstreams:
      - name: "api-prod-1"
        url: "http://10.0.0.21:3001"
        health_path: "/health"
        health_interval: "30s"
        weight: 100
      - name: "api-prod-2"
        url: "http://10.0.0.22:3001"
        health_path: "/health"
        health_interval: "30s"
        weight: 100

  - name: "staging-api"
    domains: ["api.staging.com"]
    base_path: "/"
    lua_script: "staging-routes.lua"
    dynamic_routing_enabled: false
    upstreams:
      - name: "api-stg"
        url: "http://10.0.10.5:3001"
        health_path: "/health"
        health_interval: "30s"
        weight: 100
```

---

# Modern Lua API (Secure & Fast)

**Security-first Lua environment with context cancellation and resource limits.**

## Router Bindings (Safe API)

```lua
-- List all current routes (read-only)
local routes = router:list_routes()
-- Returns: {{id="uuid", method="GET", path="/api/users", source="static|dynamic"}, ...}

-- Add new route with validation
local result = router:add_route("POST", "/api/submit", {
    upstream = "api-prod-1",  -- Route to specific upstream
    proxy = true,             -- Enable proxying (default)
    timeout = "5s",          -- Request timeout override
    headers = {              -- Add/modify headers
        ["X-Source"] = "lua-gateway"
    }
})
-- Returns: {ok=true, id="route-uuid"} or {ok=false, error="path conflicts with existing route"}

-- Remove route by ID
local result = router:delete_route("route-uuid")
-- Returns: {ok=true} or {ok=false, error="route not found"}

-- Create route group with shared prefix and middleware
local group = router:create_group("/admin", {
    middleware = {"auth", "rate_limit"}
})
group:add_route("GET", "/users", {upstream = "user-service"})
group:add_route("POST", "/users", {upstream = "user-service"})

-- Custom Lua response handler (no proxying)
router:add_route("GET", "/lua-status", {
    handler = function(req, res)
        res:header("Content-Type", "application/json")
        res:status(200)
        res:write('{"status":"ok","source":"lua","timestamp":"' .. os.date() .. '"}')
    end
})
```

## Request/Response API

```lua
-- Request object (read-only)
local method = req:method()        -- "GET", "POST", etc.
local path = req:path()            -- "/api/users/123"
local headers = req:headers()      -- {["content-type"] = "application/json", ...}
local body = req:body()            -- Request body as string
local params = req:params()        -- URL parameters {id = "123"}
local query = req:query()          -- Query parameters {limit = "10"}

-- Response object (write-only)
res:status(200)                    -- Set HTTP status code
res:header("X-Custom", "value")    -- Set response header
res:write("Hello from Lua!")       -- Write response body
res:json({message = "success"})    -- JSON response helper
```

## Security & Limits

- **Execution timeout**: 50ms default (configurable per tenant)
- **Memory limit**: 10MB heap per script execution
- **Disabled modules**: `os`, `io`, `package`, `require`, `debug`
- **Rate limiting**: Max changes per minute per tenant
- **Route validation**: Path conflicts, method validation, upstream existence
- **Context cancellation**: Scripts respect request timeouts

---

# Project Structure (Clean Architecture)

```
/cmd/keystone-gateway/
  ├── main.go                     # Entry point with modern server setup
  └── version.go                  # Build info and version

/internal/
  ├── config/
  │   ├── model.go               # YAML config structs with validation
  │   ├── loader.go              # Config loading with fsnotify
  │   └── validation.go          # Config validation rules
  │
  ├── server/
  │   ├── server.go              # HTTP server with graceful shutdown
  │   ├── middleware.go          # Custom middleware stack
  │   └── admin.go               # Admin API handlers
  │
  ├── router/
  │   ├── builder.go             # Chi router construction
  │   ├── snapshot.go            # Atomic snapshot management
  │   ├── dynamic.go             # Dynamic route changes
  │   └── tenant.go              # Tenant resolution logic
  │
  ├── proxy/
  │   ├── director.go            # Request director with load balancing
  │   ├── upstream.go            # Upstream management
  │   └── health.go              # Health checking with context
  │
  ├── lua/
  │   ├── pool.go                # Secure LState pool management
  │   ├── bindings.go            # Router API bindings
  │   ├── sandbox.go             # Security sandbox setup
  │   └── compiler.go            # Bytecode compilation & caching
  │
  ├── observability/
  │   ├── logger.go              # Structured logging setup
  │   ├── metrics.go             # Prometheus metrics
  │   └── tracing.go             # Request tracing
  │
  └── types/
      ├── tenant.go              # Tenant-related types
      ├── health.go              # Health check types
      └── lua.go                 # Lua execution types

/scripts/                        # Example Lua scripts
  ├── hello.lua                  # Basic example
  ├── auth-middleware.lua        # Authentication example
  └── rate-limiting.lua          # Rate limiting example

/config/
  └── config.example.yaml        # Example configuration

/docker/                         # Docker setup for testing
  ├── docker-compose.yaml        # Test environment
  └── backends/                  # Mock upstream services

/docs/                          # Documentation
  ├── lua-api.md                 # Lua API reference
  ├── configuration.md           # Config documentation
  └── deployment.md              # Production deployment guide
```

---

# Modern Go Implementation Patterns

## Atomic Snapshot Management

```go
// internal/router/snapshot.go
package router

import (
    "net/http"
    "sync/atomic"
    "time"
)

type Snapshot struct {
    Router      http.Handler
    Tenants     map[string]*TenantConfig
    Version     string
    BuildTime   time.Time
    LuaRoutes   map[string][]DynamicRoute
}

type SnapshotManager struct {
    current atomic.Pointer[Snapshot]
    metrics *SnapshotMetrics
}

func (sm *SnapshotManager) Load() *Snapshot {
    return sm.current.Load()
}

func (sm *SnapshotManager) Swap(new *Snapshot) *Snapshot {
    old := sm.current.Swap(new)
    sm.metrics.SwapCount.Add(1)
    return old
}

func (sm *SnapshotManager) CompareAndSwap(old, new *Snapshot) bool {
    if sm.current.CompareAndSwap(old, new) {
        sm.metrics.SwapCount.Add(1)
        return true
    }
    return false
}
```

## Context-Aware Middleware Stack

```go
// internal/server/middleware.go
package server

func BuildMiddlewareStack(logger *slog.Logger, cfg *config.Config) []func(http.Handler) http.Handler {
    return []func(http.Handler) http.Handler{
        // 1. Request infrastructure (always first)
        middleware.RequestID,
        middleware.RealIP,
        StructuredLogging(logger),        // Custom with slog
        
        // 2. Error handling
        middleware.Recoverer,
        
        // 3. Security & cleanup
        middleware.CleanPath,
        CORSMiddleware(cfg.Security.CORS),
        middleware.Timeout(cfg.Server.RequestTimeout),
        
        // 4. Content & rate limiting
        middleware.Compress(5, "application/json", "text/html"),
        httprate.LimitByIP(cfg.Security.RateLimit.RequestsPerMinute, time.Minute),
        
        // 5. Application-specific (last)
        TenantResolver(cfg),
        MetricsMiddleware(),
    }
}

func StructuredLogging(logger *slog.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            reqID := middleware.GetReqID(r.Context())
            
            reqLogger := logger.With(
                slog.String("request_id", reqID),
                slog.String("method", r.Method),
                slog.String("path", r.URL.Path),
                slog.String("remote_addr", r.RemoteAddr),
            )
            
            ctx := context.WithValue(r.Context(), "logger", reqLogger)
            ww := &responseWrapper{ResponseWriter: w, statusCode: 200}
            
            next.ServeHTTP(ww, r.WithContext(ctx))
            
            reqLogger.Info("request completed",
                slog.Int("status", ww.statusCode),
                slog.Duration("duration", time.Since(start)),
                slog.String("tenant", GetTenantFromContext(ctx)),
            )
        })
    }
}
```

## Secure Lua Pool Management

```go
// internal/lua/pool.go
package lua

type SecureLuaPool struct {
    pools   map[string]*sync.Pool  // per-tenant pools
    mu      sync.RWMutex
    metrics *PoolMetrics
    config  SecurityConfig
}

type SecurityConfig struct {
    CPUTimeout     time.Duration
    MemoryLimit    int64
    DisallowedLibs []string
    MaxRoutes      int
}

func (p *SecureLuaPool) Get(ctx context.Context, tenantID string) (*lua.LState, error) {
    p.mu.RLock()
    pool, exists := p.pools[tenantID]
    p.mu.RUnlock()
    
    if !exists {
        return nil, fmt.Errorf("tenant pool not found: %s", tenantID)
    }
    
    L := pool.Get().(*lua.LState)
    
    // Set request context for cancellation
    L.SetContext(ctx)
    
    // Apply security sandbox
    p.applySandbox(L)
    
    p.metrics.PoolHits.Add(1)
    return L, nil
}

func (p *SecureLuaPool) Put(tenantID string, L *lua.LState) {
    // Clear context and reset state
    L.SetContext(context.Background())
    p.resetLuaState(L)
    
    p.mu.RLock()
    pool := p.pools[tenantID]
    p.mu.RUnlock()
    
    if pool != nil {
        pool.Put(L)
    }
}

func (p *SecureLuaPool) applySandbox(L *lua.LState) {
    // Remove dangerous globals
    for _, lib := range p.config.DisallowedLibs {
        L.SetGlobal(lib, lua.LNil)
    }
    
    // Set memory and CPU limits
    // Implementation depends on your sandbox strategy
}
```

## Production-Ready Health Checks

```go
// internal/proxy/health.go
package proxy

func (hc *HealthChecker) CheckUpstream(ctx context.Context, upstream *Upstream) error {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    
    req, err := http.NewRequestWithContext(ctx, "GET", upstream.HealthURL(), nil)
    if err != nil {
        return err
    }
    
    resp, err := hc.client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode >= 200 && resp.StatusCode < 300 {
        upstream.MarkHealthy()
        return nil
    }
    
    upstream.MarkUnhealthy()
    return fmt.Errorf("health check failed: status %d", resp.StatusCode)
}

func (hc *HealthChecker) StartPeriodicChecks(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            hc.checkAllUpstreams(ctx)
        }
    }
}
```

---

# 6-Week Development Plan

## Week 1: Foundation & Config (Days 1-7)
- **Day 1-2**: Project setup with Go modules, CI/CD pipeline
- **Day 3-4**: Config loading with YAML validation and fsnotify
- **Day 5-6**: Basic HTTP server with graceful shutdown and middleware stack
- **Day 7**: Static routing with Chi router and basic proxy

**Deliverable**: Server routes static YAML config and proxies requests

## Week 2: Load Balancing & Health (Days 8-14)
- **Day 8-9**: Round-robin load balancer with upstream management
- **Day 10-11**: Health checking with context cancellation and parallel checks
- **Day 12-13**: Integration of health status into load balancing decisions
- **Day 14**: Admin endpoints for health status and metrics

**Deliverable**: Multi-upstream load balancing with automatic failover

## Week 3: Lua Foundation (Days 15-21)
- **Day 15-16**: Secure Lua pool with sandbox and context cancellation
- **Day 17-18**: Basic Lua environment with request/response bindings
- **Day 19-20**: Read-only router bindings (`list_routes`)
- **Day 21**: Lua script compilation and bytecode caching

**Deliverable**: Tenant Lua scripts can inspect routes and return responses

## Week 4: Dynamic Routing (Days 22-28)
- **Day 22-23**: Dynamic route change queue with validation
- **Day 24-25**: Atomic snapshot building and swapping
- **Day 26-27**: Router mutation bindings (`add_route`, `delete_route`)
- **Day 28**: Rate limiting and quota enforcement for dynamic changes

**Deliverable**: Lua scripts can modify routes at runtime with atomic updates

## Week 5: Production Features (Days 29-35)
- **Day 29-30**: Structured logging with correlation IDs and tenant context
- **Day 31-32**: Comprehensive metrics and observability
- **Day 33-34**: Security hardening: TLS, CORS, rate limiting
- **Day 35**: Hot-reload with config validation and error handling

**Deliverable**: Production-ready security and observability

## Week 6: Polish & Documentation (Days 36-42)
- **Day 36-37**: Integration tests with docker-compose environment
- **Day 38-39**: Performance optimization and memory profiling
- **Day 40-41**: Documentation: API reference, deployment guide, examples
- **Day 42**: Final testing, benchmarking, and release preparation

**Deliverable**: Release-ready binary with comprehensive documentation

---

# Acceptance Criteria

## Functional Requirements ✅
- [ ] YAML routes proxy correctly with tenant isolation
- [ ] Health checks mark upstreams and load balancer respects status
- [ ] Lua `router:add_route` adds route; requests to new path work
- [ ] Lua `router:delete_route` removes route; subsequent requests 404
- [ ] Hot reload rebuilds router without dropping connections
- [ ] Admin endpoints show tenant status and dynamic routes

## Security Requirements ✅
- [ ] Lua sandbox prevents file system and network access
- [ ] Dynamic changes respect rate limits and quotas
- [ ] Script execution times out within configured limits
- [ ] TLS configuration uses modern cipher suites
- [ ] CORS policy properly configured for cross-origin requests

## Performance Requirements ✅
- [ ] Atomic snapshot swaps with zero data races (`go test -race`)
- [ ] Request processing under 1ms p95 overhead vs direct proxy
- [ ] Lua pool reuse reduces allocation pressure
- [ ] Health checks don't impact request latency
- [ ] Load testing shows linear scaling with upstreams

## Operational Requirements ✅
- [ ] Graceful shutdown drains connections within timeout
- [ ] Structured logs include request correlation and tenant context
- [ ] Metrics expose key performance and health indicators
- [ ] Configuration validation prevents invalid deployments
- [ ] Error recovery maintains service availability

---

# Quick Start Implementation

Ready to start coding? Begin with this minimal viable setup:

```bash
# 1. Initialize project
mkdir keystone-gateway && cd keystone-gateway
go mod init keystone-gateway
go get github.com/go-chi/chi/v5
go get github.com/go-chi/chi/v5/middleware
go get github.com/yuin/gopher-lua
go get gopkg.in/yaml.v3

# 2. Create basic structure
mkdir -p cmd/keystone-gateway internal/{config,server,router,proxy,lua}
```

```go
// cmd/keystone-gateway/main.go
package main

import (
    "context"
    "log/slog"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"
    
    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
)

func main() {
    // Setup structured logging
    logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
    
    // Build router with middleware
    r := chi.NewRouter()
    r.Use(middleware.RequestID)
    r.Use(middleware.RealIP)
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)
    
    // Basic health endpoint
    r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.Write([]byte(`{"status":"healthy","version":"v1.0.0"}`))
    })
    
    // HTTP server with production defaults
    srv := &http.Server{
        Addr:              ":8080",
        Handler:           r,
        ReadHeaderTimeout: 5 * time.Second,
        IdleTimeout:       120 * time.Second,
    }
    
    // Graceful shutdown
    go func() {
        sigterm := make(chan os.Signal, 1)
        signal.Notify(sigterm, syscall.SIGINT, syscall.SIGTERM)
        <-sigterm
        
        logger.Info("shutting down server")
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        
        srv.Shutdown(ctx)
    }()
    
    logger.Info("starting server", "addr", srv.Addr)
    if err := srv.ListenAndServe(); err != http.ErrServerClosed {
        logger.Error("server failed", "error", err)
        os.Exit(1)
    }
}
```

This foundation gives you a production-ready HTTP server with proper shutdown handling, structured logging, and the chi middleware stack. From here, you can incrementally add configuration loading, proxy functionality, and Lua scripting following the weekly plan.

**Ready to build the future of API gateways! 🚀**