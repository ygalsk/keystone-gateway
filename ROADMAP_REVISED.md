# Keystone Gateway Roadmap: Realistic v1.2.x → v1.3.0
*Balancing Performance, Maintainability, and Simplicity*

## 🎯 **Philosophy Adjustment**
After analyzing the current 314-line `main.go`, we need to address **code maintainability** before aggressive performance optimization. The current `makeHandler()` function (70+ lines) and `main()` function (50+ lines) have mixed concerns that will make future development difficult.

**Key Insight**: Keystone's single-file philosophy is valuable, but internal organization within that file is essential for sustainable development.

---

## 📊 **Current State Analysis**
- **Performance**: 159 req/sec baseline (competitive for lightweight Go proxy)
- **Code Quality**: Monolithic functions with mixed concerns
- **Architecture**: Single-file with no internal structure
- **Technical Debt**: High - refactoring needed before new features

---

## 🗺️ **Revised Release Strategy**

### **v1.2.1 "Foundation" (Week 1-2)** 🏗️
*Priority: Code organization + minimal performance gains*

#### Goals:
- **Performance**: +15-20% improvement (~185 req/sec)
- **Code Quality**: Modular functions within single file
- **Risk**: LOW ⚠️

#### Core Changes:
```go
// Break down makeHandler into focused functions
func makeHandler(routers *RoutingTables) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        tenant := routers.findTenant(r)
        if tenant == nil {
            http.NotFound(w, r)
            return
        }
        tenant.serveRequest(w, r)
    }
}

// New focused functions (still in main.go)
type RoutingTables struct {
    pathRouters   map[string]*tenantRouter
    hostRouters   map[string]*tenantRouter
    hybridRouters map[string]map[string]*tenantRouter
}

func (rt *RoutingTables) findTenant(r *http.Request) *tenantMatch {
    // Extract routing logic from makeHandler
}

func (tm *tenantMatch) serveRequest(w http.ResponseWriter, r *http.Request) {
    // Extract proxy setup and serving logic
}

func initializeRouting(cfg *Config) *RoutingTables {
    // Extract routing table setup from main()
}
```

#### Performance Optimizations:
- Optimized HTTP transport (connection pooling)
- Fast host extraction with `strings.IndexByte`
- Pre-allocated response headers

#### Files Modified:
- `main.go` (refactored into ~8 focused functions)
- `main_test.go` (updated for new structure)

---

### **v1.2.2 "Optimization" (Week 3-4)** ⚡
*Priority: Performance improvements on clean foundation*

#### Goals:
- **Performance**: +25-35% improvement (~220 req/sec)
- **Features**: Basic request/response optimization
- **Risk**: LOW-MEDIUM ⚠️⚠️

#### Changes:
```go
// 1. Smart routing table organization
type OptimizedRouting struct {
    // Pre-sorted for O(1) or O(log n) lookups
    sortedPaths []pathEntry
    hostMap     map[string]*tenantRouter
    hybridIndex map[string][]pathEntry
}

// 2. Connection reuse optimization
type BackendPool struct {
    connections map[string]*http.Client
    mu          sync.RWMutex
}

// 3. Basic response optimization
func optimizeHeaders(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Server", "keystone/1.2.2")
    w.Header().Set("Connection", "keep-alive")
}
```

#### Features:
- Intelligent connection pooling per backend
- Request/response header optimization
- Basic gzip compression for large responses

---

### **v1.2.3 "Stability" (Week 5-6)** 🛡️
*Priority: Production readiness + moderate performance*

#### Goals:
- **Performance**: +35-45% improvement (~240 req/sec)
- **Reliability**: Enhanced error handling and monitoring
- **Risk**: MEDIUM ⚠️⚠️

#### Changes:
```go
// 1. Enhanced health checking
type HealthChecker struct {
    backends    []*backend
    cache       map[string]*healthStatus
    batchTicker *time.Ticker
}

// 2. Request monitoring
type RequestMetrics struct {
    requests     uint64
    errors       uint64
    avgLatency   float64
    lastUpdate   time.Time
}

// 3. Graceful degradation
func (rt *RoutingTables) findTenantWithFallback(r *http.Request) *tenantMatch {
    // Primary routing with fallback strategies
}
```

#### Features:
- Batch health checks to reduce overhead
- Basic request metrics and logging
- Graceful error handling and fallbacks
- Enhanced configuration validation

---

### **v1.3.0-alpha "Advanced Features" (Week 8-10)** 🚀
*Priority: New capabilities while maintaining simplicity*

#### Goals:
- **Performance**: +60-80% improvement (~300+ req/sec)
- **Features**: Advanced routing and middleware
- **Risk**: MEDIUM-HIGH ⚠️⚠️⚠️

#### Major Features:

##### 1. **Enhanced Routing**
```yaml
tenants:
  - name: "advanced-app"
    domains: ["*.example.com"]        # Wildcard support
    path_patterns: ["/api/v*", "/admin"] # Pattern matching
    weight: 10                        # Load balancing weights
```

##### 2. **Simple Middleware Chain**
```go
type MiddlewareFunc func(http.Handler) http.Handler

type MiddlewareChain struct {
    middlewares []MiddlewareFunc
}

// Built-in middlewares (opt-in):
- RequestIDMiddleware
- BasicLoggingMiddleware  
- SimpleRateLimitMiddleware
- HeaderManipulationMiddleware
```

##### 3. **Advanced Health Checks**
```yaml
services:
  - url: "http://backend:8080"
    health:
      path: "/health"
      method: "GET"
      timeout: 5s
      interval: 30s
      retries: 3
```

---

## 🔍 **Technical Debt Strategy**

### **Current Issues Addressed:**

1. **makeHandler() - 70+ lines** → Split into 4 focused functions:
   - `findTenant()` - routing logic
   - `serveRequest()` - proxy setup
   - `setupProxy()` - proxy configuration
   - `handleError()` - error responses

2. **main() - 50+ lines** → Split into 3 functions:
   - `initializeRouting()` - routing table setup
   - `startHealthChecks()` - health check initialization
   - `startServer()` - HTTP server setup

3. **Mixed concerns** → Clear separation:
   - Routing logic
   - Proxy configuration
   - Health checking
   - Server management

### **File Organization (Still Single File)**
```go
// main.go structure
// ==================
// 1. Types and structs (40 lines)
// 2. Configuration loading (30 lines)
// 3. Routing functions (60 lines)
// 4. Proxy functions (40 lines)
// 5. Health check functions (50 lines)
// 6. Server setup functions (30 lines)
// 7. Main function (20 lines)
// Total: ~270 lines (more maintainable)
```

---

## 📋 **Decision Points**

### **Architecture Philosophy**
- ✅ **Maintain single-file deployment**
- ✅ **Internal function organization**
- ✅ **Clear separation of concerns**
- ❌ **Multiple file modules** (breaks philosophy)

### **Performance Targets**
- **v1.2.1**: 185 req/sec (+16% - realistic with optimizations)
- **v1.2.2**: 220 req/sec (+38% - with clean architecture)
- **v1.2.3**: 240 req/sec (+51% - with enhanced features)
- **v1.3.0**: 300+ req/sec (+89% - with advanced features)

### **Feature Complexity**
- **v1.2.x**: Focus on core improvements
- **v1.3.0**: Add features that maintain simplicity
- **v1.4.0+**: Consider advanced features (TLS, auth, etc.)

---

## ⚠️ **Risk Assessment**

| Release | Code Changes | Performance Risk | Feature Risk | Overall |
|---------|--------------|------------------|--------------|---------|
| v1.2.1  | Moderate     | Low             | None         | LOW     |
| v1.2.2  | Moderate     | Low             | Low          | LOW-MED |
| v1.2.3  | Moderate     | Medium          | Medium       | MEDIUM  |
| v1.3.0  | High         | Medium          | High         | MED-HIGH|

---

## 🎯 **Success Metrics**

### **Code Quality**
- Function length: < 30 lines average
- Cyclomatic complexity: < 10 per function
- Test coverage: > 90%

### **Performance**
- Throughput: > 300 req/sec by v1.3.0
- Latency: < 5ms p95 by v1.3.0
- Memory: < 50MB under load

### **Maintainability**
- Clear function boundaries
- Single responsibility principle
- Easy to understand and modify

---

*This revised roadmap prioritizes sustainable development by addressing technical debt first, then building performance and features on a solid foundation.*
