# Configuration

## Basic Structure

```yaml
# Optional admin endpoints (default: /admin)
admin_base_path: "/admin"

# Optional server configuration
server:
  port: "8080"         # default: 8080

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

## Optional: Request Limits

```yaml
request_limits:
  max_body_size: 10485760     # 10MB (default)
  max_header_size: 1048576    # 1MB (default)
  max_url_size: 8192          # 8KB (default)
```

## Optional: Compression

```yaml
compression:
  enabled: true         # default
  level: 5             # 1-9, default 5
  content_types:       # default list
    - "application/json"
    - "text/html"
    - "text/css"
    - "text/javascript"
    - "application/xml"
    - "text/plain"
```

## Routing Priority

1. **Hybrid**: `domains` + `path_prefix`
2. **Host-based**: `domains` only
3. **Path-based**: `path_prefix` only

## Configuration Details

### Request Limits
- **max_body_size**: Maximum request body size in bytes (protects against large uploads)
- **max_header_size**: Maximum total header size in bytes (protects against header bombs)
- **max_url_size**: Maximum URL length in bytes (protects against long URL attacks)

These limits are enforced at the Lua level when reading request bodies via `request_body()`.

### Server Configuration
- **port**: HTTP server port (can also be set via command line `-addr` flag)

### Compression
Compression is enabled by default and applies to responses matching the configured content types.
- **level**: 1 (fastest) to 9 (best compression), 5 is balanced default
- **content_types**: MIME types that should be compressed

## Validation

- Domains must contain a dot
- Path prefixes must start and end with `/`
- Each tenant needs unique name
- Lua script files must exist
- Request limit values must be positive integers
- Server port must be valid (1-65535)

## Examples

See `configs/examples/` for working configurations.
