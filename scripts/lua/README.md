# Lua Scripts Directory

This directory contains Lua scripts for dynamic routing and middleware in Keystone Gateway.

## Directory Structure

```
scripts/lua/
├── examples/          # Example scripts for common patterns
│   ├── api-routes.lua     # Basic API routes example
│   ├── auth-routes.lua    # Authentication middleware example  
│   └── rate-limiting.lua  # Rate limiting example
├── utils/             # Reusable utility functions
│   └── common.lua         # Common utility functions
└── README.md          # This file
```

## Using Lua Scripts

### Script Registration
Scripts are automatically discovered by the gateway when placed in this directory or subdirectories. Scripts should have a `.lua` extension and follow the naming conventions:

- **Route scripts**: `script-name.lua` - Register routes for specific tenants
- **Global scripts**: `global-script-name.lua` - Apply to all tenants
- **Utility scripts**: Place in `utils/` directory for reuse

### Available Functions

#### Route Registration
```lua
-- Register HTTP routes
chi_route("GET", "/api/users", handler_function)
chi_route("POST", "/api/users", create_user_handler)
chi_route("PUT", "/api/users/{id}", update_user_handler)
chi_route("DELETE", "/api/users/{id}", delete_user_handler)
```

#### Middleware Registration
```lua
-- Apply middleware to route patterns
chi_middleware("/api/*", auth_middleware)
chi_middleware("/admin/*", admin_auth_middleware)
```

#### Route Groups
```lua
-- Group related routes
chi_group("/api/v1", function()
    chi_route("GET", "/users", users_handler)
    chi_route("POST", "/users", create_user_handler)
end)
```

#### Utility Functions (from utils/common.lua)
```lua
-- Get current timestamp
local timestamp = get_timestamp()

-- Set CORS headers
set_cors_headers(response, "*")

-- Send JSON response
json_response(response, '{"message": "Hello"}', 200)

-- Send health check response
health_response(response, "my-service")
```

## Best Practices

1. **Use descriptive script names** that clearly indicate their purpose
2. **Include error handling** in your route handlers
3. **Leverage utility functions** from `utils/common.lua` to avoid code duplication
4. **Test scripts thoroughly** before deploying to production
5. **Document complex routing logic** with comments
6. **Follow consistent naming conventions** for routes and handlers

## Security Considerations

- **Validate all input** in route handlers
- **Implement proper authentication** for protected routes
- **Use rate limiting** to prevent abuse
- **Sanitize output** to prevent injection attacks
- **Keep secrets out of scripts** - use environment variables or secure configuration

## Examples

See the `examples/` directory for complete examples of common patterns:
- **API Routes**: Basic CRUD operations and health checks
- **Authentication**: API key validation and protected routes
- **Rate Limiting**: Request throttling middleware

## Debugging

Use the built-in `log()` function to debug your scripts:

```lua
chi_route("GET", "/debug", function(request, response)
    log("Debug route called by: " .. request:remote_addr())
    response:write("Debug info logged")
end)
```