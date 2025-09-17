# Keystone Gateway

High-performance reverse proxy with embedded Lua scripting. Multi-tenant routing without the complexity.

## Why Keystone Gateway?

- **Embedded Lua**: Define routes in Lua scripts, no recompilation needed
- **Multi-tenant**: Route by domain, path, or both
- **Performance**: Thread-safe Lua pools, HTTP/2, connection pooling
- **Simple**: One binary, YAML config, Lua scripts. That's it.

## Quick Start

```bash
# Get it running
git clone https://github.com/your-org/keystone-gateway.git
cd keystone-gateway
make dev

# Gateway runs on :8080
curl http://localhost:8080/admin/health
```

### Basic Configuration

```yaml
# config.yaml
server:
  port: "8080"                # Optional: server port

request_limits:               # Optional: security limits
  max_body_size: 10485760     # 10MB default
  max_header_size: 1048576    # 1MB default

tenants:
  - name: "api"
    domains: ["localhost"]
    lua_routes: "api"
    services:
      - name: "backend"
        url: "http://localhost:3001"
```

```lua
-- scripts/api.lua
chi_route("GET", "/hello", function(request, response)
    response_write(response, "Hello World")
end)
```

Start: `./keystone-gateway -config config.yaml`

## How It Works

1. **Configure tenants** - Define who gets routed where
2. **Write Lua scripts** - Define routes and middleware
3. **Start gateway** - Routes traffic based on domain/path
4. **Monitor** - Check `/admin/health` for status

## Routing Strategies

```yaml
# Route by domain
tenants:
  - name: "api"
    domains: ["api.example.com"]
    lua_routes: "api"

# Route by path
  - name: "app"
    path_prefix: "/app/"
    lua_routes: "app"

# Route by both (hybrid)
  - name: "v2-api"
    domains: ["api.example.com"]
    path_prefix: "/v2/"
    lua_routes: "v2"
```

## Lua Scripting

```lua
-- Simple route
chi_route("GET", "/users", function(request, response)
    response_write(response, "User list")
end)

-- With parameters
chi_route("GET", "/users/{id}", function(request, response)
    local id = chi_param(request, "id")
    response_write(response, "User: " .. id)
end)

-- Middleware (no pattern parameter)
chi_middleware(function(request, response, next)
    response_header(response, "X-Gateway", "Keystone")
    next()
end)

-- Groups
chi_group(function()
    chi_route("GET", "/users", function(request, response)
        response_write(response, "Users")
    end)
    chi_route("POST", "/users", function(request, response)
        response_write(response, "Create user")
    end)
end)

-- HTTP client for proxying
chi_route("GET", "/proxy/{path}", function(request, response)
    local path = chi_param(request, "path")
    local body, status, headers = http_get("https://api.example.com/" .. path)
    response_status(response, status)
    response_write(response, body)
end)
```

## Advanced Features

### HTTP Client & Request Processing
```lua
-- Read request body with size limits
chi_route("POST", "/api/data", function(request, response)
    local body = request_body(request)  -- Cached, size-limited
    local method = request_method(request)
    local auth = request_header(request, "Authorization")

    if #body > 0 then
        response_header(response, "Content-Type", "application/json")
        response_write(response, '{"received": ' .. #body .. ' bytes}')
    end
end)

-- External API calls with custom headers
local headers = {["Authorization"] = "Bearer token123"}
local body, status, resp_headers = http_post("https://api.external.com/data",
    '{"key": "value"}', headers)
```

### Context Caching & Performance
```lua
-- Cache expensive operations across request pipeline
chi_middleware(function(request, response, next)
    local user_id = authenticate_user(request)
    chi_context_set(request, "user_id", user_id)  -- Cache for later use
    next()
end)

chi_route("GET", "/profile", function(request, response)
    local user_id = chi_context_get(request, "user_id")  -- Retrieve cached value
    response_write(response, "Profile for user: " .. user_id)
end)
```

### Error Handling
```lua
-- Custom 404 and 405 handlers
chi_not_found(function(request, response)
    response_status(response, 404)
    response_write(response, '{"error": "Resource not found"}')
end)

chi_method_not_allowed(function(request, response)
    response_status(response, 405)
    response_write(response, '{"error": "Method not allowed"}')
end)
```

## Load Balancing

Multiple services = automatic round-robin:

```yaml
services:
  - name: "api-1"
    url: "http://api-1:3001"
  - name: "api-2"
    url: "http://api-2:3001"
  - name: "api-3"
    url: "http://api-3:3001"
```

Health checks happen automatically.

## Production Features

- **Health monitoring**: `/admin/health`, `/admin/tenants/{name}/health`
- **Request limits**: Configurable body/header/URL size limits (default: 10MB/1MB/8KB)
- **Compression**: Configurable gzip for JSON/HTML/text/CSS/JS
- **TLS**: Configure cert/key files
- **Graceful shutdown**: SIGTERM handling
- **Performance**: HTTP/2, connection pooling, Lua state pools, bytecode compilation
- **Security**: Request size limits, configurable timeouts (⚠️ Note: Lua scripts have file system access)

### Configuration Example
```yaml
request_limits:
  max_body_size: 10485760     # 10MB
  max_header_size: 1048576    # 1MB
  max_url_size: 8192          # 8KB

compression:
  enabled: true
  level: 5
  content_types:
    - "application/json"
    - "text/html"
    - "text/css"
    - "text/javascript"
    - "application/xml"
    - "text/plain"
```

## Make Commands

```bash
make dev         # Start development
make test        # Run all tests
make staging     # Deploy to staging
make production  # Deploy to production
make clean       # Cleanup
```

## Project Structure

```
cmd/           # Main application
internal/      # Core Go packages
  config/      # YAML configuration
  lua/         # Lua engine integration
  routing/     # HTTP routing & load balancing
configs/       # Configuration examples
scripts/lua/   # Lua routing scripts
tests/         # Unit, integration, e2e tests
```

## Examples

See `configs/examples/`:
- `simple.yaml` - Single backend
- `multi-tenant.yaml` - Multiple tenants
- `production.yaml` - Production setup

See `scripts/lua/examples/`:
- `api-routes.lua` - Basic API routing
- `auth-routes.lua` - Authentication middleware

## Documentation

- **[Quick Start](docs/quick-start.md)** - 2-minute setup
- **[Configuration](docs/config.md)** - YAML reference
- **[Lua Scripting](docs/lua.md)** - Route definitions
- **[Examples](docs/examples.md)** - Real-world patterns
- **[Development](docs/development.md)** - Contributing guide

## Philosophy

Keep it simple. Get it working. Make it fast.

Keystone Gateway is a reverse proxy with embedded Lua scripting. One binary, YAML config, Lua scripts. No external dependencies, no complex setup, no microservice hell.

## License

MIT
