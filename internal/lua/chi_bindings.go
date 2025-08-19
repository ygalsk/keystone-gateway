package lua

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"keystone-gateway/internal/types"

	"github.com/go-chi/chi/v5"
	lua "github.com/yuin/gopher-lua"
)

// ChiBindings provides production-quality Chi router integration for Lua scripts
// Follows atomic operations patterns and proper error handling throughout
type ChiBindings struct {
	controller types.ChiRouterController
	statePool  *LuaStatePool
	metrics    *ChiBindingsMetrics

	// Thread-safe script registry
	mu      sync.RWMutex
	scripts map[string]string

	// Atomic counters for operations
	initialized atomic.Bool
	shutdown    atomic.Bool
}

// ChiBindingsMetrics tracks Chi operations with atomic counters (consistent with other components)
type ChiBindingsMetrics struct {
	// Route operations
	routeRegistrations atomic.Int64
	routeRemovals      atomic.Int64
	routeErrors        atomic.Int64

	// Middleware operations
	middlewareRegistrations atomic.Int64
	middlewareRemovals      atomic.Int64
	middlewareErrors        atomic.Int64

	// Group operations
	groupCreations atomic.Int64
	groupRemovals  atomic.Int64
	groupErrors    atomic.Int64

	// Execution metrics
	totalOperations atomic.Int64
	successfulOps   atomic.Int64
	failedOps       atomic.Int64

	// Performance metrics
	avgOperationTime atomic.Int64 // nanoseconds
	totalOpTime      atomic.Int64 // nanoseconds
}

// NewChiBindings creates a new Chi bindings instance with proper initialization
func NewChiBindings(controller types.ChiRouterController, statePool *LuaStatePool) *ChiBindings {
	bindings := &ChiBindings{
		controller: controller,
		statePool:  statePool,
		metrics:    &ChiBindingsMetrics{},
		scripts:    make(map[string]string),
	}

	bindings.initialized.Store(true)
	return bindings
}

// SetupChiBindings registers Chi router functions with a Lua state
// Follows official gopher-lua patterns with proper error returns instead of panics
func (cb *ChiBindings) SetupChiBindings(L *lua.LState, scriptTag, tenantName string) error {
	if !cb.initialized.Load() || cb.shutdown.Load() {
		return fmt.Errorf("chi bindings not initialized or shut down")
	}

	// Register Lua functions with proper error handling
	functions := map[string]lua.LGFunction{
		"chi_route":        cb.createRouteFunction(scriptTag, tenantName),
		"chi_middleware":   cb.createMiddlewareFunction(scriptTag, tenantName),
		"chi_group":        cb.createGroupFunction(scriptTag, tenantName),
		"chi_param":        cb.createParamFunction(),
		"chi_get_routes":   cb.createGetRoutesFunction(),
		"chi_remove_route": cb.createRemoveRouteFunction(),
	}

	// Register functions safely
	for name, fn := range functions {
		L.SetGlobal(name, L.NewFunction(fn))
	}

	return nil
}

// createRouteFunction returns a Lua function for route registration
// Follows atomic metrics patterns and proper error handling
func (cb *ChiBindings) createRouteFunction(scriptTag, tenantName string) lua.LGFunction {
	return func(L *lua.LState) int {
		// Track operation metrics
		start := time.Now()
		cb.metrics.totalOperations.Add(1)
		cb.metrics.routeRegistrations.Add(1)

		// Extract arguments safely
		method := L.CheckString(1)
		pattern := L.CheckString(2)
		handlerFunc := L.CheckFunction(3)

		// Validate inputs
		if method == "" || pattern == "" || handlerFunc == nil {
			cb.trackError()
			cb.pushErrorResult(L, "chi_route requires method, pattern, and handler function")
			return 2 // success (false), error (string)
		}

		// Create HTTP handler that executes Lua function
		httpHandler := cb.createLuaHttpHandler(scriptTag, handlerFunc)

		// Create route definition
		routeDef := types.RouteDefinition{
			TenantName:   tenantName,
			Method:       method,
			Pattern:      pattern,
			GroupPattern: cb.getCurrentGroupPattern(L),
			Handler:      httpHandler,
			RegisteredAt: time.Now(),
		}

		// Register a route with context timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := cb.controller.AddRoute(ctx, routeDef)
		if err != nil {
			cb.trackError()
			cb.pushErrorResult(L, fmt.Sprintf("failed to register route: %v", err))
			return 2
		}

		// Track success
		cb.trackSuccess(time.Since(start))
		cb.pushSuccessResult(L)
		return 2 // success (true), error (nil)
	}
}

// createMiddlewareFunction returns a Lua function for middleware registration
func (cb *ChiBindings) createMiddlewareFunction(scriptTag, tenantName string) lua.LGFunction {
	return func(L *lua.LState) int {
		start := time.Now()
		cb.metrics.totalOperations.Add(1)
		cb.metrics.middlewareRegistrations.Add(1)

		// Extract arguments safely
		pattern := L.CheckString(1)
		middlewareFunc := L.CheckFunction(2)

		if pattern == "" || middlewareFunc == nil {
			cb.trackError()
			cb.pushErrorResult(L, "chi_middleware requires pattern and middleware function")
			return 2
		}

		// Create HTTP middleware that executes a Lua function
		httpMiddleware := cb.createLuaMiddleware(scriptTag, middlewareFunc)

		// Create middleware definition
		middlewareDef := types.MiddlewareDefinition{
			TenantName:   tenantName,
			Pattern:      pattern,
			GroupPattern: cb.getCurrentGroupPattern(L),
			Middleware:   httpMiddleware,
			RegisteredAt: time.Now(),
		}

		// Register middleware with context timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := cb.controller.AddMiddleware(ctx, middlewareDef)
		if err != nil {
			cb.trackError()
			cb.pushErrorResult(L, fmt.Sprintf("failed to register middleware: %v", err))
			return 2
		}

		cb.trackSuccess(time.Since(start))
		cb.pushSuccessResult(L)
		return 2
	}
}

// createGroupFunction returns a Lua function for route group creation
func (cb *ChiBindings) createGroupFunction(scriptTag, tenantName string) lua.LGFunction {
	return func(L *lua.LState) int {
		start := time.Now()
		cb.metrics.totalOperations.Add(1)
		cb.metrics.groupCreations.Add(1)

		// Extract arguments safely
		pattern := L.CheckString(1)
		setupFunc := L.CheckFunction(2)

		if pattern == "" || setupFunc == nil {
			cb.trackError()
			cb.pushErrorResult(L, "chi_group requires pattern and setup function")
			return 2
		}

		// Create Chi setup function
		chiSetupFunc := func(r chi.Router) {
			// Store the current group context
			oldGroupContext := L.GetGlobal("__current_group_pattern")
			fullPattern := cb.buildGroupPattern(pattern, oldGroupContext)

			// Set a new group context
			L.SetGlobal("__current_group_pattern", lua.LString(fullPattern))
			defer L.SetGlobal("__current_group_pattern", oldGroupContext)

			// Execute Lua setup function with router context
			err := L.CallByParam(lua.P{
				Fn:      setupFunc,
				NRet:    0,
				Protect: true,
			}, cb.createChiRouterLuaValue(L, r))

			if err != nil {
				// Log error but don't panic (production safety)
				// TODO: Add proper logging integration
				fmt.Printf("Group setup function error: %v\n", err)
			}
		}

		// Create a group with context timeout
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := cb.controller.CreateGroup(ctx, pattern, chiSetupFunc)
		if err != nil {
			cb.trackError()
			cb.pushErrorResult(L, fmt.Sprintf("failed to create group: %v", err))
			return 2
		}

		cb.trackSuccess(time.Since(start))
		cb.pushSuccessResult(L)
		return 2
	}
}

// createParamFunction returns a Lua function for extracting URL parameters
func (cb *ChiBindings) createParamFunction() lua.LGFunction {
	return func(L *lua.LState) int {
		// Extract request table and parameter name
		requestTable := L.ToTable(1)
		paramName := L.ToString(2)

		if requestTable == nil || paramName == "" {
			L.Push(lua.LString(""))
			return 1
		}

		// Try to get from chi context (will be set by HTTP handler)
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
func (cb *ChiBindings) createGetRoutesFunction() lua.LGFunction {
	return func(L *lua.LState) int {
		routes := cb.controller.ListRoutes()

		// Create Lua table of routes
		routesTable := L.NewTable()
		for i, route := range routes {
			routeTable := L.NewTable()
			L.SetField(routeTable, "method", lua.LString(route.Method))
			L.SetField(routeTable, "pattern", lua.LString(route.Pattern))
			L.SetField(routeTable, "tenant", lua.LString(route.TenantName))
			L.SetField(routeTable, "registered", lua.LNumber(route.Registered.Unix()))

			routesTable.RawSetInt(i+1, routeTable)
		}

		L.Push(routesTable)
		return 1
	}
}

// createRemoveRouteFunction returns a Lua function for removing routes
func (cb *ChiBindings) createRemoveRouteFunction() lua.LGFunction {
	return func(L *lua.LState) int {
		start := time.Now()
		cb.metrics.totalOperations.Add(1)
		cb.metrics.routeRemovals.Add(1)

		method := L.CheckString(1)
		pattern := L.CheckString(2)

		if method == "" || pattern == "" {
			cb.trackError()
			cb.pushErrorResult(L, "chi_remove_route requires method and pattern")
			return 2
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := cb.controller.RemoveRoute(ctx, method, pattern)
		if err != nil {
			cb.trackError()
			cb.pushErrorResult(L, fmt.Sprintf("failed to remove route: %v", err))
			return 2
		}

		cb.trackSuccess(time.Since(start))
		cb.pushSuccessResult(L)
		return 2
	}
}

// Helper functions for production-quality operations

// createLuaHttpHandler creates an HTTP handler that executes Lua function
func (cb *ChiBindings) createLuaHttpHandler(scriptTag string, handlerFunc *lua.LFunction) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get Lua state from the pool
		L := cb.statePool.Get()
		defer cb.statePool.Put(L)

		// Set the execution context with timeout
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		L.SetContext(ctx)
		defer L.RemoveContext()

		// Set up Chi bindings for this execution
		if err := cb.SetupChiBindings(L, scriptTag, ""); err != nil {
			http.Error(w, fmt.Sprintf("Script setup error: %v", err), http.StatusInternalServerError)
			return
		}

		// Create Lua request/response tables
		reqTable := cb.createLuaRequest(L, r)
		respWriter := &luaResponseWriter{w: w}
		respTable := cb.createLuaResponse(L, respWriter)

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
func (cb *ChiBindings) createLuaMiddleware(scriptTag string, middlewareFunc *lua.LFunction) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get Lua state from the pool
			L := cb.statePool.Get()
			defer cb.statePool.Put(L)

			// Set execution context
			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			defer cancel()

			L.SetContext(ctx)
			defer L.RemoveContext()

			// Setup Chi bindings
			if err := cb.SetupChiBindings(L, scriptTag, ""); err != nil {
				http.Error(w, fmt.Sprintf("Middleware setup error: %v", err), http.StatusInternalServerError)
				return
			}

			// Create Lua request/response tables
			reqTable := cb.createLuaRequest(L, r)
			respWriter := &luaResponseWriter{w: w}
			respTable := cb.createLuaResponse(L, respWriter)

			// Create next the function
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

// Utility functions following production patterns

// getCurrentGroupPattern retrieves the current group pattern from Lua context
func (cb *ChiBindings) getCurrentGroupPattern(L *lua.LState) string {
	if groupCtx := L.GetGlobal("__current_group_pattern"); groupCtx != lua.LNil {
		return groupCtx.String()
	}
	return ""
}

// buildGroupPattern combines a parent group pattern with the current pattern
func (cb *ChiBindings) buildGroupPattern(pattern string, oldGroupContext lua.LValue) string {
	if oldGroupContext != lua.LNil {
		parentPattern := oldGroupContext.String()
		if parentPattern != "" {
			return parentPattern + pattern
		}
	}
	return pattern
}

// createChiRouterLuaValue creates a Lua representation of chi.Router (placeholder)
func (cb *ChiBindings) createChiRouterLuaValue(L *lua.LState, r chi.Router) lua.LValue {
	// This would create a Lua table with router methods
	// Implementation depends on specific Chi router exposure needs
	routerTable := L.NewTable()
	// Add router methods here as needed
	return routerTable
}

// Atomic metrics tracking functions (consistent with other components)

// trackError increments error counters atomically
func (cb *ChiBindings) trackError() {
	cb.metrics.failedOps.Add(1)
	cb.metrics.routeErrors.Add(1) // Could be more specific based on the operation type
}

// trackSuccess updates success metrics atomically
func (cb *ChiBindings) trackSuccess(duration time.Duration) {
	cb.metrics.successfulOps.Add(1)

	// Update timing metrics atomically
	durationNanos := duration.Nanoseconds()
	cb.metrics.totalOpTime.Add(durationNanos)

	// Calculate rolling average
	totalOps := cb.metrics.totalOperations.Load()
	if totalOps > 0 {
		avgNanos := cb.metrics.totalOpTime.Load() / totalOps
		cb.metrics.avgOperationTime.Store(avgNanos)
	}
}

// Lua result helper functions (proper error handling instead of panics)

// pushSuccessResult pushes success result to the Lua stack
func (cb *ChiBindings) pushSuccessResult(L *lua.LState) {
	L.Push(lua.LTrue) // success
	L.Push(lua.LNil)  // no error
}

// pushErrorResult pushes error result to Lua stack
func (cb *ChiBindings) pushErrorResult(L *lua.LState, errMsg string) {
	L.Push(lua.LFalse)          // not successful
	L.Push(lua.LString(errMsg)) // error message
}

// GetStats returns atomic metrics (consistent with other components)
func (cb *ChiBindings) GetStats() map[string]int64 {
	return map[string]int64{
		"route_registrations":      cb.metrics.routeRegistrations.Load(),
		"route_removals":           cb.metrics.routeRemovals.Load(),
		"route_errors":             cb.metrics.routeErrors.Load(),
		"middleware_registrations": cb.metrics.middlewareRegistrations.Load(),
		"middleware_removals":      cb.metrics.middlewareRemovals.Load(),
		"middleware_errors":        cb.metrics.middlewareErrors.Load(),
		"group_creations":          cb.metrics.groupCreations.Load(),
		"group_removals":           cb.metrics.groupRemovals.Load(),
		"group_errors":             cb.metrics.groupErrors.Load(),
		"total_operations":         cb.metrics.totalOperations.Load(),
		"successful_operations":    cb.metrics.successfulOps.Load(),
		"failed_operations":        cb.metrics.failedOps.Load(),
		"avg_operation_time_ms":    cb.metrics.avgOperationTime.Load() / 1_000_000, // Convert to ms
	}
}

// Shutdown gracefully shuts down the Chi bindings
func (cb *ChiBindings) Shutdown() error {
	if !cb.shutdown.CompareAndSwap(false, true) {
		return fmt.Errorf("chi bindings already shut down")
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Clear script registry
	cb.scripts = nil

	return nil
}
