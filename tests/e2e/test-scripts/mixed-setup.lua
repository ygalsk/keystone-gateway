
-- Some routes handled by Lua
chi_route("GET", "/custom", function(request, response)
    response:write("Custom Lua Handler")
end)

-- Other routes fall through to backend proxy
print("Mixed Lua and proxy setup complete")
