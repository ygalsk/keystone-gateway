# Keystone Gateway Documentation

Simple, interface-focused documentation.

## Getting Started

- **[Quick Start](quick-start.md)** - Get running in 5 minutes
- **[Configuration Guide](configuration.md)** - Complete YAML reference
- **[Lua API Reference](lua-api.md)** - Interfaces, inputs, and outputs
- **[Examples](examples.md)** - Working code samples

## Philosophy

Keystone Gateway is a **general-purpose HTTP routing primitive** with embedded Lua scripting.

**Core principle**: The gateway is dumb. Tenants are smart.

The gateway provides primitives (HTTP routing, Lua execution). You compose them into solutions.

## Quick Links

### Configuration
- [Middleware Options](configuration.md#middleware-options)
- [Lua Routing Setup](configuration.md#lua-routing)
- [Route Configuration](configuration.md#routes)
- [Error Handlers](configuration.md#error-handlers)

### Lua API
- [Handler Interface](lua-api.md#handler-interface)
- [Middleware Interface](lua-api.md#middleware-interface)
- [Go Primitives](lua-api.md#go-primitives)
- [Complete Examples](lua-api.md#complete-examples)

### Examples
- [REST API Gateway](examples.md#basic-rest-api-gateway)
- [JSON Processing](examples.md#json-processing-with-lua-cjson)
- [Authentication](examples.md#authentication-middleware)
- [Rate Limiting](examples.md#rate-limiting-simple-in-memory)
- [LuaRocks Integration](examples.md#luarocks-integration)

## Key Concepts

### Stateless Design
- No health checking (delegated to load balancers)
- No backend state tracking
- Horizontal scaling without coordination

### Lua State Pooling
- Pre-allocated Lua VMs for thread safety
- Configurable pool size (default: 10)
- Metrics at `/debug/lua-pool`

### LuaRocks Support
- Compatible with pure Lua modules
- **Avoid `lua-resty-*` modules** (require nginx `ngx.*` API)
- Check [LuaRocks.org](https://luarocks.org/) before implementing

### Path-Based Routing
- Multi-tenant with path prefixes
- Chi router patterns (`/users/{id}`)
- Domain routing via external reverse proxy

## Architecture

```
HTTP Request
    ↓
Chi Router (path-based)
    ↓
Middleware Chain (optional Lua functions)
    ↓
Handler (Lua function OR backend proxy)
    ↓
HTTP Response
```

## Debug Endpoints

- `/health` - Liveness check (returns 200 OK)
- `/debug/lua-pool` - State pool metrics (hits, misses, wait time)

## Performance Tips

1. **Disable logging** for highest throughput:
   ```yaml
   middleware:
     logging: false
   ```

2. **Increase state pool** for high concurrency:
   ```yaml
   lua_routing:
     state_pool_size: 60
   ```

3. **Use LuaRocks libraries** (e.g., `lua-cjson`) instead of pure Lua implementations

4. **Enable compression** for bandwidth savings:
   ```yaml
   compression:
     enabled: true
     level: 1  # Fast compression
   ```

## Common Patterns

### Handler Structure
```lua
function handler_name(req)
    -- Access request: req.method, req.path, req.headers, req.params, req.body
    -- Process logic
    return {
        status = 200,
        body = "response",
        headers = {["Content-Type"] = "text/plain"}
    }
end
```

### Middleware Structure
```lua
function middleware_name(req, next)
    -- Validate/check
    if not valid then
        return {status = 401, body = "Unauthorized"}
    end

    -- Continue chain
    next()
    return nil
end
```

### Error Handling
```lua
local resp, err = http_get(url)

if err then
    log("Error: " .. err)
    return {status = 502, body = "Service unavailable"}
end

if resp.status ~= 200 then
    return {status = resp.status, body = resp.body}
end
```

## Further Reading

- **[MANIFEST.md](../MANIFEST.md)** - Philosophy and principles
- **[DESIGN.md](../DESIGN.md)** - Technical architecture
- **[ROADMAP.md](../ROADMAP.md)** - Evolution history
- **[CHANGELOG.md](../CHANGELOG.md)** - Version history

---

**Remember**: Always check [LuaRocks.org](https://luarocks.org/) before implementing features. Use pure Lua modules, avoid `lua-resty-*` modules.
