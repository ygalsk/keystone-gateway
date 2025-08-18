package server

import (
	"context"
	"encoding/json"
	"keystone-gateway/internal/proxy"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"keystone-gateway/internal/config"
)

type Server struct {
	router        *chi.Mux
	config        *config.Config
	logger        *slog.Logger
	server        *http.Server
	loadBalancer  *proxy.LoadBalancer
	healthChecker *proxy.HealthChecker
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

	// Build a router with middleware (including proxy middleware)
	r := chi.NewRouter()
	for _, m := range BuildMiddlewareStack(logger, cfg, lb, hc) {
		r.Use(m)
	}

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
	}

	// Health endpoint for the gateway itself
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Get basic stats for the health check
		lbStats := server.loadBalancer.GetStats()

		response := map[string]interface{}{
			"status":            "healthy",
			"version":           "v1.0.0",
			"timestamp":         time.Now(),
			"total_upstreams":   lbStats.TotalUpstreams,
			"healthy_upstreams": lbStats.HealthyUpstreams,
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			server.logger.Error("failed to encode health response", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	})

	// Admin endpoints with comprehensive stats
	r.Get("/admin/stats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Get load balancer stats
		lbStats := server.loadBalancer.GetStats()

		// Create comprehensive admin response
		adminStats := map[string]interface{}{
			"timestamp": time.Now(),
			"server": map[string]interface{}{
				"version": "v1.0.0",
				"uptime":  time.Since(time.Now()), // TODO: track actual start time
			},
			"load_balancer": lbStats,
		}

		if err := json.NewEncoder(w).Encode(adminStats); err != nil {
			server.logger.Error("failed to encode admin stats", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	})

	r.Get("/admin/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Get all upstream health stats
		healthStats := server.healthChecker.GetAllUpstreamHealth()

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
			server.logger.Error("failed to encode health stats", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	})

	// Combined admin status endpoint
	r.Get("/admin/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Get comprehensive status
		lbStats := server.loadBalancer.GetStats()
		healthStats := server.healthChecker.GetAllUpstreamHealth()

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
			server.logger.Error("failed to encode status response", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	})

	return server
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

	return s.server.Shutdown(ctx)
}
