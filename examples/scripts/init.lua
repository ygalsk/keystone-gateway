-- Global initialization script
-- This runs once at startup to set up global functions and libraries

log("Initializing Lua environment...")

-- Enable LuaJIT optimizations
if jit then
    jit.opt.start(
        "maxtrace=10000",      -- Allow more traces (default: 1000)
        "maxrecord=20000",     -- Allow longer traces (default: 4000)
        "maxirconst=10000",    -- More IR constants (default: 500)
        "maxmcode=4096",       -- Bigger machine code cache in KB (default: 512)
        "maxsnap=1000",        -- More snapshots (default: 500)
        "hotexit=10",          -- Lower hotness threshold (default: 56)
        "hotloop=40",          -- Lower loop hotness (default: 56)
        "tryside=4"            -- Trace side exits (default: 4)
    )
end


-- if jit then
--     jit.off()
--     jit.flush()
-- end


-- log("Lua environment initialized")
