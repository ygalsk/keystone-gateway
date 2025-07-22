-- Test route script
log("Setting up test routes")

route("GET", "/test", function(w, r)
    w:write("test response")
end)