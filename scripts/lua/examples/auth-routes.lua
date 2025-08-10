-- Authentication Routes Example for Keystone Gateway
-- This script demonstrates how to implement authentication middleware and routes

-- STEP 1: Define ALL middleware FIRST (Chi router requirement)

-- Global headers middleware
chi_middleware("/*", function(request, response, next)
    response:header("X-Powered-By", "Keystone-Gateway")
    response:header("Content-Type", "application/json")
    next()
end)

-- Authentication middleware - checks for API key on protected routes
chi_middleware("/api/*", function(request, response, next)
    local api_key = request:header("X-API-Key")

    if not api_key or api_key == "" then
        response:status(401)
        response:write('{"error": "Missing API key", "message": "X-API-Key header is required"}')
        return  -- Don't call next() - stops the request
    end

    -- In a real implementation, you would validate the API key against a database
    if api_key ~= "valid-api-key-123" then
        response:status(403)
        response:write('{"error": "Invalid API key", "message": "The provided API key is not valid"}')
        return  -- Don't call next() - stops the request
    end

    -- API key is valid, continue to protected routes
    next()
end)

-- STEP 2: Define ALL routes AFTER middleware

-- Public login route (not under /api/* so not protected)
chi_route("POST", "/auth/login", function(request, response)
    -- In a real implementation, you would validate credentials
    local body = request:body()
    response:write('{"token": "valid-api-key-123", "expires_in": 3600, "message": "Login successful"}')
end)

-- Protected route that requires authentication (under /api/* so protected)
chi_route("GET", "/api/profile", function(request, response)
    response:write('{"user_id": "user123", "profile": {"name": "Test User", "email": "test@example.com"}}')
end)

-- Another protected route
chi_route("GET", "/api/dashboard", function(request, response)
    response:write('{"dashboard": "Welcome to your dashboard", "user": "Test User"}')
end)

print("✅ Authentication routes loaded successfully")
