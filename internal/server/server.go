package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	lua_lib "github.com/yuin/gopher-lua"

	"keystone-gateway/internal/config"
	"keystone-gateway/internal/lua"
	"keystone-gateway/internal/proxy"
)

type Server struct {
	router        *chi.Mux
	config        *config.Config
	logger        *slog.Logger
	server        *http.Server
	loadBalancer  *proxy.LoadBalancer
	healthChecker *proxy.HealthChecker
	startTime     time.Time
	// Lua components
	luaEngine    *lua.Engine
	luaChiRouter *lua.ChiRouter
}

func New(cfg *config.Config, logger *slog.Logger) *Server {
	// Create proxy components
	lb := proxy.NewLoadBalancer(cfg.Upstreams.LoadBalancing.Strategy, logger)
	hc := proxy.NewHealthChecker(cfg.Upstreams.HealthCheck, logger)

	// Setup upstreams from configuration
	for _, target := range cfg.GetEnabledUpstreams() {
		upstream, err := proxy.NewUpstream(
			target.Name,
			target.URL,
			target.Weight,
			cfg.Upstreams.HealthCheck.Path,
		)
		if err != nil {
			logger.Error("failed to create upstream", "name", target.Name, "error", err)
			continue
		}

		// Add to load balancer and health checker
		lb.AddUpstream(upstream)
		hc.AddUpstream(upstream)

		logger.Info("configured upstream", "name", target.Name, "url", target.URL)
	}

	// Start the health checker
	hc.Start()
	//TODO from here --->
	// Build a router with base middleware (excluding proxy middleware)
	r := chi.NewRouter()
	baseMiddleware := BuildBaseMiddleware(logger, cfg)
	for _, m := range baseMiddleware {
		r.Use(m)
	}

	// Initialize Lua components based on configuration
	var luaEngine *lua.Engine
	var luaChiRouter *lua.ChiRouter

	if cfg.Lua != nil && cfg.Lua.Enabled {
		// Use configuration values or defaults
		maxStates := cfg.Lua.MaxStates
		if maxStates <= 0 {
			maxStates = 10
		}
		maxScripts := cfg.Lua.MaxScripts
		if maxScripts <= 0 {
			maxScripts = 100
		}

		luaEngine = lua.NewEngine(maxStates, maxScripts)
		luaMetrics := lua.NewLuaMetrics()

		// Create state pool for the Chi router
		statePool := lua.NewLuaStatePool(maxStates, func() *lua_lib.LState {
			return lua.CreateSecureLuaState(lua.DefaultSecurityConfig())
		})

		// Initialize Lua Chi router with the main Chi router
		luaChiRouter = lua.NewChiRouter(r, statePool, luaMetrics, logger)

		// Load tenant scripts - both middleware and routing
		for tenantID, tenantConfig := range cfg.Lua.TenantScripts {
			if !tenantConfig.Enabled {
				continue
			}

			// Load middleware script first
			if tenantConfig.MiddlewareScript != "" {
				if err := loadTenantScript(luaChiRouter, statePool, cfg.Lua.ScriptsDir, tenantConfig.MiddlewareScript, tenantID, logger); err != nil {
					logger.Error("failed to load tenant middleware script",
						"tenant", tenantID,
						"script", tenantConfig.MiddlewareScript,
						"error", err)
				} else {
					logger.Info("loaded tenant middleware script",
						"tenant", tenantID,
						"script", tenantConfig.MiddlewareScript)
				}
			}

			// Load routing script second (so routes are registered after middleware)
			if tenantConfig.RoutingScript != "" {
				if err := loadTenantScript(luaChiRouter, statePool, cfg.Lua.ScriptsDir, tenantConfig.RoutingScript, tenantID, logger); err != nil {
					logger.Error("failed to load tenant routing script",
						"tenant", tenantID,
						"script", tenantConfig.RoutingScript,
						"error", err)
				} else {
					logger.Info("loaded tenant routing script",
						"tenant", tenantID,
						"script", tenantConfig.RoutingScript)
				}
			}
		}

		logger.Info("lua scripting enabled",
			"max_states", maxStates,
			"max_scripts", maxScripts,
			"scripts_dir", cfg.Lua.ScriptsDir)
	} else {
		logger.Info("lua scripting disabled")
	}
	//TODO until here <---

	// Create a server instance first to access in closures
	srv := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           r,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
	}

	server := &Server{
		router:        r,
		config:        cfg,
		logger:        logger,
		server:        srv,
		loadBalancer:  lb,
		healthChecker: hc,
		startTime:     time.Now(),
		luaEngine:     luaEngine,
		luaChiRouter:  luaChiRouter,
	}
	// TODO think about protecting admin routes (JWT, private/pub key)
	// Gateway health endpoint - handled directly by gateway
	r.Get("/health", server.handleHealth)

	// Admin endpoints - handled directly by gateway, no proxying
	r.Route("/admin", func(r chi.Router) {
		r.Get("/health", server.handleAdminHealth)
		r.Get("/stats", server.handleAdminStats)
		r.Get("/status", server.handleAdminStatus)
		r.Get("/metrics", server.handleAdminMetrics)
	})

	// Create a proxy-enabled sub-router for all other requests
	// This ensures only non-admin routes get proxied to backends
	proxyRouter := chi.NewRouter()

	// Add proxy middleware as fallback
	proxyRouter.Use(ProxyMiddleware(lb, hc, logger))

	// Mount the proxy router to handle all other paths
	// This will catch any request that doesn't match admin routes
	r.Mount("/", proxyRouter)

	return server
}

// handleHealth handles the gateway's own health endpoint
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Get basic stats for the health check
	lbStats := s.loadBalancer.GetStats()

	response := map[string]interface{}{
		"status":            "healthy",
		"version":           "v1.0.0",
		"timestamp":         time.Now(),
		"total_upstreams":   lbStats.TotalUpstreams,
		"healthy_upstreams": lbStats.HealthyUpstreams,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("failed to encode health response", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleAdminStats handles the admin stats endpoint
func (s *Server) handleAdminStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Get load balancer stats
	lbStats := s.loadBalancer.GetStats()

	// Create comprehensive admin response
	adminStats := map[string]interface{}{
		"timestamp": time.Now(),
		"server": map[string]interface{}{
			"version": "v1.0.0",
			"uptime":  time.Since(s.startTime),
		},
		"load_balancer": lbStats,
	}

	if err := json.NewEncoder(w).Encode(adminStats); err != nil {
		s.logger.Error("failed to encode admin stats", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleAdminHealth handles the admin health endpoint with detailed upstream health
func (s *Server) handleAdminHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Get all upstream health stats
	healthStats := s.healthChecker.GetAllUpstreamHealth()

	// Calculate overall health summary
	totalUpstreams := len(healthStats)
	healthyCount := 0
	for _, stats := range healthStats {
		if stats.Status.String() == "healthy" {
			healthyCount++
		}
	}

	healthResponse := map[string]interface{}{
		"timestamp": time.Now(),
		"summary": map[string]interface{}{
			"total_upstreams":   totalUpstreams,
			"healthy_upstreams": healthyCount,
			"overall_status": func() string {
				if healthyCount == 0 {
					return "critical"
				} else if healthyCount < totalUpstreams {
					return "degraded"
				}
				return "healthy"
			}(),
		},
		"upstreams": healthStats,
	}

	if err := json.NewEncoder(w).Encode(healthResponse); err != nil {
		s.logger.Error("failed to encode health stats", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleAdminStatus handles the combined admin status endpoint
func (s *Server) handleAdminStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Get comprehensive status
	lbStats := s.loadBalancer.GetStats()
	healthStats := s.healthChecker.GetAllUpstreamHealth()

	statusResponse := map[string]interface{}{
		"timestamp": time.Now(),
		"version":   "v1.0.0",
		"status":    "operational",
		"summary": map[string]interface{}{
			"total_upstreams":   lbStats.TotalUpstreams,
			"healthy_upstreams": lbStats.HealthyUpstreams,
			"load_strategy":     lbStats.Strategy,
		},
		"detailed_health": healthStats,
	}

	if err := json.NewEncoder(w).Encode(statusResponse); err != nil {
		s.logger.Error("failed to encode status response", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// TODO maybe clean up the conversion part into a designated function
// handleAdminMetrics handles the Prometheus metrics endpoint
func (s *Server) handleAdminMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")

	// Get load balancer stats
	lbStats := s.loadBalancer.GetStats()

	// Server uptime
	uptime := time.Since(s.startTime)

	// Convert to Prometheus format
	fmt.Fprintf(w, "# HELP keystone_gateway_info Information about the keystone gateway\n")
	fmt.Fprintf(w, "# TYPE keystone_gateway_info gauge\n")
	fmt.Fprintf(w, "keystone_gateway_info{version=\"v1.0.0\",strategy=\"%s\"} 1\n", lbStats.Strategy)

	fmt.Fprintf(w, "# HELP keystone_gateway_uptime_seconds Server uptime in seconds\n")
	fmt.Fprintf(w, "# TYPE keystone_gateway_uptime_seconds counter\n")
	fmt.Fprintf(w, "keystone_gateway_uptime_seconds %.2f\n", uptime.Seconds())

	fmt.Fprintf(w, "# HELP keystone_upstreams_total Total number of upstreams\n")
	fmt.Fprintf(w, "# TYPE keystone_upstreams_total gauge\n")
	fmt.Fprintf(w, "keystone_upstreams_total %d\n", lbStats.TotalUpstreams)

	fmt.Fprintf(w, "# HELP keystone_healthy_upstreams Total healthy upstreams\n")
	fmt.Fprintf(w, "# TYPE keystone_healthy_upstreams gauge\n")
	fmt.Fprintf(w, "keystone_healthy_upstreams %d\n", lbStats.HealthyUpstreams)

	// Per-upstream metrics
	fmt.Fprintf(w, "# HELP keystone_upstream_requests_total Total requests sent to upstream\n")
	fmt.Fprintf(w, "# TYPE keystone_upstream_requests_total counter\n")

	fmt.Fprintf(w, "# HELP keystone_upstream_response_time_microseconds Average response time in microseconds\n")
	fmt.Fprintf(w, "# TYPE keystone_upstream_response_time_microseconds gauge\n")

	fmt.Fprintf(w, "# HELP keystone_upstream_healthy Health status of upstream (1=healthy, 0=unhealthy)\n")
	fmt.Fprintf(w, "# TYPE keystone_upstream_healthy gauge\n")

	fmt.Fprintf(w, "# HELP keystone_upstream_active_connections Current active connections to upstream\n")
	fmt.Fprintf(w, "# TYPE keystone_upstream_active_connections gauge\n")

	fmt.Fprintf(w, "# HELP keystone_upstream_consecutive_failures Number of consecutive failures\n")
	fmt.Fprintf(w, "# TYPE keystone_upstream_consecutive_failures gauge\n")

	for _, upstream := range lbStats.UpstreamStats {
		labels := fmt.Sprintf("upstream=\"%s\",url=\"%s\"", upstream.Name, upstream.URL)

		fmt.Fprintf(w, "keystone_upstream_requests_total{%s} %d\n", labels, upstream.TotalRequests)
		fmt.Fprintf(w, "keystone_upstream_response_time_microseconds{%s} %d\n", labels, upstream.AvgResponseTime.Microseconds())
		fmt.Fprintf(w, "keystone_upstream_healthy{%s} %d\n", labels, boolToInt(upstream.Healthy))
		fmt.Fprintf(w, "keystone_upstream_active_connections{%s} %d\n", labels, upstream.ActiveConnections)
		fmt.Fprintf(w, "keystone_upstream_consecutive_failures{%s} %d\n", labels, upstream.ConsecutiveFailures)
	}
}

// boolToInt converts boolean to integer for Prometheus metrics
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (s *Server) Start() error {
	// Check if TLS is enabled
	if s.config.Server.TLS != nil && s.config.Server.TLS.Enabled {
		s.logger.Info("starting HTTPS server",
			"addr", s.server.Addr,
			"cert", s.config.Server.TLS.CertFile,
		)
		if err := s.server.ListenAndServeTLS(s.config.Server.TLS.CertFile, s.config.Server.TLS.KeyFile); err != http.ErrServerClosed {
			s.logger.Error("HTTPS server failed", "error", err)
			return err
		}
	} else {
		s.logger.Info("starting HTTP server", "addr", s.server.Addr)
		if err := s.server.ListenAndServe(); err != http.ErrServerClosed {
			s.logger.Error("HTTP server failed", "error", err)
			return err
		}
	}
	return nil
}

func (s *Server) Stop() error {
	s.logger.Info("shutting down server")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown Lua components
	if s.luaChiRouter != nil {
		if err := s.luaChiRouter.Shutdown(); err != nil {
			s.logger.Error("failed to shutdown Lua Chi router", "error", err)
		}
	}

	return s.server.Shutdown(ctx)
}

// TODO needs to be moved to engine.go or similar
// loadTenantScript loads and executes a tenant's Lua middleware script
// This allows the script to register middleware via chi_middleware() calls
func loadTenantScript(luaChiRouter *lua.ChiRouter, statePool *lua.LuaStatePool, scriptsDir, scriptFile, tenantID string, logger *slog.Logger) error {
	// Read script file
	scriptPath := filepath.Join(scriptsDir, scriptFile)
	scriptContent, err := os.ReadFile(scriptPath)
	if err != nil {
		return fmt.Errorf("failed to read script file %s: %w", scriptPath, err)
	}

	// Get Lua state from pool
	L := statePool.Get()
	defer statePool.Put(L)

	// Set up bindings so script can call chi_middleware()
	if err := luaChiRouter.SetupLuaBindings(L, scriptFile, tenantID); err != nil {
		return fmt.Errorf("failed to setup Lua bindings: %w", err)
	}

	// Execute script - this will register middleware
	if err := L.DoString(string(scriptContent)); err != nil {
		return fmt.Errorf("failed to execute script: %w", err)
	}

	return nil
}
