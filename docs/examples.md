# Working Examples

Complete, copy-paste-ready examples.

> **⚡ Pro Tip**: Always check [LuaRocks](https://luarocks.org/) for existing libraries before implementing features yourself. Keystone Gateway supports LuaRocks modules out of the box.

> **⚠️ OpenResty Compatibility**: Many `lua-resty-*` modules require nginx/OpenResty's `ngx.*` API and won't work with standard Lua. Stick to pure Lua modules like `lua-cjson`, `lpeg`, `luasocket`, `inspect`, etc. If a module uses `ngx.` in its code, it won't work.

## Basic REST API Gateway

**Config** (`config.yaml`):
```yaml
lua_routing:
  enabled: true
  scripts_dir: "./scripts"
  global_scripts:
    - "handlers"

tenants:
  - name: "api"
    path_prefix: "/api"
    routes:
      - method: "GET"
        pattern: "/users/{id}"
        handler: "get_user"

      - method: "POST"
        pattern: "/users"
        handler: "create_user"
```

**Handlers** (`scripts/handlers.lua`):
```lua
local BASE_URL = "http://localhost:3000"

function get_user(req)
    local user_id = req.params.id

    local resp, err = http_get(BASE_URL .. "/users/" .. user_id)

    if err then
        log("Error fetching user: " .. err)
        return {
            status = 502,
            body = '{"error": "Service unavailable"}'
        }
    end

    return {
        status = resp.status,
        body = resp.body,
        headers = {["Content-Type"] = "application/json"}
    }
end

function create_user(req)
    local resp, err = http_post(
        BASE_URL .. "/users",
        req.body,
        {["Content-Type"] = "application/json"}
    )

    if err then
        return {status = 502, body = '{"error": "Service error"}'}
    end

    return {
        status = resp.status,
        body = resp.body,
        headers = {["Content-Type"] = "application/json"}
    }
end
```

---

## JSON Processing with lua-cjson

> **Check LuaRocks First**: Use `lua-cjson` for fast JSON parsing (pure Lua, no nginx required).

**Install Dependencies**:
```bash
luarocks install lua-cjson
```

**Config** (`config.yaml`):
```yaml
lua_routing:
  enabled: true
  scripts_dir: "./scripts"
  state_pool_size: 60
  global_scripts:
    - "init"
    - "handlers"
  module_paths:
    - "/usr/local/share/lua/5.1/?.lua"
    - "/usr/local/share/lua/5.1/?/init.lua"
  module_cpaths:
    - "/usr/local/lib/lua/5.1/?.so"

tenants:
  - name: "api"
    path_prefix: "/api"
    routes:
      - method: "POST"
        pattern: "/data"
        handler: "process_json"
```

**Handlers** (`scripts/handlers.lua`):
```lua
local cjson = require("cjson")  -- Pure Lua/C module (works without nginx)

function process_json(req)
    local body = req.body

    if body == "" then
        return {
            status = 400,
            body = cjson.encode({error = "Empty request body"}),
            headers = {["Content-Type"] = "application/json"}
        }
    end

    -- Parse JSON with lua-cjson (fast C implementation)
    local ok, data = pcall(cjson.decode, body)

    if not ok then
        log("JSON parse error: " .. data)
        return {
            status = 400,
            body = cjson.encode({error = "Invalid JSON"}),
            headers = {["Content-Type"] = "application/json"}
        }
    end

    -- Process data
    data.processed = true
    data.timestamp = os.time()

    -- Encode response
    return {
        status = 200,
        body = cjson.encode(data),
        headers = {["Content-Type"] = "application/json"}
    }
end
```

---

## Authentication Middleware

**Config** (`config.yaml`):
```yaml
lua_routing:
  enabled: true
  scripts_dir: "./scripts"
  global_scripts:
    - "auth"
    - "handlers"

tenants:
  - name: "api"
    path_prefix: "/api"
    routes:
      - method: "GET"
        pattern: "/health"
        handler: "health_check"

      - method: "GET"
        pattern: "/users"
        handler: "list_users"
        middleware:
          - "require_auth"
```

**Auth Middleware** (`scripts/auth.lua`):
```lua
function require_auth(req, next)
    local auth_header = req.headers["Authorization"]

    if not auth_header or auth_header == "" then
        log("Missing Authorization header from " .. req.remote_addr)
        return {
            status = 401,
            body = '{"error": "Unauthorized"}',
            headers = {["Content-Type"] = "application/json"}
        }
    end

    -- Extract token
    local token = auth_header:match("^Bearer%s+(.+)$")

    if not token then
        return {
            status = 401,
            body = '{"error": "Invalid Authorization format"}'
        }
    end

    -- Validate token
    local resp, err = http_get("https://auth.example.com/validate?token=" .. token)

    if err or resp.status ~= 200 then
        log("Token validation failed: " .. (err or resp.status))
        return {
            status = 401,
            body = '{"error": "Invalid token"}'
        }
    end

    next()
    return nil
end
```

---

## Rate Limiting (Simple In-Memory)

> **Production Note**: For distributed rate limiting, use Redis with a compatible Lua client (not `lua-resty-redis` which requires nginx).

**Config** (`config.yaml`):
```yaml
lua_routing:
  enabled: true
  scripts_dir: "./scripts"
  global_scripts:
    - "rate_limit"
    - "handlers"

tenants:
  - name: "api"
    path_prefix: "/api"
    route_groups:
      - pattern: "/v1"
        middleware:
          - "rate_limit"
        routes:
          - method: "GET"
            pattern: "/data"
            handler: "get_data"
```

**Rate Limiter** (`scripts/rate_limit.lua`):
```lua
-- Simple in-memory rate limiter (not shared across instances)
local rate_limits = {}
local LIMIT = 100
local WINDOW = 60

function rate_limit(req, next)
    local ip = req.remote_addr:match("^([^:]+)")
    local now = os.time()
    local limit_data = rate_limits[ip]

    if not limit_data or now > limit_data.reset_time then
        rate_limits[ip] = {
            count = 1,
            reset_time = now + WINDOW
        }
        next()
        return nil
    end

    if limit_data.count >= LIMIT then
        local retry_after = limit_data.reset_time - now
        log("Rate limit exceeded for " .. ip)

        return {
            status = 429,
            body = '{"error": "Too Many Requests"}',
            headers = {
                ["Content-Type"] = "application/json",
                ["Retry-After"] = tostring(retry_after)
            }
        }
    end

    limit_data.count = limit_data.count + 1
    next()
    return nil
end
```

---

## Pattern Matching with LPeg

> **Check LuaRocks**: `lpeg` is a pure Lua library (works without nginx).

**Install**:
```bash
luarocks install lpeg
```

**Example** (`scripts/validators.lua`):
```lua
local lpeg = require("lpeg")

-- Email validation pattern
local P, R, S = lpeg.P, lpeg.R, lpeg.S
local email_pattern = (R"az" + R"AZ" + R"09" + S"_.-")^1 * P"@" *
                      (R"az" + R"AZ" + R"09" + S".-")^1 * P"." * R"az"^2

function validate_email(req)
    local email = req.query.email

    if not email or email == "" then
        return {status = 400, body = '{"error": "Email required"}'}
    end

    local valid = lpeg.match(email_pattern, email) ~= nil

    return {
        status = 200,
        body = '{"email": "' .. email .. '", "valid": ' .. tostring(valid) .. '}',
        headers = {["Content-Type"] = "application/json"}
    }
end
```

---

## Custom Error Handlers

**Config** (`config.yaml`):
```yaml
tenants:
  - name: "api"
    path_prefix: "/api"
    error_handlers:
      not_found: "handle_404"
      method_not_allowed: "handle_405"
```

**Error Handlers** (`scripts/errors.lua`):
```lua
function handle_404(req)
    log("404 Not Found: " .. req.path)

    return {
        status = 404,
        body = '{"error": "Not Found", "path": "' .. req.path .. '"}',
        headers = {["Content-Type"] = "application/json"}
    }
end

function handle_405(req)
    return {
        status = 405,
        body = '{"error": "Method Not Allowed"}',
        headers = {
            ["Content-Type"] = "application/json",
            ["Allow"] = "GET, POST"
        }
    }
end
```

---

## Health Check Endpoint

**Config** (`config.yaml`):
```yaml
tenants:
  - name: "admin"
    path_prefix: "/"
    routes:
      - method: "GET"
        pattern: "/health"
        handler: "health_check"
```

**Handler** (`scripts/handlers.lua`):
```lua
function health_check(req)
    local db_ok = check_database()
    local cache_ok = check_cache()

    local status = "healthy"
    local http_status = 200

    if not db_ok or not cache_ok then
        status = "unhealthy"
        http_status = 503
    end

    return {
        status = http_status,
        body = '{"status": "' .. status .. '", "database": ' .. tostring(db_ok) .. ', "cache": ' .. tostring(cache_ok) .. '}',
        headers = {["Content-Type"] = "application/json"}
    }
end

function check_database()
    local resp, err = http_get("http://db:5432/health")
    return err == nil and resp.status == 200
end

function check_cache()
    local resp, err = http_get("http://redis:6379/ping")
    return err == nil and resp.status == 200
end
```

---

## LuaRocks Integration

### Compatible Modules

> **⚠️ Important**: Avoid `lua-resty-*` modules - they require OpenResty/nginx's `ngx.*` API.

**Pure Lua modules that work:**

| Module | Use Case | Install |
|--------|----------|---------|
| `lua-cjson` | Fast JSON encoding/decoding | `luarocks install lua-cjson` |
| `lpeg` | Pattern matching | `luarocks install lpeg` |
| `luasocket` | Network operations, DNS | `luarocks install luasocket` |
| `inspect` | Table debugging | `luarocks install inspect` |
| `luajwt` | JWT validation | `luarocks install luajwt` |
| `luafilesystem` | File system operations | `luarocks install luafilesystem` |
| `penlight` | Utility functions | `luarocks install penlight` |

**Modules to avoid (require nginx):**
- ❌ `lua-resty-redis` (uses `ngx.*`)
- ❌ `lua-resty-jwt` (uses `ngx.*`)
- ❌ `lua-resty-http` (uses `ngx.*`)
- ❌ Any `lua-resty-*` module

### Configuring Paths

**Config** (`config.yaml`):
```yaml
lua_routing:
  module_paths:
    - "/usr/local/share/lua/5.1/?.lua"
    - "/usr/local/share/lua/5.1/?/init.lua"
  module_cpaths:
    - "/usr/local/lib/lua/5.1/?.so"
```

### Using Modules

```lua
-- Require at top of script
local cjson = require("cjson")
local inspect = require("inspect")

function handler(req)
    local data = cjson.decode(req.body)
    log("Data: " .. inspect(data))
    -- ...
end
```

### Finding Compatible Modules

1. Check [LuaRocks.org](https://luarocks.org/)
2. **Avoid modules with `ngx.` in code** - search GitHub repo before installing
3. Prefer pure Lua or C-based modules
4. Test in a Lua 5.1 environment (not OpenResty)

**Always check LuaRocks before implementing - there's likely a compatible library available.**

---

## See Also

- [Lua API Reference](lua-api.md) - Complete interface documentation
- [Configuration Guide](configuration.md) - YAML reference
- [LuaRocks](https://luarocks.org/) - Browse modules (avoid `lua-resty-*`)
- `examples/scripts/advanced.lua` - More LuaRocks examples
