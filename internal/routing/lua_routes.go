package routing

import (
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
)

// LuaRouteRegistry manages dynamic route registration from Lua scripts
type LuaRouteRegistry struct {
	router *chi.Mux
	mu     sync.RWMutex
}

// NewLuaRouteRegistry creates a new registry
func NewLuaRouteRegistry(router *chi.Mux) *LuaRouteRegistry {
	return &LuaRouteRegistry{
		router: router,
	}
}

func (r *LuaRouteRegistry) RegisterRoute(
	tenant, method, pattern string, handler http.HandlerFunc,
	middleware ...func(http.Handler) http.Handler,
) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Apply middleware
	finalHandler := http.Handler(handler)
	for i := len(middleware) - 1; i >= 0; i-- {
		finalHandler = middleware[i](finalHandler)
	}

	// Use Chi Method() directly on the main router
	r.router.Method(method, pattern, finalHandler)
}

// RegisterGroup creates a route group with middleware
func (r *LuaRouteRegistry) RegisterGroup(tenant, prefix string, middleware []func(http.Handler) http.Handler, setupFunc func(chi.Router)) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.router.Route(prefix, func(gr chi.Router) {
		gr.Use(middleware...)
		setupFunc(gr)
	})
}
