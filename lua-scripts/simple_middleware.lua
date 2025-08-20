-- Simple middleware for basic testing
-- Just adds a custom header and logs the request

function simple_logging(req, resp, next)
    -- Add a simple header
    resp.header("X-Simple-Test", "working")
    resp.header("X-Request-Path", req.path)
    
    -- Log the request (optional, for debugging)
    print("[SIMPLE] Processing request: " .. req.method .. " " .. req.path)
    
    -- Continue to next handler
    next()
end

-- Register the middleware for all paths
chi_middleware("/api", simple_logging)