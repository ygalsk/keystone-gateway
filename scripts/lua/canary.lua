-- Canary Deployment Script
-- Routes traffic between stable and canary backends based on headers or percentage

function on_route_request(request, backends)
    log("Processing request for: " .. request.path)
    
    -- Check for explicit canary routing header
    local canary_header = request.headers["X-Canary"]
    local canary_percent = tonumber(request.headers["X-Canary-Percent"]) or 10
    
    -- Force canary if header is set
    if canary_header == "true" then
        log("Routing to canary via header")
        return select_backend_by_name(backends, "canary")
    end
    
    -- Random canary traffic distribution
    local random_val = math.random(100)
    if random_val <= canary_percent then
        log("Routing to canary via percentage (" .. canary_percent .. "%)")
        return select_backend_by_name(backends, "canary")
    end
    
    -- Default to stable
    log("Routing to stable backend")
    return select_backend_by_name(backends, "stable")
end

function select_backend_by_name(backends, name_pattern)
    -- First, try to find a backend with the exact pattern in the name
    for i, backend in ipairs(backends) do
        if string.find(backend.name, name_pattern) and backend.health then
            return {
                selected_backend = backend.name,
                modified_headers = {
                    ["X-Routed-By"] = "lua-engine",
                    ["X-Backend-Type"] = name_pattern,
                    ["X-Backend-Selection"] = "pattern-match"
                }
            }
        end
    end
    
    -- Fallback to first healthy backend
    for i, backend in ipairs(backends) do
        if backend.health then
            log("Fallback to backend: " .. backend.name)
            return {
                selected_backend = backend.name,
                modified_headers = {
                    ["X-Routed-By"] = "lua-engine-fallback",
                    ["X-Backend-Selection"] = "fallback"
                }
            }
        end
    end
    
    -- No healthy backends available
    return {
        reject = true,
        reject_reason = "No healthy backends available for pattern: " .. name_pattern
    }
end
