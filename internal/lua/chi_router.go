package lua

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-chi/chi/v5"
	lua "github.com/yuin/gopher-lua"
)

// ChiRouter provides Lua integration with Chi router
type ChiRouter struct {
	// Core dependencies
	router    chi.Router
	statePool *LuaStatePool
	metrics   *LuaMetrics // Single metrics system
	logger    *slog.Logger

	// State management
	mu          sync.RWMutex
	routes      map[string]*RouteInfo      // method:pattern -> info
	middlewares map[string]*MiddlewareInfo // pattern -> info
	groups      map[string]*GroupInfo      // pattern -> info
	scripts     map[string]string          // scriptTag -> content

	// Lifecycle management
	initialized atomic.Bool
	shutdown    atomic.Bool
}

// RouteInfo Simplified data structures
type RouteInfo struct {
	Method     string
	Pattern    string
	Handler    http.HandlerFunc
	TenantName string
	ScriptTag  string
	Registered time.Time
}

type MiddlewareInfo struct {
	Pattern    string
	Middleware func(http.Handler) http.Handler
	TenantName string
	ScriptTag  string
	Registered time.Time
}

type GroupInfo struct {
	Pattern    string
	TenantName string
	SetupFunc  func(chi.Router)
	Router     chi.Router
	Registered time.Time
}

func NewChiRouter(router chi.Router, statePool *LuaStatePool, metrics *LuaMetrics, logger *slog.Logger) *ChiRouter {
	if router == nil {
		router = chi.NewRouter()
	}

	if logger == nil {
		logger = slog.Default()
	}

	cr := &ChiRouter{
		router:      router,
		statePool:   statePool,
		metrics:     metrics,
		logger:      logger,
		routes:      make(map[string]*RouteInfo),
		middlewares: make(map[string]*MiddlewareInfo),
		groups:      make(map[string]*GroupInfo),
		scripts:     make(map[string]string),
	}

	cr.initialized.Store(true)
	return cr
}

// SetupLuaBindings registers all Chi functions with Lua state
func (cr *ChiRouter) SetupLuaBindings(L *lua.LState, scriptTag, tenantName string) error {
	if !cr.initialized.Load() || cr.shutdown.Load() {
		return fmt.Errorf("chi router not initialized or shut down")
	}

	// Register all 6 Lua API functions
	functions := map[string]lua.LGFunction{
		"chi_route":        cr.createRouteFunction(scriptTag, tenantName),
		"chi_middleware":   cr.createMiddlewareFunction(scriptTag, tenantName),
		"chi_group":        cr.createGroupFunction(scriptTag, tenantName),
		"chi_param":        cr.createParamFunction(),
		"chi_get_routes":   cr.createGetRoutesFunction(),
		"chi_remove_route": cr.createRemoveRouteFunction(),
	}

	// Register functions safely
	for name, fn := range functions {
		L.SetGlobal(name, L.NewFunction(fn))
	}

	return nil
}

// RegisterRoute adds an HTTP route to Chi router
func (cr *ChiRouter) RegisterRoute(ctx context.Context, method, pattern string, handler http.HandlerFunc, tenantName, scriptTag string) error {
	// Track operation start
	start := time.Now()
	cr.metrics.RecordRouteAdd()

	// Validate inputs
	if err := cr.validateInput(method, pattern); err != nil {
		cr.trackOperation("route_add", start, err)
		return err
	}

	if handler == nil {
		err := fmt.Errorf("handler cannot be nil")
		cr.trackOperation("route_add", start, err)
		return err
	}

	// Check context
	if err := cr.checkContext(ctx); err != nil {
		cr.trackOperation("route_add", start, err)
		return err
	}

	cr.mu.Lock()
	defer cr.mu.Unlock()

	routeKey := cr.generateRouteKey(method, pattern)

	// Check if a route already exists
	if _, exists := cr.routes[routeKey]; exists {
		err := fmt.Errorf("route %s %s already exists", method, pattern)
		cr.trackOperation("route_add", start, err)
		return err
	}

	// Register with Chi router using method-specific handlers
	switch method {
	case http.MethodGet:
		cr.router.Get(pattern, handler)
	case http.MethodPost:
		cr.router.Post(pattern, handler)
	case http.MethodPut:
		cr.router.Put(pattern, handler)
	case http.MethodPatch:
		cr.router.Patch(pattern, handler)
	case http.MethodDelete:
		cr.router.Delete(pattern, handler)
	case http.MethodHead:
		cr.router.Head(pattern, handler)
	case http.MethodOptions:
		cr.router.Options(pattern, handler)
	default:
		cr.router.Method(method, pattern, handler)
	}

	// Store route info for tracking
	cr.routes[routeKey] = &RouteInfo{
		Method:     method,
		Pattern:    pattern,
		Handler:    handler,
		TenantName: tenantName,
		ScriptTag:  scriptTag,
		Registered: time.Now(),
	}

	cr.logger.Info("route registered successfully",
		"method", method,
		"pattern", pattern,
		"tenant", tenantName,
		"script_tag", scriptTag)

	cr.trackOperation("route_add", start, nil)
	return nil
}

// RegisterMiddleware adds middleware to Chi router
func (cr *ChiRouter) RegisterMiddleware(ctx context.Context, pattern string, middleware func(http.Handler) http.Handler, tenantName, scriptTag string) error {
	start := time.Now()
	cr.metrics.RecordMiddlewareAdd()

	// Validate inputs
	if pattern == "" {
		err := fmt.Errorf("middleware pattern cannot be empty")
		cr.trackOperation("middleware_add", start, err)
		return err
	}

	if middleware == nil {
		err := fmt.Errorf("middleware function cannot be nil")
		cr.trackOperation("middleware_add", start, err)
		return err
	}

	// Check context
	if err := cr.checkContext(ctx); err != nil {
		cr.trackOperation("middleware_add", start, err)
		return err
	}

	cr.mu.Lock()
	defer cr.mu.Unlock()

	// Check if middleware already exists
	if _, exists := cr.middlewares[pattern]; exists {
		err := fmt.Errorf("middleware for pattern %s already exists", pattern)
		cr.trackOperation("middleware_add", start, err)
		return err
	}

	// Register with Chi router
	cr.router.Use(middleware)

	// Store middleware info for tracking
	cr.middlewares[pattern] = &MiddlewareInfo{
		Pattern:    pattern,
		Middleware: middleware,
		TenantName: tenantName,
		ScriptTag:  scriptTag,
		Registered: time.Now(),
	}

	cr.logger.Info("middleware registered successfully",
		"pattern", pattern,
		"tenant", tenantName,
		"script_tag", scriptTag)

	cr.trackOperation("middleware_add", start, nil)
	return nil
}

// CreateGroup adds a route group to Chi router
func (cr *ChiRouter) CreateGroup(ctx context.Context, pattern string, setupFunc func(chi.Router), tenantName, scriptTag string) error {
	start := time.Now()
	cr.metrics.RecordGroupCreate()

	// Validate inputs
	if pattern == "" {
		err := fmt.Errorf("group pattern cannot be empty")
		cr.trackOperation("group_create", start, err)
		return err
	}

	if setupFunc == nil {
		err := fmt.Errorf("setup function cannot be nil")
		cr.trackOperation("group_create", start, err)
		return err
	}

	// Check context
	if err := cr.checkContext(ctx); err != nil {
		cr.trackOperation("group_create", start, err)
		return err
	}

	cr.mu.Lock()
	defer cr.mu.Unlock()

	// Check if a group already exists
	if _, exists := cr.groups[pattern]; exists {
		err := fmt.Errorf("group for pattern %s already exists", pattern)
		cr.trackOperation("group_create", start, err)
		return err
	}

	// Create a sub-router for the group
	var groupRouter chi.Router
	cr.router.Route(pattern, func(r chi.Router) {
		groupRouter = r
		setupFunc(r)
	})

	// Store group info for tracking
	cr.groups[pattern] = &GroupInfo{
		Pattern:    pattern,
		TenantName: tenantName,
		SetupFunc:  setupFunc,
		Router:     groupRouter,
		Registered: time.Now(),
	}

	cr.logger.Info("route group created successfully",
		"pattern", pattern,
		"tenant", tenantName,
		"script_tag", scriptTag)

	cr.trackOperation("group_create", start, nil)
	return nil
}

// GetRoutes returns all registered routes
func (cr *ChiRouter) GetRoutes() map[string]*RouteInfo {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	// Create a copy to prevent external modification
	routes := make(map[string]*RouteInfo, len(cr.routes))
	for key, route := range cr.routes {
		// Create a copy of the route info
		routeCopy := &RouteInfo{
			Method:     route.Method,
			Pattern:    route.Pattern,
			Handler:    route.Handler,
			TenantName: route.TenantName,
			ScriptTag:  route.ScriptTag,
			Registered: route.Registered,
		}
		routes[key] = routeCopy
	}

	return routes
}

// RemoveRoute removes route from Chi router
func (cr *ChiRouter) RemoveRoute(ctx context.Context, method, pattern string) error {
	start := time.Now()
	cr.metrics.RecordRouteRemove()

	// Validate inputs
	if err := cr.validateInput(method, pattern); err != nil {
		cr.trackOperation("route_remove", start, err)
		return err
	}

	// Check context
	if err := cr.checkContext(ctx); err != nil {
		cr.trackOperation("route_remove", start, err)
		return err
	}

	cr.mu.Lock()
	defer cr.mu.Unlock()

	routeKey := cr.generateRouteKey(method, pattern)

	// Check if the route exists
	if _, exists := cr.routes[routeKey]; !exists {
		err := fmt.Errorf("route %s %s not found", method, pattern)
		cr.trackOperation("route_remove", start, err)
		return err
	}

	// NOTE: Chi doesn't provide direct route removal
	// Remove from our tracking (same limitation as the original)
	//IDEA: instead of removing the routes create a new group/luaRouter WITHOUT THE REMOVED ones only redirect new traffic to the new routes and when the old routes have no traffic left then hot reaload the router with the new routes
	delete(cr.routes, routeKey)

	cr.logger.Info("route removed successfully",
		"method", method,
		"pattern", pattern)

	cr.trackOperation("route_remove", start, nil)
	return nil
}

// validateInput performs common input validation
func (cr *ChiRouter) validateInput(method, pattern string) error {
	if method == "" {
		return fmt.Errorf("method cannot be empty")
	}
	if pattern == "" {
		return fmt.Errorf("pattern cannot be empty")
	}
	return nil
}

// checkContext validates context isn't cancelled
func (cr *ChiRouter) checkContext(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// trackOperation records metrics for operations
func (cr *ChiRouter) trackOperation(operation string, start time.Time, err error) {
	duration := time.Since(start)

	if err != nil {
		// Track error based on the operation type
		switch operation {
		case "route_add", "route_remove":
			cr.metrics.RecordRouteError()
		case "middleware_add":
			cr.metrics.RecordMiddlewareError()
		case "group_create":
			cr.metrics.RecordGroupError()
		}
		cr.metrics.RecordFailedOperation(duration)
		cr.logger.Error("chi router operation failed",
			"operation", operation,
			"duration_ms", duration.Milliseconds(),
			"error", err.Error())
	} else {
		cr.metrics.RecordSuccessfulOperation(duration)
		cr.logger.Debug("chi router operation completed",
			"operation", operation,
			"duration_ms", duration.Milliseconds())
	}
}

// createLuaRouteHandler creates an HTTP handler that executes Lua code
func (cr *ChiRouter) createLuaRouteHandler(handlerFunc *lua.LFunction, scriptTag, tenantName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get Lua state from the pool
		L := cr.statePool.Get()
		defer cr.statePool.Put(L)

		// Set the execution context with timeout
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		L.SetContext(ctx)
		defer L.RemoveContext()

		// Set up Chi bindings for this execution
		if err := cr.SetupLuaBindings(L, scriptTag, tenantName); err != nil {
			http.Error(w, fmt.Sprintf("Script setup error: %v", err), http.StatusInternalServerError)
			return
		}

		// Create Lua request/response tables
		reqTable := cr.createLuaRequest(L, r)
		respWriter := &luaResponseWriter{w: w}
		respTable := cr.createLuaResponse(L, respWriter)

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

// generateRouteKey creates a consistent route key
func (cr *ChiRouter) generateRouteKey(method, pattern string) string {
	return fmt.Sprintf("%s:%s", method, pattern)
}

// ========================================
// Lua Function Implementations
// ========================================

// createRouteFunction returns a Lua function for route registration
func (cr *ChiRouter) createRouteFunction(scriptTag, tenantName string) lua.LGFunction {
	return func(L *lua.LState) int {
		// Extract arguments
		method := L.CheckString(1)
		pattern := L.CheckString(2)
		handlerFunc := L.CheckFunction(3)

		// Validate inputs
		if method == "" || pattern == "" || handlerFunc == nil {
			cr.pushErrorResult(L, "chi_route requires method, pattern, and handler function")
			return 2
		}

		// Create HTTP handler from Lua function
		httpHandler := cr.createLuaRouteHandler(handlerFunc, scriptTag, tenantName)

		// Register route with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := cr.RegisterRoute(ctx, method, pattern, httpHandler, tenantName, scriptTag)
		if err != nil {
			cr.pushErrorResult(L, fmt.Sprintf("failed to register route: %v", err))
			return 2
		}

		cr.pushSuccessResult(L)
		return 2
	}
}

// createMiddlewareFunction returns a Lua function for middleware registration
func (cr *ChiRouter) createMiddlewareFunction(scriptTag, tenantName string) lua.LGFunction {
	return func(L *lua.LState) int {
		// Extract arguments
		pattern := L.CheckString(1)
		middlewareFunc := L.CheckFunction(2)

		// Validate inputs
		if pattern == "" || middlewareFunc == nil {
			cr.pushErrorResult(L, "chi_middleware requires pattern and middleware function")
			return 2
		}

		// Create HTTP middleware from a Lua function
		httpMiddleware := cr.createLuaMiddleware(scriptTag, tenantName, middlewareFunc)

		// Register middleware with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := cr.RegisterMiddleware(ctx, pattern, httpMiddleware, tenantName, scriptTag)
		if err != nil {
			cr.pushErrorResult(L, fmt.Sprintf("failed to register middleware: %v", err))
			return 2
		}

		cr.pushSuccessResult(L)
		return 2
	}
}

// createGroupFunction returns a Lua function for route group creation
func (cr *ChiRouter) createGroupFunction(scriptTag, tenantName string) lua.LGFunction {
	return func(L *lua.LState) int {
		// Extract arguments
		pattern := L.CheckString(1)
		setupFunc := L.CheckFunction(2)

		// Validate inputs
		if pattern == "" || setupFunc == nil {
			cr.pushErrorResult(L, "chi_group requires pattern and setup function")
			return 2
		}

		// Create Chi setup function that manages group context
		chiSetupFunc := func(r chi.Router) {
			// Store old group context HERE: SAFETY ISSUE MIGHT LEAK TENANT INFO TO OTHERS
			oldGroupContext := L.GetGlobal("__current_group_pattern")
			fullPattern := cr.buildGroupPattern(pattern, oldGroupContext)

			// Set a new group context AND HERE: SAFETY ISSUE MIGHT LEAK TENANT INFO TO OTHERS
			L.SetGlobal("__current_group_pattern", lua.LString(fullPattern))
			defer L.SetGlobal("__current_group_pattern", oldGroupContext)

			// Execute Lua setup function
			err := L.CallByParam(lua.P{
				Fn:      setupFunc,
				NRet:    0,
				Protect: true,
			}, cr.createChiRouterLuaValue(L, r))

			if err != nil {
				// Log error but don't panic
				cr.logger.Error("group setup function failed",
					"pattern", pattern,
					"script_tag", scriptTag,
					"tenant", tenantName,
					"error", err.Error())
			}
		}

		// Create a group with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := cr.CreateGroup(ctx, pattern, chiSetupFunc, tenantName, scriptTag)
		if err != nil {
			cr.pushErrorResult(L, fmt.Sprintf("failed to create group: %v", err))
			return 2
		}

		cr.pushSuccessResult(L)
		return 2
	}
}

// createParamFunction returns a Lua function for extracting URL parameters
func (cr *ChiRouter) createParamFunction() lua.LGFunction {
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

// createGetRoutesFunction returns a Lua function for listing registered routes
func (cr *ChiRouter) createGetRoutesFunction() lua.LGFunction {
	return func(L *lua.LState) int {
		routes := cr.GetRoutes()

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

// createRemoveRouteFunction returns a Lua function for removing routes
func (cr *ChiRouter) createRemoveRouteFunction() lua.LGFunction {
	return func(L *lua.LState) int {
		// Extract arguments
		method := L.CheckString(1)
		pattern := L.CheckString(2)

		if method == "" || pattern == "" {
			cr.pushErrorResult(L, "chi_remove_route requires method and pattern")
			return 2
		}

		// Remove route with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := cr.RemoveRoute(ctx, method, pattern)
		if err != nil {
			cr.pushErrorResult(L, fmt.Sprintf("failed to remove route: %v", err))
			return 2
		}

		cr.pushSuccessResult(L)
		return 2
	}
}

// ========================================
// HTTP Integration Methods
// ========================================

// createLuaMiddleware creates HTTP middleware that executes Lua function
func (cr *ChiRouter) createLuaMiddleware(scriptTag, tenantName string, middlewareFunc *lua.LFunction) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get Lua state from the pool
			L := cr.statePool.Get()
			defer cr.statePool.Put(L)

			// Set execution context
			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			defer cancel()

			L.SetContext(ctx)
			defer L.RemoveContext()

			// Setup Chi bindings
			if err := cr.SetupLuaBindings(L, scriptTag, tenantName); err != nil {
				http.Error(w, fmt.Sprintf("Middleware setup error: %v", err), http.StatusInternalServerError)
				return
			}

			// Create Lua request/response tables
			reqTable := cr.createLuaRequest(L, r)
			respWriter := &luaResponseWriter{w: w}
			respTable := cr.createLuaResponse(L, respWriter)

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
// Support Methods
// ========================================

// buildGroupPattern combines a parent group pattern with the current pattern
func (cr *ChiRouter) buildGroupPattern(pattern string, oldGroupContext lua.LValue) string {
	if oldGroupContext != lua.LNil {
		parentPattern := oldGroupContext.String()
		if parentPattern != "" {
			return parentPattern + pattern // Simple concatenation
		}
	}
	return pattern
}

// createChiRouterLuaValue creates a Lua representation of chi.Router ISSUE: IS IT CREATING NEW LUA ROUTER EACH TIME THIS METHOD GETS CALLED
func (cr *ChiRouter) createChiRouterLuaValue(L *lua.LState, r chi.Router) lua.LValue {
	// Create placeholder table for router methods
	routerTable := L.NewTable()

	// Log router creation for debugging
	cr.logger.Debug("created lua router value for group setup")

	// Add router methods here as needed for the group set up
	// The 'r' parameter represents the chi.Router for this group
	// In future implementations, we could expose router methods to Lua
	_ = r // Acknowledge parameter usage

	return routerTable
}

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

// pushSuccessResult pushes success result to Lua stack
func (cr *ChiRouter) pushSuccessResult(L *lua.LState) {
	L.Push(lua.LTrue) // success
	L.Push(lua.LNil)  // no error
}

// pushErrorResult pushes error result to Lua stack
func (cr *ChiRouter) pushErrorResult(L *lua.LState, errMsg string) {
	L.Push(lua.LFalse)          // not successful
	L.Push(lua.LString(errMsg)) // error message
}

// createLuaRequest creates a Lua table representing the HTTP request
func (cr *ChiRouter) createLuaRequest(L *lua.LState, r *http.Request) *lua.LTable {
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
func (cr *ChiRouter) createLuaResponse(L *lua.LState, w *luaResponseWriter) *lua.LTable {
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

// ========================================
// Lifecycle Management
// ========================================

// GetStats returns consolidated metrics
func (cr *ChiRouter) GetStats() map[string]int64 {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	return map[string]int64{
		"routes_registered":      int64(len(cr.routes)),
		"middlewares_registered": int64(len(cr.middlewares)),
		"groups_created":         int64(len(cr.groups)),
		"total_operations":       cr.metrics.TotalOperations.Load(),
		"successful_operations":  cr.metrics.SuccessfulOperations.Load(),
		"failed_operations":      cr.metrics.FailedOperations.Load(),
		"avg_operation_time_ms":  cr.metrics.AvgOperationTime.Load() / 1_000_000,
	}
}

// Shutdown gracefully shuts down the Chi router
func (cr *ChiRouter) Shutdown() error {
	if !cr.shutdown.CompareAndSwap(false, true) {
		return fmt.Errorf("chi router already shut down")
	}

	cr.mu.Lock()
	defer cr.mu.Unlock()

	// Clear all maps
	cr.routes = nil
	cr.middlewares = nil
	cr.groups = nil
	cr.scripts = nil

	return nil
}
