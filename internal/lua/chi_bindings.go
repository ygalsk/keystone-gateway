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

// ChiBindings holds the state needed for Lua-Chi integration
type ChiBindings struct {
	tenantName string
	registry   *routing.LuaRouteRegistry
}

// setupChiBindings sets up Lua bindings for Chi router functions
func (e *Engine) setupChiBindings(L *lua.LState, tenantName string) {
	bindings := &ChiBindings{
		tenantName: tenantName,
		registry:   e.routeRegistry,
	}

	// Register Lua functions that can be called from scripts
	L.SetGlobal("chi_route", L.NewFunction(bindings.luaChiRoute))
	L.SetGlobal("chi_middleware", L.NewFunction(bindings.luaChiMiddleware))
	L.SetGlobal("chi_group", L.NewFunction(bindings.luaChiGroup))
	L.SetGlobal("chi_mount", L.NewFunction(bindings.luaChiMount))
	L.SetGlobal("chi_param", L.NewFunction(bindings.luaChiParam))
	L.SetGlobal("log", L.NewFunction(bindings.luaLog))

	// Create request and response wrapper functions
	L.SetGlobal("create_handler", L.NewFunction(bindings.luaCreateHandler))
}

// luaChiRoute handles route registration from Lua: chi_route(method, pattern, handler)
func (b *ChiBindings) luaChiRoute(L *lua.LState) int {
	method := L.ToString(1)
	pattern := L.ToString(2)
	handlerFunc := L.ToFunction(3)

	if method == "" || pattern == "" || handlerFunc == nil {
		L.ArgError(1, "chi_route requires method, pattern, and handler function")
		return 0
	}

	// Create Go HTTP handler that calls the Lua function
	handler := func(w http.ResponseWriter, r *http.Request) {
		// Create Lua request wrapper
		reqTable := createLuaRequest(L, r)

		// Create Lua response wrapper
		respWrapper := &luaResponseWriter{w: w, L: L}
		respTable := createLuaResponse(L, respWrapper)

		// Call the Lua handler function
		if err := L.CallByParam(lua.P{
			Fn:      handlerFunc,
			NRet:    0,
			Protect: true,
		}, respTable, reqTable); err != nil {
			http.Error(w, fmt.Sprintf("Lua handler error: %v", err), http.StatusInternalServerError)
		}
	}

	// Register with the route registry
	if b.registry != nil {
		err := b.registry.RegisterRoute(routing.RouteDefinition{
			TenantName: b.tenantName,
			Method:     method,
			Pattern:    pattern,
			Handler:    handler,
		})
		if err != nil {
			L.RaiseError("Failed to register route: %v", err)
		}
	}

	return 0
}

// luaChiMiddleware handles middleware registration: chi_middleware(pattern, middleware_func)
func (b *ChiBindings) luaChiMiddleware(L *lua.LState) int {
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
	if b.registry != nil {
		err := b.registry.RegisterMiddleware(routing.MiddlewareDefinition{
			TenantName: b.tenantName,
			Pattern:    pattern,
			Middleware: middleware,
		})
		if err != nil {
			L.RaiseError("Failed to register middleware: %v", err)
		}
	}

	return 0
}

// luaChiGroup handles route group registration: chi_group(pattern, setup_func)
func (b *ChiBindings) luaChiGroup(L *lua.LState) int {
	pattern := L.ToString(1)
	setupFunc := L.ToFunction(2)

	if pattern == "" || setupFunc == nil {
		L.ArgError(1, "chi_group requires pattern and setup function")
		return 0
	}

	// This is a simplified implementation
	// In a real implementation, we'd need to capture routes defined within the group
	groupDef := routing.RouteGroupDefinition{
		TenantName: b.tenantName,
		Pattern:    pattern,
		Middleware: []func(http.Handler) http.Handler{},
		Routes:     []routing.RouteDefinition{},
		Subgroups:  []routing.RouteGroupDefinition{},
	}

	// Register with the route registry
	if b.registry != nil {
		err := b.registry.RegisterRouteGroup(groupDef)
		if err != nil {
			L.RaiseError("Failed to register route group: %v", err)
		}
	}

	return 0
}

// luaChiMount handles mounting: chi_mount(path, tenant)
func (b *ChiBindings) luaChiMount(L *lua.LState) int {
	mountPath := L.ToString(1)

	if mountPath == "" {
		L.ArgError(1, "chi_mount requires mount path")
		return 0
	}

	// Mount the current tenant's routes
	if b.registry != nil {
		err := b.registry.MountTenantRoutes(b.tenantName, mountPath)
		if err != nil {
			L.RaiseError("Failed to mount routes: %v", err)
		}
	}

	return 0
}

// luaChiParam extracts URL parameters: chi_param(request, param_name)
func (b *ChiBindings) luaChiParam(L *lua.LState) int {
	reqTable := L.ToTable(1)
	paramName := L.ToString(2)

	if reqTable == nil || paramName == "" {
		L.ArgError(1, "chi_param requires request table and parameter name")
		return 0
	}

	// Extract the underlying http.Request (this is simplified)
	// In practice, we'd store a reference to the request in the table
	L.Push(lua.LString("")) // Placeholder - would extract actual param value
	return 1
}

// luaLog provides logging from Lua scripts: log(message)
func (b *ChiBindings) luaLog(L *lua.LState) int {
	message := L.ToString(1)
	fmt.Printf("[Lua:%s] %s\n", b.tenantName, message)
	return 0
}

// luaCreateHandler creates a handler function wrapper
func (b *ChiBindings) luaCreateHandler(L *lua.LState) int {
	// This would create a handler function that can be used with chi_route
	// Implementation details depend on how we want to structure the API
	return 0
}

// luaResponseWriter wraps http.ResponseWriter for Lua access
type luaResponseWriter struct {
	w http.ResponseWriter
	L *lua.LState
}

// createLuaRequest creates a Lua table representing an HTTP request
func createLuaRequest(L *lua.LState, r *http.Request) *lua.LTable {
	reqTable := L.NewTable()

	// Basic request fields
	reqTable.RawSetString("method", lua.LString(r.Method))
	reqTable.RawSetString("url", lua.LString(r.URL.String()))
	reqTable.RawSetString("path", lua.LString(r.URL.Path))
	reqTable.RawSetString("query", lua.LString(r.URL.RawQuery))
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

	// Body (for POST/PUT requests)
	if r.Body != nil {
		body, err := io.ReadAll(r.Body)
		if err == nil {
			reqTable.RawSetString("body", lua.LString(string(body)))
		}
	}

	// Helper methods
	reqTable.RawSetString("header", L.NewFunction(func(L *lua.LState) int {
		headerName := L.ToString(1)
		headerValue := r.Header.Get(headerName)
		L.Push(lua.LString(headerValue))
		return 1
	}))

	return reqTable
}

// createLuaResponse creates a Lua table representing an HTTP response
func createLuaResponse(L *lua.LState, w *luaResponseWriter) *lua.LTable {
	respTable := L.NewTable()

	// Response methods
	respTable.RawSetString("write", L.NewFunction(func(L *lua.LState) int {
		content := L.ToString(1)
		w.w.Write([]byte(content))
		return 0
	}))

	respTable.RawSetString("header", L.NewFunction(func(L *lua.LState) int {
		key := L.ToString(1)
		value := L.ToString(2)
		w.w.Header().Set(key, value)
		return 0
	}))

	respTable.RawSetString("status", L.NewFunction(func(L *lua.LState) int {
		statusCode := L.ToInt(1)
		w.w.WriteHeader(statusCode)
		return 0
	}))

	respTable.RawSetString("json", L.NewFunction(func(L *lua.LState) int {
		jsonContent := L.ToString(1)
		w.w.Header().Set("Content-Type", "application/json")
		w.w.Write([]byte(jsonContent))
		return 0
	}))

	return respTable
}
