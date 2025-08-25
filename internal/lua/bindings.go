package lua

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	lua "github.com/yuin/gopher-lua"
	"keystone-gateway/internal/metrics"
)

// TODO look at ideas.md for new binding ideas
// RouterInterface defines the operations that LuaBindings needs from a router
type RouterInterface interface {
	RegisterRoute(ctx context.Context, method, pattern string, handler http.HandlerFunc, tenantName, scriptTag string) error
	RegisterMiddleware(ctx context.Context, pattern string, middleware func(http.Handler) http.Handler, tenantName, scriptTag string) error
	CreateGroup(ctx context.Context, pattern string, setupFunc func(chi.Router), tenantName, scriptTag string) error
	GetRoutes() map[string]*RouteInfo
	RemoveRoute(ctx context.Context, method, pattern string) error
}

// LuaBindings manages the Lua function bindings for Chi router operations
type LuaBindings struct {
	// Core dependencies
	router    RouterInterface
	statePool *LuaStatePool
	metrics   *metrics.LuaMetrics
	logger    *slog.Logger

	// Current execution context
	scriptTag  string
	tenantName string

	// bindingsInitialized tracks which states have bindings set up to prevent recreating functions per request
	bindingsInitialized *sync.Map // map[*lua.LState]bool
}

// NewLuaBindings creates a new LuaBindings instance with the required dependencies
func NewLuaBindings(router RouterInterface, statePool *LuaStatePool, metrics *metrics.LuaMetrics, logger *slog.Logger) *LuaBindings {
	if logger == nil {
		logger = slog.Default()
	}

	return &LuaBindings{
		router:              router,
		statePool:           statePool,
		metrics:             metrics,
		logger:              logger,
		bindingsInitialized: &sync.Map{}, // Initialize the cache
	}
}

// WithContext returns a new LuaBindings instance configured for specific script execution context
func (lb *LuaBindings) WithContext(scriptTag, tenantName string) *LuaBindings {
	return &LuaBindings{
		router:              lb.router,
		statePool:           lb.statePool,
		metrics:             lb.metrics,
		logger:              lb.logger,
		scriptTag:           scriptTag,
		tenantName:          tenantName,
		bindingsInitialized: lb.bindingsInitialized, // Share the same cache across contexts
	}
}

// SetupLuaBindings registers all Chi functions with the provided Lua state (cached per state)
func (lb *LuaBindings) SetupLuaBindings(L *lua.LState, scriptTag, tenantName string) error {
	// Skip initialization if state is nil or cache is not available
	if L == nil || lb.bindingsInitialized == nil {
		return fmt.Errorf("invalid lua state or uninitialized bindings cache")
	}

	// Check if this state has already been initialized with bindings
	if _, alreadyInitialized := lb.bindingsInitialized.LoadOrStore(L, true); alreadyInitialized {
		// Bindings already exist for this state, skip expensive function creation
		return nil
	}

	// First time initialization for this state - create all bindings
	contextualBindings := lb.WithContext(scriptTag, tenantName)

	// Register all Lua API functions
	functions := map[string]lua.LGFunction{
		"chi_route":        contextualBindings.createRouteFunction(),
		"chi_middleware":   contextualBindings.createMiddlewareFunction(),
		"chi_group":        contextualBindings.createGroupFunction(),
		"chi_param":        contextualBindings.createParamFunction(),
		"chi_get_routes":   contextualBindings.createGetRoutesFunction(),
		"chi_remove_route": contextualBindings.createRemoveRouteFunction(),

		"http_post": createHTTPPostFunction(),
		"get_env":   createGetEnvFunction(),
	}

	// Register functions safely - this is expensive and only happens once per state
	for name, fn := range functions {
		L.SetGlobal(name, L.NewFunction(fn))
	}

	// Create pre-cached response functions to avoid per-request function creation
	lb.setupResponseFunctions(L)

	// Create pre-cached middleware next function
	nextFunc := L.NewFunction(func(L *lua.LState) int {
		// Get the request table and extract the next callback
		if reqTable := L.ToTable(1); reqTable != nil {
			if nextCallbackUserData := reqTable.RawGetString("__next_callback"); nextCallbackUserData != lua.LNil {
				if userData, ok := nextCallbackUserData.(*lua.LUserData); ok {
					if callback, ok := userData.Value.(func()); ok {
						callback()
					}
				}
			}
		}
		return 0
	})
	L.SetGlobal("__middleware_next", nextFunc)

	lb.logger.Debug("initialized lua bindings for state", 
		"script_tag", scriptTag, 
		"tenant", tenantName,
		"functions_count", len(functions)+4) // +3 response functions +1 next function

	return nil
}

// ClearCachedBindings removes a Lua state from the bindings cache
// This should be called when a state is being closed/destroyed
func (lb *LuaBindings) ClearCachedBindings(L *lua.LState) {
	if lb.bindingsInitialized != nil && L != nil {
		lb.bindingsInitialized.Delete(L)
	}
}

// setupResponseFunctions creates pre-cached response functions that access writer via userdata
func (lb *LuaBindings) setupResponseFunctions(L *lua.LState) {
	// Create write function that extracts writer from response table
	writeFunc := L.NewFunction(func(L *lua.LState) int {
		// Get response table (passed as first argument) 
		respTable := L.ToTable(1)
		data := L.ToString(2)
		
		if respTable != nil {
			if writerUserData := respTable.RawGetString("__writer"); writerUserData != lua.LNil {
				if userData, ok := writerUserData.(*lua.LUserData); ok {
					if writer, ok := userData.Value.(*luaResponseWriter); ok {
						_, _ = writer.Write([]byte(data)) // Ignore write error in Lua context
					}
				}
			}
		}
		return 0
	})
	L.SetGlobal("__response_write", writeFunc)

	// Create status function
	statusFunc := L.NewFunction(func(L *lua.LState) int {
		respTable := L.ToTable(1)
		status := int(L.ToNumber(2))
		
		if respTable != nil {
			if writerUserData := respTable.RawGetString("__writer"); writerUserData != lua.LNil {
				if userData, ok := writerUserData.(*lua.LUserData); ok {
					if writer, ok := userData.Value.(*luaResponseWriter); ok {
						writer.WriteHeader(status)
					}
				}
			}
		}
		return 0
	})
	L.SetGlobal("__response_status", statusFunc)

	// Create header function
	headerFunc := L.NewFunction(func(L *lua.LState) int {
		respTable := L.ToTable(1)
		key := L.ToString(2)
		value := L.ToString(3)
		
		if respTable != nil {
			if writerUserData := respTable.RawGetString("__writer"); writerUserData != lua.LNil {
				if userData, ok := writerUserData.(*lua.LUserData); ok {
					if writer, ok := userData.Value.(*luaResponseWriter); ok {
						writer.Header().Set(key, value)
					}
				}
			}
		}
		return 0
	})
	L.SetGlobal("__response_header", headerFunc)
}

// Util Bindings

// createHTTPPostFunction creates a simple HTTP POST helper for OAuth
func createHTTPPostFunction() lua.LGFunction {
	return func(L *lua.LState) int {
		url := L.CheckString(1)
		body := L.CheckString(2)

		// Optional 3rd param: headers table
		var contentType string = "application/x-www-form-urlencoded"
		if L.GetTop() >= 3 {
			if tbl, ok := L.Get(3).(*lua.LTable); ok {
				// Look for "Content-Type" header
				tbl.ForEach(func(k, v lua.LValue) {
					if strings.ToLower(k.String()) == "content-type" {
						contentType = v.String()
					}
				})
			}
		}

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Post(url, contentType, strings.NewReader(body))
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		defer resp.Body.Close()

		responseBody, _ := io.ReadAll(resp.Body)
		L.Push(lua.LString(string(responseBody)))
		L.Push(lua.LNumber(resp.StatusCode))
		return 2
	}
}

// TODO does this even work in embedded luaVM?
// createGetEnvFunction creates a helper to read environment variables
func createGetEnvFunction() lua.LGFunction {
	return func(L *lua.LState) int {
		key := L.CheckString(1)
		value := os.Getenv(key)
		L.Push(lua.LString(value))
		return 1
	}
}

// ========================================
// Lua Function Implementations
// ========================================

// createRouteFunction returns a Lua function for route registration
func (lb *LuaBindings) createRouteFunction() lua.LGFunction {
	return func(L *lua.LState) int {
		// Extract arguments
		method := L.CheckString(1)
		pattern := L.CheckString(2)
		handlerFunc := L.CheckFunction(3)

		// Validate inputs
		if method == "" || pattern == "" || handlerFunc == nil {
			lb.pushErrorResult(L, "chi_route requires method, pattern, and handler function")
			return 2
		}

		// Create HTTP handler from Lua function
		httpHandler := lb.createLuaRouteHandler(handlerFunc)

		// Register route with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := lb.router.RegisterRoute(ctx, method, pattern, httpHandler, lb.tenantName, lb.scriptTag)
		if err != nil {
			lb.pushErrorResult(L, fmt.Sprintf("failed to register route: %v", err))
			return 2
		}

		lb.pushSuccessResult(L)
		return 2
	}
}

// createMiddlewareFunction returns a Lua function for middleware registration
func (lb *LuaBindings) createMiddlewareFunction() lua.LGFunction {
	return func(L *lua.LState) int {
		// Extract arguments
		pattern := L.CheckString(1)
		middlewareFunc := L.CheckFunction(2)

		// Validate inputs
		if pattern == "" || middlewareFunc == nil {
			lb.pushErrorResult(L, "chi_middleware requires pattern and middleware function")
			return 2
		}

		// Create HTTP middleware from a Lua function
		httpMiddleware := lb.createLuaMiddleware(middlewareFunc)

		// Register middleware with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := lb.router.RegisterMiddleware(ctx, pattern, httpMiddleware, lb.tenantName, lb.scriptTag)
		if err != nil {
			lb.pushErrorResult(L, fmt.Sprintf("failed to register middleware: %v", err))
			return 2
		}

		lb.pushSuccessResult(L)
		return 2
	}
}

// createGetRoutesFunction returns a Lua function for listing registered routes
func (lb *LuaBindings) createGetRoutesFunction() lua.LGFunction {
	return func(L *lua.LState) int {
		routes := lb.router.GetRoutes()

		// Create Lua table of routes
		routesTable := L.NewTable()
		i := 1
		for _, route := range routes {
			routeTable := L.NewTable()
			L.SetField(routeTable, "method", lua.LString(route.Method))
			L.SetField(routeTable, "pattern", lua.LString(route.Pattern))
			L.SetField(routeTable, "tenant", lua.LString(route.TenantName))
			L.SetField(routeTable, "registered", lua.LNumber(route.Registered.Unix()))

			routesTable.RawSetInt(i, routeTable)
			i++
		}

		L.Push(routesTable)
		return 1
	}
}

// createGroupFunction returns a Lua function for route group creation
func (lb *LuaBindings) createGroupFunction() lua.LGFunction {
	return func(L *lua.LState) int {
		// Extract arguments
		pattern := L.CheckString(1)
		setupFunc := L.CheckFunction(2)

		// Validate inputs
		if pattern == "" || setupFunc == nil {
			lb.pushErrorResult(L, "chi_group requires pattern and setup function")
			return 2
		}

		// Create Chi setup function that manages group context
		chiSetupFunc := func(r chi.Router) {
			// Store old group context
			oldGroupContext := L.GetGlobal("__current_group_pattern")
			fullPattern := lb.buildGroupPattern(pattern, oldGroupContext)

			// Set a new group context
			L.SetGlobal("__current_group_pattern", lua.LString(fullPattern))
			defer L.SetGlobal("__current_group_pattern", oldGroupContext)

			// Execute Lua setup function
			err := L.CallByParam(lua.P{
				Fn:      setupFunc,
				NRet:    0,
				Protect: true,
			}, lb.createChiRouterLuaValue(L, r))

			if err != nil {
				// Log error but don't panic
				lb.logger.Error("group setup function failed",
					"pattern", pattern,
					"script_tag", lb.scriptTag,
					"tenant", lb.tenantName,
					"error", err.Error())
			}
		}

		// Create a group with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := lb.router.CreateGroup(ctx, pattern, chiSetupFunc, lb.tenantName, lb.scriptTag)
		if err != nil {
			lb.pushErrorResult(L, fmt.Sprintf("failed to create group: %v", err))
			return 2
		}

		lb.pushSuccessResult(L)
		return 2
	}
}

// createParamFunction returns a Lua function for extracting URL parameters
func (lb *LuaBindings) createParamFunction() lua.LGFunction {
	return func(L *lua.LState) int {
		// Extract arguments
		requestTable := L.ToTable(1)
		paramName := L.ToString(2)

		if requestTable == nil || paramName == "" {
			L.Push(lua.LString(""))
			return 1
		}

		// Try to get from Chi context (set by HTTP handler)
		if paramsTable := requestTable.RawGetString("params"); paramsTable != lua.LNil {
			if paramTable, ok := paramsTable.(*lua.LTable); ok {
				if param := paramTable.RawGetString(paramName); param != lua.LNil {
					L.Push(param)
					return 1
				}
			}
		}

		// Default fallback
		L.Push(lua.LString(""))
		return 1
	}
}

// createRemoveRouteFunction returns a Lua function for removing routes
func (lb *LuaBindings) createRemoveRouteFunction() lua.LGFunction {
	return func(L *lua.LState) int {
		// Extract arguments
		method := L.CheckString(1)
		pattern := L.CheckString(2)

		if method == "" || pattern == "" {
			lb.pushErrorResult(L, "chi_remove_route requires method and pattern")
			return 2
		}

		// Remove route with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := lb.router.RemoveRoute(ctx, method, pattern)
		if err != nil {
			lb.pushErrorResult(L, fmt.Sprintf("failed to remove route: %v", err))
			return 2
		}

		lb.pushSuccessResult(L)
		return 2
	}
}

// createLuaRouteHandler creates an HTTP handler that executes Lua code
func (lb *LuaBindings) createLuaRouteHandler(handlerFunc *lua.LFunction) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Acquire Lua state from the pool with a short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		L, err := lb.statePool.Get(ctx)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to get Lua state: %v", err), http.StatusServiceUnavailable)
			return
		}
		defer lb.statePool.Put(L)

		// Set execution context with per-request timeout

		L.SetContext(ctx)
		defer L.RemoveContext()

		// Setup Lua bindings for this request
		if err := lb.SetupLuaBindings(L, lb.scriptTag, lb.tenantName); err != nil {
			http.Error(w, fmt.Sprintf("Script setup error: %v", err), http.StatusInternalServerError)
			return
		}

		// Prepare request/response tables for Lua
		reqTable := lb.createLuaRequest(L, r)
		respWriter := &luaResponseWriter{w: w}
		respTable := lb.createLuaResponse(L, respWriter)

		// Execute Lua handler function
		if err := L.CallByParam(lua.P{
			Fn:      handlerFunc,
			NRet:    0,
			Protect: true,
		}, reqTable, respTable); err != nil {
			http.Error(w, fmt.Sprintf("Handler execution error: %v", err), http.StatusInternalServerError)
			return
		}
	}
}

// createLuaMiddleware creates HTTP middleware that executes Lua function
func (lb *LuaBindings) createLuaMiddleware(middlewareFunc *lua.LFunction) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get Lua state from the pool with timeout
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()

			L, err := lb.statePool.Get(ctx)
			if err != nil {
				http.Error(w, fmt.Sprintf("failed to get Lua state: %v", err), http.StatusServiceUnavailable)
				return
			}
			defer lb.statePool.Put(L)

			// Set execution context with per-request timeout

			L.SetContext(ctx)
			defer L.RemoveContext()

			// Setup Chi bindings
			if err := lb.SetupLuaBindings(L, lb.scriptTag, lb.tenantName); err != nil {
				http.Error(w, fmt.Sprintf("Middleware setup error: %v", err), http.StatusInternalServerError)
				return
			}

			// Create Lua request/response tables
			reqTable := lb.createLuaRequest(L, r)
			respWriter := &luaResponseWriter{w: w}
			respTable := lb.createLuaResponse(L, respWriter)

			// Use pre-created next function with callback mechanism
			nextCalled := false
			nextCallbackUserData := L.NewUserData()
			nextCallbackUserData.Value = func() { nextCalled = true }
			reqTable.RawSetString("__next_callback", nextCallbackUserData)
			
			// Get pre-created next function
			nextFunc := L.GetGlobal("__middleware_next")
			if nextFunc == lua.LNil {
				// Fallback if global not found
				nextFunc = L.NewFunction(func(L *lua.LState) int {
					nextCalled = true
					return 0
				})
			}

			// Execute Lua middleware function
			if err := L.CallByParam(lua.P{
				Fn:      middlewareFunc,
				NRet:    0,
				Protect: true,
			}, reqTable, respTable, nextFunc); err != nil {
				http.Error(w, fmt.Sprintf("Middleware execution error: %v", err), http.StatusInternalServerError)
				return
			}

			// Call next handler if middleware called next()
			if nextCalled && next != nil {
				next.ServeHTTP(w, r)
			}
		})
	}
}

// ========================================
// Support Methods (migrated from chi_router.go)
// ========================================

// pushSuccessResult pushes success result to Lua stack
func (lb *LuaBindings) pushSuccessResult(L *lua.LState) {
	L.Push(lua.LTrue) // success
	L.Push(lua.LNil)  // no error
}

// pushErrorResult pushes error result to Lua stack
func (lb *LuaBindings) pushErrorResult(L *lua.LState, errMsg string) {
	L.Push(lua.LFalse)          // not successful
	L.Push(lua.LString(errMsg)) // error message
}

// createLuaRequest creates a Lua table representing the HTTP request
func (lb *LuaBindings) createLuaRequest(L *lua.LState, r *http.Request) *lua.LTable {
	reqTable := L.NewTable()

	// Basic request info
	L.SetField(reqTable, "method", lua.LString(r.Method))
	L.SetField(reqTable, "path", lua.LString(r.URL.Path))
	L.SetField(reqTable, "host", lua.LString(r.Host))
	L.SetField(reqTable, "url", lua.LString(r.URL.String()))

	// Extract Chi parameters
	paramsTable := L.NewTable()
	if rctx := chi.RouteContext(r.Context()); rctx != nil {
		for i, key := range rctx.URLParams.Keys {
			if i < len(rctx.URLParams.Values) {
				L.SetField(paramsTable, key, lua.LString(rctx.URLParams.Values[i]))
			}
		}
	}
	L.SetField(reqTable, "params", paramsTable)

	// Add headers
	headersTable := L.NewTable()
	for key, values := range r.Header {
		if len(values) > 0 {
			L.SetField(headersTable, key, lua.LString(values[0]))
		}
	}
	L.SetField(reqTable, "headers", headersTable)

	return reqTable
}

// createLuaResponse creates a Lua table for HTTP response
func (lb *LuaBindings) createLuaResponse(L *lua.LState, w *luaResponseWriter) *lua.LTable {
	respTable := L.NewTable()

	// Use userdata to store the response writer and access it in pre-created functions
	writerUserData := L.NewUserData()
	writerUserData.Value = w
	L.SetField(respTable, "__writer", writerUserData)

	// Get pre-created response functions from globals (set during state initialization)
	if writeFunc := L.GetGlobal("__response_write"); writeFunc != lua.LNil {
		L.SetField(respTable, "write", writeFunc)
	}
	if statusFunc := L.GetGlobal("__response_status"); statusFunc != lua.LNil {
		L.SetField(respTable, "status", statusFunc)
	}
	if headerFunc := L.GetGlobal("__response_header"); headerFunc != lua.LNil {
		L.SetField(respTable, "header", headerFunc)
	}

	return respTable
}

// buildGroupPattern combines a parent group pattern with the current pattern
func (lb *LuaBindings) buildGroupPattern(pattern string, oldGroupContext lua.LValue) string {
	if oldGroupContext != lua.LNil {
		parentPattern := oldGroupContext.String()
		if parentPattern != "" {
			return parentPattern + pattern // Simple concatenation
		}
	}
	return pattern
}

// createChiRouterLuaValue creates a Lua representation of chi.Router
func (lb *LuaBindings) createChiRouterLuaValue(L *lua.LState, r chi.Router) lua.LValue {
	// Create placeholder table for router methods
	routerTable := L.NewTable()

	// Log router creation for debugging
	lb.logger.Debug("created lua router value for group setup")

	// Add router methods here as needed for the group set up
	// The 'r' parameter represents the chi.Router for this group
	// In future implementations, we could expose router methods to Lua
	_ = r // Acknowledge parameter usage

	return routerTable
}

// ========================================
// Public Methods for ChiRouter Integration
// ========================================

// CreateLuaRouteHandler creates an HTTP handler that executes Lua code (public method for ChiRouter)
func (lb *LuaBindings) CreateLuaRouteHandler(handlerFunc *lua.LFunction, scriptTag, tenantName string) http.HandlerFunc {
	// Create context-specific bindings for this handler
	contextualBindings := lb.WithContext(scriptTag, tenantName)
	return contextualBindings.createLuaRouteHandler(handlerFunc)
}

// CreateLuaMiddleware creates HTTP middleware that executes Lua function (public method for ChiRouter)
func (lb *LuaBindings) CreateLuaMiddleware(middlewareFunc *lua.LFunction, scriptTag, tenantName string) func(http.Handler) http.Handler {
	// Create context-specific bindings for this middleware
	contextualBindings := lb.WithContext(scriptTag, tenantName)
	return contextualBindings.createLuaMiddleware(middlewareFunc)
}

// ========================================
// luaResponseWriter - HTTP Response Integration
// ========================================

// luaResponseWriter wraps http.ResponseWriter for Lua response operations
type luaResponseWriter struct {
	w       http.ResponseWriter
	written bool
	status  int
}

// Write implements io.Writer interface
func (lw *luaResponseWriter) Write(data []byte) (int, error) {
	if !lw.written {
		lw.written = true
		if lw.status == 0 {
			lw.status = http.StatusOK
		}
		lw.w.WriteHeader(lw.status)
	}
	return lw.w.Write(data)
}

// WriteHeader stores the status code
func (lw *luaResponseWriter) WriteHeader(status int) {
	if !lw.written {
		lw.status = status
	}
}

// Header returns the ResponseWriter's header map
func (lw *luaResponseWriter) Header() http.Header {
	return lw.w.Header()
}
