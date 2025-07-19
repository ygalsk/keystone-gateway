// Package main implements Keystone Gateway, a lightweight reverse proxy with
// multi-tenant routing support, health checking, and load balancing capabilities.
//
// The gateway supports three routing modes:
// - Host-based routing using domain names
// - Path-based routing using URL prefixes
// - Hybrid routing combining both host and path matching
package main

// -------------------------
// 1. IMPORTS & TYPES
// -------------------------

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"gopkg.in/yaml.v3"
)

// -------------------------
// 2. CONSTANTS
// -------------------------

const (
	// Default timeouts
	DefaultHealthCheckInterval = 10 * time.Second
	DefaultHealthCheckTimeout  = 3 * time.Second
	DefaultRequestTimeout      = 60 * time.Second

	// Default server settings
	DefaultListenAddress = ":8080"
)

// -------------------------
// 3. CONFIGURATION
// -------------------------

// Config represents the main configuration structure for the gateway,
// containing tenant definitions and admin settings.
type Config struct {
	Tenants       []Tenant `yaml:"tenants"`
	AdminBasePath string   `yaml:"admin_base_path,omitempty"`
	// Optional: Base path for admin endpoints, defaults to "/"
}

// Tenant represents a routing configuration for a specific application or service,
// supporting host-based, path-based, or hybrid routing strategies.
type Tenant struct {
	Name       string    `yaml:"name"`
	PathPrefix string    `yaml:"path_prefix,omitempty"`
	Domains    []string  `yaml:"domains,omitempty"`
	Interval   int       `yaml:"health_interval"`
	Services   []Service `yaml:"services"`
}

// Service represents a backend service endpoint with health check configuration.
type Service struct {
	Name   string `yaml:"name"`
	URL    string `yaml:"url"`
	Health string `yaml:"health"`
}

// -------------------------
// 4. CORE TYPES
// -------------------------

// Backend represents a proxied backend server with health status tracking.
type Backend struct {
	URL   *url.URL
	Alive atomic.Bool
}

// TenantRouter manages load balancing and backend selection for a specific tenant.
type TenantRouter struct {
	Name     string
	Backends []*Backend
	RRIndex  uint64
}

// Gateway is the main reverse proxy instance that handles routing,
// load balancing, and health checking for all configured tenants.
type Gateway struct {
	config        *Config
	pathRouters   map[string]*TenantRouter
	hostRouters   map[string]*TenantRouter
	hybridRouters map[string]map[string]*TenantRouter
	startTime     time.Time
}

// -------------------------
// 5. HEALTH STATUS
// -------------------------

// HealthStatus represents the current health status of the gateway and all tenants.
type HealthStatus struct {
	Status  string            `json:"status"`
	Tenants map[string]string `json:"tenants"`
	Uptime  string            `json:"uptime"`
	Version string            `json:"version"`
}

// -------------------------
// 6. CONFIGURATION MANAGEMENT
// -------------------------

// LoadConfig reads and parses a YAML configuration file, returning a validated Config instance.
// Returns an error if the file cannot be read, parsed, or contains invalid tenant configurations.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	for _, tenant := range cfg.Tenants {
		if err := validateTenant(tenant); err != nil {
			return nil, fmt.Errorf("invalid tenant %s: %w", tenant.Name, err)
		}
	}

	return &cfg, nil
}

func validateTenant(t Tenant) error {
	if len(t.Domains) == 0 && t.PathPrefix == "" {
		return fmt.Errorf("must specify either domains or path_prefix")
	}

	for _, domain := range t.Domains {
		if !isValidDomain(domain) {
			return fmt.Errorf("invalid domain: %s", domain)
		}
	}

	if t.PathPrefix != "" {
		if !strings.HasPrefix(t.PathPrefix, "/") || !strings.HasSuffix(t.PathPrefix, "/") {
			return fmt.Errorf("path_prefix must start and end with '/'")
		}
	}

	return nil
}

func isValidDomain(domain string) bool {
	return domain != "" && !strings.Contains(domain, " ") && strings.Contains(domain, ".")
}

// -------------------------
// 7. GATEWAY CORE
// -------------------------

// NewGateway creates a new Gateway instance from the provided configuration,
// initializing all tenant routers and starting health check goroutines.
func NewGateway(cfg *Config) *Gateway {
	gw := &Gateway{
		config:        cfg,
		pathRouters:   make(map[string]*TenantRouter),
		hostRouters:   make(map[string]*TenantRouter),
		hybridRouters: make(map[string]map[string]*TenantRouter),
		startTime:     time.Now(),
	}

	gw.initializeRouters()
	return gw
}

func (gw *Gateway) initializeRouters() {
	for _, tenant := range gw.config.Tenants {
		tr := &TenantRouter{
			Name:     tenant.Name,
			Backends: make([]*Backend, 0, len(tenant.Services)),
		}

		// Initialize backends
		for _, svc := range tenant.Services {
			u, err := url.Parse(svc.URL)
			if err != nil {
				log.Printf("Warning: invalid URL for service %s: %v", svc.Name, err)
				continue
			}

			backend := &Backend{URL: u}
			backend.Alive.Store(false) // Start as unhealthy
			tr.Backends = append(tr.Backends, backend)
		}

		// Route based on configuration
		gw.registerTenantRoutes(tenant, tr)

		// Start health checks
		go gw.startHealthChecks(tenant, tr)

		log.Printf("Initialized tenant %s with %d backends", tenant.Name, len(tr.Backends))
	}
}

func (gw *Gateway) registerTenantRoutes(tenant Tenant, tr *TenantRouter) {
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

// -------------------------
// 8. ROUTING LOGIC
// -------------------------

func (gw *Gateway) MatchRoute(host, path string) (*TenantRouter, string) {
	host = extractHost(host)

	// Priority 1: Hybrid routing (host + path)
	if hostMap, exists := gw.hybridRouters[host]; exists {
		var matched *TenantRouter
		var matchedPrefix string

		for prefix, router := range hostMap {
			if strings.HasPrefix(path, prefix) && len(prefix) > len(matchedPrefix) {
				matched = router
				matchedPrefix = prefix
			}
		}

		if matched != nil {
			return matched, matchedPrefix
		}
	}

	// Priority 2: Host-only routing
	if router, exists := gw.hostRouters[host]; exists {
		return router, ""
	}

	// Priority 3: Path-only routing
	var matched *TenantRouter
	var matchedPrefix string

	for prefix, router := range gw.pathRouters {
		if strings.HasPrefix(path, prefix) && len(prefix) > len(matchedPrefix) {
			matched = router
			matchedPrefix = prefix
		}
	}

	return matched, matchedPrefix
}

func (tr *TenantRouter) NextBackend() *Backend {
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

func extractHost(hostHeader string) string {
	if colonIndex := strings.Index(hostHeader, ":"); colonIndex != -1 {
		return hostHeader[:colonIndex]
	}
	return hostHeader
}

// -------------------------
// 9. HEALTH CHECKS
// -------------------------

func (gw *Gateway) startHealthChecks(tenant Tenant, tr *TenantRouter) {
	interval := time.Duration(tenant.Interval) * time.Second
	if interval == 0 {
		interval = DefaultHealthCheckInterval
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		for i, svc := range tenant.Services {
			if i >= len(tr.Backends) {
				break
			}

			backend := tr.Backends[i]
			healthy := gw.checkBackendHealth(svc)
			backend.Alive.Store(healthy)
		}

		<-ticker.C
	}
}

func (gw *Gateway) checkBackendHealth(svc Service) bool {
	client := &http.Client{Timeout: DefaultHealthCheckTimeout}

	healthURL := strings.TrimSuffix(svc.URL, "/") + "/" + strings.TrimPrefix(svc.Health, "/")

	resp, err := client.Get(healthURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode < http.StatusBadRequest
}

// -------------------------
// 10. CHI MIDDLEWARE
// -------------------------

func (gw *Gateway) HostMiddleware(domains []string) func(http.Handler) http.Handler {
	domainMap := make(map[string]bool, len(domains))
	for _, domain := range domains {
		domainMap[domain] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host := extractHost(r.Host)
			if domainMap[host] {
				next.ServeHTTP(w, r)
			} else {
				http.NotFound(w, r)
			}
		})
	}
}

func (gw *Gateway) ProxyMiddleware(tr *TenantRouter, stripPrefix string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			backend := tr.NextBackend()
			if backend == nil {
				http.Error(w, "No backend available", http.StatusBadGateway)
				return
			}

			proxy := gw.createProxy(backend, stripPrefix)
			proxy.ServeHTTP(w, r)
		})
	}
}

func (gw *Gateway) createProxy(backend *Backend, stripPrefix string) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(backend.URL)

	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = backend.URL.Scheme
		req.URL.Host = backend.URL.Host

		if stripPrefix != "" {
			newPath := strings.TrimPrefix(req.URL.Path, stripPrefix)
			if newPath == "" {
				newPath = "/"
			}
			req.URL.Path = newPath
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

// -------------------------
// 11. HTTP HANDLERS
// -------------------------

func (gw *Gateway) ProxyHandler(w http.ResponseWriter, r *http.Request) {
	router, stripPrefix := gw.MatchRoute(r.Host, r.URL.Path)
	if router == nil {
		http.NotFound(w, r)
		return
	}

	backend := router.NextBackend()
	if backend == nil {
		http.Error(w, "No backend available", http.StatusBadGateway)
		return
	}

	proxy := gw.createProxy(backend, stripPrefix)
	proxy.ServeHTTP(w, r)
}

// -------------------------
// 12. API ENDPOINTS (for Management)
// -------------------------

func (gw *Gateway) HealthHandler(w http.ResponseWriter, r *http.Request) {
	status := HealthStatus{
		Status:  "healthy",
		Tenants: make(map[string]string),
		Version: "1.2.1",
		Uptime:  time.Since(gw.startTime).String(),
	}

	for _, tenant := range gw.config.Tenants {
		if router := gw.getTenantRouter(tenant.Name); router != nil {
			healthyCount := 0
			for _, backend := range router.Backends {
				if backend.Alive.Load() {
					healthyCount++
				}
			}
			status.Tenants[tenant.Name] = fmt.Sprintf("%d/%d healthy", healthyCount, len(router.Backends))
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(status); err != nil {
		http.Error(w, "Failed to encode health status", http.StatusInternalServerError)
		return
	}
}

func (gw *Gateway) TenantsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(gw.config.Tenants); err != nil {
		http.Error(w, "Failed to encode tenants data", http.StatusInternalServerError)
		return
	}
}

func (gw *Gateway) getTenantRouter(name string) *TenantRouter {
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

// -------------------------
// 13. CHI ROUTER SETUP
// -------------------------

func (gw *Gateway) SetupRouter() *chi.Mux {
	r := chi.NewRouter()

	// Core middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(middleware.Timeout(DefaultRequestTimeout))

	// Admin base path from config, defaulting to "/admin" if empty
	basePath := gw.config.AdminBasePath
	if basePath == "" {
		basePath = "/"
	}

	// Use basePath dynamically in Route
	r.Route(basePath, func(r chi.Router) {
		r.Get("/health", gw.HealthHandler)
		r.Get("/tenants", gw.TenantsHandler)
		// TODO: Add more management endpoints
	})

	// Setup tenant routing with Chi
	gw.setupTenantRouting(r)

	return r
}

func (gw *Gateway) setupTenantRouting(r *chi.Mux) {
	for _, tenant := range gw.config.Tenants {
		router := gw.getTenantRouter(tenant.Name)
		if router == nil {
			continue
		}

		if len(tenant.Domains) > 0 && tenant.PathPrefix != "" {
			// Hybrid routing
			r.Route(tenant.PathPrefix, func(r chi.Router) {
				r.Use(gw.HostMiddleware(tenant.Domains))
				r.Use(gw.ProxyMiddleware(router, tenant.PathPrefix))
				r.HandleFunc("/*", func(w http.ResponseWriter, r *http.Request) {
					// Middleware handles everything
				})
			})
		} else if len(tenant.Domains) > 0 {
			// Host-only routing
			r.Group(func(r chi.Router) {
				r.Use(gw.HostMiddleware(tenant.Domains))
				r.Use(gw.ProxyMiddleware(router, ""))
				r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
					// Middleware handles everything
				})
				r.HandleFunc("/*", func(w http.ResponseWriter, r *http.Request) {
					// Middleware handles everything
				})
			})
		} else if tenant.PathPrefix != "" {
			// Path-only routing
			r.Route(tenant.PathPrefix, func(r chi.Router) {
				r.Use(gw.ProxyMiddleware(router, tenant.PathPrefix))
				r.HandleFunc("/*", func(w http.ResponseWriter, r *http.Request) {
					// Middleware handles everything
				})
			})
		}
	}

	// Catch-all handlers for tenant routing
	r.HandleFunc("/", gw.ProxyHandler)
	r.HandleFunc("/*", gw.ProxyHandler)
}

// -------------------------
// 14. MAIN FUNCTION
// -------------------------

func main() {
	cfgPath := flag.String("config", "config.yaml", "path to YAML config")
	addr := flag.String("addr", DefaultListenAddress, "listen address")
	flag.Parse()

	cfg, err := LoadConfig(*cfgPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	gateway := NewGateway(cfg)
	router := gateway.SetupRouter()

	log.Printf("Keystone Gateway v1.2.1 (Chi Router) listening on %s", *addr)
	if err := http.ListenAndServe(*addr, router); err != nil {
		log.Fatal(err)
	}
}
