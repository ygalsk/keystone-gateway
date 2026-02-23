---
title: Lua API
nav_order: 4
---

# Lua API Reference

Interface documentation: what goes in, what comes out.

## Handler Interface

**Input**: Request table
**Output**: Response table

```lua
function handler_name(req)
    -- Process request
    return {
        status = 200,
        body = "response body",
        headers = {["Content-Type"] = "text/plain"}
    }
end
```

### Request Table (Input)

```lua
req = {
    method = "GET",                    -- HTTP method string
    path = "/users/123",               -- URL path string
    url = "http://...",                -- Full URL string
    host = "example.com",              -- Host header string
    remote_addr = "192.168.1.1:12345", -- Client IP:port string

    headers = {                        -- Map of header name → value
        ["Content-Type"] = "application/json",
        ["Authorization"] = "Bearer token"
    },

    params = {                         -- Chi URL parameters
        id = "123"                     -- From pattern /users/{id}
    },

    query = {                          -- Query string parameters
        foo = "bar",                   -- From ?foo=bar&limit=10
        limit = "10"
    },

    body = "raw request body"          -- String (max 10MB, cached)
}
```

**All fields are strings or tables of strings.**

### Response Table (Output)

```lua
return {
    status = 200,                      -- HTTP status code (integer)

    body = "response content",         -- Response body (string)

    headers = {                        -- Optional response headers
        ["Content-Type"] = "application/json",
        ["Cache-Control"] = "no-cache"
    }
}
```

**Required**: `status` and `body`
**Optional**: `headers`

### Default Values

If you don't provide:
- `status`: defaults to `200`
- `headers`: defaults to `{}`

## Middleware Interface

**Input**: Request table + next callback
**Output**: Response table OR nil

```lua
function middleware_name(req, next)
    -- Option 1: Short-circuit (return response)
    if not req.headers["Authorization"] then
        return {
            status = 401,
            body = "Unauthorized"
        }
    end

    -- Option 2: Continue chain
    next()
    return nil  -- Must return nil to continue
end
```

### When to Return What

| Return Value | Behavior |
|--------------|----------|
| Response table `{status, body, headers}` | **Stop chain**, send response |
| `nil` | **Continue chain** to next middleware/handler |

**Critical**: Call `next()` **before** returning `nil`.

## Go Primitive

### log(message)

**Input**: String message
**Output**: None (void)

```lua
log("User authenticated: user_id=" .. user_id)
log("Error: " .. err)
```

Writes to structured logger. Use for debugging and auditing.

## Error Handler Interface

**Input**: Request table
**Output**: Response table

```lua
function handle_404(req)
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
        headers = {["Content-Type"] = "application/json"}
    }
end
```

Same as handler interface but called automatically on HTTP errors.

## Complete Examples

### Simple Handler

```lua
function get_status(req)
    return {
        status = 200,
        body = "OK",
        headers = {["Content-Type"] = "text/plain"}
    }
end
```

**Config**:
```yaml
routes:
  - method: "GET"
    pattern: "/status"
    handler: "get_status"
```

### Authentication Middleware

```lua
function require_auth(req, next)
    local token = req.headers["Authorization"]

    if not token or token == "" then
        log("Missing authorization header")
        return {
            status = 401,
            body = '{"error": "Unauthorized"}',
            headers = {["Content-Type"] = "application/json"}
        }
    end

    -- Basic token validation (check prefix)
    if not token:match("^Bearer ") then
        log("Invalid token format")
        return {
            status = 401,
            body = '{"error": "Invalid token format"}'
        }
    end

    -- Token valid, continue
    next()
    return nil
end
```

**Config**:
```yaml
routes:
  - method: "POST"
    pattern: "/api/data"
    handler: "process_data"
    middleware:
      - "require_auth"
```

### Logging Middleware

```lua
function log_requests(req, next)
    log(req.method .. " " .. req.path .. " from " .. req.remote_addr)
    next()
    return nil
end
```

**Config**:
```yaml
route_groups:
  - pattern: "/api"
    middleware:
      - "log_requests"
    routes:
      # All /api/* routes get logged
```

## Type Reference

### Request Table Fields

| Field | Type | Example | Always Present |
|-------|------|---------|----------------|
| `method` | string | `"GET"` | ✅ |
| `path` | string | `"/users/123"` | ✅ |
| `url` | string | `"http://..."` | ✅ |
| `host` | string | `"example.com"` | ✅ |
| `remote_addr` | string | `"192.168.1.1:12345"` | ✅ |
| `headers` | table | `{["Content-Type"] = "..."}` | ✅ (may be empty) |
| `params` | table | `{id = "123"}` | ✅ (may be empty) |
| `query` | table | `{foo = "bar"}` | ✅ (may be empty) |
| `body` | string | `"..."` | ✅ (may be empty) |

### Response Table Fields

| Field | Type | Required | Example |
|-------|------|----------|---------|
| `status` | integer | ✅ | `200` |
| `body` | string | ✅ | `"Hello"` |
| `headers` | table | ❌ | `{["Content-Type"] = "text/plain"}` |

### HTTP Response Table Fields

| Field | Type | Always Present |
|-------|------|----------------|
| `status` | integer | ✅ |
| `body` | string | ✅ |
| `headers` | table | ✅ (may be empty) |

## Common Patterns

### JSON Response

```lua
function json_handler(req)
    local data = {
        success = true,
        message = "Data processed"
    }

    return {
        status = 200,
        body = encode_json(data),  -- Your JSON encoding function
        headers = {["Content-Type"] = "application/json"}
    }
end
```

### Conditional Middleware

```lua
function api_only_middleware(req, next)
    -- Only apply to /api/* paths
    if not req.path:match("^/api/") then
        next()
        return nil
    end

    -- API-specific logic here
    log("API request: " .. req.path)

    next()
    return nil
end
```

## Best Practices

1. **Set Content-Type** header for all responses
2. **Log errors** for debugging
3. **Return early** on validation failures
4. **Use nil return** in middleware to continue chain
5. **Don't forget** to call `next()` before returning nil

## Performance Notes

- **Request body cached**: Reading `req.body` multiple times is free
- **String operations**: Lua string concat (`..`) is efficient for small strings
- **Table construction**: Pre-allocated internally for request tables
