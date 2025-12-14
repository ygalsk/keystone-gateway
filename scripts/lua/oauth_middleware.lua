-- oauth_middleware.lua - Simplified OAuth proxy reading from token file
-- Uses the new object-oriented API for request and response handling.

local TOKEN_FILE_PATH = "/tmp/oauth_token.json"

local function url_encode(str)
    if not str then return "" end
    str = str:gsub("([^%w%-_.~])", function(c)
        return string.format("%%%02X", string.byte(c))
    end)
    return str
end

local function url_decode(str)
    if not str then return nil end
    str = str:gsub("+", " ")
    str = str:gsub("%%(%x%x)", function(hex)
        return string.char(tonumber(hex, 16))
    end)
    return str
end

local function get_token_from_file()
    local file = io.open(TOKEN_FILE_PATH, "r")
    if not file then
        return nil, "token file not found"
    end
    local content = file:read("*all")
    file:close()

    if not content or content == "" then
        return nil, "token file empty"
    end

    local token = content:match('"token"%s*:%s*"([^"]+)"')
    local expires_at = content:match('"expires_at"%s*:%s*([%d%.]+)')

    if not token then
        return nil, "no token in file"
    end

    if expires_at then
        if (tonumber(expires_at) - os.time()) <= 60 then
            return nil, "token expired"
        end
    end
    return token, nil
end

local function extract_redirect_info(req)
    local full_url = req.URL
    local query_string = full_url:match("%?(.+)") or ""
    local redirect_uri = query_string:match("redirect_uri=([^&]*)")

    if not redirect_uri then
        return nil, "This gateway only proxies requests with redirect_uri parameter"
    end

    local target_url = url_decode(redirect_uri)
    if not target_url then
        return nil, "invalid redirect_uri"
    end

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

local function handle_proxy_request(req, res)
    local target_url, err = extract_redirect_info(req)
    if not target_url then
        res:Status(err:match("invalid") and 400 or 404)
        res:Header("Content-Type", "application/json")
        res:Write('{"error": "' .. err .. '"}')
        return
    end

    -- Special cases for GET requests
    if req.Method == "GET" then
        local base, uuid, params = target_url:match("^(https?://[^/]+)/resource/processes/([%w%-]+)%?(.*)$")
        if base and uuid and params then
            if params:match("format=html") then
                params = params:gsub("(^&?format=html&?)", ""):gsub("(&?format=html$)", "")
                params = params:gsub("&&", "&"):gsub("^&", ""):gsub("&$", "")
                local new_query = "uuid=" .. uuid
                if #params > 0 then new_query = new_query .. "&" .. params end
                local new_url = string.format("%s/datasetdetail/process.xhtml?%s", base, new_query)
                res:Header("Location", new_url)
                res:Header("Cache-Control", "no-cache")
                res:Status(302)
                return
            end

            if params:match("format=csv") then
                local token, token_err = get_token_from_file()
                if not token then
                    res:Status(503)
                    res:Header("Content-Type", "application/json")
                    res:Write('{"error": "' .. (token_err or "auth failed") .. '"}')
                    return
                end
                local result = HTTP:Get(target_url, { headers = { ["Authorization"] = "Bearer " .. token } })
                for k, v in pairs(result.Headers) do
                    res:Header(k, v)
                end
                res:Status(result.Status)
                res:Write(result.Body)
                return
            end
        end

        local base_zip, uuid_zip, path_zip = target_url:match("^(https?://[^/]+)/resource/processes/([%w%-]+)(/[^?]*)")
        if base_zip and uuid_zip and path_zip and path_zip:match("zipexport") then
            local result = HTTP:Get(target_url)
            for k, v in pairs(result.Headers) do
                res:Header(k, v)
            end
            res:Status(result.Status)
            res:Write(result.Body)
            return
        end
    end

    -- Fallback OAuth proxy logic
    local token, token_err = get_token_from_file()
    if not token then
        res:Status(503)
        res:Header("Content-Type", "application/json")
        res:Write('{"error": "' .. (token_err or "auth failed") .. '"}')
        return
    end

    local headers = {
        ["Authorization"] = "Bearer " .. token,
        ["User-Agent"] = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"
    }

    local result
    local method = req.Method

    if method == "POST" or method == "PUT" or method == "PATCH" then
        local req_body, body_err = req:Body()
        if body_err then
            res:Status(500)
            res:Write('{"error": "failed to read request body"}')
            return
        end
        result = HTTP:Post(target_url, req_body, { headers = headers })
    else
        result = HTTP:Get(target_url, { headers = headers })
    end

    if result and result.Headers then
        for k, v in pairs(result.Headers) do
            res:Header(k, v)
        end
    end

    res:Status(result and result.Status or 502)
    res:Write(result and result.Body or '{"error": "upstream failed"}')
end

chi_middleware(function(req, res, next)
    if not req.URL:match("^[^?]*/auth") then
        next()
        return
    end
    handle_proxy_request(req, res)
end)

print("✅ OAuth middleware with new API loaded successfully")
