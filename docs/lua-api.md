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

## Go Primitives

### log(message)

**Input**: String message
**Output**: None (void)

```lua
log("User authenticated: user_id=" .. user_id)
log("Error: " .. err)
```

Writes to structured logger. Use for debugging and auditing.

### http_get(url)

**Input**: URL string
**Output**: Response table, error string

```lua
local resp, err = http_get("https://api.example.com/users/123")

if err then
    log("HTTP error: " .. err)
    return {status = 502, body = "Service unavailable"}
end

-- resp = {
--     status = 200,
--     body = '{"id": 123, "name": "John"}',
--     headers = {["Content-Type"] = "application/json"}
-- }

if resp.status == 200 then
    return {status = 200, body = resp.body}
end
```

**Response structure**:
```lua
{
    status = 200,           -- HTTP status code (integer)
    body = "...",           -- Response body (string)
    headers = {             -- Response headers (table)
        ["Content-Type"] = "application/json"
    }
}
```

**Error handling**:
- Network error: `err` is non-nil string, `resp` is nil
- HTTP error (4xx, 5xx): `err` is nil, check `resp.status`

### http_post(url, body, headers)

**Input**:
- `url`: String URL
- `body`: String request body
- `headers`: Table of header name → value

**Output**: Response table, error string

```lua
local resp, err = http_post(
    "https://api.example.com/users",
    '{"name": "John", "email": "john@example.com"}',
    {
        ["Content-Type"] = "application/json",
        ["Authorization"] = "Bearer " .. token
    }
)

if err then
    return {status = 502, body = "Failed to create user"}
end

if resp.status == 201 then
    return {status = 201, body = resp.body}
end
```

**Headers table is optional**: Can pass empty `{}` or omit third argument.

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

### Handler with URL Parameters

```lua
function get_user(req)
    local user_id = req.params.id  -- From /users/{id}

    local resp, err = http_get("https://api.example.com/users/" .. user_id)

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
```

**Config**:
```yaml
routes:
  - method: "GET"
    pattern: "/users/{id}"
    handler: "get_user"
```

### Handler with Request Body

```lua
function create_user(req)
    local body = req.body

    if body == "" then
        return {
            status = 400,
            body = '{"error": "Request body required"}'
        }
    end

    local resp, err = http_post(
        "https://api.example.com/users",
        body,
        {
            ["Content-Type"] = "application/json",
            ["Authorization"] = req.headers["Authorization"]
        }
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

**Config**:
```yaml
routes:
  - method: "POST"
    pattern: "/users"
    handler: "create_user"
    middleware:
      - "require_auth"
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

    -- Validate token (example: check with auth service)
    local resp, err = http_get("https://auth.example.com/validate?token=" .. token)

    if err or resp.status ~= 200 then
        log("Token validation failed")
        return {
            status = 401,
            body = '{"error": "Invalid token"}'
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

### Proxy with Header Pass-Through

```lua
function proxy_handler(req)
    local resp, err = http_get(
        "https://backend.example.com" .. req.path,
        req.headers  -- Pass all headers through
    )

    if err then
        return {status = 502, body = "Backend unavailable"}
    end

    return {
        status = resp.status,
        body = resp.body,
        headers = resp.headers
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

1. **Always check errors** from `http_get` and `http_post`
2. **Set Content-Type** header for all responses
3. **Log errors** for debugging
4. **Return early** on validation failures
5. **Use nil return** in middleware to continue chain
6. **Don't forget** to call `next()` before returning nil

## Performance Notes

- **Request body cached**: Reading `req.body` multiple times is free
- **String operations**: Lua string concat (`..`) is efficient for small strings
- **HTTP calls**: Pooled connections, reused across requests
- **Table construction**: Pre-allocated internally for request tables
