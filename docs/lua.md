# Lua Scripting

## ⚠️ Critical: Script Structure Order

**Chi router requires middleware to be defined BEFORE routes, or the application will panic:**

```lua
-- ✅ CORRECT: Middleware first, then routes
chi_middleware(function(request, response, next)
    response_header(response, "X-Gateway", "Keystone")
    next()
end)

chi_route("GET", "/health", function(request, response)
    response_write(response, "OK")
end)
```

```lua
-- ❌ WRONG: Routes before middleware will cause panic
chi_route("GET", "/health", function(request, response)
    response_write(response, "OK")
end)

chi_middleware(function(request, response, next)
    next()  -- This will crash the application!
end)
```

## Script Template

```lua
-- Keystone Gateway Lua Script Template

-- STEP 1: Define ALL middleware first
chi_middleware(function(request, response, next)
    response_header(response, "X-Powered-By", "Keystone-Gateway")
    next()  -- Always call next() to continue
end)

-- STEP 2: Define routes after middleware
chi_route("GET", "/health", function(request, response)
    response_header(response, "Content-Type", "application/json")
    response_write(response, '{"status": "healthy"}')
end)

-- STEP 3: Route groups also after middleware
chi_group(function()
    chi_route("GET", "/users", function(request, response)
        response_write(response, "Users endpoint")
    end)
end)

## Core Functions

### Routes
```lua
-- Basic route registration
chi_route("GET", "/users", function(request, response)
    response_write(response, "Hello")
end)

-- Route with URL parameters
chi_route("POST", "/users/{id}", function(request, response)
    local id = chi_param(request, "id")
    response_write(response, "User: " .. id)
end)
```

### Middleware
```lua
-- Global middleware (runs on all requests)
chi_middleware(function(request, response, next)
    response_header(response, "X-Gateway", "Keystone")
    next()  -- Continue to next middleware/route
end)
```

### Route Groups
```lua
-- Simple route group
chi_group(function()
    chi_route("GET", "/users", function(request, response)
        response_write(response, "Users")
    end)
    chi_route("GET", "/orders", function(request, response)
        response_write(response, "Orders")
    end)
end)

-- Pattern-based route group
chi_route_group("/api/v1", function()
    chi_route("GET", "/status", function(request, response)
        response_write(response, "API v1 Status")
    end)
end)

-- Mount handlers at specific patterns
chi_mount("/static", function(request, response)
    response_write(response, "Static content")
end)
```

### Error Handlers
```lua
-- Custom 404 handler
chi_not_found(function(request, response)
    response_status(response, 404)
    response_header(response, "Content-Type", "application/json")
    response_write(response, '{"error": "Not Found"}')
end)

-- Custom 405 handler (method not allowed)
chi_method_not_allowed(function(request, response)
    response_status(response, 405)
    response_write(response, '{"error": "Method Not Allowed"}')
end)
```

## Request Functions

```lua
-- Access request properties
local method = request_method(request)      -- "GET", "POST", etc.
local url = request_url(request)            -- Full URL string
local body = request_body(request)          -- Request body (cached, size-limited)
local header = request_header(request, "Authorization")  -- Get specific header

-- URL parameters
local id = chi_param(request, "id")         -- Get URL parameter

-- Context caching (for expensive operations)
chi_context_set(request, "user_id", "123")  -- Cache a value
local user_id = chi_context_get(request, "user_id")  -- Retrieve cached value
```

## Response Functions

```lua
-- Set response status and headers
response_status(response, 201)
response_header(response, "Content-Type", "application/json")
response_header(response, "Cache-Control", "no-cache")

-- Write response body
response_write(response, "Hello World")
response_write(response, '{"message": "success"}')
```

## HTTP Client Functions

```lua
-- GET request
local body, status, headers = http_get("https://api.example.com/data")
if status == 200 then
    response_write(response, body)
end

-- GET with custom headers
local custom_headers = {
    ["Authorization"] = "Bearer token123",
    ["User-Agent"] = "Keystone-Gateway"
}
local body, status, headers = http_get("https://api.example.com/data", custom_headers)

-- POST request
local post_data = '{"name": "John"}'
local body, status, headers = http_post("https://api.example.com/users", post_data)

-- POST with custom headers
local body, status, headers = http_post("https://api.example.com/users", post_data, custom_headers)
```

## Utility Functions

```lua
-- Logging
log("Debug message")                    -- Log to gateway

-- Environment variables
local db_url = get_env("DATABASE_URL")  -- ⚠️ Security: Use with caution
```

## Complete Examples

### Authentication & Authorization
```lua
-- Authentication middleware (defined first)
chi_middleware(function(request, response, next)
    local url = request_url(request)

    -- Only check auth on /api/* paths
    if url:match("/api/") then
        local token = request_header(request, "Authorization")
        if not token then
            response_status(response, 401)
            response_header(response, "Content-Type", "application/json")
            response_write(response, '{"error": "No token"}')
            return  -- Don't call next() - stops the request
        end

        -- Cache user info for later use
        chi_context_set(request, "authenticated", "true")
    end

    next()  -- Continue to routes
end)

-- Protected routes (defined after middleware)
chi_route("GET", "/api/users", function(request, response)
    response_header(response, "Content-Type", "application/json")
    response_write(response, '{"users": ["alice", "bob"]}')
end)
```

### Proxy with External API
```lua
-- Proxy requests to external API
chi_route("GET", "/proxy/{path}", function(request, response)
    local path = chi_param(request, "path")
    local upstream_url = "https://api.example.com/" .. path

    -- Forward auth header
    local auth = request_header(request, "Authorization")
    local headers = {}
    if auth ~= "" then
        headers["Authorization"] = auth
    end

    local body, status, resp_headers = http_get(upstream_url, headers)

    -- Forward response headers
    for k, v in pairs(resp_headers) do
        response_header(response, k, v)
    end

    response_status(response, status)
    response_write(response, body)
end)
```

### Request Body Processing
```lua
-- Handle POST requests with JSON body
chi_route("POST", "/api/data", function(request, response)
    local body = request_body(request)
    local method = request_method(request)

    log("Received " .. method .. " with body length: " .. #body)

    -- Process the data (parse JSON, validate, etc.)
    if #body > 0 then
        response_status(response, 201)
        response_header(response, "Content-Type", "application/json")
        response_write(response, '{"message": "Data received"}')
    else
        response_status(response, 400)
        response_write(response, '{"error": "Empty body"}')
    end
end)
```

### Load Balancing
Automatically handled by service configuration. Lua defines routes, gateway handles backends.

### Health Checks
```lua
chi_route("GET", "/health", function(request, response)
    response_header(response, "Content-Type", "application/json")
    response_write(response, '{"status": "ok", "timestamp": "' .. os.date() .. '"}')
end)
```

## Request Limits & Security

### Request Body Size Limits
Request bodies are automatically limited by configuration:
```lua
-- Request body reading respects configured limits
local body = request_body(request)  -- Limited by max_body_size config
```

If request body exceeds the limit, the function will raise an error. Configure limits in your config file:
```yaml
request_limits:
  max_body_size: 10485760    # 10MB (default)
  max_header_size: 1048576   # 1MB (default)
  max_url_size: 8192         # 8KB (default)
```

### Security Considerations

⚠️ **File System Access**: The current implementation allows Lua scripts to access the file system via `io.open()`. This should be used with extreme caution in multi-tenant environments.

⚠️ **Environment Variables**: The `get_env()` function provides access to all environment variables. Avoid using this in shared environments or sanitize access to specific variables only.

⚠️ **HTTP Client**: External HTTP requests have a 5-second timeout to prevent hanging. Consider implementing additional rate limiting for production use.

## Best Practices

1. **⚠️ ALWAYS define middleware before routes** - Chi router requirement
2. Keep scripts simple and focused
3. Use middleware for cross-cutting concerns (auth, headers, logging)
4. Let the gateway handle backend routing and load balancing
5. Add logging for debugging: `log("message")`
6. Handle errors gracefully with proper status codes
7. Always call `next()` in middleware to continue the request chain
8. Use context caching (`chi_context_set/get`) for expensive operations
9. Validate and sanitize all input data
10. Use clear comments to separate middleware and route sections

## Performance Notes

- **Request body caching**: Bodies are read once and cached per request
- **Bytecode compilation**: Scripts are compiled to bytecode and cached
- **State pooling**: Lua states are pooled for thread safety and performance
- **HTTP client**: Shared HTTP client with connection reuse

See `scripts/lua/examples/` for working examples.
