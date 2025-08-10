-- Keystone Gateway API Routes Example
-- Demonstrates basic middleware and routes following best practices

-- STEP 1: Define middleware FIRST (Chi router requirement)
chi_middleware("/*", function(request, response, next)
    response:header("X-Gateway", "Keystone-Production")
    response:header("X-Powered-By", "Keystone-Gateway")
    response:header("Access-Control-Allow-Origin", "*")
    next()
end)

-- STEP 2: Define routes AFTER middleware
chi_route("GET", "/health", function(request, response)
    response:header("Content-Type", "application/json")
    response:write('{"status": "healthy", "service": "keystone-gateway", "timestamp": "' .. os.date("!%Y-%m-%dT%H:%M:%SZ") .. '"}')
end)

chi_route("GET", "/time", function(request, response)
    response:header("Content-Type", "application/json")
    response:write('{"server_time": "' .. os.date("!%Y-%m-%dT%H:%M:%SZ") .. '", "gateway": "keystone"}')
end)

print("✅ API Routes loaded successfully")
