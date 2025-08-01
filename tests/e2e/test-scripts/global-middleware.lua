
chi_middleware("/", function(request, response, next)
    response:header("X-Gateway", "Keystone")
    response:header("X-Request-ID", "req-" .. math.random(1000, 9999))
    next()
end)
