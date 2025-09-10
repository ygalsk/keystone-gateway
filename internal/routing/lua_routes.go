package routing

import (
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
)

// LuaRouteRegistry manages dynamic route registration from Lua scripts
type LuaRouteRegistry struct {
	router *chi.Mux
	subMux map[string]*chi.Mux // tenant -> mux
	mu     sync.RWMutex
}

// NewLuaRouteRegistry creates a new registry
func NewLuaRouteRegistry(router *chi.Mux) *LuaRouteRegistry {
	return &LuaRouteRegistry{
		router: router,
		subMux: make(map[string]*chi.Mux),
	}
}

// getTenantMux returns the tenant's submux, creating if necessary
func (r *LuaRouteRegistry) getTenantMux(tenant string) *chi.Mux {
	r.mu.RLock()
	m, ok := r.subMux[tenant]
	r.mu.RUnlock()
	if ok {
		return m
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if m, ok := r.subMux[tenant]; ok {
		return m
	}

	m = chi.NewMux()
	r.subMux[tenant] = m
	return m
}

func (r *LuaRouteRegistry) RegisterRoute(
	tenant, method, pattern string, handler http.HandlerFunc,
	middleware ...func(http.Handler) http.Handler,
) {
	mux := r.getTenantMux(tenant)

	// Apply middleware
	finalHandler := http.Handler(handler)
	for i := len(middleware) - 1; i >= 0; i-- {
		finalHandler = middleware[i](finalHandler)
	}

	// Use Chi Method() for any HTTP verb
	mux.Method(method, pattern, finalHandler)
}

// RegisterGroup creates a route group with middleware
func (r *LuaRouteRegistry) RegisterGroup(tenant, prefix string, middleware []func(http.Handler) http.Handler, setupFunc func(chi.Router)) {
	mux := r.getTenantMux(tenant)
	mux.Route(prefix, func(gr chi.Router) {
		gr.Use(middleware...)
		setupFunc(gr)
	})
}

// MountTenant mounts a tenant's routes to main router
func (r *LuaRouteRegistry) MountTenant(tenant, basePath string) {
	r.mu.RLock()
	mux, ok := r.subMux[tenant]
	r.mu.RUnlock()
	if !ok {
		return
	}
	r.router.Mount(basePath, mux)
}

// ClearTenant removes all routes for a tenant
func (r *LuaRouteRegistry) ClearTenant(tenant string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.subMux, tenant)
}
