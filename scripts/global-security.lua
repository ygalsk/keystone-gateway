-- Global Security Script
-- Applies security headers and policies to all tenant requests

-- Global security headers middleware
chi_middleware("/*", function(request, response, next)
    -- Add security headers to all responses
    response:header("X-Content-Type-Options", "nosniff")
    response:header("X-Frame-Options", "DENY")
    response:header("X-XSS-Protection", "1; mode=block")
    response:header("Referrer-Policy", "strict-origin-when-cross-origin")
    
    -- Add gateway identification header
    response:header("X-Gateway", "Keystone-Gateway")
    
    log("Global security middleware applied to: " .. request.method .. " " .. request.path)
    
    next()
end)

-- Global admin health endpoint
chi_route("GET", "/global/health", function(request, response)
    response:header("Content-Type", "application/json")
    response:write('{"status": "healthy", "service": "keystone-gateway", "global_scripts": "active"}')
end)