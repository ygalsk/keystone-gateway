// Package main implements the Keystone Gateway chi-stone binary.
// This is the main entry point for the gateway service.
package main

import (
	"bufio"
	"context"
	"flag"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
	"keystone-gateway/internal/config"
	"keystone-gateway/internal/gateway"
)

// Constants for the application
const (
	Version                = "4.0.0"
	DefaultListenAddress   = ":8080"
	DefaultShutdownTimeout = 10 * time.Second
)

func init() {
	// Simple structured JSON logging
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))

	// Optional .env loading (efficient parser: KEY=VALUE lines, ignores # comments)
	if file, err := os.Open(".env"); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			if len(line) == 0 || line[0] == '#' {
				continue
			}
			if eq := strings.IndexByte(line, '='); eq > 0 {
				k := strings.TrimSpace(line[:eq])
				v := strings.TrimSpace(line[eq+1:])
				if _, exists := os.LookupEnv(k); !exists {
					os.Setenv(k, v)
				}
			}
		}
		if scanner.Err() == nil {
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

	// Create gateway
	gw, err := gateway.New(cfg, Version)
	if err != nil {
		slog.Error("gateway_creation_failed", "error", err)
		os.Exit(1)
	}

	// Run server with graceful shutdown
	if err := runServer(gw, *addr); err != nil {
		slog.Error("server_run_failed", "error", err)
		os.Exit(1)
	}
}

// runServer runs the server with proper context cancellation and graceful shutdown
func runServer(gw *gateway.Gateway, addr string) error {
	// Create HTTP server
	server := &http.Server{
		Addr:              addr,
		Handler:           gw.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Create context that cancels on signal
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Use errgroup for coordinated startup/shutdown
	g, _ := errgroup.WithContext(ctx)

	// Start pprof debug server on separate port
	g.Go(func() error {
		pprofServer := &http.Server{
			Addr:    ":6060",
			Handler: http.DefaultServeMux, // pprof registers itself here
		}
		slog.Info("pprof_server_starting", "address", ":6060")
		if err := pprofServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// Don't fail if pprof server has issues
			slog.Warn("pprof_server_failed", "error", err)
		}
		return nil
	})

	// Start server
	g.Go(func() error {
		slog.Info("server_starting", "version", Version, "address", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	})

	// Wait for shutdown signal
	g.Go(func() error {
		select {
		case sig := <-sigChan:
			slog.Info("shutdown_signal_received", "signal", sig.String())
			cancel() // Cancel context to trigger shutdown
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	// Shutdown handler
	g.Go(func() error {
		<-ctx.Done() // Wait for cancellation
		slog.Info("shutdown_initiated")

		// Stop application components
		gw.Stop()

		// Create shutdown context with timeout
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), DefaultShutdownTimeout)
		defer shutdownCancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("server_shutdown_forced", "error", err)
			return err
		}
		slog.Info("server_shutdown_graceful")
		return nil
	})

	return g.Wait()
}
