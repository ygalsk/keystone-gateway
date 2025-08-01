
chi_middleware("/", function(request, response, next)
    response:header("X-Global", "Applied")
    next()
end)
