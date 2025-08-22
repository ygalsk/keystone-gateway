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
	"keystone-gateway/internal/metrics"
)

// ChiRouter provides Lua integration with Chi router
type ChiRouter struct {
	// Core dependencies
	router    chi.Router
	statePool *LuaStatePool
	metrics   *metrics.LuaMetrics // Single metrics system
	logger    *slog.Logger
	bindings  *LuaBindings // Centralized Lua bindings

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

// TODO being used by Server.go check if the way its used is appropriate and needed
func NewChiRouter(router chi.Router, statePool *LuaStatePool, metrics *metrics.LuaMetrics, logger *slog.Logger) *ChiRouter {
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

	// Initialize LuaBindings with cr as the RouterInterface
	cr.bindings = NewLuaBindings(cr, statePool, metrics, logger)

	cr.initialized.Store(true)
	return cr
}

// SetupLuaBindings registers all Chi functions with Lua state
func (cr *ChiRouter) SetupLuaBindings(L *lua.LState, scriptTag, tenantName string) error {
	if !cr.initialized.Load() || cr.shutdown.Load() {
		return fmt.Errorf("chi router not initialized or shut down")
	}

	// Delegate to centralized LuaBindings
	return cr.bindings.SetupLuaBindings(L, scriptTag, tenantName)
}

// RegisterRoute adds an HTTP route to Chi router
func (cr *ChiRouter) RegisterRoute(ctx context.Context, method, pattern string, handler http.HandlerFunc, tenantName, scriptTag string) error {
	// Record metrics
	cr.metrics.RecordRouteAdd()

	// Build operation
	op, err := cr.buildRouteOperation(method, pattern, handler, tenantName, scriptTag)
	if err != nil {
		cr.metrics.TrackOperation("route_add", time.Now(), err, cr.logger)
		return err
	}

	// Execute common flow
	return cr.registerOperation(ctx, op)
}

// RegisterMiddleware adds middleware to Chi router
func (cr *ChiRouter) RegisterMiddleware(ctx context.Context, pattern string, middleware func(http.Handler) http.Handler, tenantName, scriptTag string) error {
	cr.metrics.RecordMiddlewareAdd()

	op, err := cr.buildMiddlewareOperation(pattern, middleware, tenantName, scriptTag)
	if err != nil {
		cr.metrics.TrackOperation("middleware_add", time.Now(), err, cr.logger)
		return err
	}

	return cr.registerOperation(ctx, op)
}

// CreateGroup adds a route group to Chi router
func (cr *ChiRouter) CreateGroup(ctx context.Context, pattern string, setupFunc func(chi.Router), tenantName, scriptTag string) error {
	cr.metrics.RecordGroupCreate()

	op, err := cr.buildGroupOperation(pattern, setupFunc, tenantName, scriptTag)
	if err != nil {
		cr.metrics.TrackOperation("group_create", time.Now(), err, cr.logger)
		return err
	}

	return cr.registerOperation(ctx, op)
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
		cr.metrics.TrackOperation("route_remove", start, err, cr.logger)
		return err
	}

	// Check context
	if err := cr.checkContext(ctx); err != nil {
		cr.metrics.TrackOperation("route_remove", start, err, cr.logger)
		return err
	}

	cr.mu.Lock()
	defer cr.mu.Unlock()

	routeKey := cr.generateRouteKey(method, pattern)

	// Check if the route exists
	if _, exists := cr.routes[routeKey]; !exists {
		err := fmt.Errorf("route %s %s not found", method, pattern)
		cr.metrics.TrackOperation("route_remove", start, err, cr.logger)
		return err
	}

	// NOTE: Chi doesn't provide direct route removal
	// Remove from our tracking (same limitation as the original)
	//IDEA: instead of removing the routes create a new group/luaRouter WITHOUT THE REMOVED ones only redirect new traffic to the new routes and when the old routes have no traffic left then hot reaload the router with the new routes
	delete(cr.routes, routeKey)

	cr.logger.Info("route removed successfully",
		"method", method,
		"pattern", pattern)

	cr.metrics.TrackOperation("route_remove", start, nil, cr.logger)
	return nil
}

// ========================================
// Operation Infrastructure
// ========================================

// Operation represents a registrable operation
type Operation struct {
	Type     string      // "route_add", "middleware_add", "group_create"
	Key      string      // unique identifier
	Metadata interface{} // RouteInfo, MiddlewareInfo, or GroupInfo
	ChiFunc  func() error // actual Chi operation
}

// ========================================
// Validation Functions (Single Purpose Functions)
// ========================================

// validateRouteInput validates route registration input parameters
func validateRouteInput(method, pattern string) error {
	if method == "" {
		return fmt.Errorf("method cannot be empty")
	}
	if pattern == "" {
		return fmt.Errorf("pattern cannot be empty")
	}
	return nil
}

// validateMiddlewareInput validates middleware registration input parameters
func validateMiddlewareInput(pattern string, middleware func(http.Handler) http.Handler) error {
	if pattern == "" {
		return fmt.Errorf("middleware pattern cannot be empty")
	}
	if middleware == nil {
		return fmt.Errorf("middleware function cannot be nil")
	}
	return nil
}

// validateGroupInput validates group creation input parameters
func validateGroupInput(pattern string, setupFunc func(chi.Router)) error {
	if pattern == "" {
		return fmt.Errorf("group pattern cannot be empty")
	}
	if setupFunc == nil {
		return fmt.Errorf("setup function cannot be nil")
	}
	return nil
}

// validateInput performs common input validation (legacy method for backward compatibility)
func (cr *ChiRouter) validateInput(method, pattern string) error {
	return validateRouteInput(method, pattern)
}

// ========================================
// Context Management (Explicit Error Handling)
// ========================================

// checkContextDone validates context isn't cancelled
func checkContextDone(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// checkContext validates context isn't cancelled (legacy method for backward compatibility)
func (cr *ChiRouter) checkContext(ctx context.Context) error {
	return checkContextDone(ctx)
}

// ========================================
// Registration Engine (Core Logic Extraction)
// ========================================

// registerOperation handles the common registration pattern
func (cr *ChiRouter) registerOperation(ctx context.Context, op *Operation) error {
	// 1. Start timing
	start := time.Now()
	defer func() {
		cr.metrics.TrackOperation(op.Type, start, nil, cr.logger)
	}()

	// 2. Check context
	if err := checkContextDone(ctx); err != nil {
		cr.metrics.TrackOperation(op.Type, start, err, cr.logger)
		return err
	}

	// 3. Lock and check for duplicates
	cr.mu.Lock()
	defer cr.mu.Unlock()

	if cr.isDuplicate(op) {
		err := fmt.Errorf("%s already exists: %s", op.Type, op.Key)
		cr.metrics.TrackOperation(op.Type, start, err, cr.logger)
		return err
	}

	// 4. Execute operation-specific logic
	if err := op.ChiFunc(); err != nil {
		operationErr := fmt.Errorf("failed to execute %s operation: %w", op.Type, err)
		cr.metrics.TrackOperation(op.Type, start, operationErr, cr.logger)
		return operationErr
	}

	// 5. Store metadata
	cr.storeMetadata(op)

	// 6. Log success
	cr.logger.Info(fmt.Sprintf("%s registered successfully", op.Type),
		"key", op.Key)

	return nil
}

// ========================================
// Operation Builders (Factory Pattern)
// ========================================

// buildRouteOperation creates a route operation
func (cr *ChiRouter) buildRouteOperation(method, pattern string, handler http.HandlerFunc, tenantName, scriptTag string) (*Operation, error) {
	if err := validateRouteInput(method, pattern); err != nil {
		return nil, err
	}
	if handler == nil {
		return nil, fmt.Errorf("handler cannot be nil")
	}

	routeKey := cr.generateRouteKey(method, pattern)

	return &Operation{
		Type: "route_add",
		Key:  routeKey,
		Metadata: &RouteInfo{
			Method:     method,
			Pattern:    pattern,
			Handler:    handler,
			TenantName: tenantName,
			ScriptTag:  scriptTag,
			Registered: time.Now(),
		},
		ChiFunc: func() error {
			return cr.executeChiRoute(method, pattern, handler)
		},
	}, nil
}

// buildMiddlewareOperation creates a middleware operation
func (cr *ChiRouter) buildMiddlewareOperation(pattern string, middleware func(http.Handler) http.Handler, tenantName, scriptTag string) (*Operation, error) {
	if err := validateMiddlewareInput(pattern, middleware); err != nil {
		return nil, err
	}

	return &Operation{
		Type: "middleware_add",
		Key:  pattern,
		Metadata: &MiddlewareInfo{
			Pattern:    pattern,
			Middleware: middleware,
			TenantName: tenantName,
			ScriptTag:  scriptTag,
			Registered: time.Now(),
		},
		ChiFunc: func() error {
			cr.router.Use(middleware)
			return nil
		},
	}, nil
}

// buildGroupOperation creates a group operation
func (cr *ChiRouter) buildGroupOperation(pattern string, setupFunc func(chi.Router), tenantName, scriptTag string) (*Operation, error) {
	if err := validateGroupInput(pattern, setupFunc); err != nil {
		return nil, err
	}

	return &Operation{
		Type: "group_create",
		Key:  pattern,
		Metadata: &GroupInfo{
			Pattern:    pattern,
			TenantName: tenantName,
			SetupFunc:  setupFunc,
			Registered: time.Now(),
		},
		ChiFunc: func() error {
			var groupRouter chi.Router
			cr.router.Route(pattern, func(r chi.Router) {
				groupRouter = r
				setupFunc(r)
			})
			// Store router reference in metadata after creation
			if groupInfo, ok := cr.groups[pattern]; ok {
				groupInfo.Router = groupRouter
			}
			return nil
		},
	}, nil
}

// ========================================
// Helper Functions (Single Responsibility)
// ========================================

// isDuplicate checks if operation already exists
func (cr *ChiRouter) isDuplicate(op *Operation) bool {
	switch op.Type {
	case "route_add":
		_, exists := cr.routes[op.Key]
		return exists
	case "middleware_add":
		_, exists := cr.middlewares[op.Key]
		return exists
	case "group_create":
		_, exists := cr.groups[op.Key]
		return exists
	}
	return false
}

// storeMetadata stores operation metadata in appropriate map
func (cr *ChiRouter) storeMetadata(op *Operation) {
	switch op.Type {
	case "route_add":
		cr.routes[op.Key] = op.Metadata.(*RouteInfo)
	case "middleware_add":
		cr.middlewares[op.Key] = op.Metadata.(*MiddlewareInfo)
	case "group_create":
		cr.groups[op.Key] = op.Metadata.(*GroupInfo)
	}
}

// executeChiRoute performs the method-specific route registration
func (cr *ChiRouter) executeChiRoute(method, pattern string, handler http.HandlerFunc) error {
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
	return nil
}

// generateRouteKey creates a consistent route key
func (cr *ChiRouter) generateRouteKey(method, pattern string) string {
	return fmt.Sprintf("%s:%s", method, pattern)
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
