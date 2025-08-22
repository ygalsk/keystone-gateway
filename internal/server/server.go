package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"keystone-gateway/internal/config"
	"keystone-gateway/internal/lua"
	"keystone-gateway/internal/metrics"
	"keystone-gateway/internal/middleware"
	"keystone-gateway/internal/proxy"
	"keystone-gateway/internal/router"
)

// TODO(human): Update luaMetrics variables and handleAdminMetrics function below

type Server struct {
	router        *chi.Mux
	config        *config.Config
	logger        *slog.Logger
	server        *http.Server
	loadBalancer  *proxy.LoadBalancer
	healthChecker *proxy.HealthChecker
	startTime     time.Time
	metrics       *metrics.LuaMetrics
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

	// Create router with all middleware and Lua components
	routerResult, err := router.NewRouter(router.RouterConfig{
		Logger: logger,
		Config: cfg,
	})
	if err != nil {
		logger.Error("failed to create router", "error", err)
		return nil
	}

	r := routerResult.Router
	luaEngine := routerResult.LuaEngine
	luaChiRouter := routerResult.LuaChiRouter

	// Create a server instance first to access in closures
	srv := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           r,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
	}

	// Initialize metrics (needed for admin endpoints)
	luaMetrics := metrics.NewLuaMetrics()

	server := &Server{
		router:        r,
		config:        cfg,
		logger:        logger,
		server:        srv,
		loadBalancer:  lb,
		healthChecker: hc,
		startTime:     time.Now(),
		metrics:       luaMetrics,
		luaEngine:     luaEngine,
		luaChiRouter:  luaChiRouter,
	}
	// Gateway health endpoint - handled directly by gateway
	r.Get("/health", server.handleHealth)

	// Admin endpoints - handled directly by gateway, no proxying
	r.Route("/admin", func(r chi.Router) {
		// Apply admin security middleware to all admin routes
		r.Use(middleware.AdminSecurityMiddleware(cfg.Server.Admin, logger))
		r.Get("/health", server.handleAdminHealth)
		r.Get("/stats", server.handleAdminStats)
		r.Get("/status", server.handleAdminStatus)
		r.Get("/metrics", server.handleAdminMetrics)
	})

	// Create a proxy-enabled sub-router for all other requests
	// This ensures only non-admin routes get proxied to backends
	proxyRouter := chi.NewRouter()

	// Add proxy middleware as fallback
	proxyRouter.Use(middleware.ProxyMiddleware(lb, hc, logger))

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

// handleAdminMetrics handles the Prometheus metrics endpoint
func (s *Server) handleAdminMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")

	// Get load balancer stats
	lbStats := s.loadBalancer.GetStats()

	// Convert to metrics formatter compatible format
	formatterStats := metrics.LoadBalancerStats{
		Strategy:         lbStats.Strategy,
		TotalUpstreams:   lbStats.TotalUpstreams,
		HealthyUpstreams: lbStats.HealthyUpstreams,
		UpstreamStats:    make([]metrics.UpstreamStat, len(lbStats.UpstreamStats)),
	}

	// Convert upstream stats to formatter format
	for i, upstream := range lbStats.UpstreamStats {
		formatterStats.UpstreamStats[i] = metrics.UpstreamStat{
			Name:                upstream.Name,
			URL:                 upstream.URL,
			TotalRequests:       upstream.TotalRequests,
			AvgResponseTime:     upstream.AvgResponseTime,
			Healthy:             upstream.Healthy,
			ActiveConnections:   int64(upstream.ActiveConnections),
			ConsecutiveFailures: int64(upstream.ConsecutiveFailures),
		}
	}

	// Format load balancer metrics using consolidated formatter
	if err := metrics.FormatPrometheusMetrics(w, formatterStats, s.startTime); err != nil {
		s.logger.Error("failed to format load balancer metrics", "error", err)
	}

	// Add Lua metrics if available
	if s.luaChiRouter != nil {
		luaStats := s.metrics.GetStats(
			len(s.luaChiRouter.GetRoutes()),
			0, // middlewares count - would need to be tracked separately
			0, // groups count - would need to be tracked separately  
		)
		if err := metrics.FormatLuaMetrics(w, luaStats); err != nil {
			s.logger.Error("failed to format lua metrics", "error", err)
		}
	}
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

