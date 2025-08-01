# Configuration

## Basic Structure

```yaml
# Optional admin endpoints (default: /admin)  
admin_base_path: "/admin"

# Required: Lua routing
lua_routing:
  enabled: true
  scripts_dir: "./scripts"

# Required: At least one tenant
tenants:
  - name: "my-api"
    domains: ["api.example.com"]        # OR
    path_prefix: "/api/"                # OR both
    lua_routes: "api-routes"
    health_interval: 30                 # seconds
    services:
      - name: "backend"
        url: "http://backend:3001"
        health: "/health"               # endpoint
```

## Optional: Compression

```yaml
compression:
  enabled: true         # default
  level: 5             # 1-9, default 5
  content_types:       # default list
    - "application/json"
    - "text/html"
```

## Routing Priority

1. **Hybrid**: `domains` + `path_prefix`
2. **Host-based**: `domains` only  
3. **Path-based**: `path_prefix` only

## Validation

- Domains must contain a dot
- Path prefixes must start and end with `/`
- Each tenant needs unique name
- Lua script files must exist

## Examples

See `configs/examples/` for working configurations.