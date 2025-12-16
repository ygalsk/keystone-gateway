-- Advanced Lua handlers demonstrating LuaRocks modules
-- Install modules with:
--   sudo luarocks install lua-cjson
--   sudo luarocks install inspect
--   sudo luarocks install lpeg
--   sudo luarocks install luasocket

-- Try to load various LuaRocks modules
local modules = {
    cjson = pcall(require, "cjson"),
    inspect = pcall(require, "inspect"),
    lpeg = pcall(require, "lpeg"),
    socket = pcall(require, "socket")
}

-- Log which modules are available
-- log("=== LuaRocks Module Status ===")
for name, loaded in pairs(modules) do
    -- log(string.format("  %s: %s", name, loaded and "LOADED" or "NOT AVAILABLE"))
end

-- Try to get the actual modules
local cjson_ok, cjson = pcall(require, "cjson")
local inspect_ok, inspect = pcall(require, "inspect")
local lpeg_ok, lpeg = pcall(require, "lpeg")
local socket_ok, socket = pcall(require, "socket")

-- Handler: Show LuaRocks module status
function luarocks_status(req)
    local status_info = {
        luajit_version = jit and jit.version or "Not running on LuaJIT",
        lua_version = _VERSION,
        modules = {}
    }

    -- Check which modules are loaded
    if cjson_ok then
        status_info.modules.cjson = {
            loaded = true,
            version = cjson._VERSION or "unknown"
        }
    else
        status_info.modules.cjson = {
            loaded = false,
            install = "sudo luarocks install lua-cjson"
        }
    end

    if inspect_ok then
        status_info.modules.inspect = {
            loaded = true,
            description = "Human-readable table representation"
        }
    else
        status_info.modules.inspect = {
            loaded = false,
            install = "sudo luarocks install inspect"
        }
    end

    if lpeg_ok then
        status_info.modules.lpeg = {
            loaded = true,
            version = lpeg.version or "unknown",
            description = "Pattern matching library"
        }
    else
        status_info.modules.lpeg = {
            loaded = false,
            install = "sudo luarocks install lpeg"
        }
    end

    if socket_ok then
        status_info.modules.socket = {
            loaded = true,
            description = "Network support library"
        }
    else
        status_info.modules.socket = {
            loaded = false,
            install = "sudo luarocks install luasocket"
        }
    end

    -- Use inspect if available for pretty output
    local body
    if inspect_ok then
        body = "LuaRocks Status:\n\n" .. inspect(status_info)
    else
        body = cjson_ok and cjson.encode(status_info) or "Install lua-cjson for JSON output"
    end

    return {
        status = 200,
        body = body,
        headers = {
            ["Content-Type"] = inspect_ok and "text/plain" or "application/json"
        }
    }
end

-- Handler: Advanced JSON processing with cjson
function json_processing(req)
    if not cjson_ok then
        return {
            status = 503,
            body = '{"error":"cjson module not available","install":"sudo luarocks install lua-cjson"}',
            headers = {["Content-Type"] = "application/json"}
        }
    end

    -- Parse request body as JSON
    local data
    if req.body and req.body ~= "" then
        local ok, result = pcall(cjson.decode, req.body)
        if not ok then
            return {
                status = 400,
                body = cjson.encode({error = "Invalid JSON", details = result}),
                headers = {["Content-Type"] = "application/json"}
            }
        end
        data = result
    else
        data = {message = "No body provided"}
    end

    -- Process and return
    local response = {
        received = data,
        processed_at = os.time(),
        luajit = jit and jit.version or _VERSION
    }

    return {
        status = 200,
        body = cjson.encode(response),
        headers = {["Content-Type"] = "application/json"}
    }
end

-- Handler: Pattern matching with LPEG
function pattern_matching(req)
    if not lpeg_ok then
        return {
            status = 503,
            body = '{"error":"lpeg module not available","install":"sudo luarocks install lpeg"}',
            headers = {["Content-Type"] = "application/json"}
        }
    end

    -- Get text from query param
    local text = req.query.text or "Hello, World! Email: test@example.com"

    -- Simple email pattern matcher using LPEG
    local alpha = lpeg.R("az", "AZ")
    local digit = lpeg.R("09")
    local dot = lpeg.P(".")
    local at = lpeg.P("@")

    -- Email pattern: word@word.word
    local word = (alpha + digit + lpeg.P("_") + lpeg.P("-"))^1
    local domain_part = (alpha + digit + lpeg.P("-"))^1
    local email_pattern = lpeg.C(word * at * domain_part * (dot * domain_part)^1)

    -- Find all emails
    local emails = {}
    local pos = 1
    while pos <= #text do
        local match = lpeg.match(email_pattern, text, pos)
        if match then
            table.insert(emails, match)
            pos = pos + #match
        else
            pos = pos + 1
        end
    end

    local response = {
        text = text,
        emails_found = emails,
        count = #emails,
        lpeg_version = lpeg.version or "unknown"
    }

    local body = cjson_ok and cjson.encode(response) or
                 (inspect_ok and inspect(response) or "Install cjson or inspect")

    return {
        status = 200,
        body = body,
        headers = {["Content-Type"] = cjson_ok and "application/json" or "text/plain"}
    }
end

-- Handler: Network operations with luasocket
function network_info(req)
    if not socket_ok then
        return {
            status = 503,
            body = '{"error":"socket module not available","install":"sudo luarocks install luasocket"}',
            headers = {["Content-Type"] = "application/json"}
        }
    end

    local response = {
        client_ip = req.remote_addr,
        server_time = socket.gettime(),
        dns_available = socket.dns ~= nil,
        example = "Use socket.dns.toip('google.com') for DNS lookups"
    }

    -- Try DNS lookup if hostname provided
    if req.query.lookup and socket.dns then
        local ip, err = socket.dns.toip(req.query.lookup)
        if ip then
            response.dns_lookup = {
                hostname = req.query.lookup,
                ip = ip
            }
        else
            response.dns_error = err
        end
    end

    local body = cjson_ok and cjson.encode(response) or
                 (inspect_ok and inspect(response) or "Install cjson")

    return {
        status = 200,
        body = body,
        headers = {["Content-Type"] = cjson_ok and "application/json" or "text/plain"}
    }
end

-- Handler: Comprehensive demo combining all modules
function luarocks_demo(req)
    local features = {}

    -- Feature 1: JSON encoding (cjson)
    if cjson_ok then
        table.insert(features, {
            module = "cjson",
            capability = "Fast JSON encoding/decoding",
            example = "cjson.encode({hello='world'})"
        })
    end

    -- Feature 2: Pretty printing (inspect)
    if inspect_ok then
        table.insert(features, {
            module = "inspect",
            capability = "Human-readable table output",
            example = "inspect({nested={table=true}})"
        })
    end

    -- Feature 3: Pattern matching (lpeg)
    if lpeg_ok then
        table.insert(features, {
            module = "lpeg",
            capability = "Powerful pattern matching",
            example = "Email extraction, parsing, etc."
        })
    end

    -- Feature 4: Networking (luasocket)
    if socket_ok then
        table.insert(features, {
            module = "luasocket",
            capability = "Network operations, DNS, timing",
            example = "socket.dns.toip('example.com')"
        })
    end

    local response = {
        message = "LuaRocks + LuaJIT + Go-Owned Routing Demo",
        luajit = jit and jit.version or _VERSION,
        features_available = features,
        modules_loaded = {
            cjson = cjson_ok,
            inspect = inspect_ok,
            lpeg = lpeg_ok,
            luasocket = socket_ok
        },
        api_endpoints = {
            "/api/luarocks-status",
            "/api/json-processing",
            "/api/pattern-matching?text=your-text",
            "/api/network-info?lookup=google.com"
        }
    }

    -- Use best available formatter
    local body
    if inspect_ok then
        body = "=== LuaRocks Demo ===\n\n" .. inspect(response)
    elseif cjson_ok then
        body = cjson.encode(response)
    else
        body = "Install lua-cjson or inspect for formatted output"
    end

    return {
        status = 200,
        body = body,
        headers = {
            ["Content-Type"] = inspect_ok and "text/plain" or "application/json"
        }
    }
end

-- log("Advanced LuaRocks handlers loaded")
