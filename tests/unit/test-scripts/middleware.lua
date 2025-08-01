
chi_middleware("/", function(request, response, next)
    response:header("X-Custom-Header", "Added-By-Lua")
    response:header("X-Gateway", "Keystone")
    next()
end)
