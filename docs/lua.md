# Lua Scripting

## ⚠️ Critical: Script Structure Order

**Chi router requires middleware to be defined BEFORE routes, or the application will panic:**

```lua
-- ✅ CORRECT: Middleware first, then routes
chi_middleware("/*", function(request, response, next)
    response:header("X-Gateway", "Keystone")
    next()
end)

chi_route("GET", "/health", function(request, response)
    response:write("OK")
end)
```

```lua
-- ❌ WRONG: Routes before middleware will cause panic
chi_route("GET", "/health", function(request, response)
    response:write("OK")
end)

chi_middleware("/*", function(request, response, next)
    next()  -- This will crash the application!
end)
```

## Script Template

```lua
-- Keystone Gateway Lua Script Template

-- STEP 1: Define ALL middleware first
chi_middleware("/*", function(request, response, next)
    response:header("X-Powered-By", "Keystone-Gateway")
    next()  -- Always call next() to continue
end)

-- STEP 2: Define routes after middleware
chi_route("GET", "/health", function(request, response)
    response:header("Content-Type", "application/json")
    response:write('{"status": "healthy"}')
end)

-- STEP 3: Route groups also after middleware
chi_group("/api/v1", function()
    chi_route("GET", "/users", function(request, response)
        response:write("Users endpoint")
    end)
end)
```

## Core Functions

### Routes
```lua
chi_route("GET", "/users", function(request, response)
    response:write("Hello")
end)

chi_route("POST", "/users/{id}", function(request, response)
    local id = chi_param(request, "id")
    response:write("User: " .. id)
end)
```

### Middleware
```lua
chi_middleware("/api/*", function(request, response, next)
    response:header("X-Gateway", "Keystone")
    next()
end)
```

### Groups
```lua
chi_group("/api", function()
    chi_route("GET", "/users", users_handler)
    chi_route("GET", "/orders", orders_handler)
end)
```

## Request Object

```lua
request.method          -- "GET", "POST", etc
request.path           -- "/api/users"
request.body           -- request body string
request.headers        -- table: request.headers["Content-Type"]
request.remote_addr    -- client IP
```

## Response Object

```lua
response:status(201)                    -- set status code
response:header("Content-Type", "json") -- set header
response:write("Hello World")           -- write body
```

## Utility Functions

```lua
chi_param(request, "id")     -- get URL parameter
log("Debug message")         -- log to gateway
```

## Examples

### Complete Authentication Script
```lua
-- Authentication middleware (defined first)
chi_middleware("/api/*", function(request, response, next)
    local token = request.headers["Authorization"]
    if not token then
        response:status(401)
        response:write('{"error": "No token"}')
        return  -- Don't call next() - stops the request
    end
    next()  -- Token exists, continue to routes
end)

-- Protected routes (defined after middleware)
chi_route("GET", "/api/users", function(request, response)
    response:header("Content-Type", "application/json")
    response:write('{"users": ["alice", "bob"]}')
end)
```

### Load Balancing
Automatically handled by service configuration. Lua defines routes, gateway handles backends.

### Health Checks
```lua
chi_route("GET", "/health", function(request, response)
    response:header("Content-Type", "application/json")
    response:write('{"status": "ok"}')
end)
```

## Best Practices

1. **⚠️ ALWAYS define middleware before routes** - Chi router requirement
2. Keep scripts simple and focused
3. Use middleware for cross-cutting concerns (auth, headers, logging)
4. Let the gateway handle backend routing and load balancing
5. Add logging for debugging: `log("message")`
6. Handle errors gracefully with proper status codes
7. Always call `next()` in middleware to continue the request chain
8. Use clear comments to separate middleware and route sections

See `scripts/lua/examples/` for working examples.
