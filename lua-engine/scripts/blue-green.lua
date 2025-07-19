-- Blue/Green Deployment Script
function on_route_request(request, backends)
    local deployment_state = request.headers["X-Deployment-State"] or "blue"
    
    -- Route based on deployment state
    if deployment_state == "green" then
        return select_backend_by_name(backends, "green")
    else
        return select_backend_by_name(backends, "blue")
    end
end

function select_backend_by_name(backends, name_pattern)
    for i, backend in ipairs(backends) do
        if string.find(backend.name, name_pattern) and backend.health then
            return {
                selected_backend = backend.name,
                modified_headers = {
                    ["X-Routed-By"] = "lua-engine",
                    ["X-Deployment"] = name_pattern
                }
            }
        end
    end
    
    -- Fallback to any healthy backend
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