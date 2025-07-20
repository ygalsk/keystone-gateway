-- Development Routes
-- Routes for local development and testing

-- Development info endpoint
chi_route("GET", "/dev/info", function(request, response)
    response:header("Content-Type", "application/json")
    response:write('{"environment": "development", "debug": true, "timestamp": "' .. os.date() .. '"}')
end)

-- Echo endpoint for testing
chi_route("POST", "/dev/echo", function(request, response)
    response:header("Content-Type", "application/json")
    local body = request.body or ""
    response:write('{"received": "' .. body .. '", "method": "' .. request.method .. '"}')
end)

-- Headers inspection
chi_route("GET", "/dev/headers", function(request, response)
    response:header("Content-Type", "application/json")
    local headers_json = "{"
    for key, value in pairs(request.headers) do
        headers_json = headers_json .. '"' .. key .. '": "' .. value .. '",'
    end
    headers_json = headers_json:sub(1, -2) .. "}" -- Remove last comma
    response:write(headers_json)
end)

-- Development middleware with verbose logging
chi_middleware("/dev/*", function(request, response, next)
    response:header("X-Environment", "development")
    response:header("X-Debug", "true")
    
    log("DEV Request: " .. request.method .. " " .. request.path .. " from " .. (request.headers["X-Real-IP"] or "unknown"))
    
    next()
end)