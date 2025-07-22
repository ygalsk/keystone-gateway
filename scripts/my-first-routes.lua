-- Host-based Routes Example
-- Routes for app1 domain-based tenant

-- Root endpoint for app1
chi_route("GET", "/", function(request, response)
    response:header("Content-Type", "application/json")
    response:write('{"message": "Welcome to App1", "tenant": "app1", "host": "' .. request.headers["Host"] .. '"}')
end)

-- Same path as other tenants but different handler due to host-based routing
chi_route("GET", "/health", function(request, response)
    response:header("Content-Type", "application/json")
    response:write('{"status": "healthy", "service": "app1", "tenant": "app1"}')
end)

-- App1 specific API
chi_route("GET", "/api/data", function(request, response)
    response:header("Content-Type", "application/json")
    response:write('{"data": "app1-specific-data", "tenant": "app1"}')
end)

-- Host-specific middleware
chi_middleware("/*", function(request, response, next)
    response:header("X-App", "app1")
    response:header("X-Host-Based", "true")
    log("App1 Request: " .. request.method .. " " .. request.path .. " on host: " .. request.headers["Host"])
    next()
end)