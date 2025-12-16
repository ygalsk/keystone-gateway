-- Example Lua handlers using LuaRocks modules
-- Demonstrates Go-Owned Routing + LuaJIT architecture

-- Try to load cjson from LuaRocks (falls back to simple JSON if not available)
local cjson_ok, cjson = pcall(require, "cjson")
if not cjson_ok then
    -- log("WARNING: cjson not available, using simple JSON encoding")
    -- log("Install with: sudo luarocks install lua-cjson")
end

-- Helper: encode JSON using cjson or fallback
local function encode_json(data)
    if cjson_ok then
        return cjson.encode(data)
    else
        -- Fallback to simple encoding from init.lua
        return json_response(data).body
    end
end

-- Helper: decode JSON using cjson
local function decode_json(str)
    if cjson_ok then
        return cjson.decode(str)
    else
        -- log("ERROR: JSON decoding requires cjson module")
        return nil
    end
end

-- Simple hello handler
function hello_handler(req)
    -- log("Hello handler called for path: " .. req.path)

    return {
        status = 200,
        body = encode_json({
            message = "Hello from LuaJIT!",
            method = req.method,
            path = req.path,
            luajit = jit and jit.version or "unknown"
        }),
        headers = {["Content-Type"] = "application/json"}
    }
end

-- Handler with URL params (from Chi)
function get_user(req)
    local user_id = req.params.id

    -- log("Getting user: " .. user_id)

    -- Simulate backend call using http_get primitive
    local resp, err = http_get("https://jsonplaceholder.typicode.com/users/" .. user_id)

    if err then
        return {
            status = 500,
            body = encode_json({error = err}),
            headers = {["Content-Type"] = "application/json"}
        }
    end

    if resp.status == 404 then
        return {
            status = 404,
            body = encode_json({error = "User not found", id = user_id}),
            headers = {["Content-Type"] = "application/json"}
        }
    end

    -- Return the user data
    return {
        status = 200,
        body = resp.body,
        headers = {["Content-Type"] = "application/json"}
    }
end

-- Handler that creates a user
function create_user(req)
    -- log("Creating user")

    -- Parse request body
    local user_data = req.body

    if not user_data or user_data == "" then
        return {
            status = 400,
            body = encode_json({error = "Request body required"}),
            headers = {["Content-Type"] = "application/json"}
        }
    end

    -- Make POST request to backend
    local resp, err = http_post(
        "https://jsonplaceholder.typicode.com/users",
        user_data,
        {["Content-Type"] = "application/json"}
    )

    if err then
        return {
            status = 500,
            body = encode_json({error = err}),
            headers = {["Content-Type"] = "application/json"}
        }
    end

    return {
        status = resp.status,
        body = resp.body,
        headers = {["Content-Type"] = "application/json"}
    }
end

-- Middleware: require authentication
function require_auth(req, next)
    local auth_header = req.headers["Authorization"]

    if not auth_header or not auth_header:match("^Bearer ") then
        -- log("Authentication failed - no bearer token")
        return {
            status = 401,
            body = encode_json({
                error = "Unauthorized",
                message = "Bearer token required"
            }),
            headers = {["Content-Type"] = "application/json"}
        }
    end

    -- log("Authentication successful")

    -- Call next handler in chain
    next()
    return nil  -- nil means middleware passed, continue chain
end

-- Article handlers (for route groups)
function list_articles(req)
    -- Get query parameters
    local limit = req.query.limit or "10"

    -- log("Listing articles, limit: " .. limit)

    return {
        status = 200,
        body = encode_json({
            articles = {},
            limit = tonumber(limit),
            total = 0
        }),
        headers = {["Content-Type"] = "application/json"}
    }
end

function create_article(req)
    -- log("Creating article")

    return {
        status = 201,
        body = encode_json({
            message = "Article created",
            id = "123"
        }),
        headers = {["Content-Type"] = "application/json"}
    }
end

-- Error handlers
function handle_404(req)
    -- log("404 Not Found: " .. req.path)

    return {
        status = 404,
        body = encode_json({
            error = "Not Found",
            path = req.path,
            message = "The requested resource does not exist"
        }),
        headers = {["Content-Type"] = "application/json"}
    }
end

function handle_405(req)
    -- log("405 Method Not Allowed: " .. req.method .. " " .. req.path)

    return {
        status = 405,
        body = encode_json({
            error = "Method Not Allowed",
            method = req.method,
            path = req.path
        }),
        headers = {["Content-Type"] = "application/json"}
    }
end

-- log("Handlers loaded successfully")
