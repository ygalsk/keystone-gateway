package lua

import (
	"net/http"

	"keystone-gateway/internal/lua/modules"

	"github.com/go-chi/chi/v5"
	lua "github.com/yuin/gopher-lua"
	"layeh.com/gopher-luar"
)

// SetupChiBindings exposes modules and routing functions to the Lua state.
// This function now uses gopher-luar to eliminate manual bindings.
func (e *Engine) SetupChiBindings(L *lua.LState, r chi.Router) {
	// --- Module Bindings (Deep Modules) ---
	// Expose the HTTP client as a global.
	L.SetGlobal("HTTP", luar.New(L, modules.NewHTTPClient()))

	// --- Routing Functions ---
	// These still require manual functions because they interact with the router 'r',
	// but they are now much simpler. They create the deep modules (Request, Response)
	// and pass them to the Lua handler.

	// chi_route(method, pattern, handler)
	L.SetGlobal("chi_route", L.NewFunction(func(ls *lua.LState) int {
		method := ls.CheckString(1)
		pattern := ls.CheckString(2)
		handler := ls.CheckFunction(3)

		r.Method(method, pattern, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			// Get a Lua state from the pool for this request
			L := e.statePool.Get()
			defer e.statePool.Put(L)

			// Create our deep modules (wrappers)
			reqModule := modules.NewRequest(req, e.config.RequestLimits.MaxBodySize)
			resModule := modules.NewResponse(w)

			// Call the Lua handler function with the modules.
			// gopher-luar will automatically make their fields and methods available.
			if err := L.CallByParam(lua.P{
				Fn:      handler,
				NRet:    0,
				Protect: true,
			}, luar.New(L, reqModule), luar.New(L, resModule)); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}))

		return 0 // No return values to Lua
	}))

	// chi_middleware(handler)
	L.SetGlobal("chi_middleware", L.NewFunction(func(ls *lua.LState) int {
		handler := ls.CheckFunction(1)

		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				L := e.statePool.Get()
				defer e.statePool.Put(L)

				reqModule := modules.NewRequest(req, e.config.RequestLimits.MaxBodySize)
				resModule := modules.NewResponse(w)

				// Create the 'next' function for the Lua middleware to call
				nextFunc := L.NewFunction(func(L *lua.LState) int {
					// When 'next()' is called in Lua, we need to pass the *original* request
					// to the next Go handler, not our wrapper.
					next.ServeHTTP(w, req)
					return 0
				})

				// Call the Lua middleware
				if err := L.CallByParam(lua.P{
					Fn:      handler,
					NRet:    0,
					Protect: true,
				}, luar.New(L, reqModule), luar.New(L, resModule), nextFunc); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
			})
		})
		return 0
	}))
}