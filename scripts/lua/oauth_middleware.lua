-- oauth_middleware.lua - Simple OAuth proxy

-- Config
local OAUTH_AUTH_URL = get_env("OAUTH_AUTH_URL")
local OAUTH_AUDIENCE = get_env("OAUTH_AUDIENCE") 
local OAUTH_EXCHANGE_URL = get_env("OAUTH_EXCHANGE_URL")
local OAUTH_CLIENT_ID = get_env("OAUTH_CLIENT_ID")
local OAUTH_CLIENT_SECRET = get_env("OAUTH_CLIENT_SECRET")

-- Cache keys
local CACHE_KEY_TOKEN = "oauth_token"
local CACHE_KEY_LOCK = "oauth_lock"

local function url_encode(str)
    if not str then return "" end
    str = str:gsub("([^%w%-_.~])", function(c)
        return string.format("%%%02X", string.byte(c))
    end)
    return str
end

-- URL decode
local function url_decode(str)
    if not str then return nil end
    str = str:gsub("+", " ")
    str = str:gsub("%%(%x%x)", function(hex)
        return string.char(tonumber(hex, 16))
    end)
    return str
end

-- Get OAuth token with atomic locking to prevent thundering herd
local function get_token()
    -- Check global cache first
    local cached_token = cache_get(CACHE_KEY_TOKEN)
    if cached_token then
        return cached_token
    end
    
    -- Try to acquire lock atomically - only ONE request should do OAuth
    local got_lock = cache_add(CACHE_KEY_LOCK, "locked", 10) -- 10 second timeout
    if not got_lock then
        -- Another request is getting the token, fast-fail for client retry
        return nil, "token_acquisition_in_progress"  
    end
    
    -- We have the lock, proceed with OAuth
    local success, token, error_msg = pcall(function()
        -- Step 1: Get subject token
        local body1 = string.format(
            "grant_type=client_credentials&client_id=%s&client_secret=%s&audience=%s",
            OAUTH_CLIENT_ID, OAUTH_CLIENT_SECRET, OAUTH_AUDIENCE
        )
        
        local resp1, status1 = http_post(OAUTH_AUTH_URL, body1, 
            {["Content-Type"] = "application/x-www-form-urlencoded"})
        
        if status1 ~= 200 then
            error("auth failed: " .. status1)
        end
        
        local subject_token = resp1:match('"access_token"%s*:%s*"([^"]+)"')
        if not subject_token then
            error("no subject token in response")
        end
        
        -- Step 2: Exchange for portal token
        local body2 = string.format([[{
            "grant_type": "urn:ietf:params:oauth:grant-type:token-exchange",
            "subject_token": "%s",
            "subject_token_type": "urn:ietf:params:oauth:token-type:access_token"
        }]], subject_token)
        
        local resp2, status2 = http_post(OAUTH_EXCHANGE_URL, body2,
            {["Content-Type"] = "application/json"})
        
        if status2 ~= 200 then
            error("exchange failed: " .. status2)
        end
        
        local portal_token = resp2:match('"access_token"%s*:%s*"([^"]+)"')
        local expires = resp2:match('"expires_in"%s*:%s*(%d+)') or "3600"
        
        if not portal_token then
            error("no portal token in response")
        end
        
        return portal_token, tonumber(expires)
    end)
    
    -- Always release the lock
    cache_delete(CACHE_KEY_LOCK)
    
    if success then
        local portal_token, expires = token, error_msg
        -- Cache with 60 second buffer  
        local cache_ttl = expires - 60
        if cache_ttl > 0 then
            cache_set(CACHE_KEY_TOKEN, portal_token, cache_ttl)
        else
            -- Fallback: cache for at least 300 seconds if expires is too low
            cache_set(CACHE_KEY_TOKEN, portal_token, 300)
        end
        return portal_token
    else
        return nil, token -- token contains error message when success is false
    end
end

-- Simple proxy middleware
chi_middleware("/*", function(request, response, next)
    -- Get the full query string and extract redirect_uri
    local query_string = request.url:match("%?(.+)") or ""
    local redirect_uri = query_string:match("redirect_uri=([^&]*)")
    if not redirect_uri then
        -- This middleware only handles requests with redirect_uri
        response:status(404)
        response:header("Content-Type", "application/json")
        response:write('{"error": "This gateway only proxies requests with redirect_uri parameter"}')
        return
    end

    -- Decode the redirect_uri
    local target_url = url_decode(redirect_uri)
    if not target_url then
        response:status(400)
        response:header("Content-Type", "application/json")
        response:write('{"error": "invalid redirect_uri"}')
        return
    end

-- Existing HTML and CSV logic (unchanged)
    local base, uuid, params = target_url:match("^(https?://[^/]+)/resource/processes/([%w%-]+)%?(.*)$")
    if base and uuid and params then
        -- Check if format=html is present
        if params:match("format=html") then
            -- Remove format=html parameter
            params = params:gsub("(^&?format=html&?)", ""):gsub("(&?format=html$)", "")
            -- Clean up any double ampersands or leading/trailing ampersands  
            params = params:gsub("&&", "&"):gsub("^&", ""):gsub("&$", "")

            local new_query = "uuid=" .. uuid
            if #params > 0 then new_query = new_query .. "&" .. params end
            local new_url = string.format("%s/datasetdetail/process.xhtml?%s", base, new_query)

            -- Redirect for HTML format
            response:header("Location", new_url)
            response:header("Cache-Control", "no-cache")
            response:status(302)
            return
        end

        -- CSV download
        if params:match("format=csv") then
            local body, headers, status = http_get(target_url)

            -- Forward all headers from the remote response
            for k, v in pairs(headers) do
                if type(v) == "string" then
                    response:header(k, v)
                elseif type(v) == "table" then
                    response:header(k, table.concat(v, ", "))
                end
            end

            response:write(body)
            response:status(status)
            return
        end
    end

    -- NEW separate check for ZIP export (pass-through headers)
    local base_zip, uuid_zip, path_zip = target_url:match("^(https?://[^/]+)/resource/processes/([%w%-]+)(/[^?]*)")
    if base_zip and uuid_zip and path_zip and path_zip:match("zipexport") then
        local body, headers, status = http_get(target_url)

        -- Forward all headers from the remote response
        for k, v in pairs(headers) do
            if type(v) == "string" then
                response:header(k, v)
            elseif type(v) == "table" then
                response:header(k, table.concat(v, ", "))
            end
        end

        response:write(body)
        response:status(status)
        return
    end

    -- FALLBACK: OAuth proxy logic for all other requests
    -- Add other params (except redirect_uri itself)
    local other_params = {}
    for param in query_string:gmatch("([^&]+)") do
        if not param:match("^redirect_uri=") then
            table.insert(other_params, param)
        end
    end
    if #other_params > 0 then
        local separator = target_url:match("%?") and "&" or "?"
        target_url = target_url .. separator .. table.concat(other_params, "&")
    end

    -- Get token
    local token, err = get_token()
    if not token then
        if err == "token_acquisition_in_progress" then
            -- Fast-fail with retry guidance for concurrent requests
            response:status(503)
            response:header("Retry-After", "2")
            response:header("Content-Type", "application/json")
            response:write('{"error": "token_acquisition_in_progress", "retry_after_seconds": 2}')
        else
            -- Other auth failures
            response:status(503)
            response:header("Content-Type", "application/json")
            response:write('{"error": "' .. (err or "auth failed") .. '"}')
        end
        return
    end

    -- Proxy the request with auth headers
    local headers_table = {
        ["Authorization"] = "Bearer " .. token,
        ["User-Agent"] = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
    }
    local body, resp_headers, status = http_get(target_url, headers_table)

    response:status(status or 502)
    response:write(body or '{"error": "upstream failed"}')
end)