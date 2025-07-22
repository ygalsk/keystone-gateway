-- Development Routes Example
-- Routes for v2 API tenant

-- Health check - same pattern as basic-routes but different tenant
chi_route("GET", "/health", function(request, response)
    response:header("Content-Type", "application/json")
    response:write('{"status": "healthy", "service": "v2-api", "tenant": "v2"}')
end)

-- V2 specific endpoints
chi_route("GET", "/users", function(request, response)
    response:header("Content-Type", "application/json")
    response:write('{"users": [], "version": "2.0", "tenant": "v2"}')
end)

chi_route("POST", "/users", function(request, response)
    response:header("Content-Type", "application/json")
    response:status(201)
    response:write('{"message": "User created", "version": "2.0", "tenant": "v2"}')
end)

-- Middleware for all v2 routes
chi_middleware("/*", function(request, response, next)
    response:header("X-Tenant", "v2")
    response:header("X-API-Version", "2.0")
    log("V2 Request: " .. request.method .. " " .. request.path)
    next()
end)