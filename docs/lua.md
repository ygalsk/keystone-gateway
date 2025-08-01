# Lua Scripting

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

### Authentication
```lua
chi_middleware("/api/*", function(request, response, next)
    local token = request.headers["Authorization"]
    if not token then
        response:status(401)
        response:write('{"error": "No token"}')
        return
    end
    next()
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

1. Keep scripts simple
2. Use middleware for cross-cutting concerns
3. Let the gateway handle backend routing
4. Add logging for debugging: `log("message")`
5. Handle errors gracefully

See `scripts/lua/examples/` for working examples.