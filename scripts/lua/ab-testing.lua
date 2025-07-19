-- A/B Testing Script
-- Routes traffic between different versions for A/B testing based on user segments

function on_route_request(request, backends)
    log("A/B Testing routing for: " .. request.path)
    
    -- Check for explicit version targeting
    local target_version = request.headers["X-Target-Version"]
    local user_id = request.headers["X-User-ID"]
    local ab_test_enabled = request.headers["X-AB-Test"] ~= "false"
    
    -- If specific version is requested, route there
    if target_version then
        log("Routing to specific version: " .. target_version)
        return select_backend_by_version(backends, target_version)
    end
    
    -- Skip A/B testing if disabled
    if not ab_test_enabled then
        log("A/B testing disabled, routing to version A")
        return select_backend_by_version(backends, "a")
    end
    
    -- Perform A/B routing based on user ID hash
    if user_id then
        local version = hash_user_to_version(user_id)
        log("User " .. user_id .. " routed to version " .. version)
        return select_backend_by_version(backends, version)
    end
    
    -- Random assignment if no user ID
    local random_val = math.random(100)
    local version = random_val <= 50 and "a" or "b"
    log("Random assignment to version " .. version)
    return select_backend_by_version(backends, version)
end

function hash_user_to_version(user_id)
    -- Simple hash-based assignment for consistent user experience
    local hash = 0
    for i = 1, string.len(user_id) do
        hash = hash + string.byte(user_id, i)
    end
    return hash % 2 == 0 and "a" or "b"
end

function select_backend_by_version(backends, version)
    -- Find backend for the specified version
    for i, backend in ipairs(backends) do
        if string.find(backend.name, "version-" .. version) and backend.health then
            return {
                selected_backend = backend.name,
                modified_headers = {
                    ["X-Routed-By"] = "lua-engine",
                    ["X-AB-Version"] = version,
                    ["X-Test-Strategy"] = "ab-testing"
                }
            }
        end
    end
    
    -- Fallback to version A if target version not available
    if version ~= "a" then
        log("Version " .. version .. " not available, falling back to version A")
        for i, backend in ipairs(backends) do
            if string.find(backend.name, "version-a") and backend.health then
                return {
                    selected_backend = backend.name,
                    modified_headers = {
                        ["X-Routed-By"] = "lua-engine-fallback",
                        ["X-AB-Version"] = "a",
                        ["X-Test-Strategy"] = "ab-testing-fallback"
                    }
                }
            end
        end
    end
    
    -- Final fallback to any healthy backend
    for i, backend in ipairs(backends) do
        if backend.health then
            return {
                selected_backend = backend.name,
                modified_headers = {
                    ["X-Routed-By"] = "lua-engine-emergency",
                    ["X-Test-Strategy"] = "emergency-fallback"
                }
            }
        end
    end
    
    return {
        reject = true,
        reject_reason = "No healthy backends available for A/B testing"
    }
end
