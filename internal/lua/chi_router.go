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

// TODO organize util functions
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
