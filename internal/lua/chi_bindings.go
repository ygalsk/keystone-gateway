// Package lua provides Lua-to-Chi bridge functions that allow Lua scripts
// to register routes, middleware, and route groups directly with the Chi router.
package lua

import (
	"fmt"
	"net/http"

	lua "github.com/yuin/gopher-lua"

	"keystone-gateway/internal/routing"
)

// setupChiBindings sets up Lua bindings for Chi router functions
func (e *Engine) SetupChiBindings(L *lua.LState, scriptTag, tenantName string) {
	// Register Lua functions that can be called from scripts
	L.SetGlobal("chi_route", L.NewFunction(func(L *lua.LState) int {
		return e.luaChiRoute(L, scriptTag, tenantName)
	}))
	L.SetGlobal("chi_middleware", L.NewFunction(func(L *lua.LState) int {
		return e.luaChiMiddleware(L, scriptTag, tenantName)
	}))
	L.SetGlobal("chi_group", L.NewFunction(func(L *lua.LState) int {
		return e.luaChiGroup(L, tenantName)
	}))
	L.SetGlobal("chi_param", L.NewFunction(func(L *lua.LState) int {
		// This will be overridden in the actual handler context with real parameter values
		requestTable := L.ToTable(1)
		paramName := L.ToString(2)

		if requestTable != nil {
			if paramsTable := requestTable.RawGetString("params"); paramsTable != lua.LNil {
				if paramTable, ok := paramsTable.(*lua.LTable); ok {
					if param := paramTable.RawGetString(paramName); param != lua.LNil {
						L.Push(param)
						return 1
					}
				}
			}
		}

		// Default fallback
		L.Push(lua.LString(""))
		return 1
	}))
}

// luaChiRoute handles route registration from Lua: chi_route(method, pattern, handler)
func (e *Engine) luaChiRoute(L *lua.LState, scriptTag, tenantName string) int {
	method := L.ToString(1)
	pattern := L.ToString(2)
	handlerFunc := L.ToFunction(3)

	if method == "" || pattern == "" || handlerFunc == nil {
		L.ArgError(1, "chi_route requires method, pattern, and handler function")
		return 0
	}

	// Extract the Lua function source code for later execution
	functionName := fmt.Sprintf("handler_%s_%s_%d", method, pattern, L.GetTop())
	L.SetGlobal(functionName, handlerFunc)

	// Get the script content for this handler
	scriptContent, exists := e.GetScript(scriptTag)
	if !exists {
		L.RaiseError("Script not found: %s", scriptTag)
		return 0
	}

	// Create a thread-safe handler using the state pool
	luaHandler := NewLuaHandler(scriptContent, functionName, tenantName, scriptTag, e.statePool, e)

	
	// Register the route with the simplified registry
	err := e.routeRegistry.RegisterRoute(routing.RouteDefinition{
		TenantName: tenantName,
		Method:     method,
		Pattern:    pattern,
		Handler:    luaHandler.ServeHTTP,
	})
	if err != nil {
		L.RaiseError("Failed to register route: %v", err)
	}

	return 0
}

// luaChiMiddleware handles middleware registration: chi_middleware(pattern, middleware_func)
func (e *Engine) luaChiMiddleware(L *lua.LState, scriptTag, tenantName string) int {
	pattern := L.ToString(1)
	middlewareFunc := L.ToFunction(2)


	if pattern == "" || middlewareFunc == nil {
		L.ArgError(1, "chi_middleware requires pattern and middleware function")
		return 0
	}

	// Store the middleware function with a global name for later access
	funcName := fmt.Sprintf("middleware_%s_%s_%d", tenantName, pattern, L.GetTop())
	L.SetGlobal(funcName, middlewareFunc)
	
	// Get the script content for this middleware
	scriptContent, exists := e.GetScript(scriptTag)
	if !exists {
		L.RaiseError("Script not found: %s", scriptTag)
		return 0
	}
	
	// Create Go middleware that calls the Lua function using state pool for safety
	middleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Use state pool for thread-safe middleware execution
			luaState := e.statePool.Get()
			defer e.statePool.Put(luaState)

			// Execute the script to load all functions including the middleware
			if err := luaState.DoString(scriptContent); err != nil {
				// On error, continue to next handler
				next.ServeHTTP(w, r)
				return
			}

			// Set up Chi bindings for this execution context
			e.SetupChiBindings(luaState, scriptTag, tenantName)

			// Get the middleware function from the loaded script
			middlewareFunc := luaState.GetGlobal(funcName)
			if middlewareFunc.Type() != lua.LTFunction {
				// If function not found, continue to next handler
				next.ServeHTTP(w, r)
				return
			}

			// Execute middleware function with proper context
			respWriter := &luaResponseWriter{w: w}
			respTable := createLuaResponse(luaState, respWriter)
			reqTable := createLuaRequest(luaState, r)

			// Track whether next was called
			nextCalled := false
			
			// Create next function wrapper that tracks if it was called
			nextFunc := luaState.NewFunction(func(L *lua.LState) int {
				nextCalled = true
				return 0
			})

			// Call middleware function with parameters in the right order: (w, r, next)
			err := luaState.CallByParam(lua.P{
				Fn:      middlewareFunc.(*lua.LFunction),
				NRet:    0,
				Protect: true,
			}, respTable, reqTable, nextFunc)

			// Only call next handler if Lua middleware called next()
			if err != nil || nextCalled {
				next.ServeHTTP(w, r)
			}
		})
	}

	// Register with the route registry
	err := e.routeRegistry.RegisterMiddleware(routing.MiddlewareDefinition{
		TenantName: tenantName,
		Pattern:    pattern,
		Middleware: middleware,
	})
	if err != nil {
		L.RaiseError("Failed to register middleware: %v", err)
	}

	return 0
}

// luaChiGroup handles route group registration: chi_group(pattern, setup_func)
func (e *Engine) luaChiGroup(L *lua.LState, tenantName string) int {
	pattern := L.ToString(1)
	setupFunc := L.ToFunction(2)

	if pattern == "" || setupFunc == nil {
		L.ArgError(1, "chi_group requires pattern and setup function")
		return 0
	}

	// Simplified group implementation
	groupDef := routing.RouteGroupDefinition{
		TenantName: tenantName,
		Pattern:    pattern,
		Middleware: []func(http.Handler) http.Handler{},
		Routes:     []routing.RouteDefinition{},
		Subgroups:  []routing.RouteGroupDefinition{},
	}

	err := e.routeRegistry.RegisterRouteGroup(groupDef)
	if err != nil {
		L.RaiseError("Failed to register route group: %v", err)
	}

	return 0
}
