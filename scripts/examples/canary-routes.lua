-- Canary Deployment Routes
-- Demonstrates how to register routes with canary deployment logic

log("Setting up canary deployment routes for tenant")

-- Health check endpoint
chi_route("GET", "/health", function(w, r)
    w:header("Content-Type", "application/json")
    w:write('{"status":"healthy","deployment":"canary"}')
end)

-- API routes with canary logic
chi_group("/api/v1", function()
    -- Middleware for canary routing decisions
    chi_middleware("/*", function(next)
        return function(w, r)
            local canary_header = r:header("X-Canary")
            local canary_percent = tonumber(r:header("X-Canary-Percent")) or 10
            
            -- Add routing decision headers
            if canary_header == "true" then
                w:header("X-Route-Decision", "forced-canary")
            else
                local random_val = math.random(100)
                if random_val <= canary_percent then
                    w:header("X-Route-Decision", "canary-percent")
                else
                    w:header("X-Route-Decision", "stable")
                end
            end
            
            w:header("X-Canary-Percent", tostring(canary_percent))
            next(w, r)
        end
    end)
    
    -- User management endpoints
    chi_route("GET", "/users", function(w, r)
        local route_decision = w:header("X-Route-Decision")
        w:header("Content-Type", "application/json")
        
        if route_decision:find("canary") then
            w:write('{"users":[],"version":"canary-v1.1","features":["new-ui","enhanced-search"]}')
        else
            w:write('{"users":[],"version":"stable-v1.0","features":["basic-ui"]}')
        end
    end)
    
    chi_route("GET", "/users/{id}", function(w, r)
        local user_id = chi_param(r, "id")
        local route_decision = w:header("X-Route-Decision")
        w:header("Content-Type", "application/json")
        
        if route_decision:find("canary") then
            w:write('{"id":"' .. user_id .. '","name":"User ' .. user_id .. '","version":"canary","features":["profile-v2"]}')
        else
            w:write('{"id":"' .. user_id .. '","name":"User ' .. user_id .. '","version":"stable","features":["profile-v1"]}')
        end
    end)
    
    -- Admin endpoints with enhanced auth for canary
    chi_group("/admin", function()
        chi_middleware("/*", function(next)
            return function(w, r)
                local auth_header = r:header("Authorization")
                local route_decision = w:header("X-Route-Decision")
                
                -- Canary requires enhanced auth
                if route_decision:find("canary") then
                    if not auth_header or not auth_header:find("Bearer canary%-token") then
                        w:status(401)
                        w:write("Canary access requires enhanced authentication")
                        return
                    end
                else
                    -- Standard auth for stable
                    if not auth_header or auth_header ~= "Bearer admin-token" then
                        w:status(401)
                        w:write("Unauthorized")
                        return
                    end
                end
                
                next(w, r)
            end
        end)
        
        chi_route("GET", "/stats", function(w, r)
            local route_decision = w:header("X-Route-Decision")
            w:header("Content-Type", "application/json")
            
            if route_decision:find("canary") then
                w:write('{"requests":1234,"uptime":"24h","version":"canary","metrics":["advanced-analytics","real-time-data"]}')
            else
                w:write('{"requests":1234,"uptime":"24h","version":"stable","metrics":["basic-stats"]}')
            end
        end)
    end)
end)

log("Canary deployment routes registered successfully")
