-- Test Middleware and Business Logic
-- Demonstrates chi_middleware() and complex routing logic

-- Middleware 1: Request logging
chi_middleware(function(req, res, next)
    print(string.format("[LOG] %s %s (User-Agent: %s)",
        req.Method, req.Path, req:Header("User-Agent") or "none"))

    -- Add custom header to response
    res:Header("X-Middleware-Applied", "request-logger")

    -- Continue to next middleware/handler
    next()
end)

-- Middleware 2: API key validation
chi_middleware(function(req, res, next)
    -- Only check API key for /lua/secure/* paths
    if not req.Path:match("^/lua/secure") then
        next()
        return
    end

    local api_key = req:Header("X-API-Key")

    if not api_key or api_key ~= "test-key-12345" then
        res:Status(401)
        res:Header("Content-Type", "application/json")
        res:Write('{"error": "Invalid or missing API key"}')
        return  -- Don't call next() - stop processing
    end

    -- API key valid, add user context
    res:Header("X-API-Key-Valid", "true")
    next()
end)

-- Middleware 3: Request timing
chi_middleware(function(req, res, next)
    local start_time = os.clock()

    next()  -- Process request

    local duration = (os.clock() - start_time) * 1000  -- Convert to ms
    res:Header("X-Request-Duration-Ms", string.format("%.2f", duration))
end)

-- Business Logic Route 1: Conditional routing based on query params
chi_route("GET", "/lua/conditional", function(req, res)
    local format = req:Query("format")
    local data = {
        message = "Conditional response",
        format = format or "json",
        path = req.Path
    }

    if format == "xml" then
        res:Header("Content-Type", "application/xml")
        res:Write(string.format([[<?xml version="1.0"?>
<response>
  <message>%s</message>
  <format>%s</format>
  <path>%s</path>
</response>]], data.message, data.format, data.path))
    elseif format == "text" then
        res:Header("Content-Type", "text/plain")
        res:Write(string.format("Message: %s\nFormat: %s\nPath: %s",
            data.message, data.format, data.path))
    else
        -- Default to JSON
        res:Header("Content-Type", "application/json")
        res:Write(string.format([[{
  "message": "%s",
  "format": "%s",
  "path": "%s"
}]], data.message, data.format, data.path))
    end
end)

-- Business Logic Route 2: Secure endpoint (requires API key from middleware)
chi_route("GET", "/lua/secure/data", function(req, res)
    res:Header("Content-Type", "application/json")
    res:Write([[{
  "message": "Secure data accessed successfully",
  "data": ["item1", "item2", "item3"],
  "authenticated": true
}]])
end)

-- Business Logic Route 3: Data aggregation from multiple sources
chi_route("GET", "/lua/aggregate/{type}", function(req, res)
    local data_type = req:Param("type")

    -- Simulate calling multiple backends
    local results = {}

    if data_type == "users" then
        -- Call API backend
        local api_response = HTTP:Get("http://localhost:9001/users", {})
        table.insert(results, {
            source = "api",
            status = api_response.Status,
            data = api_response.Body
        })

        -- Call admin backend
        local admin_response = HTTP:Get("http://localhost:9002/users", {})
        table.insert(results, {
            source = "admin",
            status = admin_response.Status,
            data = admin_response.Body
        })
    elseif data_type == "metrics" then
        -- Different backends for metrics
        local api_metrics = HTTP:Get("http://localhost:9001/metrics", {})
        local admin_metrics = HTTP:Get("http://localhost:9002/metrics", {})

        table.insert(results, {
            source = "api-metrics",
            status = api_metrics.Status,
            data = api_metrics.Body
        })
        table.insert(results, {
            source = "admin-metrics",
            status = admin_metrics.Status,
            data = admin_metrics.Body
        })
    else
        res:Status(400)
        res:Header("Content-Type", "application/json")
        res:Write('{"error": "Unknown data type. Use: users or metrics"}')
        return
    end

    -- Aggregate results
    res:Header("Content-Type", "application/json")
    local response = string.format([[{
  "type": "%s",
  "sources_count": %d,
  "results": [
]], data_type, #results)

    for i, result in ipairs(results) do
        response = response .. string.format([[
    {
      "source": "%s",
      "status": %d,
      "data": %s
    }]], result.source, result.status, result.data)

        if i < #results then
            response = response .. ","
        end
        response = response .. "\n"
    end

    response = response .. "  ]\n}"
    res:Write(response)
end)

-- Business Logic Route 4: POST with validation
chi_route("POST", "/lua/validate", function(req, res)
    local body = req:Body()

    -- Parse JSON manually (simple approach)
    local name = body:match('"name"%s*:%s*"([^"]+)"')
    local email = body:match('"email"%s*:%s*"([^"]+)"')
    local age = body:match('"age"%s*:%s*(%d+)')

    local errors = {}

    if not name or #name < 2 then
        table.insert(errors, "Name must be at least 2 characters")
    end

    if not email or not email:match("^[^@]+@[^@]+%.[^@]+$") then
        table.insert(errors, "Invalid email format")
    end

    if age and tonumber(age) < 18 then
        table.insert(errors, "Age must be 18 or older")
    end

    if #errors > 0 then
        res:Status(400)
        res:Header("Content-Type", "application/json")
        local error_json = '{"errors": ['
        for i, err in ipairs(errors) do
            error_json = error_json .. '"' .. err .. '"'
            if i < #errors then
                error_json = error_json .. ', '
            end
        end
        error_json = error_json .. ']}'
        res:Write(error_json)
    else
        res:Status(200)
        res:Header("Content-Type", "application/json")
        res:Write(string.format([[{
  "message": "Validation passed",
  "data": {
    "name": "%s",
    "email": "%s",
    "age": %s
  }
}]], name, email, age or "null"))
    end
end)

print("✓ Middleware and business logic routes registered")
print("  - 3 middleware layers: logging, auth, timing")
print("  - 4 business logic routes: conditional, secure, aggregate, validate")
