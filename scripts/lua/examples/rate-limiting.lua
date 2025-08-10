-- Rate Limiting Example for Keystone Gateway
-- This script demonstrates how to implement rate limiting middleware

-- Simple in-memory rate limiter (use Redis in production)
local rate_limits = {}
local window_size = 60 -- 60 seconds
local max_requests = 100 -- 100 requests per minute

-- STEP 1: Define middleware FIRST (Chi router requirement)
chi_middleware("/api/*", function(request, response, next)
    local client_ip = request:remote_addr()
    local current_time = os.time()
    local window_start = math.floor(current_time / window_size) * window_size

    -- Initialize or clean old entries
    if not rate_limits[client_ip] then
        rate_limits[client_ip] = {count = 0, window = window_start}
    elseif rate_limits[client_ip].window < window_start then
        rate_limits[client_ip] = {count = 0, window = window_start}
    end

    -- Check rate limit
    if rate_limits[client_ip].count >= max_requests then
        response:header("Content-Type", "application/json")
        response:header("X-RateLimit-Limit", tostring(max_requests))
        response:header("X-RateLimit-Remaining", "0")
        response:header("X-RateLimit-Reset", tostring(window_start + window_size))
        response:status(429)
        response:write('{"error": "Rate limit exceeded", "message": "Too many requests. Try again later."}')
        return
    end

    -- Increment counter
    rate_limits[client_ip].count = rate_limits[client_ip].count + 1

    -- Add rate limit headers
    response:header("X-RateLimit-Limit", tostring(max_requests))
    response:header("X-RateLimit-Remaining", tostring(max_requests - rate_limits[client_ip].count))
    response:header("X-RateLimit-Reset", tostring(window_start + window_size))

    next()
end)

-- STEP 2: Define routes AFTER middleware
chi_route("GET", "/api/test", function(request, response)
    response:header("Content-Type", "application/json")
    response:write('{"message": "Request successful", "timestamp": "' .. os.date("!%Y-%m-%dT%H:%M:%SZ") .. '"}')
end)

print("✅ Rate limiting middleware loaded successfully")
