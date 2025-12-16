-- Test Lua Routes
-- This demonstrates dynamic routing with Lua scripts

-- Route 1: Simple JSON response
chi_route("GET", "/lua/hello", function(req, res)
    res:Header("Content-Type", "application/json")
    res:Write('{"message": "Hello from Lua!", "path": "/lua/hello"}')
end)

-- Route 2: Echo request information
chi_route("GET", "/lua/echo", function(req, res)
    local response = string.format([[{
  "message": "Lua Echo",
  "method": "%s",
  "path": "%s",
  "host": "%s",
  "user_agent": "%s"
}]], req.Method, req.Path, req.Host, req:Header("User-Agent"))

    res:Header("Content-Type", "application/json")
    res:Write(response)
end)

-- Route 3: URL parameters
chi_route("GET", "/lua/user/{id}", function(req, res)
    local user_id = req:Param("id")
    local response = string.format([[{
  "message": "User endpoint",
  "user_id": "%s",
  "route": "/lua/user/{id}"
}]], user_id)

    res:Header("Content-Type", "application/json")
    res:Write(response)
end)

-- Route 4: POST with body handling
chi_route("POST", "/lua/data", function(req, res)
    local body = req:Body()
    local response = string.format([[{
  "message": "Received POST data",
  "body_received": %s,
  "body_length": %d
}]], body, #body)

    res:Header("Content-Type", "application/json")
    res:Write(response)
end)

-- Route 5: HTTP client test (calling external service)
chi_route("GET", "/lua/proxy/{service}", function(req, res)
    local service = req:Param("service")

    -- Map service names to ports
    local ports = {
        api = "9001",
        admin = "9002",
        default = "9003"
    }

    local port = ports[service] or "9003"
    local url = "http://localhost:" .. port .. "/lua-proxied"

    -- Make HTTP call using the HTTP client
    local response = HTTP:Get(url, {})

    -- Return the proxied response
    res:Header("Content-Type", "application/json")
    res:Write(response.Body)
end)

print("✓ Lua routes registered: /lua/hello, /lua/echo, /lua/user/{id}, /lua/data, /lua/proxy/{service}")
