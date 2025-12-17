# Keystone Gateway

[![Go](https://img.shields.io/badge/Go-1.21-blue)](https://golang.org) [![License: MIT](https://img.shields.io/badge/license-MIT-green)](LICENSE)

Keystone Gateway is a **general-purpose HTTP routing primitive** with **embedded Lua scripting**.  
It is designed for engineers who want **control without opinions**: the gateway handles HTTP efficiently, while tenants implement authentication, rate limiting, transformations, and other policies in Lua.

> ⚡ **Dumb gateway, smart tenants.** Keystone provides primitives, not workflows.

---

## TL;DR

- Multi-tenant routing by domain/path
- Embedded Lua scripting for routes and middleware
- Deep modules: simple API, complex implementation
- High performance: HTTP/2, connection pooling, Lua state pools
- No built-in auth, rate limiting, or opinions

---

## Philosophy

1. **Deep Modules** – Interfaces are simpler than implementations (e.g., Lua engine, HTTP client, request wrapper).
2. **Information Hiding** – Users never manage Lua states, bytecode, or connection pools.
3. **Pull Complexity Down** – Complex logic lives in Go, Lua stays simple.
4. **General-Purpose** – Works for many use cases, not just one.
5. **Gateway is Dumb** – Primitives only; tenants compose business logic.

---

## Documentation

### Core Documents
- **[MANIFEST.md](MANIFEST.md)** - Project philosophy, principles, and development guidelines
- **[DESIGN.md](DESIGN.md)** - Technical architecture and implementation details
- **[ROADMAP.md](ROADMAP.md)** - Evolution from v1.0.0 to v5.0.0 and lessons learned
- **[CHANGELOG.md](CHANGELOG.md)** - Detailed version history and migration guides

### Additional Resources
- **[docs/lua.md](docs/lua.md)** - Complete Lua API reference
- **[docs/config.md](docs/config.md)** - Configuration guide
- **[docs/examples.md](docs/examples.md)** - Example implementations
- **[docs/development.md](docs/development.md)** - Development and contribution guide

---

## Quick Start

```bash
git clone https://github.com/ygalsk/keystone-gateway.git
cd keystone-gateway
make dev

# Health check
curl http://localhost:8080/health
```

### Minimal Configuration Example

```yaml
lua_routing:
  enabled: true
  scripts_dir: "./examples/scripts"
  global_scripts:
    - "init"
    - "handlers"

tenants:
  - name: "api"
    path_prefix: "/api"
    routes:
      - method: "GET"
        pattern: "/hello"
        handler: "hello_handler"
```

### Lua Handler Example

```lua
-- examples/scripts/handlers.lua
function hello_handler(req)
    return {
        status = 200,
        body = "Hello from Keystone Gateway!",
        headers = {["Content-Type"] = "text/plain"}
    }
end
```

---

## Lua API Highlights

```lua
-- Handler function receives request table
function my_handler(req)
    -- Access request properties
    local method = req.method      -- "GET", "POST", etc.
    local path = req.path          -- "/users/123"
    local user_id = req.params.id  -- Chi URL parameters
    local token = req.headers["Authorization"]
    local query_val = req.query.foo

    -- Use LuaRocks modules for HTTP requests, JSON, etc.
    -- Example: local http = require("http.request")

    -- Return response table
    return {
        status = 200,
        body = "Response body",
        headers = {["Content-Type"] = "application/json"}
    }
end

-- Middleware function with next callback
function require_auth(req, next)
    if not req.headers["Authorization"] then
        return {
            status = 401,
            body = "Unauthorized"
        }
    end
    next()  -- Continue to handler
    return nil
end
```

**Key Points:**
- **Request table**: `req.method`, `req.path`, `req.headers`, `req.params`, `req.query`, `req.body`
- **Response table**: `{status = 200, body = "...", headers = {...}}`
- **Go primitive**: `log(msg)` for structured logging
- **LuaRocks support**: Use modules like `http`, `lua-cjson` for advanced functionality
- **Configuration**: Routes and middleware defined in YAML config
- See example scripts in `examples/scripts/` directory  

---

## Why Keystone Exists

Most gateways bake in opinions: auth, rate limiting, and workflows. Keystone provides **general-purpose primitives**, letting tenants implement exactly what they need — nothing more, nothing less.

- No built-in auth or rate limiting  
- No service discovery baked in  
- No GraphQL or protocol assumptions  

All policies and business logic live in Lua scripts.

---

## Features

- Multi-tenant routing with path-based prefixes
- Embedded Lua scripting with LuaJIT support
- Stateless design (delegates load balancing to infrastructure)
- Lua state pooling for high performance
- HTTP/2 and connection pooling
- Optional compression and request limits
- LuaRocks module support
- Graceful shutdown  

---

## Project Structure

```
cmd/           # Main app
internal/      # Core modules: Lua, routing, HTTP client
configs/       # YAML examples
scripts/lua/   # Tenant Lua scripts
tests/         # Unit and integration tests
```

---

## Contributing

- Contributions welcome!  
- See `docs/development.md` for guidelines.  
- Use Lua primitives; keep business logic in tenant scripts.

---

## License

MIT
