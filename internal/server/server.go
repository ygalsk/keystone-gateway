package server

import (
	"context"
	"encoding/json"
	"fmt"
	"keystone-gateway/internal/router"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"keystone-gateway/internal/config"
	"keystone-gateway/internal/lua"
	"keystone-gateway/internal/metrics"
	"keystone-gateway/internal/middleware"
)

type Server struct {
	router        *chi.Mux
	config        *config.Config
	logger        *slog.Logger
	server        *http.Server
	gateway       *router.Gateway
	startTime     time.Time
	metrics       *metrics.LuaMetrics
	// Lua components
	luaEngine   *lua.Engine
	luaRegistry *router.LuaRouteRegistry
}

func New(cfg *config.Config, logger *slog.Logger) *Server {

	// Create simplified router - handles both static and Lua routes
	routerResult, err := router.NewRouter(router.RouterConfig{
		Logger: logger,
		Config: cfg,
	})
	if err != nil {
		logger.Error("failed to create router", "error", err)
		return nil
	}
	// Extract router from result
	r := routerResult.Router

	// Create a server instance
	srv := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           r,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
	}

	// Initialize metrics
	luaMetrics := metrics.NewLuaMetrics()

	server := &Server{
		router:        r,
		config:        cfg,
		logger:        logger,
		server:        srv,
		gateway:       routerResult.Gateway,
		startTime:     time.Now(),
		metrics:       luaMetrics,
		luaEngine:     routerResult.LuaEngine,
		luaRegistry:   routerResult.LuaRouteRegistry,
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

	// Mount tenant routes dynamically from the Lua route registry
	if server.luaRegistry != nil {
		tenants := server.luaRegistry.ListTenants()
		logger.Info("lua route registry available", "tenant_count", len(tenants))
		// Note: Actual route mounting will be handled by the Gateway's internal routing
		// The LuaRouteRegistry integrates directly with the Gateway's MatchRoute system
	}

	// Add fallback handler for tenant routing and proxying via Gateway
	r.NotFound(server.handleTenantRequests)

	return server
}

// handleHealth handles the server's basic health endpoint
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	response := map[string]interface{}{
		"status":    "healthy",
		"version":   "v1.0.0",
		"timestamp": time.Now(),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("failed to encode health response", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleTenantRequests handles requests using the Gateway's tenant routing system
func (s *Server) handleTenantRequests(w http.ResponseWriter, r *http.Request) {
	// Use Gateway to find the appropriate tenant router
	tenantRouter, stripPrefix := s.gateway.MatchRoute(r.Host, r.URL.Path)
	if tenantRouter == nil {
		s.logger.Warn("no tenant route matched", "host", r.Host, "path", r.URL.Path)
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		return
	}

	// Get next healthy backend from tenant router
	backend := tenantRouter.NextBackend()
	if backend == nil {
		s.logger.Error("no healthy backends available", "tenant", tenantRouter.Name)
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		return
	}

	// Create proxy and forward request
	proxy := s.gateway.CreateProxy(backend, stripPrefix)
	proxy.ServeHTTP(w, r)
}

// handleAdminStats handles the admin stats endpoint
func (s *Server) handleAdminStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Create comprehensive admin response using Gateway stats
	adminStats := map[string]interface{}{
		"timestamp": time.Now(),
		"server": map[string]interface{}{
			"version": "v1.0.0",
			"uptime":  time.Since(s.startTime),
		},
		"gateway": map[string]interface{}{
			"transport_stats":    s.gateway.GetTransportStats(),
			"proxy_cache_stats": s.gateway.GetProxyCacheStats(),
			"start_time":         s.gateway.GetStartTime(),
		},
	}

	if err := json.NewEncoder(w).Encode(adminStats); err != nil {
		s.logger.Error("failed to encode admin stats", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleAdminHealth handles the admin health endpoint with detailed backend health
func (s *Server) handleAdminHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// TODO(human): Implement Gateway-based health status collection
	// The Gateway handles health checks internally, but we need to expose the health status
	// through a new method like GetAllBackendHealth() similar to the old health checker
	healthResponse := map[string]interface{}{
		"timestamp": time.Now(),
		"summary": map[string]interface{}{
			"status": "gateway_health_integration_pending",
			"note":   "Gateway health checks are running, but health status API needs implementation",
		},
	}

	if err := json.NewEncoder(w).Encode(healthResponse); err != nil {
		s.logger.Error("failed to encode health stats", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleAdminStatus handles the combined admin status endpoint
func (s *Server) handleAdminStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	statusResponse := map[string]interface{}{
		"timestamp": time.Now(),
		"version":   "v1.0.0",
		"status":    "operational",
		"gateway": map[string]interface{}{
			"transport_stats":    s.gateway.GetTransportStats(),
			"proxy_cache_stats": s.gateway.GetProxyCacheStats(),
			"start_time":         s.gateway.GetStartTime(),
		},
	}

	if err := json.NewEncoder(w).Encode(statusResponse); err != nil {
		s.logger.Error("failed to encode status response", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleAdminMetrics handles the Prometheus metrics endpoint
func (s *Server) handleAdminMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")

	// TODO(human): Implement Gateway metrics collection
	// Need to create a method to extract backend statistics from Gateway's internal state
	// for metrics formatting, similar to the old LoadBalancer.GetStats()
	
	// For now, provide basic server metrics
	fmt.Fprintf(w, "# HELP keystone_gateway_info Gateway information\n")
	fmt.Fprintf(w, "# TYPE keystone_gateway_info gauge\n")
	fmt.Fprintf(w, "keystone_gateway_info{version=\"v1.0.0\"} 1\n")
	fmt.Fprintf(w, "# HELP keystone_gateway_uptime_seconds Gateway uptime in seconds\n")
	fmt.Fprintf(w, "# TYPE keystone_gateway_uptime_seconds counter\n")
	fmt.Fprintf(w, "keystone_gateway_uptime_seconds %.2f\n", time.Since(s.startTime).Seconds())

	// Add Lua metrics if available
	if s.luaEngine != nil {
		luaStats := s.metrics.GetStats(
			0, // routes count - would need to be tracked separately
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

	// Stop Gateway health checks
	if s.gateway != nil {
		s.gateway.StopHealthChecks()
	}

	return s.server.Shutdown(ctx)
}
