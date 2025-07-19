-- A/B Testing Routes
-- Demonstrates how to register routes with A/B testing logic

log("Setting up A/B testing routes for tenant")

-- Feature flags endpoint
chi_route("GET", "/features", function(w, r)
    local user_id = r:header("X-User-ID")
    local variant = "A" -- Default variant
    
    -- Simple hash-based A/B assignment
    if user_id then
        local hash = 0
        for i = 1, #user_id do
            hash = hash + string.byte(user_id, i)
        end
        variant = (hash % 2 == 0) and "A" or "B"
    end
    
    w:header("Content-Type", "application/json")
    w:header("X-AB-Variant", variant)
    
    if variant == "A" then
        w:write('{"features":{"new_checkout":false,"enhanced_search":false},"variant":"A"}')
    else
        w:write('{"features":{"new_checkout":true,"enhanced_search":true},"variant":"B"}')
    end
end)

-- Product listing with A/B variants
chi_group("/api/products", function()
    -- A/B testing middleware
    chi_middleware("/*", function(next)
        return function(w, r)
            local user_id = r:header("X-User-ID")
            local forced_variant = r:header("X-Force-Variant")
            local variant = "A"
            
            if forced_variant and (forced_variant == "A" or forced_variant == "B") then
                variant = forced_variant
            elseif user_id then
                local hash = 0
                for i = 1, #user_id do
                    hash = hash + string.byte(user_id, i)
                end
                variant = (hash % 2 == 0) and "A" or "B"
            end
            
            w:header("X-AB-Variant", variant)
            w:header("X-AB-Test", "product-listing-v2")
            next(w, r)
        end
    end)
    
    chi_route("GET", "/", function(w, r)
        local variant = w:header("X-AB-Variant")
        w:header("Content-Type", "application/json")
        
        if variant == "A" then
            -- Original product listing
            w:write('{"products":[{"id":1,"name":"Product 1","price":10.99}],"layout":"grid","sort":"popularity"}')
        else
            -- Enhanced product listing (Variant B)
            w:write('{"products":[{"id":1,"name":"Product 1","price":10.99,"rating":4.5,"reviews":42}],"layout":"card","sort":"relevance","features":["ratings","reviews","wishlist"]}')
        end
    end)
    
    chi_route("GET", "/{id}", function(w, r)
        local product_id = chi_param(r, "id")
        local variant = w:header("X-AB-Variant")
        w:header("Content-Type", "application/json")
        
        if variant == "A" then
            w:write('{"id":"' .. product_id .. '","name":"Product ' .. product_id .. '","price":10.99,"description":"Basic product info"}')
        else
            w:write('{"id":"' .. product_id .. '","name":"Product ' .. product_id .. '","price":10.99,"description":"Enhanced product info","rating":4.5,"reviews":[{"user":"Alice","rating":5,"comment":"Great product!"}],"recommendations":["Product 2","Product 3"]}')
        end
    end)
end)

-- Checkout flow with A/B testing
chi_group("/api/checkout", function()
    chi_middleware("/*", function(next)
        return function(w, r)
            local variant = w:header("X-AB-Variant") or r:header("X-AB-Variant")
            if not variant then
                -- Fallback A/B assignment
                variant = (math.random(2) == 1) and "A" or "B"
                w:header("X-AB-Variant", variant)
            end
            
            w:header("X-AB-Test", "checkout-flow-v3")
            next(w, r)
        end
    end)
    
    chi_route("POST", "/", function(w, r)
        local variant = w:header("X-AB-Variant")
        w:header("Content-Type", "application/json")
        
        if variant == "A" then
            -- Traditional checkout
            w:write('{"checkout_id":"ck_123","steps":["cart","shipping","payment","confirm"],"variant":"A"}')
        else
            -- One-page checkout
            w:write('{"checkout_id":"ck_123","steps":["one-page-checkout"],"variant":"B","features":["auto-fill","express-payment","guest-checkout"]}')
        end
    end)
end)

-- Analytics endpoint for A/B test results
chi_route("GET", "/analytics/ab-tests", function(w, r)
    w:header("Content-Type", "application/json")
    w:write('{"tests":[{"name":"product-listing-v2","variants":{"A":{"users":1000,"conversions":150},"B":{"users":1000,"conversions":180}}},{"name":"checkout-flow-v3","variants":{"A":{"users":500,"conversions":75},"B":{"users":500,"conversions":95}}}]}')
end)

log("A/B testing routes registered successfully")
