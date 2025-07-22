-- Basic Routes Example
-- Simple routing script for getting started

-- Welcome endpoint  
chi_route("GET", "/", function(request, response)
    response:header("Content-Type", "application/json")
    response:write('{"message": "Welcome to Keystone Gateway", "version": "1.0.0"}')
end)

-- API health check
chi_route("GET", "/api/health", function(request, response)
    response:header("Content-Type", "application/json")
    response:write('{"status": "healthy", "service": "keystone-gateway"}')
end)