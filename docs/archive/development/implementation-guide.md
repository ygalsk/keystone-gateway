# Implementation Guide
*Building Keystone Gateway Features with Simplicity in Mind*

## 📋 **Table of Contents**
1. [Development Philosophy](#development-philosophy)
2. [Core Features Implementation](#core-features-implementation)
3. [Lua Extensibility Pattern](#lua-extensibility-pattern)
4. [Testing Strategy](#testing-strategy)
5. [Migration Guidelines](#migration-guidelines)
6. [Performance Considerations](#performance-considerations)

---

## 🎯 **Development Philosophy**

**Core Principle**: Keep Keystone simple, make complexity optional through Lua scripting.

### **Feature Development Guidelines**
- **Core Layer**: Essential reverse proxy functionality only
- **Lua Layer**: Advanced features, custom logic, and integrations
- **Zero Breaking Changes**: All features must be backward compatible
- **KMU-First**: Features should solve real problems for small businesses

---

## 🔧 **Core Features Implementation**

### **Current Architecture (v1.1.x)**
```yaml
# Simple configuration that works out of the box
routes:
  - path_prefix: "/api/"
    backend: "http://localhost:3000"
  - path_prefix: "/app/"  
    backend: "http://localhost:3001"
```

### **Next: Chi Router Integration (v1.2.0)**
**Goal**: Professional routing performance without complexity

**Benefits**:
- ⚡ **Performance**: Professional routing engine
- 🧹 **Code Quality**: Cleaner, maintainable handlers  
- 📈 **Scalability**: Better performance under load
- 🔄 **Standards**: Industry-standard middleware patterns

**Implementation Plan**:
```go
// Before (stdlib): Manual routing, 70+ line functions
func makeHandler(...) http.HandlerFunc {
    // Complex manual routing logic
}

// After (Chi): Clean, focused handlers
func setupRoutes() chi.Router {
    r := chi.NewRouter()
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)
    
    // Clean route definitions
    r.Route("/api/*", apiHandler)
    r.Route("/app/*", appHandler)
    
    return r
}
```

### **Future: Lua Extensibility (v1.3.0)**
**Goal**: Optional advanced features without core complexity

```yaml
# Core remains simple
routes:
  - path_prefix: "/api/"
    backend: "http://localhost:3000"
    
# Advanced features via Lua (optional)
lua_scripts:
  - name: "rate_limiting"
    script: "./scripts/rate_limit.lua"
    routes: ["/api/"]
  - name: "auth_middleware"  
    script: "./scripts/jwt_auth.lua"
    routes: ["/admin/"]
```

---

## 🌟 **Lua Extensibility Pattern**

### **Design Principles**
1. **Optional**: Core works without Lua
2. **Modular**: Each script handles one concern
3. **Community-Driven**: Users contribute scripts
4. **Safe**: Sandboxed execution environment

### **Lua Script Structure**
```lua
-- Example: Rate limiting middleware
function on_request(req)
    local client_ip = req.headers["X-Real-IP"] or req.remote_addr
    local current_requests = get_request_count(client_ip)
    
    if current_requests > 100 then
        return {
            status = 429,
            body = "Rate limit exceeded",
            headers = {["Retry-After"] = "60"}
        }
    end
    
    increment_request_count(client_ip)
    return nil  -- Continue to backend
end

function on_response(req, resp)
    -- Optional response modification
    resp.headers["X-Rate-Limit-Remaining"] = tostring(100 - get_request_count(req.remote_addr))
    return resp
end
```

### **Community Script Repository**
```
scripts/
├── auth/
│   ├── jwt_validation.lua
│   ├── basic_auth.lua
│   └── oauth2_proxy.lua
├── security/
│   ├── rate_limiting.lua
│   ├── ip_whitelist.lua
│   └── cors_handler.lua
├── monitoring/
│   ├── prometheus_metrics.lua
│   ├── health_checks.lua
│   └── logging_enhanced.lua
└── transformation/
    ├── request_rewriter.lua
    ├── response_transformer.lua
    └── header_injection.lua
```

### **Lua Integration Architecture**
```go
// Lua engine integration (v1.3.0)
type LuaEngine struct {
    scripts map[string]*lua.LState
    sandbox *lua.LState
}

func (e *LuaEngine) ExecuteScript(scriptName string, req *http.Request) (*LuaResponse, error) {
    // Sandboxed execution
    // Performance monitoring
    // Error handling with fallback
}
```

---

## 🧪 **Testing Strategy**

### **Core Feature Testing**
```go
// Unit tests for each component
func TestChiRouterIntegration(t *testing.T) {
    // Test routing performance
    // Verify backward compatibility
    // Check middleware integration
}

func TestConfigurationValidation(t *testing.T) {
    // Test YAML parsing
    // Validate error handling
    // Check default values
}
```

### **Lua Script Testing**
```go
func TestLuaScriptExecution(t *testing.T) {
    // Script syntax validation
    // Sandbox security testing
    // Performance impact measurement
    // Error handling verification
}
```

### **Integration Testing**
```bash
# Performance regression testing
go test -bench=. ./...

# Load testing with real backends
wrk -t12 -c400 -d30s --script=test.lua http://localhost:8080/

# Compatibility testing
./test_configs.sh  # Test all example configurations
```

---

## 🔄 **Migration Guidelines**

### **From v1.1.x to v1.2.0 (Chi Router)**
**Zero Configuration Changes Required**

1. **Automatic Benefits**:
   - Improved performance (20-30% faster)
   - Better error handling
   - Professional middleware support

2. **Monitoring Migration**:
   - Response times should improve
   - Memory usage may slightly decrease
   - No behavioral changes expected

3. **Verification Steps**:
   ```bash
   # Before upgrade
   curl -w "%{time_total}" http://localhost:8080/api/health
   
   # After upgrade - should be faster
   curl -w "%{time_total}" http://localhost:8080/api/health
   ```

### **From v1.2.x to v1.3.0 (Lua Extensibility)**
**Core Functionality Unchanged**

1. **Optional Lua Features**:
   ```yaml
   # Add to existing config only if needed
   lua_scripts:
     - name: "basic_auth"
       script: "./scripts/auth.lua"
       routes: ["/admin/"]
   ```

2. **Community Scripts**:
   ```bash
   # Browse available scripts
   keystone scripts list
   
   # Install community script
   keystone scripts install rate_limiting
   
   # Test script in development
   keystone scripts test rate_limiting.lua
   ```

3. **Gradual Adoption**:
   - Start with simple monitoring scripts
   - Add authentication for admin routes
   - Expand to custom business logic as needed

---

## ⚡ **Performance Considerations**

### **Core Performance Targets**
- **Baseline (v1.1.x)**: 150+ req/sec
- **Chi Router (v1.2.0)**: 200+ req/sec (30% improvement)
- **Lua Enabled (v1.3.0)**: 180+ req/sec (minimal overhead)
- **Memory Usage**: < 50MB for typical KMU workloads

### **Chi Router Benefits**
```
Performance Comparison:
├── stdlib (v1.1.x):  159 req/sec
├── Chi (v1.2.0):     200+ req/sec  (+25%)
└── Memory:           Reduced allocation overhead
```

### **Lua Performance Guidelines**
- **Script Overhead**: Target < 1ms per request
- **Memory Isolation**: 10MB max per script sandbox
- **Compilation**: Lua scripts compiled once, cached
- **Fallback**: Core continues functioning if Lua fails

### **Monitoring Integration**
```yaml
# Built-in performance metrics
monitoring:
  enabled: true
  endpoint: "/metrics"
  include_lua: true  # v1.3.0+
  
# Example metrics output
keystone_requests_total{route="/api/", lua_script="auth"} 1234
keystone_request_duration_seconds{route="/api/"} 0.045
keystone_lua_execution_duration_seconds{script="auth"} 0.001
```

---

## 🔨 **Development Workflow**

### **Feature Development Process**
1. **Core First**: Implement essential functionality
2. **Simple Configuration**: YAML-based, minimal required fields
3. **Backward Compatibility**: All existing configs must work
4. **Performance Testing**: Benchmark against previous version
5. **Documentation**: Update guides and examples

### **Lua Script Development**
1. **Local Testing**: Use built-in Lua REPL
2. **Sandbox Validation**: Verify security constraints
3. **Performance Profiling**: Measure execution overhead
4. **Community Review**: Submit to script repository
5. **Integration Testing**: Test with real Keystone instances

### **Code Organization**
```
internal/
├── core/           # Essential reverse proxy logic
├── router/         # Chi router integration (v1.2.0)
├── lua/           # Lua engine and sandboxing (v1.3.0)
├── config/        # Configuration parsing and validation
├── monitoring/    # Metrics and health checks
└── middleware/    # Reusable middleware components
```

---

## 📚 **Further Reading**

- [Chi Router Migration Plan](chi-migration.md) - Detailed Chi integration steps
- [Version Evolution Matrix](version-matrix.md) - Feature timeline and compatibility
- [Architecture Decisions](../architecture/decisions.md) - Framework choice rationale
- [Performance Benchmarks](../architecture/performance.md) - Detailed performance analysis
- [Lua Scripting Guide](lua-scripting.md) - Complete Lua development guide *(coming in v1.3.0)*

---

## 🎯 **Key Takeaways**

1. **Simplicity First**: Core Keystone remains simple and reliable
2. **Performance Focused**: Chi router provides professional-grade performance  
3. **Extensibility Optional**: Lua scripting enables advanced features without complexity
4. **Community Driven**: Script repository enables sharing and collaboration
5. **KMU Perfect**: Scales from simple setups to advanced enterprise needs

*"The best proxy is the one you don't have to think about"* - Keystone Philosophy