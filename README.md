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

## Quick Start

```bash
git clone https://github.com/ygalsk/keystone-gateway.git
cd keystone-gateway
make dev

# Health check
curl http://localhost:8080/admin/health
```

### Minimal Configuration Example

```yaml
server:
  port: "8080"

tenants:
  - name: "api"
    domains: ["localhost"]
    lua_routes: "api"
    services:
      - name: "backend"
        url: "http://localhost:3001"
```

### Lua Route Example

```lua
-- scripts/api.lua
chi_route("GET", "/hello", function(req, res)
    res:Write("Hello World")
end)
```

---

## Lua API Highlights

```lua
-- Access request properties
print(req.Method)  -- GET, POST, etc. (property, not method!)
print(req.URL)     -- Full URL
print(req.Path)    -- URL path

-- Request methods
local body, err = req:Body()
local auth = req:Header("Authorization")
local id = req:Param("id")  -- From route pattern

-- Make HTTP requests with options
local resp = HTTP:Get("https://example.com/data", {
    headers = {
        Authorization = "Bearer token"
    },
    timeout = 5000,           -- milliseconds
    follow_redirects = false
})

if resp.Status == 200 then
    res:Write(resp.Body)
end

-- Middleware (MUST be defined before routes!)
chi_middleware(function(req, res, next)
    res:Header("X-Gateway", "Keystone")
    next()
end)
```

**Key Points:**
- Properties: `req.Method`, `req.URL`, `req.Path`, `req.Host`
- Methods: `req:Header()`, `req:Body()`, `req:Param()`, `req:Query()`
- HTTP client: `HTTP:Get/Post/Put/Delete(url, options)`
- See [docs/lua.md](docs/lua.md) for complete API reference  

---

## Why Keystone Exists

Most gateways bake in opinions: auth, rate limiting, and workflows. Keystone provides **general-purpose primitives**, letting tenants implement exactly what they need — nothing more, nothing less.

- No built-in auth or rate limiting  
- No service discovery baked in  
- No GraphQL or protocol assumptions  

All policies and business logic live in Lua scripts.

---

## Features

- Multi-tenant routing (domain, path, hybrid)  
- Load balancing & health checks  
- Hot-reloadable Lua scripts  
- TLS support & graceful shutdown  
- HTTP/2 and connection pooling  
- Optional compression & request limits  

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
