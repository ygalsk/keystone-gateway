# Chi Router Migration Plan

**Evolving Keystone Gateway's core while preserving simplicity**

## 🎯 **Migration Philosophy**

This migration aligns with Keystone Gateway's core principles:
- **🎯 Simplicity First**: Chi is lightweight and stdlib-compatible
- **⚡ Performance Focus**: Target 300+ req/sec (vs current 159)
- **🔧 Maintainability**: Professional patterns without complexity
- **🏢 KMU-Optimized**: Zero breaking changes for existing users
- **📦 Self-Contained**: Still single binary deployment

## 🚀 **Why Chi Router**

### **Perfect Philosophy Match**
- **Lightweight**: ~1000 LOC, similar to Keystone's philosophy
- **Compatible**: 100% net/http compatible (no breaking changes)
- **Professional**: Industry-standard patterns
- **Performant**: 300+ req/sec achievable
- **Simple**: Minimal learning curve

### **Real-World Benefits for KMUs**
```yaml
# Your existing config.yaml works unchanged
tenants:
  - name: "client-website"
    domains: ["example.com"]
    services:
      - name: "wordpress" 
        url: "http://wordpress:80"
        health: "/health"
```

```go
// Chi makes this routing faster and cleaner
r.Group(func(r chi.Router) {
    r.Use(HostMiddleware("example.com"))
    r.HandleFunc("/*", proxyHandler) // 2x faster routing
})
```
## 📋 **Implementation Timeline**

### **Week 1: Foundation (Days 1-7)**
```bash
# Day 1: Add Chi dependency
go get github.com/go-chi/chi/v5
go mod tidy
```

```go
// Day 2-3: Basic Chi integration
func main() {
    r := chi.NewRouter()
    
    // Essential middleware only
    r.Use(middleware.Recoverer)
    r.Use(middleware.RealIP) 
    
    setupTenantRouting(r, config)
    http.ListenAndServe(":8080", r)
}
```

```go
// Day 4-5: Tenant routing conversion
func setupTenantRouting(r *chi.Mux, cfg *Config) {
    for _, tenant := range cfg.Tenants {
        if len(tenant.Domains) > 0 {
            // Host-based routing
            r.Group(func(r chi.Router) {
                r.Use(HostMiddleware(tenant.Domains))
                r.HandleFunc("/*", ProxyHandler(tenant))
            })
        } else {
            // Path-based routing (backward compatibility)
            r.Route(tenant.PathPrefix, func(r chi.Router) {
                r.HandleFunc("/*", ProxyHandler(tenant))
            })
        }
    }
}
```

### **Week 2: Optimization (Days 8-14)**
```go
// Performance middleware
func OptimizedTransport() func(http.Handler) http.Handler {
    transport := &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     90 * time.Second,
    }
    
    return func(next http.Handler) http.Handler {
        // Attach optimized transport
        return next
    }
}
```

## 🎯 **Success Metrics**

| Metric | Before | Target | Benefit |
|--------|--------|--------|---------|
| **Req/sec** | 159 | 300+ | +89% performance |
| **Latency** | 6.3ms | <4ms | Better user experience |
| **Code Lines** | 314 | ~300 | Cleaner architecture |
| **Dependencies** | 1 | 2 | Still minimal |

## ✅ **Migration Checklist**

### **Phase 1: Basic Integration**
- [ ] Add Chi dependency
- [ ] Convert basic routing
- [ ] Test backward compatibility
- [ ] Maintain all existing features

### **Phase 2: Enhancement**  
- [ ] Add performance middleware
- [ ] Optimize proxy configuration
- [ ] Benchmark performance gains
- [ ] Update documentation

## 🚀 **Post-Migration Benefits**

### **For KMUs**
- ✅ **Same simplicity**: Config unchanged, deployment unchanged
- ✅ **Better performance**: 2x faster request handling
- ✅ **More reliable**: Professional error handling

### **For DevOps Teams**
- ✅ **Better architecture**: Foundation for future Lua scripting
- ✅ **Easier maintenance**: Cleaner, more testable code
- ✅ **Industry standard**: Chi is widely used and supported

### **For Future Development**
- ✅ **Middleware ready**: Easy to add features like metrics, compression
- ✅ **Lua hooks**: Perfect foundation for Lua scripting layer
- ✅ **Scalable**: Architecture supports advanced features

---

*Chi Router: The perfect evolution step that respects Keystone's simplicity while unlocking professional capabilities.*
