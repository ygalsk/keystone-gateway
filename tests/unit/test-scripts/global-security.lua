
-- Global security middleware
chi_middleware("/", function(request, response, next)
    response:header("X-Frame-Options", "DENY")
    response:header("X-Content-Type-Options", "nosniff")
    next()
end)
