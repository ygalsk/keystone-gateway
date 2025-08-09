-- Authentication Routes Example for Keystone Gateway
-- This script demonstrates how to implement authentication middleware and routes

-- Authentication middleware - checks for API key
chi_middleware("/api/*", function(request, response, next)
    local api_key = request:header("X-API-Key")

    if not api_key or api_key == "" then
        response:header("Content-Type", "application/json")
        response:status(401)
        response:write('{"error": "Missing API key", "message": "X-API-Key header is required"}')
        return
    end

    -- In a real implementation, you would validate the API key against a database
    if api_key ~= "valid-api-key-123" then
        response:header("Content-Type", "application/json")
        response:status(403)
        response:write('{"error": "Invalid API key", "message": "The provided API key is not valid"}')
        return
    end

    -- Add user context to request (this would come from key validation)
    request:set_context("user_id", "user123")
    request:set_context("api_key", api_key)

    next()
end)

-- Protected route that requires authentication
chi_route("GET", "/api/profile", function(request, response)
    local user_id = request:get_context("user_id")

    response:header("Content-Type", "application/json")
    response:write('{"user_id": "' .. user_id .. '", "profile": {"name": "Test User", "email": "test@example.com"}}')
end)

-- Public login route
chi_route("POST", "/auth/login", function(request, response)
    -- In a real implementation, you would validate credentials
    local body = request:body()

    response:header("Content-Type", "application/json")
    response:write('{"token": "valid-api-key-123", "expires_in": 3600, "message": "Login successful"}')
end)

print("✅ Authentication routes loaded successfully")
