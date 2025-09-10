// Package main implements the Keystone Gateway chi-stone binary.
// This is the main entry point for the gateway service.
package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"keystone-gateway/internal/app"
	"keystone-gateway/internal/config"
)

// Constants for the application
const (
	Version                = "1.2.1"
	DefaultListenAddress   = ":8080"
	DefaultShutdownTimeout = 10 * time.Second
)

func init() {
	// Simple structured JSON logging
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))

	// Optional .env loading (simple parser: KEY=VALUE lines, ignores # comments)
	if _, err := os.Stat(".env"); err == nil {
		if data, err := os.ReadFile(".env"); err == nil {
			lines := strings.Split(string(data), "\n")
			for _, l := range lines {
				l = strings.TrimSpace(l)
				if l == "" || strings.HasPrefix(l, "#") {
					continue
				}
				if eq := strings.IndexByte(l, '='); eq > 0 {
					k := strings.TrimSpace(l[:eq])
					v := strings.TrimSpace(l[eq+1:])
					if _, exists := os.LookupEnv(k); !exists {
						os.Setenv(k, v)
					}
				}
			}
			slog.Info("env_file_loaded", "file", ".env", "component", "startup")
		}
	}
}

func main() {
	cfgPath := flag.String("config", "config.yaml", "path to YAML config")
	addr := flag.String("addr", DefaultListenAddress, "listen address")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(*cfgPath)
	if err != nil {
		slog.Error("config_load_failed", "error", err, "path", *cfgPath)
		os.Exit(1)
	}

	// Create application
	application, err := app.New(cfg, Version)
	if err != nil {
		slog.Error("app_creation_failed", "error", err)
		os.Exit(1)
	}

	// Create HTTP server
	server := &http.Server{
		Addr:              *addr,
		Handler:           application.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Setup graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start server
	go func() {
		slog.Info("server_starting", "version", Version, "address", *addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server_failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal
	<-stop
	slog.Info("shutdown_initiated")

	// Graceful shutdown
	application.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), DefaultShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("server_shutdown_forced", "error", err)
	} else {
		slog.Info("server_shutdown_graceful")
	}
}

