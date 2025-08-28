---
--- Created by dkremer.
--- DateTime: 8/22/25 3:41 PM
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

-- Load (no defaults) so script is generic for any Auth0 tenant. get_env must be provided by the host.
local OAUTH_AUTH_URL       = get_env("OAUTH_AUTH_URL") or nil
local OAUTH_AUDIENCE       = get_env("OAUTH_AUDIENCE") or nil
local OAUTH_SCOPE          = get_env("OAUTH_SCOPE") or nil
local OAUTH_EXCHANGE_URL   = get_env("OAUTH_EXCHANGE_URL") or nil
local OAUTH_EXPIRES_LEEWAY = tonumber((get_env("OAUTH_EXPIRES_LEEWAY")) or "60") or 60
local OAUTH_CLIENT_ID      = get_env("OAUTH_CLIENT_ID") or nil
local OAUTH_CLIENT_SECRET  = get_env("OAUTH_CLIENT_SECRET") or nil

local function validate_config()
    local missing = {}
    local function req(name, value)
        if not value or value == '' then table.insert(missing, name) end
    end
    req("OAUTH_CLIENT_ID", OAUTH_CLIENT_ID)
    req("OAUTH_CLIENT_SECRET", OAUTH_CLIENT_SECRET)
    req("OAUTH_AUTH_URL", OAUTH_AUTH_URL)
    req("OAUTH_AUDIENCE", OAUTH_AUDIENCE)
    req("OAUTH_EXCHANGE_URL", OAUTH_EXCHANGE_URL)
    
    if #missing > 0 then
        return false, missing
    end
    return true, nil
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

-- Step 1: get subject_token from auth server
local function get_subject_token()
    local ok, missing = validate_config()
    if not ok then
        return nil, 'missing_env: ' .. table.concat(missing, ',')
    end
    
    local base = string.format(
        "grant_type=client_credentials&client_id=%s&client_secret=%s&audience=%s",
        OAUTH_CLIENT_ID, OAUTH_CLIENT_SECRET, OAUTH_AUDIENCE
    )
    local body = base
    if OAUTH_SCOPE and OAUTH_SCOPE ~= '' then
        body = body .. "&scope=" .. OAUTH_SCOPE
    end
    
    local response, status = http_post(
        OAUTH_AUTH_URL,
        body,
        { ["Content-Type"] = "application/x-www-form-urlencoded" }
    )
    
    if status ~= 200 then
        local snippet = response and response:sub(1,180) or ''
        return nil, "oauth_client_credentials_failed: " .. status .. (snippet ~= '' and (" body=" .. snippet) or '')
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
        OAUTH_EXCHANGE_URL,
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
        return nil, "token_exchange_failed"
    end
    
    token_cache.token = portal_token
    token_cache.expires_at = os.time() + (expires_in or 3600) - OAUTH_EXPIRES_LEEWAY
    
    return token_cache.token
end

-- MIDDLEWARE - NO SECURITY CHECKS
chi_middleware("/*", function(request, response, next)
    local path = request.path or ""
    print("OAuth middleware called for:", path)
    
    -- Always perform token retrieval (will use cache after first time)
    local token, err = get_portal_token()
    if not token then
        print("OAuth token fetch failed:", err)
        if err and err:match('missing_env') then
            response:status(500)
        else
            response:status(503)
        end
        response:header("Content-Type", "application/json")
        response:write('{"error": "' .. (err or "Failed to get portal token") .. '"}')
        return
    end
    
    -- Special handling for auth endpoint
    if path == "/auth" or path == "/auth/" or path == "/api/auth" or path == "/api/auth/" then
        -- Accept redirect_uri as target resource to fetch server-side
        local target = nil
        if request.query and request.query["redirect_uri"] then
            target = request.query["redirect_uri"]
        else
            target = get_redirect_uri(request.url or "")
        end
        
        if not target or target == "" then
            response:status(400)
            response:header("Content-Type", "application/json")
            response:write('{"error":"missing redirect_uri"}')
            return
        end
        
        local decoded = url_decode(target) or target
        print("Target URL to fetch:", decoded)
        
        -- REMOVED ALL HOST VALIDATION - ALLOW ANY URL
        
        -- Perform server-side GET with bearer token
        local headers = { ["Authorization"] = "Bearer " .. token }
        local body, status = http_get(decoded, headers)
        
        if not body then
            response:status(502)
            response:header("Content-Type", "application/json")
            response:write('{"error":"upstream_fetch_failed"}')
            return
        end
        
        response:status(status)
        -- naive content type detection
        if body:sub(1,1) == "{" then
            response:header("Content-Type", "application/json")
        else
            response:header("Content-Type", "text/plain; charset=utf-8")
        end
        response:write(body)
        return
    end
    
    -- Default behavior: attach Authorization header and continue
    request.headers["Authorization"] = "Bearer " .. token
    next()
end)