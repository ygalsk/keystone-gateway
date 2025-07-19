-- A/B Testing Script
function on_route_request(request, backends)
    local user_id = request.headers["X-User-ID"]
    
    if not user_id then
        return {
            reject = true,
            reject_reason = "X-User-ID header required for A/B testing"
        }
    end
    
    -- Simple hash-based A/B testing
    local hash = simple_hash(user_id)
    local variant = (hash % 2 == 0) and "a" or "b"
    
    return select_backend_by_variant(backends, variant)
end

function simple_hash(str)
    local hash = 0
    for i = 1, #str do
        hash = hash + string.byte(str, i)
    end
    return hash
end

function select_backend_by_variant(backends, variant)
    local pattern = "version-" .. variant
    
    for i, backend in ipairs(backends) do
        if string.find(backend.name, pattern) and backend.health then
            return {
                selected_backend = backend.name,
                modified_headers = {
                    ["X-Routed-By"] = "lua-engine",
                    ["X-Variant"] = variant
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