-- Authentication & Authorization Routes
-- Demonstrates how to register routes with authentication middleware

log("Setting up authentication routes for tenant")

-- Public endpoints (no auth required)
chi_route("GET", "/", function(w, r)
    w:header("Content-Type", "application/json")
    w:write('{"service":"Auth Gateway","version":"1.0.0","public":true}')
end)

chi_route("GET", "/health", function(w, r)
    w:header("Content-Type", "application/json")
    w:write('{"status":"healthy","timestamp":"' .. os.time() .. '"}')
end)

-- Auth endpoints
chi_group("/auth", function()
    chi_route("POST", "/login", function(w, r)
        -- Simplified login logic
        local body = r.body
        w:header("Content-Type", "application/json")
        
        if body and body:find('"username":"admin"') and body:find('"password":"secret"') then
            w:write('{"token":"jwt-token-123","user":{"id":1,"username":"admin","role":"admin"},"expires_in":3600}')
        elseif body and body:find('"username":"user"') and body:find('"password":"pass"') then
            w:write('{"token":"jwt-token-456","user":{"id":2,"username":"user","role":"user"},"expires_in":3600}')
        else
            w:status(401)
            w:write('{"error":"Invalid credentials"}')
        end
    end)
    
    chi_route("POST", "/logout", function(w, r)
        w:header("Content-Type", "application/json")
        w:write('{"message":"Logged out successfully"}')
    end)
    
    chi_route("POST", "/refresh", function(w, r)
        local auth_header = r:header("Authorization")
        if auth_header and auth_header:find("Bearer jwt%-token") then
            w:header("Content-Type", "application/json")
            w:write('{"token":"jwt-token-refreshed","expires_in":3600}')
        else
            w:status(401)
            w:write('{"error":"Invalid token"}')
        end
    end)
end)

-- Protected API endpoints with JWT middleware
chi_group("/api", function()
    -- JWT authentication middleware
    chi_middleware("/*", function(next)
        return function(w, r)
            local auth_header = r:header("Authorization")
            
            if not auth_header or not auth_header:find("Bearer jwt%-token") then
                w:status(401)
                w:header("Content-Type", "application/json")
                w:write('{"error":"Authentication required","code":"AUTH_REQUIRED"}')
                return
            end
            
            -- Extract user info from token (simplified)
            local user_role = "user"
            if auth_header:find("jwt%-token%-123") then
                user_role = "admin"
                w:header("X-User-ID", "1")
                w:header("X-User-Role", "admin")
            elseif auth_header:find("jwt%-token%-456") then
                user_role = "user"
                w:header("X-User-ID", "2")
                w:header("X-User-Role", "user")
            end
            
            w:header("X-Authenticated", "true")
            next(w, r)
        end
    end)
    
    -- User profile endpoints
    chi_route("GET", "/profile", function(w, r)
        local user_id = w:header("X-User-ID")
        local user_role = w:header("X-User-Role")
        
        w:header("Content-Type", "application/json")
        w:write('{"id":"' .. user_id .. '","role":"' .. user_role .. '","profile":{"name":"User ' .. user_id .. '","email":"user' .. user_id .. '@example.com"}}')
    end)
    
    chi_route("PUT", "/profile", function(w, r)
        local user_id = w:header("X-User-ID")
        
        w:header("Content-Type", "application/json")
        w:write('{"id":"' .. user_id .. '","message":"Profile updated successfully"}')
    end)
    
    -- Admin-only endpoints
    chi_group("/admin", function()
        -- Admin authorization middleware
        chi_middleware("/*", function(next)
            return function(w, r)
                local user_role = w:header("X-User-Role")
                
                if user_role ~= "admin" then
                    w:status(403)
                    w:header("Content-Type", "application/json")
                    w:write('{"error":"Admin access required","code":"INSUFFICIENT_PERMISSIONS"}')
                    return
                end
                
                next(w, r)
            end
        end)
        
        chi_route("GET", "/users", function(w, r)
            w:header("Content-Type", "application/json")
            w:write('{"users":[{"id":1,"username":"admin","role":"admin"},{"id":2,"username":"user","role":"user"}],"total":2}')
        end)
        
        chi_route("POST", "/users", function(w, r)
            w:header("Content-Type", "application/json")
            w:write('{"id":3,"username":"newuser","role":"user","created":"' .. os.time() .. '"}')
        end)
        
        chi_route("DELETE", "/users/{id}", function(w, r)
            local user_id = chi_param(r, "id")
            w:header("Content-Type", "application/json")
            w:write('{"message":"User ' .. user_id .. ' deleted successfully"}')
        end)
        
        chi_route("GET", "/audit-log", function(w, r)
            w:header("Content-Type", "application/json")
            w:write('{"logs":[{"action":"login","user":"admin","timestamp":"' .. (os.time() - 3600) .. '"},{"action":"user_created","user":"admin","timestamp":"' .. (os.time() - 1800) .. '"}]}')
        end)
    end)
end)

-- Rate limiting example
chi_group("/api/public", function()
    -- Simple rate limiting middleware
    chi_middleware("/*", function(next)
        return function(w, r)
            local client_ip = r:header("X-Real-IP") or "unknown"
            -- In practice, you'd use a proper rate limiter with Redis/etc
            w:header("X-RateLimit-Limit", "100")
            w:header("X-RateLimit-Remaining", "99")
            w:header("X-RateLimit-Reset", tostring(os.time() + 3600))
            next(w, r)
        end
    end)
    
    chi_route("GET", "/status", function(w, r)
        w:header("Content-Type", "application/json")
        w:write('{"status":"ok","public_api":true,"rate_limited":true}')
    end)
end)

log("Authentication routes registered successfully")
