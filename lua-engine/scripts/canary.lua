-- Canary Deployment Script
function on_route_request(request, backends)
    local canary_header = request.headers["X-Canary"]
    local canary_percent = tonumber(request.headers["X-Canary-Percent"]) or 10
    
    -- Force canary routing if header is set
    if canary_header == "true" then
        return select_backend_by_name(backends, "canary")
    end
    
    -- Random canary traffic based on percentage
    local random_val = math.random(100)
    if random_val <= canary_percent then
        return select_backend_by_name(backends, "canary")
    end
    
    -- Default to stable
    return select_backend_by_name(backends, "stable")
end

function select_backend_by_name(backends, name_pattern)
    for i, backend in ipairs(backends) do
        if string.find(backend.name, name_pattern) and backend.health then
            return {
                selected_backend = backend.name,
                modified_headers = {
                    ["X-Routed-By"] = "lua-engine",
                    ["X-Backend-Type"] = name_pattern
                }
            }
        end
    end
    
    -- Fallback to first healthy backend
    for i, backend in ipairs(backends) do
        if backend.health then
            return {
                selected_backend = backend.name,
                modified_headers = {
                    ["X-Routed-By"] = "lua-engine-fallback"
                }
            }
        end
    end
    
    return {
        reject = true,
        reject_reason = "No healthy backends available"
    }
end