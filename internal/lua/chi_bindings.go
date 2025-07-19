// Package lua provides Lua-to-Chi bridge functions that allow Lua scripts
// to register routes, middleware, and route groups directly with the Chi router.
package lua

import (
	"fmt"
	"io"
	"net/http"

	lua "github.com/yuin/gopher-lua"

	"keystone-gateway/internal/routing"
)

// setupChiBindings sets up Lua bindings for Chi router functions
func (e *Engine) SetupChiBindings(L *lua.LState, scriptTag, tenantName string) {
	// Register Lua functions that can be called from scripts
	L.SetGlobal("chi_route", L.NewFunction(func(L *lua.LState) int {
		return e.luaChiRoute(L, tenantName)
	}))
	L.SetGlobal("chi_middleware", L.NewFunction(func(L *lua.LState) int {
		return e.luaChiMiddleware(L, tenantName)
	}))
	L.SetGlobal("chi_group", L.NewFunction(func(L *lua.LState) int {
		return e.luaChiGroup(L, tenantName)
	}))
	L.SetGlobal("log", L.NewFunction(e.luaLog))
}

// luaChiRoute handles route registration from Lua: chi_route(method, pattern, handler)
func (e *Engine) luaChiRoute(L *lua.LState, tenantName string) int {
	method := L.ToString(1)
	pattern := L.ToString(2)
	handlerFunc := L.ToFunction(3)

	if method == "" || pattern == "" || handlerFunc == nil {
		L.ArgError(1, "chi_route requires method, pattern, and handler function")
		return 0
	}

	// Create efficient Go HTTP handler that reuses the current Lua function
	handler := func(w http.ResponseWriter, r *http.Request) {
		// Create lightweight response wrapper
		respWriter := &luaResponseWriter{w: w}
		respTable := createLuaResponse(L, respWriter)
		reqTable := createLuaRequest(L, r)

		// Call the Lua handler function directly
		err := L.CallByParam(lua.P{
			Fn:      handlerFunc,
			NRet:    0,
			Protect: true,
		}, respTable, reqTable)

		if err != nil {
			http.Error(w, fmt.Sprintf("Lua handler error: %v", err), http.StatusInternalServerError)
		}
	}

	// Register the route with the simplified registry
	err := e.routeRegistry.RegisterRoute(routing.RouteDefinition{
		TenantName: tenantName,
		Method:     method,
		Pattern:    pattern,
		Handler:    handler,
	})
	if err != nil {
		L.RaiseError("Failed to register route: %v", err)
	}

	return 0
}

// luaChiMiddleware handles middleware registration: chi_middleware(pattern, middleware_func)
func (e *Engine) luaChiMiddleware(L *lua.LState, tenantName string) int {
	pattern := L.ToString(1)
	middlewareFunc := L.ToFunction(2)

	if pattern == "" || middlewareFunc == nil {
		L.ArgError(1, "chi_middleware requires pattern and middleware function")
		return 0
	}

	// Create Go middleware that calls the Lua function
	middleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Create Lua request wrapper
			reqTable := createLuaRequest(L, r)

			// Create Lua response wrapper
			respWrapper := &luaResponseWriter{w: w, L: L}
			respTable := createLuaResponse(L, respWrapper)

			// Create next function for Lua
			nextFunc := L.NewFunction(func(L *lua.LState) int {
				next.ServeHTTP(w, r)
				return 0
			})

			// Call the Lua middleware function
			if err := L.CallByParam(lua.P{
				Fn:      middlewareFunc,
				NRet:    0,
				Protect: true,
			}, respTable, reqTable, nextFunc); err != nil {
				http.Error(w, fmt.Sprintf("Lua middleware error: %v", err), http.StatusInternalServerError)
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

// luaLog provides logging from Lua scripts
func (e *Engine) luaLog(L *lua.LState) int {
	message := L.ToString(1)
	fmt.Printf("[Lua] %s\n", message)
	return 0
}

// luaResponseWriter wraps http.ResponseWriter for Lua integration
type luaResponseWriter struct {
	w http.ResponseWriter
	L *lua.LState
}

// createLuaRequest creates a Lua table representing an HTTP request
func createLuaRequest(L *lua.LState, r *http.Request) *lua.LTable {
	reqTable := L.NewTable()

	// Basic request info
	reqTable.RawSetString("method", lua.LString(r.Method))
	reqTable.RawSetString("url", lua.LString(r.URL.String()))
	reqTable.RawSetString("path", lua.LString(r.URL.Path))
	reqTable.RawSetString("host", lua.LString(r.Host))

	// Headers
	headersTable := L.NewTable()
	for key, values := range r.Header {
		if len(values) > 0 {
			headersTable.RawSetString(key, lua.LString(values[0]))
		}
	}
	reqTable.RawSetString("headers", headersTable)

	// URL parameters (would be populated by Chi)
	paramsTable := L.NewTable()
	reqTable.RawSetString("params", paramsTable)

	// Body content storage
	var bodyContent string
	if r.Body != nil {
		body, err := io.ReadAll(r.Body)
		if err == nil {
			bodyContent = string(body)
		}
	}

	// Helper methods with colon syntax support
	headerFunc := L.NewFunction(func(L *lua.LState) int {
		startIdx := 1
		if L.GetTop() > 1 && L.Get(1) == reqTable {
			startIdx = 2
		}
		headerName := L.ToString(startIdx)
		headerValue := r.Header.Get(headerName)
		L.Push(lua.LString(headerValue))
		return 1
	})

	// Add body() method for colon syntax support
	bodyFunc := L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(bodyContent))
		return 1
	})

	reqTable.RawSetString("header", headerFunc)
	reqTable.RawSetString("body", bodyFunc)

	return reqTable
}

// createLuaResponse creates a Lua table representing an HTTP response with colon method support
func createLuaResponse(L *lua.LState, w *luaResponseWriter) *lua.LTable {
	respTable := L.NewTable()

	// Create method functions that work with both colon and dot syntax
	writeFunc := L.NewFunction(func(L *lua.LState) int {
		// Skip 'self' parameter if called with colon syntax
		startIdx := 1
		if L.GetTop() > 1 && L.Get(1) == respTable {
			startIdx = 2
		}
		content := L.ToString(startIdx)
		w.w.Write([]byte(content))
		return 0
	})

	headerFunc := L.NewFunction(func(L *lua.LState) int {
		startIdx := 1
		if L.GetTop() > 2 && L.Get(1) == respTable {
			startIdx = 2
		}
		key := L.ToString(startIdx)
		value := L.ToString(startIdx + 1)
		w.w.Header().Set(key, value)
		return 0
	})

	statusFunc := L.NewFunction(func(L *lua.LState) int {
		startIdx := 1
		if L.GetTop() > 1 && L.Get(1) == respTable {
			startIdx = 2
		}
		statusCode := L.ToInt(startIdx)
		w.w.WriteHeader(statusCode)
		return 0
	})

	jsonFunc := L.NewFunction(func(L *lua.LState) int {
		startIdx := 1
		if L.GetTop() > 1 && L.Get(1) == respTable {
			startIdx = 2
		}
		jsonContent := L.ToString(startIdx)
		w.w.Header().Set("Content-Type", "application/json")
		w.w.Write([]byte(jsonContent))
		return 0
	})

	// Set methods on table
	respTable.RawSetString("write", writeFunc)
	respTable.RawSetString("header", headerFunc)
	respTable.RawSetString("status", statusFunc)
	respTable.RawSetString("json", jsonFunc)

	return respTable
}