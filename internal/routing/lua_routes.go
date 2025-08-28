// Package routing provides dynamic route registration capabilities for Lua scripts.
// This file defines the API that allows Lua scripts to register routes, middleware,
// and route groups directly with the Chi router at runtime.
package routing

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	lua "github.com/yuin/gopher-lua"
)

// LuaRouteRegistry manages dynamic route registration from Lua scripts with thread safety
type LuaRouteRegistry struct {
	router           *chi.Mux
	routeGroups      map[string]*chi.Mux               // tenant -> submux for tenant routes
	registeredRoutes map[string]bool                   // track registered routes to prevent duplicates
	middleware       map[string][]MiddlewareDefinition // tenant -> middleware definitions
	mu               sync.RWMutex                      // protects all maps
	Engine           interface {
		GetScript(string) (string, bool)
		SetupChiBindings(*lua.LState, string, string)
	}
}

// RouteDefinition represents a route registered by Lua
type RouteDefinition struct {
	TenantName   string
	Method       string
	Pattern      string
	GroupPattern string // Group context (empty if global, "/api/v1" if in group)
	Handler      http.HandlerFunc
}

// MiddlewareDefinition represents middleware registered by Lua
type MiddlewareDefinition struct {
	TenantName   string
	Pattern      string // Pattern to match for middleware (e.g., "/api/*")
	GroupPattern string // Group context (empty if global, "/api/v1" if in group)
	Middleware   func(http.Handler) http.Handler
}

// RouteGroupDefinition represents a route group registered by Lua
type RouteGroupDefinition struct {
	TenantName string
	Pattern    string
	Middleware []func(http.Handler) http.Handler
	Routes     []RouteDefinition
	Subgroups  []RouteGroupDefinition
}

// NewLuaRouteRegistry creates a new registry for Lua-defined routes
func NewLuaRouteRegistry(router *chi.Mux, engine interface {
	GetScript(string) (string, bool)
	SetupChiBindings(*lua.LState, string, string)
}) *LuaRouteRegistry {
	return &LuaRouteRegistry{
		router:           router,
		routeGroups:      make(map[string]*chi.Mux),
		registeredRoutes: make(map[string]bool),
		middleware:       make(map[string][]MiddlewareDefinition),
		Engine:           engine,
	}
}

// RegisterRoute registers a single route from a Lua script on tenant submux
func (r *LuaRouteRegistry) RegisterRoute(def RouteDefinition) error {
	// Create unique route key
	routeKey := fmt.Sprintf("%s:%s:%s", def.TenantName, def.Method, def.Pattern)

	// Check if route already exists
	r.mu.Lock()
	if r.registeredRoutes[routeKey] {
		r.mu.Unlock()
		// Route already exists, skip registration
		return nil
	}
	r.registeredRoutes[routeKey] = true
	r.mu.Unlock()

	// Validate route pattern before registration
	if err := validateRoutePattern(def.Pattern); err != nil {
		// Remove from registered routes since validation failed
		r.mu.Lock()
		delete(r.registeredRoutes, routeKey)
		r.mu.Unlock()
		return fmt.Errorf("invalid route pattern '%s': %w", def.Pattern, err)
	}

	// Get tenant submux and register route on it
	submux := r.getTenantSubmux(def.TenantName)
	r.registerRouteByMethod(submux, def)

	return nil
}

// RegisterMiddleware registers middleware for a pattern from a Lua script
func (r *LuaRouteRegistry) RegisterMiddleware(def MiddlewareDefinition) error {
	// Store middleware for later application (Chi requires middleware before routes)
	r.mu.Lock()
	r.middleware[def.TenantName] = append(r.middleware[def.TenantName], def)
	r.mu.Unlock()

	return nil
}

// RegisterRouteGroup registers a group of routes with shared middleware with duplicate prevention
func (r *LuaRouteRegistry) RegisterRouteGroup(def RouteGroupDefinition) error {
	// Create unique group key
	groupKey := fmt.Sprintf("%s:group:%s", def.TenantName, def.Pattern)

	// Check if group already exists
	r.mu.Lock()
	if r.registeredRoutes[groupKey] {
		r.mu.Unlock()
		// Group already exists, skip registration
		return nil
	}
	r.registeredRoutes[groupKey] = true
	r.mu.Unlock()

	submux := r.getTenantSubmux(def.TenantName)

	// Create route group with pattern and middleware
	submux.Route(def.Pattern, func(gr chi.Router) {
		// Apply group middleware
		for _, mw := range def.Middleware {
			gr.Use(mw)
		}

		// Register routes in the group
		for _, route := range def.Routes {
			r.registerRouteByMethod(gr, route)
		}

		// Register subgroups recursively
		for _, subgroup := range def.Subgroups {
			r.registerSubgroup(gr, subgroup)
		}
	})

	return nil
}

// ClearTenantRoutes removes all routes for a specific tenant
func (r *LuaRouteRegistry) ClearTenantRoutes(tenantName string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Remove tenant submux and middleware
	delete(r.routeGroups, tenantName)
	delete(r.middleware, tenantName)

	// Remove any registeredRoutes entries for this tenant (routes and groups)
	prefix := tenantName + ":"
	for k := range r.registeredRoutes {
		if strings.HasPrefix(k, prefix) {
			delete(r.registeredRoutes, k)
		}
	}
}

// GetTenantRoutes returns the submux for a tenant (for inspection/debugging)
func (r *LuaRouteRegistry) GetTenantRoutes(tenantName string) *chi.Mux {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.routeGroups[tenantName]
}

// HasRoutes checks if a tenant has actual routes registered (not just middleware)
func (r *LuaRouteRegistry) HasRoutes(tenantName string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	// Check if any registered routes belong to this tenant
	for routeKey := range r.registeredRoutes {
		// Route keys are formatted as "tenantName:method:pattern"
		if strings.HasPrefix(routeKey, tenantName+":") {
			return true
		}
	}
	return false
}

// ListTenants returns all tenants that have registered routes
func (r *LuaRouteRegistry) ListTenants() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tenants := make([]string, 0, len(r.routeGroups))
	for tenant := range r.routeGroups {
		tenants = append(tenants, tenant)
	}
	return tenants
}

// ApplyMiddleware applies all stored middleware for a tenant (call after routes are registered)
func (r *LuaRouteRegistry) ApplyMiddleware(tenantName string) error {
	r.mu.RLock()
	middlewares := r.middleware[tenantName]
	r.mu.RUnlock()

	if len(middlewares) == 0 {
		return nil
	}

	submux := r.getTenantSubmux(tenantName)
	for _, def := range middlewares {
		submux.Use(def.Middleware)
	}
	return nil
}

// getTenantSubmux gets or creates a submux for a tenant
func (r *LuaRouteRegistry) getTenantSubmux(tenantName string) *chi.Mux {
	// Check if submux exists (read lock)
	r.mu.RLock()
	if submux, exists := r.routeGroups[tenantName]; exists {
		r.mu.RUnlock()
		return submux
	}
	r.mu.RUnlock()

	// Create new submux for tenant (write lock)
	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check pattern - another goroutine might have created it
	if submux, exists := r.routeGroups[tenantName]; exists {
		return submux
	}

	submux := chi.NewMux()
	r.routeGroups[tenantName] = submux
	return submux
}

// registerSubgroup recursively registers subgroups
func (r *LuaRouteRegistry) registerSubgroup(parent chi.Router, def RouteGroupDefinition) {
	parent.Route(def.Pattern, func(gr chi.Router) {
		// Apply group middleware
		for _, mw := range def.Middleware {
			gr.Use(mw)
		}

		// Register routes in the subgroup
		for _, route := range def.Routes {
			r.registerRouteByMethod(gr, route)
		}
		// Register nested subgroups
		for _, subgroup := range def.Subgroups {
			r.registerSubgroup(gr, subgroup)
		}
	})
}

// RouteRegistryAPI provides the high-level API for Lua script integration
type RouteRegistryAPI struct {
	registry *LuaRouteRegistry
}

// NewRouteRegistryAPI creates a new API wrapper
func NewRouteRegistryAPI(router *chi.Mux) *RouteRegistryAPI {
	return &RouteRegistryAPI{
		registry: NewLuaRouteRegistry(router, nil),
	}
}

// Route registers a simple route (called from Lua via chi_route function)
func (api *RouteRegistryAPI) Route(tenantName, method, pattern string, handler http.HandlerFunc) error {
	return api.registry.RegisterRoute(RouteDefinition{
		TenantName: tenantName,
		Method:     method,
		Pattern:    pattern,
		Handler:    handler,
	})
}

// Middleware registers middleware for a pattern (called from Lua via chi_middleware function)
func (api *RouteRegistryAPI) Middleware(tenantName, pattern string, middleware func(http.Handler) http.Handler) error {
	return api.registry.RegisterMiddleware(MiddlewareDefinition{
		TenantName: tenantName,
		Pattern:    pattern,
		Middleware: middleware,
	})
}

// Group registers a route group (called from Lua via chi_group function)
func (api *RouteRegistryAPI) Group(tenantName, pattern string, middleware []func(http.Handler) http.Handler, setupFunc func(*RouteRegistryAPI)) error {
	// This will be used by the Lua bindings to set up groups
	// The setupFunc will be called with the API to register routes within the group
	def := RouteGroupDefinition{
		TenantName: tenantName,
		Pattern:    pattern,
		Middleware: middleware,
		Routes:     []RouteDefinition{},
		Subgroups:  []RouteGroupDefinition{},
	}

	return api.registry.RegisterRouteGroup(def)
}

// Mount mounts tenant routes under a path (called from Lua via chi_mount function)
func (api *RouteRegistryAPI) Mount(tenantName, mountPath string) error {
	// Mount functionality is now handled in main.go using Chi's native Mount()
	// This method is kept for API compatibility but doesn't need to do anything
	return nil
}

// Clear removes all routes for a tenant
func (api *RouteRegistryAPI) Clear(tenantName string) {
	api.registry.ClearTenantRoutes(tenantName)
}

// registerRouteByMethod consolidates the duplicate route registration logic
func (r *LuaRouteRegistry) registerRouteByMethod(router chi.Router, route RouteDefinition) {
	// Apply middleware that matches this route pattern
	handler := r.applyMiddleware(route)

	switch route.Method {
	case "GET":
		router.Get(route.Pattern, handler)
	case "POST":
		router.Post(route.Pattern, handler)
	case "PUT":
		router.Put(route.Pattern, handler)
	case "DELETE":
		router.Delete(route.Pattern, handler)
	case "PATCH":
		router.Patch(route.Pattern, handler)
	case "OPTIONS":
		router.Options(route.Pattern, handler)
	case "HEAD":
		router.Head(route.Pattern, handler)
	default:
		// Handle custom methods that Chi might not support
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Chi doesn't support this HTTP method, silently skip
					fmt.Printf("Warning: HTTP method '%s' is not supported by Chi router\n", route.Method)
				}
			}()
			router.Method(route.Method, route.Pattern, handler)
		}()
	}
}

// applyMiddleware applies middleware that matches the route pattern
func (r *LuaRouteRegistry) applyMiddleware(route RouteDefinition) http.HandlerFunc {
	r.mu.RLock()
	middlewares := r.middleware[route.TenantName]
	r.mu.RUnlock()

	var handler http.Handler = route.Handler

	// Apply middleware in reverse order (last registered middleware wraps first)
	for i := len(middlewares) - 1; i >= 0; i-- {
		mw := middlewares[i]
		if r.routeMatchesPattern(route, mw) {
			handler = mw.Middleware(handler)
		}
	}

	return handler.ServeHTTP
}

// routeMatchesPattern checks if a route pattern matches a middleware pattern
// considering group context for proper scoping
func (r *LuaRouteRegistry) routeMatchesPattern(route RouteDefinition, middleware MiddlewareDefinition) bool {
	// Get the effective patterns to compare
	middlewarePattern := middleware.Pattern
	routePattern := route.Pattern

	// Handle group-scoped middleware
	if middleware.GroupPattern != "" {
		// Group middleware: only applies to routes in the same group
		if middleware.GroupPattern != route.GroupPattern {
			return false
		}
		// For group middleware, resolve the pattern relative to the group
		// e.g., group="/api/v1", middleware pattern="/*" should match "/api/v1/users"
		middlewarePattern = middleware.GroupPattern + middleware.Pattern
	}
	// Global middleware (empty GroupPattern): applies to all routes regardless of group

	// Handle wildcard patterns like "/protected/*" or "/api/v1/*"
	if strings.HasSuffix(middlewarePattern, "/*") {
		prefix := strings.TrimSuffix(middlewarePattern, "/*")
		return strings.HasPrefix(routePattern, prefix)
	}

	// Exact match
	return routePattern == middlewarePattern
}

// validateRoutePattern validates Chi router pattern format
func validateRoutePattern(pattern string) error {
	if pattern == "" {
		return fmt.Errorf("route pattern cannot be empty")
	}

	if !strings.HasPrefix(pattern, "/") {
		return fmt.Errorf("route pattern must begin with '/'")
	}

	// Check for unmatched parameter braces
	braceCount := 0
	for i, char := range pattern {
		switch char {
		case '{':
			braceCount++
		case '}':
			braceCount--
			if braceCount < 0 {
				return fmt.Errorf("unmatched closing brace '}' at position %d", i)
			}
		}
	}

	if braceCount > 0 {
		return fmt.Errorf("route param closing delimiter '}' is missing")
	}

	return nil
}
