---
--- Created by dkremer.
--- DateTime: 8/22/25 3:41 PM
---


-- lua-scripts/oauth_middleware.lua

-- Simple JSON decode (very minimal)
local function parse_json(str)
    local token = str:match('"access_token"%s*:%s*"([^"]+)"')
                  or str:match('"token"%s*:%s*"([^"]+)"')
    local expires = str:match('"expires_in"%s*:%s*(%d+)')
    return {
        access_token = token,
        expires_in = tonumber(expires) or 3600
    }
end

-- URL decode function
local function url_decode(str)
    if not str then
        return nil
    end

    -- Replace + with spaces
    str = str:gsub("+", " ")

    -- Replace %XX with corresponding character
    str = str:gsub("%%(%x%x)", function(hex)
        return string.char(tonumber(hex, 16))
    end)

    return str
end

-- Extract and decode redirect_uri parameter
local function get_redirect_uri(url)
    local redirect_uri = url:match("[?&]redirect_uri=([^&]*)")
    if redirect_uri then
        return url_decode(redirect_uri)
    end
    return nil
end

-- Token cache for final portal token
local token_cache = {
    token = nil,
    expires_at = 0
}

-- Step 1: get subject_token from auth.eco-platform.org
local function get_subject_token()
    local client_id = "byeY9sp8Snuu7MwmgXCqgeeXB5fdP8D6"
    local client_secret = "qCVw6759CwpNDNTw8JJcr5S88ZQPfCztYbY3tENt7XdTo4btlVTGO9z0rBeA4cVc"

    local body = string.format(
        "grant_type=client_credentials&client_id=%s&client_secret=%s&audience=https://portal.eco-platform.org&scope=create:token",
        client_id, client_secret
    )

    local response, status = http_post(
        "https://auth.eco-platform.org/oauth/token",
        body,
        { ["Content-Type"] = "application/x-www-form-urlencoded" }
    )

    if status ~= 200 then
        return nil, "Step1 OAuth failed: " .. status
    end

    local auth = parse_json(response)
    return auth.access_token
end

-- Step 2: exchange subject_token for final portal token
local function exchange_token(subject_token)
    local body = string.format([[
    {
        "grant_type": "urn:ietf:params:oauth:grant-type:token-exchange",
        "subject_token": "%s",
        "subject_token_type": "urn:ietf:params:oauth:token-type:access_token"
    }]], subject_token)

    local response, status = http_post(
        "https://portal.eco-platform.org/resource/authenticate/token/exchange",
        body,
        {
            ["Content-Type"] = "application/json",
            ["Accept"] = "application/json"
        }
    )

    print("Step2 status:", status)
    print("Step2 response:", response)

    if status ~= 200 then
        return nil, "Step2 exchange failed: " .. status .. " body=" .. (response or "")
    end

    local auth = parse_json(response)
    return auth.access_token, auth.expires_in
end


-- Get cached or fresh portal token
local function get_portal_token()
    if token_cache.token and os.time() < token_cache.expires_at then
        return token_cache.token
    end

    -- Step 1: subject token
    local subject_token, err = get_subject_token()
    if not subject_token then
        return nil, err
    end

    -- Step 2: exchange for portal token
    local portal_token, expires_in = exchange_token(subject_token)
    if not portal_token then
        return nil, "Failed to exchange subject token"
    end

    token_cache.token = portal_token
    token_cache.expires_at = os.time() + (expires_in or 3600) - 60
    return token_cache.token
end

-- STEP 1: Define middleware FIRST (Chi router requirement)
-- Since path_prefix is /api/, we use /* to catch all requests under this tenant
chi_middleware("/*", function(request, response, next)
    local path = request.path or ""
    print("OAuth middleware called for:", path)

    -- Always perform token retrieval (will use cache after first time)
    local token, err = get_portal_token()
    if not token then
        print("OAuth token fetch failed:", err)
        response:status(503)
        response:header("Content-Type", "application/json")
        response:write('{"error": "' .. (err or "Failed to get portal token") .. '"}')
        return
    end

    -- Special handling for auth endpoint (supports root or prefixed like /api/auth)
    if path == "/auth" or path == "/auth/" or path == "/api/auth" or path == "/api/auth/" then
        -- Prefer structured query table if present
        local redirect_uri = nil
        if request.query and request.query["redirect_uri"] then
            redirect_uri = request.query["redirect_uri"]
        else
            redirect_uri = get_redirect_uri(request.url or "")
        end

        if not redirect_uri or redirect_uri == "" then
            print("Redirect block: missing redirect_uri")
            response.status(400)
            response.header("Content-Type", "application/json")
            response.write('{"error":"missing redirect_uri"}')
            return
        end

    local decoded = url_decode(redirect_uri) or redirect_uri
    print("Redirecting to decoded redirect_uri:", decoded)

    -- IMPORTANT: Set headers BEFORE writing status (WriteHeader sends headers immediately)
    response:header("Location", decoded)
    response:header("Cache-Control", "no-store")
    response:header("Pragma", "no-cache")
    response:status(302)
    response:write("")
        return
    end

    -- Default behavior: attach Authorization header and continue
    request.headers["Authorization"] = "Bearer " .. token
    next()
end)





-- STEP 2: No routes needed - middleware only
-- The middleware above adds OAuth token, then LuaFallbackHandler will proxy to backend
-- Since no routes are registered, the gateway will automatically proxy after middleware

print("OAuth middleware registered for tenant routes (call /api/auth?redirect_uri=...) ")