-- Blue/Green Deployment Script
-- Routes all traffic to either blue or green environment based on deployment state

function on_route_request(request, backends)
    log("Blue/Green routing for: " .. request.path)
    
    -- Check deployment state via header (typically set by CI/CD)
    local deployment_state = request.headers["X-Deployment-State"] or "blue"
    local force_green = request.headers["X-Force-Green"]
    
    -- Force green deployment for testing
    if force_green == "true" then
        log("Forcing green deployment via header")
        return select_backend_by_name(backends, "green")
    end
    
    -- Route based on deployment state
    if deployment_state == "green" then
        log("Routing to green deployment")
        return select_backend_by_name(backends, "green")
    else
        log("Routing to blue deployment")
        return select_backend_by_name(backends, "blue")
    end
end

function select_backend_by_name(backends, environment)
    -- Find backend for the specified environment
    for i, backend in ipairs(backends) do
        if string.find(backend.name, environment) and backend.health then
            return {
                selected_backend = backend.name,
                modified_headers = {
                    ["X-Routed-By"] = "lua-engine",
                    ["X-Environment"] = environment,
                    ["X-Deployment-Strategy"] = "blue-green"
                }
            }
        end
    end
    
    -- If target environment is not available, try the other one
    local fallback_env = environment == "blue" and "green" or "blue"
    log("Target environment " .. environment .. " not available, trying " .. fallback_env)
    
    for i, backend in ipairs(backends) do
        if string.find(backend.name, fallback_env) and backend.health then
            return {
                selected_backend = backend.name,
                modified_headers = {
                    ["X-Routed-By"] = "lua-engine-fallback",
                    ["X-Environment"] = fallback_env,
                    ["X-Deployment-Strategy"] = "blue-green-fallback"
                }
            }
        end
    end
    
    -- No suitable backend found
    return {
        reject = true,
        reject_reason = "No healthy backends available for blue/green deployment"
    }
end
