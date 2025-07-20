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

-- Simple API endpoint
chi_route("GET", "/api/hello", function(request, response)
    response:header("Content-Type", "application/json")
    response:write('{"message": "Hello from Keystone Gateway!"}')
end)

-- Basic middleware for all API routes
chi_middleware("/api/*", function(request, response, next)
    -- Add common headers
    response:header("X-Gateway", "Keystone")
    response:header("X-Version", "1.0.0")
    
    -- Log request
    log("API Request: " .. request.method .. " " .. request.path)
    
    -- Continue to next handler
    next()
end)