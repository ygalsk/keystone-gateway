package fixtures

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"keystone-gateway/internal/config"
	"keystone-gateway/internal/routing"
)

// E2EGateway represents a real gateway server for E2E testing
type E2EGateway struct {
	Config   *config.Config
	Gateway  *routing.Gateway
	Server   *http.Server
	URL      string
	Port     int
	listener net.Listener
}

// StartRealGateway starts a real gateway server for E2E testing
func StartRealGateway(t *testing.T, cfg *config.Config) *E2EGateway {
	// Get a random available port
	port := GetRandomPort()
	
	// Create router and gateway
	router := chi.NewRouter()
	gateway := routing.NewGatewayWithRouter(cfg, router)
	
	// Create HTTP server
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: createGatewayHandler(gateway),
		// Set reasonable timeouts for E2E testing
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}
	
	// Start listening
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		t.Fatalf("Failed to start E2E gateway listener: %v", err)
	}
	
	// Start server in goroutine
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			t.Errorf("E2E gateway server error: %v", err)
		}
	}()
	
	// Wait for server to be ready
	url := fmt.Sprintf("http://localhost:%d", port)
	if !waitForServer(url, 5*time.Second) {
		t.Fatalf("E2E gateway server failed to start within timeout")
	}
	
	e2eGateway := &E2EGateway{
		Config:   cfg,
		Gateway:  gateway,
		Server:   server,
		URL:      url,
		Port:     port,
		listener: listener,
	}
	
	t.Logf("Started E2E gateway server at %s", url)
	return e2eGateway
}

// Stop gracefully stops the E2E gateway server
func (g *E2EGateway) Stop() error {
	if g.Server == nil {
		return nil
	}
	
	// Create context with timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// Gracefully shutdown server
	err := g.Server.Shutdown(ctx)
	if g.listener != nil {
		if closeErr := g.listener.Close(); closeErr != nil {
			log.Printf("Failed to close gateway listener: %v", closeErr)
		}
	}
	
	return err
}

// createGatewayHandler creates the HTTP handler for the E2E gateway
func createGatewayHandler(gateway *routing.Gateway) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Validate headers for malformed content (same as main.go)
		for name := range r.Header {
			for _, char := range name {
				if char == 0 { // null byte
					http.Error(w, "Bad Request: Invalid header name", http.StatusBadRequest)
					return
				}
			}
		}

		// Validate path for null bytes and excessive length
		if len(r.URL.Path) > 1024 { // Reject paths longer than 1KB
			http.NotFound(w, r)
			return
		}
		for _, char := range r.URL.Path {
			if char == 0 { // null byte in path
				http.NotFound(w, r)
				return
			}
		}

		// Route request through gateway
		tenantRouter, stripPrefix := gateway.MatchRoute(r.Host, r.URL.Path)
		if tenantRouter == nil {
			http.NotFound(w, r)
			return
		}

		backend := tenantRouter.NextBackend()
		if backend == nil {
			http.Error(w, "No backend available", http.StatusBadGateway)
			return
		}

		proxy := gateway.CreateProxy(backend, stripPrefix)
		proxy.ServeHTTP(w, r)
	})
}

// GetRandomPort returns a random available port for testing
func GetRandomPort() int {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(fmt.Sprintf("Failed to get random port: %v", err))
	}
	defer func() {
		if err := listener.Close(); err != nil {
			log.Printf("Failed to close test listener: %v", err)
		}
	}()
	
	return listener.Addr().(*net.TCPAddr).Port
}

// waitForServer waits for a server to become available
func waitForServer(url string, timeout time.Duration) bool {
	client := &http.Client{
		Timeout: 1 * time.Second,
	}
	
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := client.Get(url + "/health-check-probe")
		if err == nil {
			if err := resp.Body.Close(); err != nil {
				log.Printf("Failed to close response body: %v", err)
			}
			return true
		}
		
		// Even if health check fails, server might be up
		if resp != nil {
			if err := resp.Body.Close(); err != nil {
				log.Printf("Failed to close response body: %v", err)
			}
			return true
		}
		
		time.Sleep(100 * time.Millisecond)
	}
	
	return false
}

// E2EGatewayCluster represents multiple gateway instances for testing
type E2EGatewayCluster struct {
	Gateways []*E2EGateway
	LoadBalancerURL string
}

// StartGatewayCluster starts multiple gateway instances for cluster testing
func StartGatewayCluster(t *testing.T, configs []*config.Config) *E2EGatewayCluster {
	if len(configs) == 0 {
		t.Fatal("At least one config required for gateway cluster")
	}
	
	var gateways []*E2EGateway
	
	for i, cfg := range configs {
		gateway := StartRealGateway(t, cfg)
		gateways = append(gateways, gateway)
		t.Logf("Started gateway %d/%d at %s", i+1, len(configs), gateway.URL)
	}
	
	cluster := &E2EGatewayCluster{
		Gateways: gateways,
		// For simplicity, use first gateway as primary
		LoadBalancerURL: gateways[0].URL,
	}
	
	return cluster
}

// Stop stops all gateways in the cluster
func (c *E2EGatewayCluster) Stop() error {
	var lastErr error
	
	for i, gateway := range c.Gateways {
		if err := gateway.Stop(); err != nil {
			lastErr = err
			// Continue stopping other gateways even if one fails
		} else {
			// Log successful shutdown
			_ = i // Gateway index for potential logging
		}
	}
	
	return lastErr
}

// GetGatewayURLs returns all gateway URLs in the cluster
func (c *E2EGatewayCluster) GetGatewayURLs() []string {
	urls := make([]string, len(c.Gateways))
	for i, gateway := range c.Gateways {
		urls[i] = gateway.URL
	}
	return urls
}

// E2ETestEnvironment represents a complete E2E testing environment
type E2ETestEnvironment struct {
	Gateway  *E2EGateway
	Backends []*E2EBackend
	Config   *config.Config
}

// SetupE2EEnvironment sets up a complete E2E testing environment
func SetupE2EEnvironment(t *testing.T, cfg *config.Config) *E2ETestEnvironment {
	// Start backend servers for each service in the config
	var backends []*E2EBackend
	
	// Create a copy of config to modify with real backend URLs
	configCopy := *cfg
	configCopy.Tenants = make([]config.Tenant, len(cfg.Tenants))
	
	for i, tenant := range cfg.Tenants {
		configCopy.Tenants[i] = tenant
		configCopy.Tenants[i].Services = make([]config.Service, len(tenant.Services))
		
		for j, service := range tenant.Services {
			// Start real backend for this service
			backend := StartRealBackend(t, "simple")
			backends = append(backends, backend)
			
			// Update service URL to point to real backend
			configCopy.Tenants[i].Services[j] = service
			configCopy.Tenants[i].Services[j].URL = backend.URL
		}
	}
	
	// Start gateway with updated config
	gateway := StartRealGateway(t, &configCopy)
	
	env := &E2ETestEnvironment{
		Gateway:  gateway,
		Backends: backends,
		Config:   &configCopy,
	}
	
	t.Logf("Setup E2E environment with gateway at %s and %d backends", 
		gateway.URL, len(backends))
	
	return env
}

// Cleanup cleans up the entire E2E testing environment
func (env *E2ETestEnvironment) Cleanup() error {
	var lastErr error
	
	// Stop gateway
	if env.Gateway != nil {
		if err := env.Gateway.Stop(); err != nil {
			lastErr = err
		}
	}
	
	// Stop all backends
	for _, backend := range env.Backends {
		if err := backend.Stop(); err != nil {
			lastErr = err
		}
	}
	
	return lastErr
}