-- Common Lua utilities for Keystone Gateway
-- This file provides reusable utility functions for route scripts

-- Utility function to get current ISO timestamp
function get_timestamp()
    return os.date("!%Y-%m-%dT%H:%M:%SZ")
end

-- Utility function to set CORS headers
function set_cors_headers(response, origin)
    origin = origin or "*"
    response:header("Access-Control-Allow-Origin", origin)
    response:header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
    response:header("Access-Control-Allow-Headers", "Content-Type, Authorization")
end

-- Utility function to send JSON response
function json_response(response, data, status_code)
    status_code = status_code or 200
    response:header("Content-Type", "application/json")
    response:status(status_code)
    response:write(data)
end

-- Utility function to create health check response
function health_response(response, service_name)
    service_name = service_name or "keystone-gateway"
    local health_data = '{"status": "healthy", "service": "' .. service_name .. '", "timestamp": "' .. get_timestamp() .. '"}'
    json_response(response, health_data)
end

print("✅ Common Lua utilities loaded")