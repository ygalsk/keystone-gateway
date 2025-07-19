# Architecture Decisions
*Framework Evolution & Design Philosophy*

## 🎯 **Executive Summary**

Keystone Gateway's architectural evolution follows a clear principle: **"Start simple, scale smartly."** Our analysis reveals that the Chi router provides professional-grade performance and maintainability without compromising our core philosophy of simplicity.

**Key Decision**: Migrate from stdlib to Chi router in v1.2.0, preparing for optional Lua extensibility in v1.3.0.

---

## 🏗️ **Core Architecture Philosophy**

### **Design Principles**
1. **Simple Core**: Essential reverse proxy functionality only
2. **Optional Complexity**: Advanced features via Lua scripting
3. **KMU-First**: Perfect for small businesses and agencies
4. **Zero Breaking Changes**: Seamless upgrades always

### **Two-Layer Architecture Vision**
```
┌─────────────────────────────────────┐
│           Lua Scripts               │  ← Optional
│    (Auth, Rate Limiting, etc.)     │    Community-driven
├─────────────────────────────────────┤    Advanced features
│         Chi Router Core             │  ← Essential
│    (Routing, Middleware, Proxy)    │    Professional performance
└─────────────────────────────────────┘    Zero-config setup
```

---

## 📊 **Framework Analysis & Decision**

### **Current State: stdlib-only (v1.1.x)**
```go
// Manual everything - functional but limited
func makeHandler(...) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        host := extractHost(r.Host)  // Manual parsing
        // 70+ lines of mixed routing logic
        // Manual proxy setup
        // Manual error handling
    }
}
// Result: 159 req/sec, monolithic functions
```

**Limitations**:
- Performance ceiling at ~200 req/sec
- Code complexity in 70+ line functions  
- Manual implementation of standard patterns
- Difficult to add professional features

### **Solution: Chi Router (v1.2.0)**
```go
// Clean, professional, performant
func setupRoutes() chi.Router {
    r := chi.NewRouter()
    
    // Standard middleware
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)
    r.Use(middleware.Timeout(60 * time.Second))
    
    // Clean route definitions
    r.Route("/api/*", func(r chi.Router) {
        r.Use(apiMiddleware)
        r.Handle("/*", proxyHandler("http://localhost:3000"))
    })
    
    return r
}
// Result: 200+ req/sec, focused functions
```

**Benefits**:
- ⚡ **25% Performance Improvement**: Professional routing engine
- 🧹 **Cleaner Code**: Focused, maintainable handlers
- 📈 **Professional Patterns**: Industry-standard middleware support
- 🔄 **Future-Ready**: Prepares for Lua extensibility

### **Future: Lua Extensibility (v1.3.0)**
**Goal**: Optional advanced features without core complexity

```yaml
# Core remains simple
routes:
  - path_prefix: "/api/"
    backend: "http://localhost:3000"
    
# Advanced features optional
lua_scripts:
  - name: "rate_limiting"
    script: "./scripts/rate_limit.lua" 
    routes: ["/api/"]
```

**Philosophy**: 
- Core Keystone: Simple, reliable, zero-config
- Lua Scripts: Advanced features for power users
- Community-driven script repository

---

## ⚖️ **Decision Matrix**

### **Framework Comparison**
| Aspect | stdlib | Chi Router | Gin | Echo |
|--------|--------|------------|-----|------|
| **Simplicity** | ✅ Simple | ✅ Clean | ❌ Too opinionated | ❌ Too opinionated |
| **Performance** | ❌ Limited | ✅ Excellent | ✅ Fast | ✅ Fast |
| **Philosophy Fit** | ✅ Minimal | ✅ Composable | ❌ Full framework | ❌ Full framework |
| **Learning Curve** | ✅ None | ✅ Minimal | ❌ Moderate | ❌ Moderate |
| **Community** | ✅ Standard | ✅ Active | ✅ Large | ✅ Large |
| **Future-Proof** | ❌ Limited | ✅ Extensible | ❌ Locked-in | ❌ Locked-in |

**Winner: Chi Router** - Best balance of performance and simplicity

### **Why Not Gin or Echo?**
- **Too Opinionated**: Force specific patterns and structures
- **Feature Bloat**: Include many features we don't need
- **Philosophy Mismatch**: Framework approach vs. library approach
- **Complexity**: More complex than our KMU users need

### **Why Chi Router?**
- **Composable**: Use only what you need
- **Performance**: Professional routing engine
- **Minimal**: Doesn't force patterns
- **Compatible**: Works with existing stdlib code
- **Future-Ready**: Excellent for Lua integration

---

## 🚀 **Implementation Strategy**

### **Phase 1: Chi Migration (v1.2.0)**
**Timeline**: 2-3 weeks
**Goal**: Drop-in performance improvement

```go
// Migration approach: Gradual replacement
// 1. Create Chi router structure
// 2. Move handlers one by one  
// 3. Maintain exact same behavior
// 4. Verify performance improvements
```

**Success Metrics**:
- ✅ 20%+ performance improvement
- ✅ Zero breaking changes
- ✅ Cleaner codebase
- ✅ Same configuration format

### **Phase 2: Lua Preparation (v1.2.1)**
**Timeline**: 1 week
**Goal**: Prepare architecture for Lua

```go
// Add hooks for future Lua integration
type MiddlewareHook interface {
    OnRequest(*http.Request) (*http.Request, error)
    OnResponse(*http.Response) (*http.Response, error)
}

// Chi middleware that can call Lua scripts
func LuaMiddleware(scriptName string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Future: Execute Lua script here
            next.ServeHTTP(w, r)
        })
    }
}
```

### **Phase 3: Lua Integration (v1.3.0)**  
**Timeline**: 4-6 weeks
**Goal**: Optional advanced features

```lua
-- Example Lua script: Rate limiting
function on_request(req)
    local ip = req.headers["X-Real-IP"] or req.remote_addr
    local count = redis.get("rate_limit:" .. ip) or 0
    
    if count > 100 then
        return {
            status = 429,
            body = "Rate limit exceeded"
        }
    end
    
    redis.incr("rate_limit:" .. ip)
    redis.expire("rate_limit:" .. ip, 60)
    
    return nil  -- Continue to backend
end
```

---

## 📈 **Performance Expectations**

### **Benchmark Targets**
```
Current (v1.1.x):     159 req/sec
Chi Router (v1.2.0):  200+ req/sec  (+25%)
Lua Enabled (v1.3.0): 180+ req/sec  (minimal overhead)
```

### **Memory Usage**
```
Current:     45MB typical usage
Chi Router:  40MB (better allocation patterns)
Lua:         50MB (with script sandboxing)
```

### **Latency Impact**
```
Core Routing:    <0.1ms (Chi router overhead)
Lua Execution:   <1ms per script (target)
Total Overhead:  <2ms for advanced features
```

---

## 🔒 **Security Considerations**

### **Chi Router Security**
- ✅ **Battle-tested**: Used in production by many companies
- ✅ **Minimal Attack Surface**: Library approach, not framework
- ✅ **Standard Patterns**: Uses Go's security best practices

### **Lua Sandboxing (v1.3.0)**
- 🔐 **Isolated Execution**: Each script runs in sandbox
- 🔐 **Resource Limits**: Memory and CPU constraints
- 🔐 **API Restrictions**: Limited access to system functions
- 🔐 **Safe Defaults**: Scripts can't access filesystem or network

```go
// Lua security implementation
type LuaSandbox struct {
    memoryLimit int64  // 10MB per script
    cpuLimit    time.Duration  // 100ms per request
    allowedAPIs []string  // Whitelist of available functions
}
```

---

## 🎯 **Decision Rationale**

### **Why This Approach Works**
1. **Gradual Evolution**: Each step adds value without breaking changes
2. **Performance Focus**: Chi provides immediate performance benefits
3. **Future-Proofing**: Lua extensibility enables unlimited growth
4. **Community Benefits**: Script sharing creates ecosystem
5. **KMU Perfect**: Simple by default, powerful when needed

### **Risk Mitigation**
- **Dependency Risk**: Chi is mature, stable, widely adopted
- **Complexity Risk**: Optional Lua means core stays simple
- **Performance Risk**: Benchmarking ensures no regressions
- **Security Risk**: Lua sandboxing prevents malicious scripts

### **Alternative Approaches Considered**
- **Stay with stdlib**: Limited growth potential
- **Full framework migration**: Would break simplicity philosophy
- **Plugin system in Go**: More complex than Lua scripting
- **Microservices approach**: Overkill for proxy functionality

---

## 📚 **References & Further Reading**

- [Chi Router Documentation](https://github.com/go-chi/chi)
- [Go Performance Best Practices](https://dave.cheney.net/high-performance-go-workshop/dotgo-paris.html)
- [Lua Scripting Security](https://www.lua.org/manual/5.4/manual.html#8)
- [Reverse Proxy Patterns](https://docs.nginx.com/nginx/admin-guide/web-server/reverse-proxy/)

---

## ✅ **Decision Summary**

**Approved Approach**: 
1. **v1.2.0**: Migrate to Chi router for performance and code quality
2. **v1.3.0**: Add optional Lua extensibility for advanced features
3. **Community**: Build script repository for sharing

**Core Philosophy Maintained**:
- Simple configuration for basic use cases
- Professional performance for production use
- Optional complexity for advanced requirements
- Zero breaking changes between versions

*"Keystone stays simple, complexity is optional"* - Our architectural north star
