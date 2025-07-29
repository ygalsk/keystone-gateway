// Package routing provides routing logic for Keystone Gateway.
// It handles tenant-based routing, load balancing, and backend selection.
package routing

import (
	"context"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"keystone-gateway/internal/config"

	"github.com/go-chi/chi/v5"
)

// GatewayBackend represents a proxied backend server with health status tracking.
type GatewayBackend struct {
	URL        *url.URL
	Alive      atomic.Bool
	HealthPath string // Health check endpoint path

	// ✅ ADD: Cache proxies per strip prefix to avoid recreation
	proxyCache sync.Map // map[string]*httputil.ReverseProxy
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

	// Shared HTTP transport for connection pooling
	transport *http.Transport

	// Health check management
	healthCtx    context.Context
	healthCancel context.CancelFunc
	healthWG     sync.WaitGroup
}

// NewGatewayWithRouter creates a Gateway with an existing Chi router for dynamic routing
func NewGatewayWithRouter(cfg *config.Config, router *chi.Mux) *Gateway {
	healthCtx, healthCancel := context.WithCancel(context.Background())

	gw := &Gateway{
		config:        cfg,
		pathRouters:   make(map[string]*TenantRouter),
		hostRouters:   make(map[string]*TenantRouter),
		hybridRouters: make(map[string]map[string]*TenantRouter),
		startTime:     time.Now(),
		routeRegistry: NewLuaRouteRegistry(router, nil),
		healthCtx:     healthCtx,
		healthCancel:  healthCancel,

		// Configure optimized HTTP transport
		transport: &http.Transport{
			MaxIdleConns:        100,               // Total idle connections across all hosts
			MaxIdleConnsPerHost: 50,                // Idle connections per backend host (increased for better performance)
			IdleConnTimeout:     120 * time.Second, // How long to keep idle connections (extended for better reuse)
			DisableKeepAlives:   false,             // Enable HTTP keep-alive
			ForceAttemptHTTP2:   true,              // Enable HTTP/2 for multiplexing and improved performance

			// Connection timeouts
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,

			// Prevent connection leaks
			MaxConnsPerHost:       100, // Max total connections per host (increased for high-traffic scenarios)
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	gw.initializeRouters()
	gw.StartHealthChecks()
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

			// Validate that the URL has the required components for a backend
			if u.Scheme == "" || u.Host == "" {
				log.Printf("Warning: invalid backend URL for service %s: missing scheme or host", svc.Name)
				continue
			}

			backend := &GatewayBackend{
				URL:        u,
				HealthPath: svc.Health,
			}
			backend.Alive.Store(false) // Start as unhealthy, will be updated by health checks
			tr.Backends = append(tr.Backends, backend)
		}

		// Route based on configuration
		gw.registerTenantRoutes(tenant, tr)

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
	// Reject paths with null bytes
	for _, char := range path {
		if char == 0 {
			return nil, ""
		}
	}

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

// CreateProxy creates or retrieves a cached reverse proxy for the given backend
// ✅ FIXED: Now caches proxy objects to eliminate per-request allocation
func (gw *Gateway) CreateProxy(backend *GatewayBackend, stripPrefix string) *httputil.ReverseProxy {
	// Use stripPrefix as cache key since proxy behavior depends on it
	cacheKey := stripPrefix

	// Try to get existing proxy from cache
	if cached, ok := backend.proxyCache.Load(cacheKey); ok {
		return cached.(*httputil.ReverseProxy)
	}

	// Create new proxy if not cached
	proxy := httputil.NewSingleHostReverseProxy(backend.URL)

	// Use the shared transport with connection pooling
	proxy.Transport = gw.transport

	// Create director function (this closure is only created once per proxy)
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

	// Cache the proxy for future use
	backend.proxyCache.Store(cacheKey, proxy)

	return proxy
}

// Optional method to get transport stats for monitoring
func (gw *Gateway) GetTransportStats() map[string]interface{} {
	return map[string]interface{}{
		"max_idle_conns":          gw.transport.MaxIdleConns,
		"max_idle_conns_per_host": gw.transport.MaxIdleConnsPerHost,
		"max_conns_per_host":      gw.transport.MaxConnsPerHost,
		"idle_conn_timeout":       gw.transport.IdleConnTimeout.String(),
		"force_attempt_http2":     gw.transport.ForceAttemptHTTP2,
	}
}

// ✅ ADD: Method to get proxy cache stats for monitoring
func (gw *Gateway) GetProxyCacheStats() map[string]int {
	stats := make(map[string]int)
	totalCached := 0

	// Count cached proxies across all tenants and backends
	for _, tr := range gw.pathRouters {
		for _, backend := range tr.Backends {
			count := 0
			backend.proxyCache.Range(func(key, value interface{}) bool {
				count++
				return true
			})
			if count > 0 {
				stats[backend.URL.String()] = count
				totalCached += count
			}
		}
	}

	for _, tr := range gw.hostRouters {
		for _, backend := range tr.Backends {
			count := 0
			backend.proxyCache.Range(func(key, value interface{}) bool {
				count++
				return true
			})
			if count > 0 {
				stats[backend.URL.String()] = count
				totalCached += count
			}
		}
	}

	for _, hostMap := range gw.hybridRouters {
		for _, tr := range hostMap {
			for _, backend := range tr.Backends {
				count := 0
				backend.proxyCache.Range(func(key, value interface{}) bool {
					count++
					return true
				})
				if count > 0 {
					stats[backend.URL.String()] = count
					totalCached += count
				}
			}
		}
	}

	stats["total_cached_proxies"] = totalCached
	return stats
}

// StartHealthChecks starts background health check monitoring for all backends
func (gw *Gateway) StartHealthChecks() {
	for _, tenant := range gw.config.Tenants {
		tr := gw.GetTenantRouter(tenant.Name)
		if tr == nil {
			continue
		}

		// Default health check interval to 30 seconds if not configured
		interval := 30 * time.Second
		if tenant.Interval > 0 {
			interval = time.Duration(tenant.Interval) * time.Second
		}

		// Start health checker for each backend in this tenant
		for _, backend := range tr.Backends {
			if backend.HealthPath == "" {
				log.Printf("Warning: no health path configured for backend %s, skipping health checks", backend.URL.String())
				// If no health path, assume backend is healthy
				backend.Alive.Store(true)
				continue
			}

			gw.healthWG.Add(1)
			go gw.healthCheckWorker(backend, interval)
		}

		log.Printf("Started health checks for tenant %s with %d backends (interval: %v)", tenant.Name, len(tr.Backends), interval)
	}
}

// healthCheckWorker runs periodic health checks for a single backend
func (gw *Gateway) healthCheckWorker(backend *GatewayBackend, interval time.Duration) {
	defer gw.healthWG.Done()

	// Perform initial health check
	gw.performHealthCheck(backend)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-gw.healthCtx.Done():
			log.Printf("Health check worker for %s stopping", backend.URL.String())
			return
		case <-ticker.C:
			gw.performHealthCheck(backend)
		}
	}
}

// performHealthCheck performs a single health check against a backend
func (gw *Gateway) performHealthCheck(backend *GatewayBackend) {
	// Build health check URL
	healthURL := &url.URL{
		Scheme: backend.URL.Scheme,
		Host:   backend.URL.Host,
		Path:   backend.HealthPath,
	}

	// Create health check request with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", healthURL.String(), nil)
	if err != nil {
		log.Printf("Health check error for %s: failed to create request: %v", backend.URL.String(), err)
		gw.markBackendHealth(backend, false)
		return
	}

	// Use a dedicated HTTP client for health checks to avoid interference with proxy traffic
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:          10,
			MaxIdleConnsPerHost:   2,
			IdleConnTimeout:       30 * time.Second,
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: 5 * time.Second,
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Health check failed for %s: %v", backend.URL.String(), err)
		gw.markBackendHealth(backend, false)
		return
	}
	defer resp.Body.Close()

	// Consider 2xx status codes as healthy
	healthy := resp.StatusCode >= 200 && resp.StatusCode < 300
	gw.markBackendHealth(backend, healthy)

	if healthy {
		log.Printf("Health check passed for %s (status: %d)", backend.URL.String(), resp.StatusCode)
	} else {
		log.Printf("Health check failed for %s (status: %d)", backend.URL.String(), resp.StatusCode)
	}
}

// markBackendHealth updates the health status of a backend with logging
func (gw *Gateway) markBackendHealth(backend *GatewayBackend, healthy bool) {
	wasAlive := backend.Alive.Load()
	backend.Alive.Store(healthy)

	// Log status changes
	if wasAlive != healthy {
		if healthy {
			log.Printf("Backend %s is now HEALTHY", backend.URL.String())
		} else {
			log.Printf("Backend %s is now UNHEALTHY", backend.URL.String())
		}
	}
}

// StopHealthChecks gracefully stops all health check workers
func (gw *Gateway) StopHealthChecks() {
	log.Println("Stopping health check workers...")
	gw.healthCancel()
	gw.healthWG.Wait()
	log.Println("All health check workers stopped")
}
