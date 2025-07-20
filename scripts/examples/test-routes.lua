-- Test Routes for Development
-- Simple routing logic for testing Lua integration

function on_route_request(request, backends)
    -- Log request for debugging
    print("Processing request:", request.method, request.path)
    
    -- Simple header-based routing
    local test_version = request.headers["X-Test-Version"]
    
    if test_version == "v2" then
        print("Routing to v2 backend")
        return filter_backends(backends, "v2")
    elseif test_version == "canary" then
        print("Routing to canary backend")
        return filter_backends(backends, "canary")
    end
    
    -- Default routing
    print("Using default routing")
    return backends
end

function on_response(response, request)
    -- Add debug headers
    response.headers["X-Debug-Mode"] = "true"
    response.headers["X-Lua-Version"] = "test-routes-1.0"
    response.headers["X-Request-ID"] = generate_request_id()
    
    return response
end

-- Helper function to generate request ID
function generate_request_id()
    return string.format("req_%d_%d", os.time(), math.random(1000, 9999))
end