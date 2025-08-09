// Package routing provides routing logic for Keystone Gateway.
// It handles tenant-based routing, load balancing, and backend selection.
package routing

import (
	"context"
	"log/slog"
	"net"
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

	// Circuit breaker state (unexported)
	cbState           atomic.Uint32 // 0=closed,1=open,2=half-open
	cbFailures        atomic.Uint32 // consecutive failures
	cbLastFailureUnix atomic.Int64  // unix nano timestamp
	cbHalfOpenRemains atomic.Int32  // remaining probes in half-open
}

// TenantRouter manages load balancing and backend selection for a specific tenant.
type TenantRouter struct {
	Name     string
	Backends []*GatewayBackend
	RRIndex  int64 // Changed to int64 to avoid overflow issues
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

// Constants used for proxy header names and messages to avoid magic strings
const (
	xForwardedForHeader   = "X-Forwarded-For"
	xForwardedHostHeader  = "X-Forwarded-Host"
	xForwardedProtoHeader = "X-Forwarded-Proto"
	defaultBadGatewayMsg  = "Bad gateway"

	// Circuit breaker constants
	cbStateClosed        = uint32(0)
	cbStateOpen          = uint32(1)
	cbStateHalfOpen      = uint32(2)
	cbFailureThreshold   = uint32(5)        // N consecutive failures to open
	cbCooldownDuration   = 30 * time.Second // Open -> HalfOpen after cooldown
	cbHalfOpenMaxProbes  = int32(1)         // Allowed test requests in half-open
	serverErrorStatusMin = 500              // Treat >=500 as failure
)

// Human-readable names for breaker states (for logs)
var cbStateNames = map[uint32]string{
	cbStateClosed:   "CLOSED",
	cbStateOpen:     "OPEN",
	cbStateHalfOpen: "HALF-OPEN",
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
				slog.Warn("invalid_service_url",
					"service", svc.Name,
					"url", svc.URL,
					"error", err,
					"component", "gateway")
				continue
			}

			// Validate that the URL has the required components for a backend
			if u.Scheme == "" || u.Host == "" {
				slog.Warn("invalid_backend_url",
					"service", svc.Name,
					"url", svc.URL,
					"reason", "missing scheme or host",
					"component", "gateway")
				continue
			}

			backend := &GatewayBackend{
				URL:        u,
				HealthPath: svc.Health,
			}
			backend.Alive.Store(false) // Start as unhealthy, will be updated by health checks
			backend.cbState.Store(cbStateClosed)
			backend.cbHalfOpenRemains.Store(cbHalfOpenMaxProbes)
			tr.Backends = append(tr.Backends, backend)
		}

		// Route based on configuration
		gw.registerTenantRoutes(tenant, tr)

		slog.Info("tenant_initialized",
			"tenant", tenant.Name,
			"backend_count", len(tr.Backends),
			"component", "gateway")
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

	// Round-robin with health checks and circuit breaker allowance
	backendCount := len(tr.Backends)
	for i := 0; i < backendCount; i++ {
		// Safe round-robin index calculation using int64 atomic operations
		rrValue := atomic.AddInt64(&tr.RRIndex, 1)
		// Safe modulo operation - no conversion needed
		idx := int(rrValue % int64(backendCount))
		if idx < 0 {
			idx = -idx // Handle negative modulo results
		}
		backend := tr.Backends[idx]

		if !backend.Alive.Load() {
			continue
		}

		// Circuit breaker gate
		state := backend.cbState.Load()
		allowed := false
		switch state {
		case cbStateClosed:
			allowed = true
		case cbStateOpen:
			// Cooldown check -> half-open
			last := time.Unix(0, backend.cbLastFailureUnix.Load())
			if time.Since(last) >= cbCooldownDuration {
				backend.cbState.Store(cbStateHalfOpen)
				backend.cbHalfOpenRemains.Store(cbHalfOpenMaxProbes)
				slog.Info("circuit_breaker_state_change", "backend", backend.URL.String(), "from_state", cbStateNames[state], "to_state", cbStateNames[cbStateHalfOpen], "component", "circuit_breaker")
				allowed = true
			}
		case cbStateHalfOpen:
			// Allow limited probes
			if backend.cbHalfOpenRemains.Add(-1) >= 0 {
				allowed = true
			}
		}

		if allowed {
			return backend
		}
	}

	// Fallbacks to preserve availability while preferring safer choices
	// 1) If any backend is marked Alive, return the first Alive (ignoring breaker gate)
	for _, b := range tr.Backends {
		if b.Alive.Load() {
			return b
		}
	}
	// 2) If any backend is not OPEN, prefer it to avoid known-bad backends
	for _, b := range tr.Backends {
		if b.cbState.Load() != cbStateOpen {
			return b
		}
	}
	// 3) Last resort: original behavior returns the first backend
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

// ExtractHost extracts the hostname from a host header (removing port if present).
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
func (gw *Gateway) CreateProxy(backend *GatewayBackend, stripPrefix string) *httputil.ReverseProxy {
	// Always build a fresh proxy to avoid any unintended state reuse across requests/tests
	return gw.buildReverseProxy(backend, stripPrefix)
}

// buildReverseProxy constructs a new reverse proxy with proper configuration
func (gw *Gateway) buildReverseProxy(backend *GatewayBackend, stripPrefix string) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(backend.URL)
	proxy.Transport = gw.transport
	proxy.Director = gw.createDirectorFunction(backend, stripPrefix)

	// Robust error handler to keep gateway resilient
	proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
		// Mark as failure for circuit breaker
		backend.cbLastFailureUnix.Store(time.Now().UnixNano())
		backend.cbFailures.Add(1)
		old := backend.cbState.Load()
		newState := old
		if old == cbStateHalfOpen {
			newState = cbStateOpen
		} else if backend.cbFailures.Load() >= cbFailureThreshold {
			newState = cbStateOpen
		}
		if newState != old {
			backend.cbState.Store(newState)
			slog.Info("circuit_breaker_state_change", "backend", backend.URL.String(), "from_state", cbStateNames[old], "to_state", cbStateNames[newState], "component", "circuit_breaker")
		}
		slog.Error("proxy_error",
			"backend", backend.URL.String(),
			"error", err,
			"component", "proxy")
		http.Error(rw, defaultBadGatewayMsg, http.StatusBadGateway)
	}

	// Observe responses to update breaker state
	proxy.ModifyResponse = func(resp *http.Response) error {
		if resp.StatusCode >= serverErrorStatusMin {
			backend.cbLastFailureUnix.Store(time.Now().UnixNano())
			backend.cbFailures.Add(1)
			old := backend.cbState.Load()
			newState := old
			if old == cbStateHalfOpen {
				newState = cbStateOpen
			} else if backend.cbFailures.Load() >= cbFailureThreshold {
				newState = cbStateOpen
			}
			if newState != old {
				backend.cbState.Store(newState)
				slog.Info("circuit_breaker_state_change", "backend", backend.URL.String(), "from_state", cbStateNames[old], "to_state", cbStateNames[newState], "component", "circuit_breaker")
			}
		} else {
			// success -> close/reset breaker
			old := backend.cbState.Load()
			backend.cbFailures.Store(0)
			backend.cbHalfOpenRemains.Store(cbHalfOpenMaxProbes)
			if old != cbStateClosed {
				backend.cbState.Store(cbStateClosed)
				slog.Info("circuit_breaker_state_change", "backend", backend.URL.String(), "from_state", cbStateNames[old], "to_state", cbStateNames[cbStateClosed], "component", "circuit_breaker")
			}
		}
		return nil
	}

	return proxy
}

// createDirectorFunction creates the director function for request modification
func (gw *Gateway) createDirectorFunction(backend *GatewayBackend, stripPrefix string) func(*http.Request) {
	return func(req *http.Request) {
		// Capture original client-facing values for forwarding headers
		originalHost := req.Host
		if originalHost == "" {
			// Fallback to URL host if Host header was empty
			originalHost = req.URL.Host
		}
		originalProto := "http"
		if req.TLS != nil || strings.EqualFold(req.Header.Get(xForwardedProtoHeader), "https") {
			originalProto = "https"
		}

		// Compute original client IP robustly (IPv4/IPv6)
		var originalFor string
		if req.RemoteAddr != "" {
			if h, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
				originalFor = h
			} else {
				// If no port present or parsing failed, use as-is
				originalFor = req.RemoteAddr
			}
		}

		// Existing target/url/path logic
		gw.setTargetURL(req, backend)
		gw.handlePathStripping(req, stripPrefix)
		gw.prependBackendPath(req, backend)
		gw.mergeQueryParams(req, backend)

		// X-Forwarded-* headers
		if originalFor != "" {
			if prior := req.Header.Get(xForwardedForHeader); prior != "" {
				req.Header.Set(xForwardedForHeader, prior+", "+originalFor)
			} else {
				req.Header.Set(xForwardedForHeader, originalFor)
			}
		}
		if originalHost != "" {
			req.Header.Set(xForwardedHostHeader, originalHost)
		}
		req.Header.Set(xForwardedProtoHeader, originalProto)

		// Upstream Host should match target backend host
		req.Host = backend.URL.Host
	}
}

// setTargetURL sets the target scheme and host
func (gw *Gateway) setTargetURL(req *http.Request, backend *GatewayBackend) {
	req.URL.Scheme = backend.URL.Scheme
	req.URL.Host = backend.URL.Host
}

// handlePathStripping handles path prefix stripping
func (gw *Gateway) handlePathStripping(req *http.Request, stripPrefix string) {
	if stripPrefix == "" {
		return
	}

	newPath := strings.TrimPrefix(req.URL.Path, stripPrefix)
	if newPath == "" {
		newPath = "/"
	} else if !strings.HasPrefix(newPath, "/") {
		newPath = "/" + newPath
	}
	req.URL.Path = newPath
}

// prependBackendPath prepends the backend URL path if it exists
func (gw *Gateway) prependBackendPath(req *http.Request, backend *GatewayBackend) {
	if backend.URL.Path == "" || backend.URL.Path == "/" {
		return
	}

	backendPath := strings.TrimSuffix(backend.URL.Path, "/")
	if req.URL.Path == "/" {
		req.URL.Path = backendPath + "/"
	} else {
		req.URL.Path = backendPath + req.URL.Path
	}
}

// mergeQueryParams merges query parameters from backend and request
func (gw *Gateway) mergeQueryParams(req *http.Request, backend *GatewayBackend) {
	if backend.URL.RawQuery == "" || req.URL.RawQuery == "" {
		req.URL.RawQuery = backend.URL.RawQuery + req.URL.RawQuery
	} else {
		req.URL.RawQuery = backend.URL.RawQuery + "&" + req.URL.RawQuery
	}
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
	// Proxy caching disabled; return zero stats for compatibility
	return map[string]int{"total_cached_proxies": 0}
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
				slog.Warn("health_check_skipped",
					"backend", backend.URL.String(),
					"reason", "no health path configured",
					"component", "health_checker")
				// If no health path, assume backend is healthy
				backend.Alive.Store(true)
				continue
			}

			gw.healthWG.Add(1)
			go gw.healthCheckWorker(backend, interval)
		}

		slog.Info("health_checks_started",
			"tenant", tenant.Name,
			"backend_count", len(tr.Backends),
			"interval", interval,
			"component", "health_checker")
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
			slog.Info("health_worker_stopping",
				"backend", backend.URL.String(),
				"component", "health_checker")
			return
		case <-ticker.C:
			gw.performHealthCheck(backend)
		}
	}
}

// performHealthCheck performs a single health check against a backend
func (gw *Gateway) performHealthCheck(backend *GatewayBackend) {
	// Build health check URL (normalize path)
	path := backend.HealthPath
	if path == "" {
		path = "/"
	} else if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	healthURL := &url.URL{
		Scheme: backend.URL.Scheme,
		Host:   backend.URL.Host,
		Path:   path,
	}

	// Create health check request with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", healthURL.String(), nil)
	if err != nil {
		slog.Error("health_check_request_failed",
			"backend", backend.URL.String(),
			"error", err,
			"component", "health_checker")
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
		slog.Error("health_check_failed",
			"backend", backend.URL.String(),
			"error", err,
			"component", "health_checker")
		gw.markBackendHealth(backend, false)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	// Consider 2xx status codes as healthy
	healthy := resp.StatusCode >= 200 && resp.StatusCode < 300
	gw.markBackendHealth(backend, healthy)

	if healthy {
		slog.Info("health_check_passed",
			"backend", backend.URL.String(),
			"status_code", resp.StatusCode,
			"component", "health_checker")
	} else {
		slog.Warn("health_check_failed",
			"backend", backend.URL.String(),
			"status_code", resp.StatusCode,
			"component", "health_checker")
	}
}

// markBackendHealth updates the health status of a backend with logging
func (gw *Gateway) markBackendHealth(backend *GatewayBackend, healthy bool) {
	wasAlive := backend.Alive.Load()
	backend.Alive.Store(healthy)

	// On healthy, close/reset breaker as well
	if healthy {
		old := backend.cbState.Load()
		backend.cbFailures.Store(0)
		backend.cbHalfOpenRemains.Store(cbHalfOpenMaxProbes)
		if old != cbStateClosed {
			backend.cbState.Store(cbStateClosed)
			slog.Info("circuit_breaker_reset",
				"backend", backend.URL.String(),
				"from_state", cbStateNames[old],
				"to_state", cbStateNames[cbStateClosed],
				"component", "circuit_breaker")
		}
	}

	// Log status changes
	if wasAlive != healthy {
		if healthy {
			slog.Info("backend_healthy",
				"backend", backend.URL.String(),
				"component", "health_checker")
		} else {
			slog.Error("backend_unhealthy",
				"backend", backend.URL.String(),
				"component", "health_checker")
		}
	}
}

// StopHealthChecks gracefully stops all health check workers
func (gw *Gateway) StopHealthChecks() {
	slog.Info("health_workers_stopping", "component", "health_checker")
	gw.healthCancel()
	gw.healthWG.Wait()
	slog.Info("health_workers_stopped", "component", "health_checker")
}
