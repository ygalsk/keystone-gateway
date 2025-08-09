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
    response:write("Hello World")
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
    response:write("User list")
end)

-- With parameters
chi_route("GET", "/users/{id}", function(request, response)
    local id = chi_param(request, "id")
    response:write("User: " .. id)
end)

-- Middleware
chi_middleware("/api/*", function(request, response, next)
    response:header("X-Gateway", "Keystone")
    next()
end)

-- Groups
chi_group("/api", function()
    chi_route("GET", "/users", users_handler)
    chi_route("POST", "/users", create_users)
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
- **Compression**: Configurable gzip for JSON/HTML/text
- **TLS**: Configure cert/key files
- **Graceful shutdown**: SIGTERM handling
- **Performance**: HTTP/2, connection pooling, Lua state pools

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
