-- oauth_middleware.lua - Simplified OAuth proxy reading from token file
-- Reads cached tokens from /tmp/oauth_token.json managed by external cron service

-- Token file path
local TOKEN_FILE_PATH = "/tmp/oauth_token.json"

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

-- Read token from file managed by external cron service
local function get_token_from_file()
    print("[DEBUG] Attempting to read token from: " .. TOKEN_FILE_PATH)
    
    local file = io.open(TOKEN_FILE_PATH, "r")
    if not file then
        print("[DEBUG] Token file not found: " .. TOKEN_FILE_PATH)
        return nil, "token file not found"
    end
    
    local content = file:read("*all")
    file:close()
    
    print("[DEBUG] Token file content length: " .. (content and #content or 0))
    
    if not content or content == "" then
        print("[DEBUG] Token file is empty")
        return nil, "token file empty"
    end
    
    -- Simple JSON parsing for the token field
    local token = content:match('"token"%s*:%s*"([^"]+)"')
    local expires_at = content:match('"expires_at"%s*:%s*([%d%.]+)')
    
    print("[DEBUG] Parsed token: " .. (token and "found" or "not found"))
    print("[DEBUG] Parsed expires_at: " .. (expires_at or "not found"))
    
    if not token then
        print("[DEBUG] No token found in file content")
        return nil, "no token in file"
    end
    
    -- Check if token is still valid (with 60 second buffer)
    if expires_at then
        local current_time = os.time()
        local time_until_expiry = tonumber(expires_at) - current_time
        print("[DEBUG] Token expires in " .. time_until_expiry .. " seconds")
        
        if time_until_expiry <= 60 then
            return nil, "token expired"
        end
    end
    
    return token, nil
end

-- Shared function for URL and parameter processing
local function extract_redirect_info(request)
    local full_url = request_url(request)
    local query_string = full_url:match("%?(.+)") or ""
    local redirect_uri = query_string:match("redirect_uri=([^&]*)") 
    
    if not redirect_uri then
        return nil, "This gateway only proxies requests with redirect_uri parameter"
    end
    
    local target_url = url_decode(redirect_uri)
    if not target_url then
        return nil, "invalid redirect_uri"
    end
    
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
    
    return target_url, nil
end

-- Shared proxy logic for all HTTP methods
local function handle_proxy_request(request, response)
    print("[DEBUG] === Handling proxy request ===")
    print("[DEBUG] Method: " .. request_method(request))
    print("[DEBUG] Full URL: " .. request_url(request))
    
    -- Extract and validate redirect_uri
    local target_url, err = extract_redirect_info(request)
    if not target_url then
        print("[DEBUG] Failed to extract redirect_uri: " .. err)
        response_status(response, err:match("invalid") and 400 or 404)
        response_header(response, "Content-Type", "application/json")
        response_write(response, '{"error": "' .. err .. '"}')
        return
    end
    
    print("[DEBUG] Target URL: " .. target_url)

    -- Handle special cases (HTML, CSV, ZIP) - only for GET requests
    if request_method(request) == "GET" then
        -- Handle HTML and CSV logic (unchanged from original)
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
                response_header(response, "Location", new_url)
                response_header(response, "Cache-Control", "no-cache")
                response_status(response, 302)
                return
            end

            -- CSV download
            if params:match("format=csv") then
                -- Get token for authentication
                local token, err = get_token_from_file()
                if not token then
                    print("[DEBUG] Failed to get token for CSV: " .. (err or "unknown error"))
                    response_status(response, 503)
                    response_header(response, "Content-Type", "application/json")
                    response_write(response, '{"error": "' .. (err or "auth failed") .. '"}')
                    return
                end

                local body, status, headers = http_get(target_url, headers_table)

                -- Forward all headers from the remote response
                for k, v in pairs(headers) do
                    if type(v) == "string" then
                        response_header(response, k, v)
                    elseif type(v) == "table" then
                        response_header(response, k, table.concat(v, ", "))
                    end
                end

                response_write(response, body)
                response_status(response, status)
                return
            end
        end

        -- NEW separate check for ZIP export (pass-through headers)
        local base_zip, uuid_zip, path_zip = target_url:match("^(https?://[^/]+)/resource/processes/([%w%-]+)(/[^?]*)")
        if base_zip and uuid_zip and path_zip and path_zip:match("zipexport") then
            local body, status, headers = http_get(target_url)

            -- Forward all headers from the remote response
            for k, v in pairs(headers) do
                if type(v) == "string" then
                    response_header(response, k, v)
                elseif type(v) == "table" then
                    response_header(response, k, table.concat(v, ", "))
                end
            end

            response_write(response, body)
            response_status(response, status)
            return
        end
    end

    -- FALLBACK: OAuth proxy logic for all other requests
    print("[DEBUG] Getting token from file...")

    -- Get token from file
    local token, err = get_token_from_file()
    if not token then
        print("[DEBUG] Failed to get token: " .. (err or "unknown error"))
        response_status(response, 503)
        response_header(response, "Content-Type", "application/json")
        response_write(response, '{"error": "' .. (err or "auth failed") .. '"}')
        return
    end

    print("[DEBUG] Token retrieved successfully, making proxy request...")

    -- Proxy the request with auth header and User-Agent
    local headers_table = {
        ["Authorization"] = "Bearer " .. token,
        ["User-Agent"] = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"
    }

    print("[DEBUG] Request headers: Authorization=Bearer <token>")
    print("[DEBUG] Headers table type:", type(headers_table))
    print("[DEBUG] Headers table content:", headers_table and "exists" or "nil")

    local body, status, resp_headers
    local method = request_method(request)

    print("[DEBUG] Making " .. method .. " request to: " .. target_url)
    print("[DEBUG] About to call http_get with target_url and headers...")

    -- Debug the exact parameters being passed
    print("[DEBUG] Parameter 1 (target_url): " .. tostring(target_url))
    print("[DEBUG] Parameter 2 (headers_table) type: " .. type(headers_table))
    if type(headers_table) == "table" then
        print("[DEBUG] Headers table contents:")
        for k, v in pairs(headers_table) do
            if k == "Authorization" then
                print("[DEBUG]   " .. k .. " = Bearer <token_hidden>")
            else
                print("[DEBUG]   " .. k .. " = " .. tostring(v))
            end
        end
    end

    if method == "POST" or method == "PUT" or method == "PATCH" then
        -- Use new request body API for requests with body
        local req_body = request_body(request)
        print("[DEBUG] Request body length: " .. (req_body and #req_body or 0))
        print("[DEBUG] Calling http_post...")
        body, status, resp_headers = http_post(target_url, req_body, headers_table)
        print("[DEBUG] http_post returned")
    else
        -- GET, DELETE, HEAD, etc.
        print("[DEBUG] Calling http_get...")
        body, status, resp_headers = http_get(target_url, headers_table)
        print("[DEBUG] http_get returned")
    end

    print("[DEBUG] Response status: " .. (status or "nil"))
    print("[DEBUG] Response body length: " .. (body and #body or 0))

    -- Forward response headers from upstream
    if resp_headers and type(resp_headers) == "table" then
        print("[DEBUG] Forwarding " .. #resp_headers .. " response headers")
        for k, v in pairs(resp_headers) do
            if type(v) == "string" then
                response_header(response, k, v)
            elseif type(v) == "table" then
                response_header(response, k, table.concat(v, ", "))
            end
        end
    else
        print("[DEBUG] No headers to forward (type: " .. type(resp_headers) .. ")")
    end

    response_status(response, status or 502)
    response_write(response, body or '{"error": "upstream failed"}')
end

-- OAuth middleware - processes all requests on /auth* paths
chi_middleware(function(request, response, next)
    local url = request_url(request)
    local method = request_method(request)

    -- Only process requests that start with /auth
    if not url:match("^[^?]*/auth") then
        next()
        return
    end

    handle_proxy_request(request, response)
    -- Don't call next() - we handled the request completely
end)

print("✅ Simplified OAuth middleware loaded successfully")
print("🚀 Features: File-based token reading, no HTTP calls")
print("⚡ Performance: Eliminates OAuth API timeouts completely")