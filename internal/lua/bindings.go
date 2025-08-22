package lua

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	lua "github.com/yuin/gopher-lua"
	"keystone-gateway/internal/metrics"
)

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
}

// NewLuaBindings creates a new LuaBindings instance with the required dependencies
func NewLuaBindings(router RouterInterface, statePool *LuaStatePool, metrics *metrics.LuaMetrics, logger *slog.Logger) *LuaBindings {
	if logger == nil {
		logger = slog.Default()
	}

	return &LuaBindings{
		router:    router,
		statePool: statePool,
		metrics:   metrics,
		logger:    logger,
	}
}

// WithContext returns a new LuaBindings instance configured for specific script execution context
func (lb *LuaBindings) WithContext(scriptTag, tenantName string) *LuaBindings {
	return &LuaBindings{
		router:     lb.router,
		statePool:  lb.statePool,
		metrics:    lb.metrics,
		logger:     lb.logger,
		scriptTag:  scriptTag,
		tenantName: tenantName,
	}
}

// SetupLuaBindings registers all Chi functions with the provided Lua state
func (lb *LuaBindings) SetupLuaBindings(L *lua.LState, scriptTag, tenantName string) error {
	// Create context-specific bindings
	contextualBindings := lb.WithContext(scriptTag, tenantName)

	// Register all Lua API functions
	functions := map[string]lua.LGFunction{
		"chi_route":        contextualBindings.createRouteFunction(),
		"chi_middleware":   contextualBindings.createMiddlewareFunction(),
		"chi_group":        contextualBindings.createGroupFunction(),
		"chi_param":        contextualBindings.createParamFunction(),
		"chi_get_routes":   contextualBindings.createGetRoutesFunction(),
		"chi_remove_route": contextualBindings.createRemoveRouteFunction(),
	}

	// Register functions safely
	for name, fn := range functions {
		L.SetGlobal(name, L.NewFunction(fn))
	}

	return nil
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
		// Get Lua state from the pool
		L := lb.statePool.Get()
		defer lb.statePool.Put(L)

		// Set the execution context with timeout
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		L.SetContext(ctx)
		defer L.RemoveContext()

		// Set up Chi bindings for this execution
		if err := lb.SetupLuaBindings(L, lb.scriptTag, lb.tenantName); err != nil {
			http.Error(w, fmt.Sprintf("Script setup error: %v", err), http.StatusInternalServerError)
			return
		}

		// Create Lua request/response tables
		reqTable := lb.createLuaRequest(L, r)
		respWriter := &luaResponseWriter{w: w}
		respTable := lb.createLuaResponse(L, respWriter)

		// Execute Lua handler function
		err := L.CallByParam(lua.P{
			Fn:      handlerFunc,
			NRet:    0,
			Protect: true,
		}, reqTable, respTable)

		if err != nil {
			http.Error(w, fmt.Sprintf("Handler execution error: %v", err), http.StatusInternalServerError)
			return
		}
	}
}

// createLuaMiddleware creates HTTP middleware that executes Lua function
func (lb *LuaBindings) createLuaMiddleware(middlewareFunc *lua.LFunction) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get Lua state from the pool
			L := lb.statePool.Get()
			defer lb.statePool.Put(L)

			// Set execution context
			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			defer cancel()

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

			// Create a next function
			nextCalled := false
			nextFunc := L.NewFunction(func(L *lua.LState) int {
				nextCalled = true
				return 0
			})

			// Execute Lua middleware function
			err := L.CallByParam(lua.P{
				Fn:      middlewareFunc,
				NRet:    0,
				Protect: true,
			}, reqTable, respTable, nextFunc)

			if err != nil {
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

	// Add response methods (write, header, etc.)
	writeFunc := L.NewFunction(func(L *lua.LState) int {
		data := L.ToString(1)
		_, _ = w.Write([]byte(data)) // Ignore write error in Lua context
		return 0
	})
	L.SetField(respTable, "write", writeFunc)

	statusFunc := L.NewFunction(func(L *lua.LState) int {
		status := int(L.ToNumber(1))
		w.WriteHeader(status)
		return 0
	})
	L.SetField(respTable, "status", statusFunc)

	headerFunc := L.NewFunction(func(L *lua.LState) int {
		key := L.ToString(1)
		value := L.ToString(2)
		w.Header().Set(key, value)
		return 0
	})
	L.SetField(respTable, "header", headerFunc)

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