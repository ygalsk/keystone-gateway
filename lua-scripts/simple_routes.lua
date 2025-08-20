-- Simple routes for basic testing
-- Just a few basic endpoints to test functionality

-- Simple hello endpoint
chi_route("GET", "/api/hello", function(req, resp)
    resp.status(200)
    resp.write('{"message": "Hello from Lua!", "path": "' .. req.path .. '"}')
end)

-- Simple test endpoint with parameter
chi_route("GET", "/api/test/{id}", function(req, resp)
    local test_id = chi_param(req, "id")
    resp.status(200)
    resp.write('{"test_id": "' .. test_id .. '", "message": "Parameter test successful"}')
end)

-- Simple health check
chi_route("GET", "/api/simple-health", function(req, resp)
    resp.status(200)
    resp.write('{"status": "ok", "service": "simple-lua-test"}')
end)

-- Simple echo endpoint for POST testing
chi_route("POST", "/api/echo", function(req, resp)
    resp.status(200)
    resp.write('{"method": "' .. req.method .. '", "path": "' .. req.path .. '", "echo": "success"}')
end)