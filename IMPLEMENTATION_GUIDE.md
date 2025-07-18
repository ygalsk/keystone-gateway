# Host-Based Routing Implementation Guide

## 📋 **Table of Contents**
1. [Feature Overview](#feature-overview)
2. [Semantic Versioning Plan](#semantic-versioning-plan)
3. [Implementation Phases](#implementation-phases)
4. [Code Changes Breakdown](#code-changes-breakdown)
5. [Testing Strategy](#testing-strategy)
6. [Migration Path](#migration-path)
7. [Documentation Updates](#documentation-updates)

---

## 🎯 **Feature Overview**

**Goal**: Add host-based routing to complement existing path-based routing while maintaining 100% backward compatibility.

**Current State**: Only `path_prefix` routing (v1.1.0)
**Target State**: Both `domains` and `path_prefix` routing with flexible combinations

---

## 📈 **Semantic Versioning Plan**

### **Phase 1: v1.2.0 (Minor Release) - Core Host Routing**
- ✅ Add basic host-based routing support
- ✅ Maintain backward compatibility
- ✅ Update configuration schema
- ✅ Basic documentation

### **Phase 2: v1.2.1 (Patch Release) - Bug Fixes**
- 🐛 Fix edge cases discovered in production
- 🐛 Performance optimizations
- 📚 Documentation improvements

### **Phase 3: v1.3.0 (Minor Release) - Advanced Features**
- ✨ Wildcard domain support (`*.example.com`)
- ✨ Regex-based routing patterns
- ✨ Header-based routing conditions

### **Phase 4: v2.0.0 (Major Release) - Breaking Changes**
- 🚨 Only if fundamental architecture changes are needed
- 🚨 Configuration format changes (if required)

---

## 🏗️ **Implementation Phases**

### **Phase 1.1: Configuration Schema Extension (Week 1)**

#### 1.1.1 Update Configuration Structs
```go
// filepath: internal/config/config.go
type Tenant struct {
    Name         string    `yaml:"name"`
    Domains      []string  `yaml:"domains,omitempty"`      // NEW: Host-based routing
    PathPrefix   string    `yaml:"path_prefix,omitempty"`  // EXISTING: Path-based routing
    HealthInterval int     `yaml:"health_interval,omitempty"`
    Services     []Service `yaml:"services"`
}
```

#### 1.1.2 Configuration Validation
```go
// filepath: internal/config/validation.go
func (t *Tenant) Validate() error {
    // Must have either domains OR path_prefix (or both)
    if len(t.Domains) == 0 && t.PathPrefix == "" {
        return fmt.Errorf("tenant '%s' must specify either domains or path_prefix", t.Name)
    }
    
    // Validate domain formats
    for _, domain := range t.Domains {
        if !isValidDomain(domain) {
            return fmt.Errorf("invalid domain format: %s", domain)
        }
    }
    
    // Validate path_prefix format (existing logic)
    if t.PathPrefix != "" {
        if !strings.HasPrefix(t.PathPrefix, "/") || !strings.HasSuffix(t.PathPrefix, "/") {
            return fmt.Errorf("path_prefix must start and end with '/'")
        }
    }
    
    return nil
}
```

### **Phase 1.2: Routing Engine Redesign (Week 2)**

#### 1.2.1 Create Router Interface
```go
// filepath: internal/router/interface.go
type Router interface {
    Match(r *http.Request) (*Tenant, bool)
    GetBackend(tenant *Tenant) (*Backend, error)
}

type HostRouter struct {
    hostMap map[string]*Tenant
}

type PathRouter struct {
    pathMap map[string]*Tenant
}

type HybridRouter struct {
    hostRouters map[string]*PathRouter
    globalPathRouter *PathRouter
    globalHostRouter *HostRouter
}
```

#### 1.2.2 Implement Routing Logic
```go
// filepath: internal/router/hybrid.go
func (hr *HybridRouter) Match(r *http.Request) (*Tenant, bool) {
    host := extractHost(r.Host)
    path := r.URL.Path
    
    // Priority 1: Host + Path combination
    if hostPathRouter, exists := hr.hostRouters[host]; exists {
        if tenant, matched := hostPathRouter.Match(r); matched {
            return tenant, true
        }
    }
    
    // Priority 2: Host-only routing
    if tenant, matched := hr.globalHostRouter.Match(r); matched {
        return tenant, true
    }
    
    // Priority 3: Path-only routing (backward compatibility)
    if tenant, matched := hr.globalPathRouter.Match(r); matched {
        return tenant, true
    }
    
    return nil, false
}

func extractHost(hostHeader string) string {
    // Remove port if present
    if colonIndex := strings.Index(hostHeader, ":"); colonIndex != -1 {
        return hostHeader[:colonIndex]
    }
    return hostHeader
}
```

### **Phase 1.3: Handler Integration (Week 2)**

#### 1.3.1 Update Main Handler
```go
// filepath: internal/handler/handler.go
func NewHandler(config *Config) *Handler {
    return &Handler{
        router: router.NewHybridRouter(config.Tenants),
        backends: initializeBackends(config.Tenants),
    }
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    tenant, matched := h.router.Match(r)
    if !matched {
        http.NotFound(w, r)
        return
    }
    
    backend, err := h.router.GetBackend(tenant)
    if err != nil {
        http.Error(w, "No healthy backends", http.StatusBadGateway)
        return
    }
    
    // Existing proxy logic
    h.proxyRequest(w, r, backend, tenant)
}
```

### **Phase 1.4: Health Check Updates (Week 3)**

#### 1.4.1 Update Health Check Manager
```go
// filepath: internal/health/manager.go
func (hm *HealthManager) Start() {
    for _, tenant := range hm.tenants {
        // Create health checkers for both domain and path-based tenants
        for _, service := range tenant.Services {
            checker := &HealthChecker{
                tenant:   tenant,
                service:  service,
                interval: time.Duration(tenant.HealthInterval) * time.Second,
            }
            
            go checker.Start()
        }
    }
}
```

### **Phase 1.5: Testing Implementation (Week 3-4)**

#### 1.5.1 Unit Tests
```go
// filepath: internal/router/hybrid_test.go
func TestHybridRouter_HostRouting(t *testing.T) {
    tenants := []config.Tenant{
        {
            Name:    "host-tenant",
            Domains: []string{"example.com", "www.example.com"},
            Services: []config.Service{{Name: "web", URL: "http://backend:8080"}},
        },
    }
    
    router := NewHybridRouter(tenants)
    
    req := httptest.NewRequest("GET", "http://example.com/", nil)
    tenant, matched := router.Match(req)
    
    assert.True(t, matched)
    assert.Equal(t, "host-tenant", tenant.Name)
}

func TestHybridRouter_PathRouting_BackwardCompatibility(t *testing.T) {
    tenants := []config.Tenant{
        {
            Name:       "path-tenant",
            PathPrefix: "/api/",
            Services:   []config.Service{{Name: "api", URL: "http://api:3000"}},
        },
    }
    
    router := NewHybridRouter(tenants)
    
    req := httptest.NewRequest("GET", "http://any.domain.com/api/users", nil)
    tenant, matched := router.Match(req)
    
    assert.True(t, matched)
    assert.Equal(t, "path-tenant", tenant.Name)
}

func TestHybridRouter_HybridRouting(t *testing.T) {
    tenants := []config.Tenant{
        {
            Name:       "hybrid-tenant",
            Domains:    []string{"api.example.com"},
            PathPrefix: "/v1/",
            Services:   []config.Service{{Name: "api", URL: "http://api:3000"}},
        },
    }
    
    router := NewHybridRouter(tenants)
    
    // Should match
    req1 := httptest.NewRequest("GET", "http://api.example.com/v1/users", nil)
    tenant1, matched1 := router.Match(req1)
    assert.True(t, matched1)
    assert.Equal(t, "hybrid-tenant", tenant1.Name)
    
    // Should NOT match (wrong domain)
    req2 := httptest.NewRequest("GET", "http://other.com/v1/users", nil)
    _, matched2 := router.Match(req2)
    assert.False(t, matched2)
    
    // Should NOT match (wrong path)
    req3 := httptest.NewRequest("GET", "http://api.example.com/v2/users", nil)
    _, matched3 := router.Match(req3)
    assert.False(t, matched3)
}
```

#### 1.5.2 Integration Tests
```go
// filepath: tests/integration/host_routing_test.go
func TestHostRoutingIntegration(t *testing.T) {
    // Start test backend servers
    backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(200)
        w.Write([]byte("backend1"))
    }))
    defer backend1.Close()
    
    // Create test configuration
    config := &config.Config{
        Tenants: []config.Tenant{
            {
                Name:    "test-tenant",
                Domains: []string{"test.example.com"},
                Services: []config.Service{
                    {Name: "web", URL: backend1.URL, Health: "/"},
                },
            },
        },
    }
    
    // Start Keystone Gateway
    gateway := startGateway(config)
    defer gateway.Close()
    
    // Test request
    client := &http.Client{}
    req, _ := http.NewRequest("GET", gateway.URL+"/", nil)
    req.Host = "test.example.com"
    
    resp, err := client.Do(req)
    assert.NoError(t, err)
    assert.Equal(t, 200, resp.StatusCode)
    
    body, _ := ioutil.ReadAll(resp.Body)
    assert.Equal(t, "backend1", string(body))
}
```

---

## 🔧 **Code Changes Breakdown**

### **Files to Modify**

1. **Configuration Package**
   - `internal/config/config.go` - Add `Domains` field to `Tenant` struct
   - `internal/config/validation.go` - Add domain validation logic

2. **Router Package** (New)
   - `internal/router/interface.go` - Define router interfaces
   - `internal/router/host.go` - Host-based routing implementation
   - `internal/router/path.go` - Path-based routing implementation
   - `internal/router/hybrid.go` - Combined routing logic

3. **Handler Package**
   - `internal/handler/handler.go` - Update to use new router interface
   - `internal/handler/proxy.go` - Ensure proper request forwarding

4. **Health Check Package**
   - `internal/health/manager.go` - Ensure compatibility with new routing

5. **Main Application**
   - `main.go` - Update initialization to use new router

### **New Dependencies**
- No external dependencies required
- All changes use Go standard library

---

## 🧪 **Testing Strategy**

### **Test Categories**

1. **Unit Tests** (95% coverage target)
   - Configuration validation
   - Router matching logic
   - Backward compatibility

2. **Integration Tests**
   - End-to-end request routing
   - Health check integration
   - Multi-tenant scenarios

3. **Performance Tests**
   - Routing latency benchmarks
   - Memory usage analysis
   - Concurrent request handling

4. **Backward Compatibility Tests**
   - Existing configurations continue working
   - No breaking changes in API

### **Test Data Sets**

```yaml
# Test configurations for different scenarios
test_configs:
  backward_compatibility:
    tenants:
      - name: "legacy"
        path_prefix: "/api/"
        services: [...]
        
  host_only:
    tenants:
      - name: "modern"
        domains: ["app.example.com"]
        services: [...]
        
  hybrid:
    tenants:
      - name: "complex"
        domains: ["api.example.com"]
        path_prefix: "/v1/"
        services: [...]
        
  mixed_environment:
    tenants:
      - name: "legacy-api"
        path_prefix: "/legacy/"
        services: [...]
      - name: "new-app"
        domains: ["app.example.com"]
        services: [...]
      - name: "versioned-api"
        domains: ["api.example.com"]
        path_prefix: "/v2/"
        services: [...]
```

---

## 🔄 **Migration Path**

### **For Existing Users (v1.1.0 → v1.2.0)**

1. **No Action Required**
   - Existing configurations work unchanged
   - `path_prefix` behavior remains identical

2. **Optional: Migrate to Host-Based Routing**
   ```yaml
   # Before (v1.1.0)
   tenants:
     - name: "my-app"
       path_prefix: "/app/"
       services: [...]
   
   # After (v1.2.0) - Optional migration
   tenants:
     - name: "my-app"
       domains: ["my-app.example.com"]
       services: [...]
   ```

3. **Gradual Migration Strategy**
   ```yaml
   # Step 1: Add both (hybrid approach)
   tenants:
     - name: "my-app"
       domains: ["my-app.example.com"]     # New clean URLs
       path_prefix: "/app/"                # Keep legacy URLs working
       services: [...]
   
   # Step 2: Eventually remove path_prefix when ready
   tenants:
     - name: "my-app"
       domains: ["my-app.example.com"]
       services: [...]
   ```

### **Migration Tools** (Future v1.2.1)

```bash
# Configuration migration helper
keystone-gateway migrate-config --from v1.1 --to v1.2 config.yaml

# Validation tool
keystone-gateway validate-config config.yaml --version v1.2
```

---

## 📚 **Documentation Updates**

### **Files to Update**

1. **README.md**
   - Add host-based routing examples
   - Update feature list
   - Add migration guide

2. **Configuration Documentation**
   - Document new `domains` field
   - Provide configuration examples
   - Explain routing priority

3. **Examples**
   - Create host-based routing examples
   - Update docker-compose examples
   - Add real-world use cases

### **Documentation Structure**

```markdown
## Configuration Reference

### Tenant Configuration

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Unique tenant identifier |
| `domains` | []string | No* | List of domains for host-based routing |
| `path_prefix` | string | No* | Path prefix for path-based routing |
| `health_interval` | int | No | Health check interval in seconds |
| `services` | []Service | Yes | Backend services |

*Note: Either `domains` or `path_prefix` (or both) must be specified.

### Routing Priority

1. **Host + Path Match** (highest priority)
2. **Host-only Match**
3. **Path-only Match** (backward compatibility)
4. **404 Not Found**

### Example Configurations

#### Host-Based Routing
```yaml
tenants:
  - name: "production-app"
    domains: ["app.example.com", "www.app.example.com"]
    health_interval: 30
    services:
      - name: "web-server"
        url: "http://web:8080"
        health: "/health"
```

#### Mixed Environment
```yaml
tenants:
  - name: "legacy-api"
    path_prefix: "/api/v1/"
    services: [...]
    
  - name: "new-app"
    domains: ["app.example.com"]
    services: [...]
    
  - name: "versioned-api"
    domains: ["api.example.com"]
    path_prefix: "/v2/"
    services: [...]
```
```

---

## 📋 **Implementation Checklist**

### **Phase 1.1: Configuration Schema** ✅
- [ ] Add `Domains` field to `Tenant` struct
- [ ] Implement domain validation
- [ ] Add configuration parsing tests
- [ ] Update schema documentation

### **Phase 1.2: Routing Engine** 🔄
- [ ] Create router interfaces
- [ ] Implement host-based router
- [ ] Implement hybrid router with priority logic
- [ ] Add comprehensive unit tests

### **Phase 1.3: Handler Integration** ⏳
- [ ] Update main handler to use new router
- [ ] Ensure proper request forwarding
- [ ] Test with real HTTP requests
- [ ] Performance benchmarking

### **Phase 1.4: Health Checks** ⏳
- [ ] Verify health check compatibility
- [ ] Test with mixed routing scenarios
- [ ] Update health endpoints if needed

### **Phase 1.5: Testing & Documentation** ⏳
- [ ] Complete integration test suite
- [ ] Backward compatibility verification
- [ ] Update all documentation
- [ ] Create migration examples

### **Release Preparation** ⏳
- [ ] Version tagging (v1.2.0)
- [ ] Release notes
- [ ] Docker image build
- [ ] GitHub release

---

## 🎯 **Success Criteria**

1. **Functionality**
   - ✅ Host-based routing works correctly
   - ✅ Path-based routing remains unchanged
   - ✅ Hybrid routing (host + path) works
   - ✅ Health checks work with all routing types

2. **Performance**
   - ✅ No significant latency increase (<5ms)
   - ✅ Memory usage increase <10%
   - ✅ Handles concurrent requests efficiently

3. **Compatibility**
   - ✅ 100% backward compatibility
   - ✅ Existing configurations work unchanged
   - ✅ No breaking API changes

4. **Quality**
   - ✅ >95% test coverage
   - ✅ No critical security issues
   - ✅ Complete documentation

---

**Timeline: 4 weeks for v1.2.0 implementation**
**Contributors Welcome: Looking for community input and testing! 🙌**