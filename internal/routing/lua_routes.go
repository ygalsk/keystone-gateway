// Package routing provides dynamic route registration capabilities for Lua scripts.
// This file defines the API that allows Lua scripts to register routes, middleware,
// and route groups directly with the Chi router at runtime.
package routing

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// LuaRouteRegistry manages dynamic route registration from Lua scripts
type LuaRouteRegistry struct {
	router      *chi.Mux
	routeGroups map[string]*chi.Mux // tenant -> submux for tenant routes
}

// RouteDefinition represents a route registered by Lua
type RouteDefinition struct {
	TenantName string
	Method     string
	Pattern    string
	Handler    http.HandlerFunc
}

// MiddlewareDefinition represents middleware registered by Lua
type MiddlewareDefinition struct {
	TenantName string
	Pattern    string // Pattern to match for middleware (e.g., "/api/*")
	Middleware func(http.Handler) http.Handler
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
func NewLuaRouteRegistry(router *chi.Mux) *LuaRouteRegistry {
	return &LuaRouteRegistry{
		router:      router,
		routeGroups: make(map[string]*chi.Mux),
	}
}

// RegisterRoute registers a single route from a Lua script
func (r *LuaRouteRegistry) RegisterRoute(def RouteDefinition) error {
	// Get or create tenant submux
	submux := r.getTenantSubmux(def.TenantName)

	// Register the route with the appropriate method
	switch def.Method {
	case "GET":
		submux.Get(def.Pattern, def.Handler)
	case "POST":
		submux.Post(def.Pattern, def.Handler)
	case "PUT":
		submux.Put(def.Pattern, def.Handler)
	case "DELETE":
		submux.Delete(def.Pattern, def.Handler)
	case "PATCH":
		submux.Patch(def.Pattern, def.Handler)
	case "OPTIONS":
		submux.Options(def.Pattern, def.Handler)
	case "HEAD":
		submux.Head(def.Pattern, def.Handler)
	default:
		submux.Method(def.Method, def.Pattern, def.Handler)
	}

	return nil
}

// RegisterMiddleware registers middleware for a pattern from a Lua script
func (r *LuaRouteRegistry) RegisterMiddleware(def MiddlewareDefinition) error {
	submux := r.getTenantSubmux(def.TenantName)

	// Apply middleware to the pattern
	submux.Group(func(gr chi.Router) {
		gr.Use(def.Middleware)
		// The actual routes will be registered later
	})

	return nil
}

// RegisterRouteGroup registers a group of routes with shared middleware
func (r *LuaRouteRegistry) RegisterRouteGroup(def RouteGroupDefinition) error {
	submux := r.getTenantSubmux(def.TenantName)

	// Create route group with pattern and middleware
	submux.Route(def.Pattern, func(gr chi.Router) {
		// Apply group middleware
		for _, mw := range def.Middleware {
			gr.Use(mw)
		}

		// Register routes in the group
		for _, route := range def.Routes {
			switch route.Method {
			case "GET":
				gr.Get(route.Pattern, route.Handler)
			case "POST":
				gr.Post(route.Pattern, route.Handler)
			case "PUT":
				gr.Put(route.Pattern, route.Handler)
			case "DELETE":
				gr.Delete(route.Pattern, route.Handler)
			case "PATCH":
				gr.Patch(route.Pattern, route.Handler)
			case "OPTIONS":
				gr.Options(route.Pattern, route.Handler)
			case "HEAD":
				gr.Head(route.Pattern, route.Handler)
			default:
				gr.Method(route.Method, route.Pattern, route.Handler)
			}
		}

		// Register subgroups recursively
		for _, subgroup := range def.Subgroups {
			r.registerSubgroup(gr, subgroup)
		}
	})

	return nil
}

// MountTenantRoutes mounts all routes for a tenant under a specific path
func (r *LuaRouteRegistry) MountTenantRoutes(tenantName, mountPath string) error {
	if submux, exists := r.routeGroups[tenantName]; exists {
		r.router.Mount(mountPath, submux)
	}
	return nil
}

// ClearTenantRoutes removes all routes for a specific tenant
func (r *LuaRouteRegistry) ClearTenantRoutes(tenantName string) {
	delete(r.routeGroups, tenantName)
}

// GetTenantRoutes returns the submux for a tenant (for inspection/debugging)
func (r *LuaRouteRegistry) GetTenantRoutes(tenantName string) *chi.Mux {
	return r.routeGroups[tenantName]
}

// ListTenants returns all tenants that have registered routes
func (r *LuaRouteRegistry) ListTenants() []string {
	tenants := make([]string, 0, len(r.routeGroups))
	for tenant := range r.routeGroups {
		tenants = append(tenants, tenant)
	}
	return tenants
}

// getTenantSubmux gets or creates a submux for a tenant
func (r *LuaRouteRegistry) getTenantSubmux(tenantName string) *chi.Mux {
	if submux, exists := r.routeGroups[tenantName]; exists {
		return submux
	}

	// Create new submux for tenant
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
			switch route.Method {
			case "GET":
				gr.Get(route.Pattern, route.Handler)
			case "POST":
				gr.Post(route.Pattern, route.Handler)
			case "PUT":
				gr.Put(route.Pattern, route.Handler)
			case "DELETE":
				gr.Delete(route.Pattern, route.Handler)
			case "PATCH":
				gr.Patch(route.Pattern, route.Handler)
			case "OPTIONS":
				gr.Options(route.Pattern, route.Handler)
			case "HEAD":
				gr.Head(route.Pattern, route.Handler)
			default:
				gr.Method(route.Method, route.Pattern, route.Handler)
			}
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
		registry: NewLuaRouteRegistry(router),
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
	return api.registry.MountTenantRoutes(tenantName, mountPath)
}

// Clear removes all routes for a tenant
func (api *RouteRegistryAPI) Clear(tenantName string) {
	api.registry.ClearTenantRoutes(tenantName)
}
