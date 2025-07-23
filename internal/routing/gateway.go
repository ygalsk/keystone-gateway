// Package routing provides routing logic for Keystone Gateway.
// It handles tenant-based routing, load balancing, and backend selection.
package routing

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"keystone-gateway/internal/config"

	"github.com/go-chi/chi/v5"
)

// GatewayBackend represents a proxied backend server with health status tracking.
type GatewayBackend struct {
	URL   *url.URL
	Alive atomic.Bool
}

// TenantRouter manages load balancing and backend selection for a specific tenant.
type TenantRouter struct {
	Name     string
	Backends []*GatewayBackend
	RRIndex  uint64
}

// Gateway is the main reverse proxy instance that handles routing,
// load balancing, and health checking for all configured tenants.
type Gateway struct {
	config        *config.Config
	pathRouters   map[string]*TenantRouter
	hostRouters   map[string]*TenantRouter
	hybridRouters map[string]map[string]*TenantRouter
	startTime     time.Time

	// New: Dynamic route registry for Lua-defined routes
	routeRegistry *LuaRouteRegistry
}

// NewGatewayWithRouter creates a Gateway with an existing Chi router for dynamic routing
func NewGatewayWithRouter(cfg *config.Config, router *chi.Mux) *Gateway {
	gw := &Gateway{
		config:        cfg,
		pathRouters:   make(map[string]*TenantRouter),
		hostRouters:   make(map[string]*TenantRouter),
		hybridRouters: make(map[string]map[string]*TenantRouter),
		startTime:     time.Now(),
		routeRegistry: NewLuaRouteRegistry(router, nil),
	}

	gw.initializeRouters()
	return gw
}

// initializeRouters sets up all tenant routers from the configuration.
func (gw *Gateway) initializeRouters() {
	for _, tenant := range gw.config.Tenants {
		tr := &TenantRouter{
			Name:     tenant.Name,
			Backends: make([]*GatewayBackend, 0, len(tenant.Services)),
		}

		// Initialize backends
		for _, svc := range tenant.Services {
			u, err := url.Parse(svc.URL)
			if err != nil {
				log.Printf("Warning: invalid URL for service %s: %v", svc.Name, err)
				continue
			}

			backend := &GatewayBackend{URL: u}
			backend.Alive.Store(false) // Start as unhealthy
			tr.Backends = append(tr.Backends, backend)
		}

		// Route based on configuration
		gw.registerTenantRoutes(tenant, tr)

		// TODO: Start health checks (will be moved to health package)

		log.Printf("Initialized tenant %s with %d backends", tenant.Name, len(tr.Backends))
	}
}

// registerTenantRoutes registers tenant routes based on the configuration.
func (gw *Gateway) registerTenantRoutes(tenant config.Tenant, tr *TenantRouter) {
	if len(tenant.Domains) > 0 && tenant.PathPrefix != "" {
		// Hybrid routing
		for _, domain := range tenant.Domains {
			if gw.hybridRouters[domain] == nil {
				gw.hybridRouters[domain] = make(map[string]*TenantRouter)
			}
			gw.hybridRouters[domain][tenant.PathPrefix] = tr
		}
	} else if len(tenant.Domains) > 0 {
		// Host-only routing
		for _, domain := range tenant.Domains {
			gw.hostRouters[domain] = tr
		}
	} else if tenant.PathPrefix != "" {
		// Path-only routing
		gw.pathRouters[tenant.PathPrefix] = tr
	}
}

// MatchRoute finds the appropriate tenant router for a given host and path.
func (gw *Gateway) MatchRoute(host, path string) (*TenantRouter, string) {
	host = ExtractHost(host)

	// Priority 1: Hybrid routing (host + path)
	if hostMap, exists := gw.hybridRouters[host]; exists {
		if matched, prefix := gw.findBestPathMatch(path, hostMap); matched != nil {
			return matched, prefix
		}
	}

	// Priority 2: Host-only routing
	if router, exists := gw.hostRouters[host]; exists {
		return router, ""
	}

	// Priority 3: Path-only routing
	return gw.findBestPathMatch(path, gw.pathRouters)
}

// NextBackend returns the next healthy backend using round-robin algorithm.
func (tr *TenantRouter) NextBackend() *GatewayBackend {
	if len(tr.Backends) == 0 {
		return nil
	}

	// Round-robin with health checks
	for i := 0; i < len(tr.Backends); i++ {
		idx := int(atomic.AddUint64(&tr.RRIndex, 1) % uint64(len(tr.Backends)))
		backend := tr.Backends[idx]

		if backend.Alive.Load() {
			return backend
		}
	}

	// Fallback to first backend even if unhealthy
	return tr.Backends[0]
}

// GetTenantRouter finds a tenant router by name.
func (gw *Gateway) GetTenantRouter(name string) *TenantRouter {
	for _, tr := range gw.pathRouters {
		if tr.Name == name {
			return tr
		}
	}
	for _, tr := range gw.hostRouters {
		if tr.Name == name {
			return tr
		}
	}
	for _, hostMap := range gw.hybridRouters {
		for _, tr := range hostMap {
			if tr.Name == name {
				return tr
			}
		}
	}
	return nil
}

// GetConfig returns the gateway configuration.
func (gw *Gateway) GetConfig() *config.Config {
	return gw.config
}

// GetStartTime returns when the gateway was started.
func (gw *Gateway) GetStartTime() time.Time {
	return gw.startTime
}

// GetRouteRegistry returns the dynamic route registry
func (gw *Gateway) GetRouteRegistry() *LuaRouteRegistry {
	return gw.routeRegistry
}

// extractHost extracts the hostname from a host header (removing port if present).
func ExtractHost(hostHeader string) string {
	// Handle IPv6 addresses wrapped in brackets: [::1]:8080 -> [::1]
	if strings.HasPrefix(hostHeader, "[") {
		if closeBracket := strings.Index(hostHeader, "]"); closeBracket != -1 {
			return hostHeader[:closeBracket+1]
		}
	}
	
	// Handle IPv4 addresses or hostnames: example.com:8080 -> example.com  
	if colonIndex := strings.Index(hostHeader, ":"); colonIndex != -1 {
		return hostHeader[:colonIndex]
	}
	return hostHeader
}

// findBestPathMatch finds the best matching path prefix from a router map
func (gw *Gateway) findBestPathMatch(path string, routers map[string]*TenantRouter) (*TenantRouter, string) {
	var matched *TenantRouter
	var matchedPrefix string

	for prefix, router := range routers {
		if strings.HasPrefix(path, prefix) && len(prefix) > len(matchedPrefix) {
			matched = router
			matchedPrefix = prefix
		}
	}

	return matched, matchedPrefix
}

// CreateProxy creates a reverse proxy for the given backend
func (gw *Gateway) CreateProxy(backend *GatewayBackend, stripPrefix string) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(backend.URL)

	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = backend.URL.Scheme
		req.URL.Host = backend.URL.Host

		// Handle path stripping and backend path prepending
		if stripPrefix != "" {
			newPath := strings.TrimPrefix(req.URL.Path, stripPrefix)
			if newPath == "" {
				newPath = "/"
			} else if !strings.HasPrefix(newPath, "/") {
				newPath = "/" + newPath
			}
			req.URL.Path = newPath
		}

		// Prepend backend URL path if it exists
		if backend.URL.Path != "" && backend.URL.Path != "/" {
			backendPath := strings.TrimSuffix(backend.URL.Path, "/")
			if req.URL.Path == "/" {
				req.URL.Path = backendPath + "/"
			} else {
				req.URL.Path = backendPath + req.URL.Path
			}
		}

		// Merge query parameters
		if backend.URL.RawQuery == "" || req.URL.RawQuery == "" {
			req.URL.RawQuery = backend.URL.RawQuery + req.URL.RawQuery
		} else {
			req.URL.RawQuery = backend.URL.RawQuery + "&" + req.URL.RawQuery
		}
	}

	return proxy
}
