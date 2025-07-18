# Feature Request: Add Host-Based Routing Support

## 🎯 **Problem Statement**

Currently, Keystone Gateway only supports **path-based routing** via the `path_prefix` configuration. This works well for multi-tenant scenarios like `/tenant1/`, `/tenant2/`, but creates challenges for production setups where users expect clean domain-based routing.

**Current Limitation:**
- ✅ `https://example.com/tenant1/` → Works with `path_prefix: "/tenant1/"`
- ❌ `https://tenant1.example.com/` → Not possible (requires ugly `/tenant1/` prefix)

## 🚀 **Feature Request**

Add support for **host-based routing** to complement the existing path-based routing.

### **Proposed Configuration Extension**

```yaml
tenants:
  # Existing path-based routing (keep backward compatibility)
  - name: "legacy-tenant"
    path_prefix: "/legacy/"
    services:
      - name: "webapp"
        url: "http://backend:8080"
        health: "/health"

  # NEW: Host-based routing  
  - name: "modern-tenant"
    domains: ["tenant.example.com", "www.tenant.example.com"]
    services:
      - name: "webapp"
        url: "http://backend:8080"
        health: "/health"
        
  # NEW: Mixed routing (both host AND path)
  - name: "hybrid-tenant"
    domains: ["api.example.com"]
    path_prefix: "/v1/"
    services:
      - name: "api-server"
        url: "http://api-backend:3000"
        health: "/status"
```

## 💼 **Real-World Use Case**

We're migrating from nginx-proxy-manager to Keystone Gateway in a production setup with **21+ services** across multiple domains:

```
salmonis.lca-data.net    → EPDHub Service
blonk.lca-data.net       → Blonk Service  
sphera.lca-data.net      → Sphera Service
ecoinvent.lca-data.net   → Ecoinvent Service
... (18+ more domains)
```

**Current Workaround:**
- Either bypass Keystone Gateway for specific domains (losing health checks)
- Or force users to use ugly URLs like `salmonis.lca-data.net/epdhub-staging/`

## 🔧 **Implementation Suggestions**

### **1. Configuration Structure**
```go
type Tenant struct {
    Name       string   `yaml:"name"`
    Domains    []string `yaml:"domains"`    // NEW: Host-based routing
    PathPrefix string   `yaml:"path_prefix"` // EXISTING: Path-based routing  
    Interval   int      `yaml:"health_interval"`
    Services   []Service `yaml:"services"`
}
```

### **2. Routing Logic Priority**
1. **Host + Path Match** (highest priority)
2. **Host-only Match** 
3. **Path-only Match** (current behavior)
4. **404 Not Found**

### **3. Example Handler Logic**
```go
func makeHandler(routers map[string]*tenantRouter, hostRouters map[string]*tenantRouter) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        host := strings.Split(r.Host, ":")[0] // Remove port
        path := r.URL.Path
        
        // 1. Try host + path combination
        if hostRouter, exists := hostRouters[host]; exists {
            if pathRouter, exists := routers[path]; exists {
                // Both host and path match
                // Use more specific routing logic
            }
        }
        
        // 2. Try host-only routing
        if hostRouter, exists := hostRouters[host]; exists {
            // Route based on host
            backend := hostRouter.nextBackend()
            // ... proxy logic
        }
        
        // 3. Fallback to existing path-based routing
        // ... existing logic
    }
}
```

## ✅ **Benefits**

1. **Production-Ready URLs**: Clean domain-based routing
2. **Backward Compatibility**: Existing configs continue working
3. **Flexibility**: Support mixed routing scenarios
4. **Health Checks**: Keep intelligent health-based load balancing
5. **Multi-Tenant**: Still supports complex tenant isolation

## 🎨 **Alternative Configuration Formats**

### **Option A: Inline Domains**
```yaml
tenants:
  - name: "epdhub"
    domains: ["salmonis.lca-data.net"]
    services: [...]
```

### **Option B: Routing Section**
```yaml
routing:
  host_based:
    "salmonis.lca-data.net": "epdhub"
    "blonk.lca-data.net": "blonk"
  path_based:
    "/legacy/": "legacy-tenant"

tenants:
  - name: "epdhub"
    services: [...]
```

## 🚨 **Migration Considerations**

- **Backward Compatibility**: Existing `path_prefix` configs must continue working
- **Default Behavior**: If no `domains` specified, use existing path-based routing
- **Conflict Resolution**: Clear priority when both host and path routing could match
- **Documentation**: Update examples and migration guide

## 📊 **Impact Assessment**

This feature would make Keystone Gateway a complete replacement for:
- nginx-proxy-manager (host-based routing)
- Traefik (flexible routing rules)  
- HAProxy (mixed routing scenarios)

While maintaining the **simplicity** and **performance** advantages that make Keystone Gateway attractive for SME/agency use cases.

---

**Would love to contribute to the implementation if you're open to this feature! 🙌**

## 🔗 **Related Issues**
- [ ] Add support for wildcard domains (`*.example.com`)
- [ ] Add support for regex-based routing
- [ ] Add support for header-based routing
