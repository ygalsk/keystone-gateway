# Quick Reference

## Essential Commands

```bash
# Build the gateway
go build -o keystone-gateway ./cmd/

# Run with configuration
./keystone-gateway -config config.yaml

# Validate configuration
./keystone-gateway -config config.yaml --validate
```

## Basic Configuration Template

```yaml
admin_base_path: "/admin"
lua_routing:
  enabled: true
  scripts_dir: "./scripts"
tenants:
  - name: "api"
    domains: ["localhost"]
    lua_routes: "basic-routes.lua"
    services:
      - name: "backend"
        url: "http://localhost:3001"
        health: "/health"
```

## Essential Lua Functions

```lua
-- Route registration
chi_route("GET", "/api/users", handler_function)

-- Middleware
chi_middleware("/api/*", middleware_function)

-- Route groups
chi_group("/api", function() ... end)

-- URL parameters
local id = chi_param(request, "id")

-- Logging
log("Message here")
```

## Common Middleware Patterns

```lua
-- Authentication
chi_middleware("/api/*", function(request, response, next)
    local token = request.headers["Authorization"]
    if not token then
        response:status(401)
        response:write('{"error": "Unauthorized"}')
        return
    end
    next()
end)

-- CORS
chi_middleware("/*", function(request, response, next)
    response:header("Access-Control-Allow-Origin", "*")
    next()
end)

-- Logging
chi_middleware("/*", function(request, response, next)
    log(request.method .. " " .. request.path)
    next()
end)
```

## Admin Endpoints

```bash
# Gateway health
curl http://localhost:8080/admin/health

# List tenants
curl http://localhost:8080/admin/tenants

# Tenant health
curl http://localhost:8080/admin/tenants/{name}/health
```

## Response Object Methods

```lua
-- Set status code
response:status(200)

-- Set headers
response:header("Content-Type", "application/json")

-- Write response body
response:write('{"message": "Hello"}')
```

## Request Object Properties

```lua
-- HTTP method
request.method  -- "GET", "POST", etc.

-- Request path
request.path    -- "/api/users"

-- Headers table
request.headers["Authorization"]

-- Request body
request.body    -- string
```

## Routing Strategies

```yaml
# Host-based routing
domains: ["api.example.com"]

# Path-based routing  
path_prefix: "/api/"

# Hybrid routing (both)
domains: ["api.example.com"]
path_prefix: "/v2/"
```

## Troubleshooting

```bash
# Check logs for errors
./keystone-gateway -config config.yaml 2>&1 | grep ERROR

# Test backend directly
curl http://localhost:3001/health

# Validate Lua syntax
lua -c scripts/your-script.lua
```