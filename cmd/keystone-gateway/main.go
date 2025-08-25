package main

import (
	"log"
	"log/slog"
	"net/http"
	_ "net/http/pprof" // Enable pprof endpoints
	"os"
	"os/signal"
	"syscall"

	"keystone-gateway/internal/config"
	"keystone-gateway/internal/server"
)

func main() {
	// Setup structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Start pprof server for memory profiling
	go func() {
		logger.Info("starting pprof server on :6060")
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	// Determine config file path
	configPath := "config.yaml" // default
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	// Load configuration
	cfg, err := config.LoadFromFile(configPath)
	if err != nil {
		logger.Error("failed to load configuration", "error", err, "config_path", configPath)
		os.Exit(1)
	}

	logger.Info("loaded configuration",
		"upstreams", len(cfg.GetEnabledUpstreams()),
		"strategy", cfg.Upstreams.LoadBalancing.Strategy,
		"health_check_enabled", cfg.Upstreams.HealthCheck.Enabled)

	// Create server
	srv := server.New(cfg, logger)

	// Graceful shutdown
	go func() {
		sigterm := make(chan os.Signal, 1)
		signal.Notify(sigterm, syscall.SIGINT, syscall.SIGTERM)
		<-sigterm

		logger.Info("shutting down...")
		if err := srv.Stop(); err != nil {
			logger.Error("error during shutdown", "error", err)
		}
	}()

	// Start server
	if err := srv.Start(); err != nil {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}
