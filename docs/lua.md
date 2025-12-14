# Lua API Reference

Complete API reference for Lua scripting in Keystone Gateway.

---

## ⚠️ Critical: Script Structure

**Middleware MUST be defined BEFORE routes or the application will panic:**

```lua
-- ✅ CORRECT
chi_middleware(function(req, res, next)
    next()
end)

chi_route("GET", "/path", function(req, res)
    res:Write("OK")
end)

-- ❌ WRONG - Will crash!
chi_route("GET", "/path", ...)
chi_middleware(...)  -- Too late
```

---

## Global Functions

### `chi_route(method, pattern, handler)`

Register an HTTP route.

```lua
chi_route("GET", "/users/{id}", function(req, res)
    local id = req:Param("id")
    res:Write("User: " .. id)
end)

-- Wildcards
chi_route("GET", "/files/{path:.*}", function(req, res)
    local path = req:Param("path")  -- Captures everything
end)
```

**Parameters:** `method` (string), `pattern` (string), `handler` (function)

---

### `chi_middleware(handler)`

Register middleware (runs before routes).

```lua
chi_middleware(function(req, res, next)
    print(req.Method .. " " .. req.Path)
    next()  -- Continue to next handler
end)

-- Auth example (stops on failure)
chi_middleware(function(req, res, next)
    if req:Header("Authorization") == "" then
        res:Status(401)
        res:Write("Unauthorized")
        return  -- Don't call next() - stops request
    end
    next()
end)
```

**Parameters:** `handler` (function receiving req, res, next)

---

## Request Object

### Properties (Field Access)

| Property | Type | Example |
|----------|------|---------|
| `Method` | string | `req.Method` → "GET" |
| `URL` | string | `req.URL` → "http://example.com/path?q=1" |
| `Path` | string | `req.Path` → "/users/123" |
| `Host` | string | `req.Host` → "example.com" |

```lua
print(req.Method)  -- No parentheses!
print(req.URL)
```

### Methods

#### `req:Header(key)` → string
Get header value (empty string if not found).

```lua
local auth = req:Header("Authorization")
```

#### `req:Query(key)` → string
Get URL query parameter.

```lua
local search = req:Query("q")  -- /search?q=golang
```

#### `req:Param(key)` → string
Get URL path parameter from route pattern.

```lua
chi_route("GET", "/users/{id}", function(req, res)
    local id = req:Param("id")
end)
```

#### `req:Body()` → (string, error)
Get request body (cached, size-limited).

```lua
local body, err = req:Body()
if err then
    res:Status(500)
    res:Write("Read error: " .. err)
    return
end
```

#### `req:Headers()` → table
Get all headers as `[name] = {values}` table.

#### `req:ContextSet(key, value)`
Store value in request context (for passing data between middleware/handlers).

```lua
req:ContextSet("user_id", "123")
```

#### `req:ContextGet(key)` → any
Retrieve value from request context.

```lua
local userId = req:ContextGet("user_id")
```

---

## Response Object

### Methods

#### `res:Status(code)`
Set HTTP status code.

```lua
res:Status(404)
```

Common codes: 200 (OK), 201 (Created), 400 (Bad Request), 401 (Unauthorized), 404 (Not Found), 500 (Internal Server Error), 502 (Bad Gateway)

#### `res:Header(key, value)`
Set response header. **Must be called BEFORE `res:Write()`**.

```lua
res:Header("Content-Type", "application/json")
res:Header("Cache-Control", "no-cache")
```

#### `res:Write(content)` → (bytes, error)
Write response body (can be called multiple times to append).

```lua
res:Write("Hello, World!")
```

---

## HTTP Module

Global HTTP client: `HTTP`

### Methods

#### `HTTP:Get(url, options)` → (response, error)

```lua
-- Simple GET
local resp, err = HTTP:Get("https://api.example.com/users")
if err then
    res:Status(502)
    res:Write("Request failed: " .. err)
    return
end

-- With options
local resp = HTTP:Get("https://api.example.com/data", {
    headers = {
        Authorization = "Bearer token123",
        ["User-Agent"] = "Gateway/1.0"
    },
    timeout = 5000,           -- milliseconds (default: 10000)
    follow_redirects = false  -- default: true
})

if resp.Status == 200 then
    res:Write(resp.Body)
end
```

#### `HTTP:Post(url, body, options)` → (response, error)

```lua
local resp = HTTP:Post(
    "https://api.example.com/users",
    '{"name": "John"}',
    {
        headers = {
            ["Content-Type"] = "application/json",
            Authorization = "Bearer " .. token
        }
    }
)
```

#### `HTTP:Put(url, body, options)` → (response, error)
#### `HTTP:Delete(url, options)` → (response, error)

### Options Table

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `headers` | table | `{}` | Request headers |
| `timeout` | number | `10000` | Timeout in milliseconds |
| `follow_redirects` | boolean | `true` | Follow HTTP redirects |

### Response Object

```lua
{
    Body = "response body string",
    Status = 200,
    Headers = {
        ["Content-Type"] = "application/json",
        -- ...
    }
}
```

---

## Complete Examples

### REST API Proxy

```lua
chi_route("GET", "/api/users/{id}", function(req, res)
    local id = req:Param("id")

    local result = HTTP:Get("https://backend.example.com/users/" .. id, {
        headers = {
            Authorization = req:Header("Authorization")
        }
    })

    res:Status(result.Status)
    res:Header("Content-Type", "application/json")
    res:Write(result.Body)
end)
```

### Authentication Middleware

```lua
chi_middleware(function(req, res, next)
    if not req.Path:match("^/api/") then
        next()
        return
    end

    if req:Header("Authorization") == "" then
        res:Status(401)
        res:Write('{"error": "Unauthorized"}')
        return
    end

    next()
end)
```

### Request Validation

```lua
chi_route("POST", "/data", function(req, res)
    local body, err = req:Body()

    if err then
        res:Status(500)
        res:Write("Read error")
        return
    end

    if body == "" then
        res:Status(400)
        res:Write("Body required")
        return
    end

    res:Status(201)
    res:Write("Created")
end)
```

---

## Common Patterns

**Error handling:**
```lua
local resp, err = HTTP:Get(url)
if err then
    res:Status(502)
    res:Write("Network error: " .. err)
    return
end

if resp.Status ~= 200 then
    res:Status(502)
    res:Write("Backend error: " .. resp.Status)
    return
end
```

**Content types:**
```lua
res:Header("Content-Type", "application/json")
res:Write('{"status": "ok"}')
```

**Health check:**
```lua
chi_route("GET", "/health", function(req, res)
    res:Header("Content-Type", "application/json")
    res:Write('{"status": "healthy"}')
end)
```

---

## Best Practices

1. ⚠️ Define middleware BEFORE routes
2. Use properties for simple values: `req.Method`, `req.URL`
3. Use methods for operations: `req:Header()`, `req:Body()`
4. Always check errors from `HTTP` calls and `req:Body()`
5. Set `Content-Type` header for all responses
6. Call `next()` in middleware to continue chain
7. Return early on errors
8. Validate all input

---

## Performance

- Request bodies cached after first read
- Lua scripts compiled to bytecode and cached
- Lua states pooled for thread safety
- HTTP/2 client with connection pooling
- Property access (`req.Method`) pre-computed at request creation

---

## Troubleshooting

**nil value errors:** Check objects exist before accessing
```lua
if req then print(req.Method) end
```

**Routes not matching:** Check pattern and order (earlier routes match first)

**Headers not sent:** Set headers BEFORE `res:Write()`

**Middleware not running:** Define middleware BEFORE routes

---

## Further Reading

- [Chi Router](https://github.com/go-chi/chi)
- [Lua 5.1 Reference](https://www.lua.org/manual/5.1/)
- [HTTP Status Codes](https://httpstatuses.com/)
- [Design Document](../DESIGN.md)

See `scripts/lua/examples/` for working examples.
