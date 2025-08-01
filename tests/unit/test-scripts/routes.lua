
chi_route("GET", "/hello", function(request, response)
    response:header("Content-Type", "text/plain")
    response:write("Hello from Lua route!")
end)

chi_route("POST", "/data", function(request, response)
    response:header("Content-Type", "application/json")
    response:write('{"message": "Data received"}')
end)
