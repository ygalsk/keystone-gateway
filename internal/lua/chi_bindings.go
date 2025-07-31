// Package lua provides Lua-to-Chi bridge functions that allow Lua scripts
// to register routes, middleware, and route groups directly with the Chi router.
package lua

import (
	"fmt"
	"net/http"
	"net/url"
	"sync"

	lua "github.com/yuin/gopher-lua"

	"keystone-gateway/internal/routing"
)

// MiddlewareAction represents a cached middleware action that can be executed in Go
type MiddlewareAction struct {
	Type  string                 // "set_header", "check_auth", "log", etc.
	Key   string                 // Header name, auth key, etc.
	Value string                 // Header value, expected value, etc.
	Data  map[string]interface{} // Additional data for complex actions
}

// MiddlewareLogic represents the cached logic for a middleware function
type MiddlewareLogic struct {
	Pattern    string             `json:"pattern"`
	TenantName string             `json:"tenant_name"`
	Actions    []MiddlewareAction `json:"actions"`
	CallNext   bool               `json:"call_next"`
}

// MiddlewareCache provides thread-safe caching of middleware logic
type MiddlewareCache struct {
	mu    sync.RWMutex
	cache map[string]*MiddlewareLogic // key: tenant_pattern
}

// Mock types for parsing middleware logic
type mockResponseWriter struct {
	headers http.Header
}

func (m *mockResponseWriter) Header() http.Header {
	if m.headers == nil {
		m.headers = make(http.Header)
	}
	return m.headers
}

func (m *mockResponseWriter) Write([]byte) (int, error) {
	return 0, nil
}

func (m *mockResponseWriter) WriteHeader(statusCode int) {
	// Capture status code changes if needed
}

func (m *mockResponseWriter) getActions() []MiddlewareAction {
	var actions []MiddlewareAction
	// Convert header operations to actions
	for key, values := range m.headers {
		for _, value := range values {
			actions = append(actions, MiddlewareAction{
				Type:  "set_header",
				Key:   key,
				Value: value,
			})
		}
	}
	return actions
}

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

	// Check if we're inside a group context
	var groupPattern string
	if groupCtx := L.GetGlobal("__current_group_pattern"); groupCtx != lua.LNil {
		if groupStr := groupCtx.String(); groupStr != "" {
			groupPattern = groupStr
			// Prepend group pattern to route pattern
			pattern = groupStr + pattern
		}
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
		TenantName:   tenantName,
		Method:       method,
		Pattern:      pattern,
		GroupPattern: groupPattern,
		Handler:      luaHandler.ServeHTTP,
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

	// Get the current group context if we're inside a group
	var groupPattern string
	if groupCtx := L.GetGlobal("__current_group_pattern"); groupCtx != lua.LNil {
		groupPattern = groupCtx.String()
	}

	// Check if we already have cached logic for this middleware
	if cachedLogic, exists := e.getCachedMiddleware(tenantName, pattern, groupPattern); exists {
		// Use cached logic for performance
		middleware := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				e.executeMiddlewareLogic(cachedLogic, w, r, next)
			})
		}

		// Register with the route registry, including group context
		err := e.routeRegistry.RegisterMiddleware(routing.MiddlewareDefinition{
			TenantName:   tenantName,
			Pattern:      pattern,
			GroupPattern: groupPattern,
			Middleware:   middleware,
		})
		if err != nil {
			L.RaiseError("Failed to register cached middleware: %v", err)
		}
		return 0
	}

	// Parse and cache the middleware logic for first-time registration
	logic, err := e.parseLuaMiddlewareLogic(L, middlewareFunc, pattern)
	if err != nil {
		L.RaiseError("Failed to parse middleware logic: %v", err)
		return 0
	}

	// Cache the parsed logic
	logic.TenantName = tenantName
	e.setCachedMiddleware(tenantName, pattern, groupPattern, logic)

	// Create Go middleware that executes the cached logic directly
	middleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			e.executeMiddlewareLogic(logic, w, r, next)
		})
	}

	// Register with the route registry, including group context
	err = e.routeRegistry.RegisterMiddleware(routing.MiddlewareDefinition{
		TenantName:   tenantName,
		Pattern:      pattern,
		GroupPattern: groupPattern,
		Middleware:   middleware,
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

	// Execute the setup function to collect group routes and middleware
	// Save current group context
	oldGroupContext := L.GetGlobal("__current_group_pattern")

	// Build nested group pattern by combining with parent group
	var fullPattern string
	if oldGroupContext != lua.LNil {
		parentPattern := oldGroupContext.String()
		if parentPattern != "" {
			fullPattern = parentPattern + pattern
		} else {
			fullPattern = pattern
		}
	} else {
		fullPattern = pattern
	}

	L.SetGlobal("__current_group_pattern", lua.LString(fullPattern))

	// Execute the setup function
	err := L.CallByParam(lua.P{
		Fn:      setupFunc,
		NRet:    0,
		Protect: true,
	})

	// Restore previous group context
	L.SetGlobal("__current_group_pattern", oldGroupContext)

	if err != nil {
		L.RaiseError("Failed to execute group setup function: %v", err)
	}

	return 0
}

// Cache methods for middleware logic

// getCachedMiddleware retrieves cached middleware logic thread-safely
func (e *Engine) getCachedMiddleware(tenantName, pattern, groupPattern string) (*MiddlewareLogic, bool) {
	key := fmt.Sprintf("%s_%s_%s", tenantName, groupPattern, pattern)
	e.middlewareCache.mu.RLock()
	defer e.middlewareCache.mu.RUnlock()
	logic, exists := e.middlewareCache.cache[key]
	return logic, exists
}

// setCachedMiddleware stores middleware logic thread-safely
func (e *Engine) setCachedMiddleware(tenantName, pattern, groupPattern string, logic *MiddlewareLogic) {
	key := fmt.Sprintf("%s_%s_%s", tenantName, groupPattern, pattern)
	e.middlewareCache.mu.Lock()
	defer e.middlewareCache.mu.Unlock()
	e.middlewareCache.cache[key] = logic
}

// parseLuaMiddlewareLogic extracts actions from a Lua middleware function
func (e *Engine) parseLuaMiddlewareLogic(L *lua.LState, middlewareFunc *lua.LFunction, pattern string) (*MiddlewareLogic, error) {
	// Execute the function in a controlled environment to extract its logic
	// Create a mock HTTP request and response writer to capture actions

	// Create a proper mock request with URL
	mockURL := &url.URL{Path: "/"}
	mockReq := &http.Request{
		Method: "GET",
		URL:    mockURL,
		Header: make(http.Header),
	}

	mockWriter := &mockResponseWriter{}
	respWriter := &luaResponseWriter{w: mockWriter}
	respTable := createLuaResponse(L, respWriter)
	reqTable := createLuaRequest(L, mockReq)

	nextCalled := false
	nextFunc := L.NewFunction(func(L *lua.LState) int {
		nextCalled = true
		return 0
	})

	// Execute the middleware function to capture its actions
	err := L.CallByParam(lua.P{
		Fn:      middlewareFunc,
		NRet:    0,
		Protect: true,
	}, respTable, reqTable, nextFunc)

	if err != nil {
		return nil, fmt.Errorf("failed to parse middleware logic: %v", err)
	}

	// Extract actions from the mock response writer
	actions := mockWriter.getActions()

	return &MiddlewareLogic{
		Pattern:  pattern,
		Actions:  actions,
		CallNext: nextCalled,
	}, nil
}

// executeMiddlewareLogic executes cached middleware logic directly in Go
func (e *Engine) executeMiddlewareLogic(logic *MiddlewareLogic, w http.ResponseWriter, r *http.Request, next http.Handler) {
	// Execute each cached action
	for _, action := range logic.Actions {
		switch action.Type {
		case "set_header":
			w.Header().Set(action.Key, action.Value)
		case "add_header":
			w.Header().Add(action.Key, action.Value)
		case "delete_header":
			w.Header().Del(action.Key)
			// Add more action types as needed
		}
	}

	// Call next handler if the original middleware called next
	if logic.CallNext && next != nil {
		next.ServeHTTP(w, r)
	}
}
