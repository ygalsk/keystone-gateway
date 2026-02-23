---
title: Configuration
nav_order: 3
---

# Configuration Guide

Complete YAML configuration reference.

## Structure

```yaml
# Global middleware settings
middleware:
  request_id: true/false
  real_ip: true/false
  logging: true/false
  recovery: true/false
  timeout: 10              # seconds
  throttle: 100            # max concurrent requests

# Compression settings
compression:
  enabled: true/false
  level: 5                 # 1-9 (default: 5)

# Request limits
request_limits:
  max_body_size: 10485760  # bytes (default: 10MB)

# Lua engine configuration
lua_routing:
  enabled: true/false
  scripts_dir: "./path/to/scripts"
  state_pool_size: 10      # number of Lua VMs (default: 10)
  global_scripts:          # loaded at startup
    - "init"
    - "handlers"
  module_paths:            # LuaRocks Lua modules
    - "/usr/local/share/lua/5.1/?.lua"
  module_cpaths:           # LuaRocks C modules
    - "/usr/local/lib/lua/5.1/?.so"

# Multi-tenant configuration
tenants:
  - name: "api"
    path_prefix: "/api"    # optional, defaults to "/*"

    # Explicit routes
    routes:
      - method: "GET"      # HTTP method
        pattern: "/hello"  # Chi pattern (supports {id})
        handler: "hello_handler"  # Lua function name

      - method: "POST"
        pattern: "/users"
        handler: "create_user"
        middleware:        # Lua middleware functions
          - "require_auth"

      - method: "GET"
        pattern: "/legacy/*"
        backend: "legacy-service"  # proxy instead of Lua

    # Route groups (shared middleware)
    route_groups:
      - pattern: "/articles"
        middleware:
          - "require_auth"
        routes:
          - method: "GET"
            pattern: "/"
            handler: "list_articles"
          - method: "POST"
            pattern: "/"
            handler: "create_article"

    # Backend services for proxying
    services:
      - name: "legacy-service"
        url: "http://localhost:3000"

    # Custom error handlers
    error_handlers:
      not_found: "handle_404"
      method_not_allowed: "handle_405"
```

## Middleware Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `request_id` | bool | `true` | Generate X-Request-ID header |
| `real_ip` | bool | `true` | Parse real IP from X-Forwarded-For |
| `logging` | bool | `true` | Log requests (disable for performance) |
| `recovery` | bool | `true` | Recover from panics |
| `timeout` | int | `10` | Request timeout in seconds |
| `throttle` | int | `100` | Max concurrent requests |

## Lua Routing

### Scripts Directory

All Lua files in `scripts_dir` are available. Use `.lua` extension.

### Global Scripts

Scripts listed in `global_scripts` are loaded into **every** Lua state at startup. Use for:
- Shared functions
- Helper utilities
- Handler definitions

Example `init.lua`:
```lua
function json_response(data, status)
    return {
        status = status or 200,
        body = encode_json(data),
        headers = {["Content-Type"] = "application/json"}
    }
end
```

### State Pool Size

Number of pre-allocated Lua VMs. Increase for high concurrency:
- Low traffic: 10 (default)
- Medium traffic: 30-60
- High traffic: 60+

## Routes

### Method

Any valid HTTP method: `GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `OPTIONS`, `HEAD`

### Pattern

Chi router patterns:
- Static: `/users`
- Parameter: `/users/{id}`
- Wildcard: `/files/*` or `/files/{path:.*}`

### Handler vs Backend

**Handler**: Lua function name (must be defined in global scripts)

```yaml
handler: "my_handler"
```

**Backend**: Service name for proxying (must be defined in `services`)

```yaml
backend: "legacy-service"
```

**Mutually exclusive**: Route has either `handler` OR `backend`, not both.

### Middleware

List of Lua function names to execute before handler:

```yaml
middleware:
  - "require_auth"
  - "rate_limit"
```

Middleware executes in order. If any returns a response, chain stops.

## Route Groups

Share middleware across multiple routes:

```yaml
route_groups:
  - pattern: "/api/v2"
    middleware:
      - "require_auth"
      - "api_v2_middleware"
    routes:
      - method: "GET"
        pattern: "/users"
        handler: "list_users"
      - method: "POST"
        pattern: "/users"
        handler: "create_user"
```

Pattern is **prepended** to nested route patterns:
- Group pattern: `/api/v2`
- Route pattern: `/users`
- Final path: `/api/v2/users`

## Error Handlers

Custom Lua functions for HTTP errors:

```yaml
error_handlers:
  not_found: "handle_404"
  method_not_allowed: "handle_405"
```

Error handler signature:
```lua
function handle_404(req)
    return {
        status = 404,
        body = '{"error": "Not Found"}',
        headers = {["Content-Type"] = "application/json"}
    }
end
```

## Performance Tuning

### Disable Middleware for Speed

```yaml
middleware:
  request_id: false
  real_ip: false
  logging: false    # Biggest performance gain
  throttle: 5000    # Increase concurrency limit
```

### Increase Lua State Pool

```yaml
lua_routing:
  state_pool_size: 60  # Match expected concurrency
```

### Compression

```yaml
compression:
  enabled: true
  level: 1  # Faster compression, larger files
```

## Complete Example

See `examples/configs/config-golua.yaml` for a full-featured configuration with all options.
