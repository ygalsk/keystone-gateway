
-- Custom API routes
chi_route("GET", "/health", function(request, response)
    response:header("Content-Type", "application/json")
    response:write('{"status": "healthy", "service": "keystone-gateway"}')
end)

chi_route("GET", "/version", function(request, response)
    response:header("Content-Type", "application/json")
    response:write('{"version": "1.0.0", "build": "lua-powered"}')
end)
