# Lua Scripting Guide

Keystone Gateway's power comes from embedded Lua scripting that allows you to define dynamic routes, middleware, and request processing logic without recompiling the gateway.

## Table of Contents

- [Quick Start](#quick-start)
- [Core Functions](#core-functions)
- [Request and Response Objects](#request-and-response-objects)
- [Middleware Patterns](#middleware-patterns)
- [Route Groups](#route-groups)
- [Examples](#examples)
- [Best Practices](#best-practices)
- [Security Considerations](#security-considerations)
- [Debugging](#debugging)

## Quick Start

Create a simple Lua routing script:

```lua
-- hello.lua
chi_route("GET", "/hello", function(request, response)
    response:header("Content-Type", "application/json")
    response:write('{"message": "Hello World!"}')
end)
```

Reference this script in your configuration:

```yaml
tenants:
  - name: "api"
    domains: ["localhost"]
    lua_routes: "hello.lua"
    services: [...]
```

## Core Functions

### Route Registration

#### `chi_route(method, pattern, handler)`

Register an HTTP route with a handler function.

**Parameters:**
- `method` (string): HTTP method ("GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS")
- `pattern` (string): URL pattern (supports Chi router patterns)
- `handler` (function): Handler function that receives `(request, response)` parameters

**Examples:**
```lua
-- Simple GET route
chi_route("GET", "/users", function(request, response)
    response:write("User list")
end)

-- Route with parameters
chi_route("GET", "/users/{id}", function(request, response)
    local user_id = chi_param(request, "id")
    response:write("User ID: " .. user_id)
end)

-- POST route with body handling
chi_route("POST", "/users", function(request, response)
    local body = request.body or ""
    log("Creating user: " .. body)
    response:status(201)
    response:write("User created")
end)
```

**URL Patterns:**
```lua
-- Static routes
chi_route("GET", "/health", handler)

-- Parameters
chi_route("GET", "/users/{id}", handler)
chi_route("GET", "/files/{category}/{filename}", handler)

-- Wildcards
chi_route("GET", "/static/*", handler)

-- Optional parameters
chi_route("GET", "/posts/{slug}", handler)
chi_route("GET", "/posts/{slug}/comments/{id}", handler)
```

### Middleware Registration

#### `chi_middleware(pattern, middleware_func)`

Register middleware that executes before route handlers.

**Parameters:**
- `pattern` (string): URL pattern to match
- `middleware_func` (function): Middleware function that receives `(request, response, next)`

**Examples:**
```lua
-- Authentication middleware
chi_middleware("/api/*", function(request, response, next)
    local auth_header = request.headers["Authorization"]
    if not auth_header then
        response:status(401)
        response:write("Unauthorized")
        return
    end
    
    -- Add user info to request context
    request.user_id = extract_user_id(auth_header)
    next()
end)

-- CORS middleware
chi_middleware("/*", function(request, response, next)
    response:header("Access-Control-Allow-Origin", "*")
    response:header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE")
    next()
end)

-- Logging middleware
chi_middleware("/*", function(request, response, next)
    local start_time = os.clock()
    next()
    local duration = os.clock() - start_time
    log(request.method .. " " .. request.path .. " - " .. duration .. "s")
end)
```

### Route Groups

#### `chi_group(pattern, setup_func)`

Create a group of routes with shared middleware and configuration.

**Parameters:**
- `pattern` (string): Base path for the group
- `setup_func` (function): Function to set up routes within the group

**Examples:**
```lua
-- API v1 group
chi_group("/api/v1", function()
    -- Middleware for all v1 routes
    chi_middleware("/*", function(request, response, next)
        response:header("API-Version", "v1")
        next()
    end)
    
    -- Routes within the group
    chi_route("GET", "/users", users_handler)
    chi_route("POST", "/users", create_user_handler)
    chi_route("GET", "/posts", posts_handler)
end)

-- Admin group with authentication
chi_group("/admin", function()
    chi_middleware("/*", admin_auth_middleware)
    
    chi_route("GET", "/dashboard", admin_dashboard)
    chi_route("GET", "/users", admin_users)
    chi_route("POST", "/settings", update_settings)
end)
```

### Utility Functions

#### `chi_param(request, param_name)`

Extract URL parameters from the request.

```lua
chi_route("GET", "/users/{id}/posts/{post_id}", function(request, response)
    local user_id = chi_param(request, "id")
    local post_id = chi_param(request, "post_id")
    response:write("User: " .. user_id .. ", Post: " .. post_id)
end)
```

#### `log(message)`

Log messages to the gateway's logging system.

```lua
log("Processing request for user: " .. user_id)
log("Error: " .. error_message)
```

## Request and Response Objects

### Request Object

The request object contains information about the incoming HTTP request:

```lua
function handler(request, response)
    -- HTTP method
    local method = request.method  -- "GET", "POST", etc.
    
    -- Request path
    local path = request.path      -- "/api/users/123"
    
    -- Headers (table)
    local content_type = request.headers["Content-Type"]
    local auth = request.headers["Authorization"]
    
    -- Request body (string)
    local body = request.body or ""
    
    -- Custom properties (set by middleware)
    local user_id = request.user_id
end
```

**Available Properties:**
- `request.method` - HTTP method
- `request.path` - Request path
- `request.headers` - Table of HTTP headers
- `request.body` - Request body as string
- Custom properties can be added by middleware

### Response Object

The response object is used to send HTTP responses:

#### `response:header(name, value)`

Set a response header.

```lua
response:header("Content-Type", "application/json")
response:header("Cache-Control", "max-age=3600")
response:header("X-Custom-Header", "value")
```

#### `response:status(code)`

Set the HTTP status code.

```lua
response:status(200)    -- OK
response:status(201)    -- Created
response:status(400)    -- Bad Request
response:status(401)    -- Unauthorized
response:status(404)    -- Not Found
response:status(500)    -- Internal Server Error
```

#### `response:write(content)`

Write content to the response body.

```lua
-- Plain text
response:write("Hello World")

-- JSON (manual)
response:header("Content-Type", "application/json")
response:write('{"message": "Hello", "status": "success"}')

-- HTML
response:header("Content-Type", "text/html")
response:write("<h1>Welcome</h1>")
```

## Middleware Patterns

### Authentication Middleware

```lua
function auth_middleware(request, response, next)
    local token = request.headers["Authorization"]
    
    if not token then
        response:status(401)
        response:header("Content-Type", "application/json")
        response:write('{"error": "Missing authorization token"}')
        return
    end
    
    -- Validate token (simplified)
    if not is_valid_token(token) then
        response:status(401)
        response:header("Content-Type", "application/json")
        response:write('{"error": "Invalid token"}')
        return
    end
    
    -- Add user context
    request.user = get_user_from_token(token)
    next()
end

-- Apply to protected routes
chi_middleware("/api/protected/*", auth_middleware)
```

### Rate Limiting Middleware

```lua
-- Simple rate limiting (in-memory)
local rate_limits = {}

function rate_limit_middleware(request, response, next)
    local client_ip = request.headers["X-Real-IP"] or "unknown"
    local current_time = os.time()
    
    -- Initialize or get existing rate limit data
    if not rate_limits[client_ip] then
        rate_limits[client_ip] = {count = 0, window_start = current_time}
    end
    
    local limit_data = rate_limits[client_ip]
    
    -- Reset window if expired (60 second window)
    if current_time - limit_data.window_start > 60 then
        limit_data.count = 0
        limit_data.window_start = current_time
    end
    
    -- Check limit (100 requests per minute)
    if limit_data.count >= 100 then
        response:status(429)
        response:header("Content-Type", "application/json")
        response:write('{"error": "Rate limit exceeded"}')
        return
    end
    
    limit_data.count = limit_data.count + 1
    next()
end

chi_middleware("/api/*", rate_limit_middleware)
```

### CORS Middleware

```lua
function cors_middleware(request, response, next)
    response:header("Access-Control-Allow-Origin", "*")
    response:header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
    response:header("Access-Control-Allow-Headers", "Content-Type, Authorization")
    
    -- Handle preflight requests
    if request.method == "OPTIONS" then
        response:status(200)
        response:write("")
        return
    end
    
    next()
end

chi_middleware("/*", cors_middleware)
```

## Examples

### Complete Authentication System

```lua
-- Authentication routes with user management
chi_group("/auth", function()
    -- Login endpoint
    chi_route("POST", "/login", function(request, response)
        local credentials = parse_json(request.body)
        
        if validate_credentials(credentials.username, credentials.password) then
            local token = generate_jwt_token(credentials.username)
            response:header("Content-Type", "application/json")
            response:write('{"token": "' .. token .. '", "status": "success"}')
        else
            response:status(401)
            response:header("Content-Type", "application/json")
            response:write('{"error": "Invalid credentials"}')
        end
    end)
    
    -- Protected user profile
    chi_route("GET", "/profile", function(request, response)
        response:header("Content-Type", "application/json")
        response:write('{"user": "' .. request.user .. '", "role": "user"}')
    end)
end)

-- Apply auth middleware to protected routes
chi_middleware("/auth/profile", auth_middleware)
```

### A/B Testing Implementation

```lua
-- A/B testing with feature flags
function ab_test_middleware(request, response, next)
    local user_agent = request.headers["User-Agent"] or ""
    local test_variant = "A"
    
    -- Simple hash-based assignment
    local hash = string.len(user_agent) % 2
    if hash == 1 then
        test_variant = "B"
    end
    
    request.test_variant = test_variant
    response:header("X-Test-Variant", test_variant)
    next()
end

chi_middleware("/api/*", ab_test_middleware)

chi_route("GET", "/api/feature", function(request, response)
    local variant = request.test_variant
    local feature_data = {}
    
    if variant == "A" then
        feature_data = {feature = "original", color = "blue"}
    else
        feature_data = {feature = "new", color = "green"}
    end
    
    response:header("Content-Type", "application/json")
    response:write('{"variant": "' .. variant .. '", "data": ' .. json_encode(feature_data) .. '}')
end)
```

## Best Practices

### Performance

1. **Minimize global variables** - Use local variables when possible
2. **Cache expensive operations** - Store results of complex calculations
3. **Avoid blocking operations** - Keep request processing fast
4. **Use efficient string operations** - Prefer string concatenation patterns

```lua
-- Good: Local variables and efficient string building
local function build_response(user_id, message)
    local parts = {}
    parts[1] = '{"user_id": "'
    parts[2] = user_id
    parts[3] = '", "message": "'
    parts[4] = message
    parts[5] = '"}'
    return table.concat(parts)
end

-- Avoid: Global variables and inefficient string concatenation
```

### Error Handling

```lua
function safe_handler(request, response)
    local success, result = pcall(function()
        -- Your route logic here
        return process_request(request)
    end)
    
    if not success then
        log("Error in handler: " .. tostring(result))
        response:status(500)
        response:header("Content-Type", "application/json")
        response:write('{"error": "Internal server error"}')
        return
    end
    
    response:write(result)
end
```

### Code Organization

```lua
-- Separate concerns into functions
local function validate_request(request)
    -- Validation logic
end

local function process_user_data(data)
    -- Business logic
end

local function format_response(data)
    -- Response formatting
end

-- Use the functions in routes
chi_route("POST", "/users", function(request, response)
    if not validate_request(request) then
        response:status(400)
        response:write("Invalid request")
        return
    end
    
    local result = process_user_data(request.body)
    local formatted = format_response(result)
    
    response:header("Content-Type", "application/json")
    response:write(formatted)
end)
```

## Security Considerations

### Input Validation

```lua
function validate_json_input(request, response, next)
    local body = request.body or ""
    
    -- Check content type
    if request.headers["Content-Type"] ~= "application/json" then
        response:status(400)
        response:write("Content-Type must be application/json")
        return
    end
    
    -- Validate JSON
    local success, data = pcall(parse_json, body)
    if not success then
        response:status(400)
        response:write("Invalid JSON")
        return
    end
    
    request.json_data = data
    next()
end
```

### SQL Injection Prevention

```lua
-- Don't do this (vulnerable to injection)
local function bad_example(user_input)
    local query = "SELECT * FROM users WHERE name = '" .. user_input .. "'"
    return execute_query(query)
end

-- Do this instead (parameterized queries)
local function safe_example(user_input)
    local query = "SELECT * FROM users WHERE name = ?"
    return execute_query(query, {user_input})
end
```

### Sensitive Data Handling

```lua
-- Never log sensitive data
function login_handler(request, response)
    local credentials = parse_json(request.body)
    
    -- DON'T DO THIS
    -- log("Login attempt: " .. request.body)
    
    -- DO THIS
    log("Login attempt for user: " .. (credentials.username or "unknown"))
    
    -- Process login...
end
```

## Debugging

### Logging

```lua
-- Use structured logging
function debug_request(request, response, next)
    log("Request: " .. request.method .. " " .. request.path)
    log("Headers: " .. table_to_string(request.headers))
    
    if request.body and string.len(request.body) > 0 then
        log("Body length: " .. string.len(request.body))
    end
    
    next()
end

-- Conditional debug middleware
if os.getenv("DEBUG") == "true" then
    chi_middleware("/*", debug_request)
end
```

### Error Information

```lua
function error_handler(request, response)
    local success, result = pcall(function()
        return risky_operation(request)
    end)
    
    if not success then
        -- Log detailed error for debugging
        log("Error in " .. request.path .. ": " .. tostring(result))
        
        -- Return generic error to client
        response:status(500)
        response:header("Content-Type", "application/json")
        response:write('{"error": "Internal server error", "request_id": "' .. request.id .. '"}')
        return
    end
    
    response:write(result)
end
```

### Testing Routes

```lua
-- Add test endpoints for development
if os.getenv("ENVIRONMENT") == "development" then
    chi_route("GET", "/debug/state", function(request, response)
        response:header("Content-Type", "application/json")
        response:write('{"lua_version": "' .. _VERSION .. '", "time": "' .. os.date() .. '"}')
    end)
    
    chi_route("POST", "/debug/echo", function(request, response)
        response:header("Content-Type", "application/json")
        response:write('{"received": ' .. (request.body or '""') .. '}')
    end)
end
```

For more examples, see the [scripts/examples/](../scripts/examples/) directory in the repository.