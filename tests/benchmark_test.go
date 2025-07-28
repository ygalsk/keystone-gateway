package tests

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"keystone-gateway/internal/config"
	"keystone-gateway/internal/routing"
	"keystone-gateway/tests/fixtures"
)

// BenchmarkGatewayRouting benchmarks the core gateway routing performance
func BenchmarkGatewayRouting(b *testing.B) {
	// Setup test environment
	backend := createBenchmarkBackend()
	defer backend.Close()

	cfg := fixtures.CreateTestConfig("bench-tenant", "/api/")
	cfg.Tenants[0].Services[0].URL = backend.URL

	router := chi.NewRouter()
	gateway := routing.NewGatewayWithRouter(cfg, router)

	// Create proxy handler
	handler := createBenchmarkHandler(gateway)

	// Create test request
	req := httptest.NewRequest("GET", "/api/test", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		
		if w.Code != http.StatusOK {
			b.Fatalf("Expected status 200, got %d", w.Code)
		}
	}
}

// BenchmarkGatewayRoutingConcurrent benchmarks concurrent gateway routing
func BenchmarkGatewayRoutingConcurrent(b *testing.B) {
	// Setup test environment
	backend := createBenchmarkBackend()
	defer backend.Close()

	cfg := fixtures.CreateTestConfig("concurrent-bench-tenant", "/concurrent/")
	cfg.Tenants[0].Services[0].URL = backend.URL

	router := chi.NewRouter()
	gateway := routing.NewGatewayWithRouter(cfg, router)
	handler := createBenchmarkHandler(gateway)

	// Create test request
	req := httptest.NewRequest("GET", "/concurrent/test", nil)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			
			if w.Code != http.StatusOK {
				b.Fatalf("Expected status 200, got %d", w.Code)
			}
		}
	})
}

// BenchmarkLoadBalancing benchmarks load balancing performance
func BenchmarkLoadBalancing(b *testing.B) {
	// Setup multiple backends for load balancing
	backend1 := createBenchmarkBackend()
	defer backend1.Close()

	backend2 := createBenchmarkBackend()
	defer backend2.Close()

	backend3 := createBenchmarkBackend()
	defer backend3.Close()

	// Create config with multiple backends
	cfg := &config.Config{
		Tenants: []config.Tenant{
			{
				Name:       "lb-bench-tenant",
				PathPrefix: "/lb/",
				Interval:   30,
				Services: []config.Service{
					{Name: "service1", URL: backend1.URL, Health: "/health"},
					{Name: "service2", URL: backend2.URL, Health: "/health"},
					{Name: "service3", URL: backend3.URL, Health: "/health"},
				},
			},
		},
	}

	router := chi.NewRouter()
	gateway := routing.NewGatewayWithRouter(cfg, router)

	// Mark all backends as alive for benchmarking
	if tenantRouter := gateway.GetTenantRouter("lb-bench-tenant"); tenantRouter != nil {
		for _, backend := range tenantRouter.Backends {
			backend.Alive.Store(true)
		}
	}

	handler := createBenchmarkHandler(gateway)
	req := httptest.NewRequest("GET", "/lb/test", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		
		if w.Code != http.StatusOK {
			b.Fatalf("Expected status 200, got %d", w.Code)
		}
	}
}

// BenchmarkLoadBalancingConcurrent benchmarks concurrent load balancing
func BenchmarkLoadBalancingConcurrent(b *testing.B) {
	// Setup multiple backends
	backends := make([]*httptest.Server, 5)
	for i := 0; i < 5; i++ {
		backends[i] = createBenchmarkBackend()
		defer backends[i].Close()
	}

	// Create config with multiple backends
	services := make([]config.Service, 5)
	for i, backend := range backends {
		services[i] = config.Service{
			Name:   fmt.Sprintf("service%d", i+1),
			URL:    backend.URL,
			Health: "/health",
		}
	}

	cfg := &config.Config{
		Tenants: []config.Tenant{
			{
				Name:       "concurrent-lb-bench-tenant",
				PathPrefix: "/concurrent-lb/",
				Interval:   30,
				Services:   services,
			},
		},
	}

	router := chi.NewRouter()
	gateway := routing.NewGatewayWithRouter(cfg, router)

	// Mark all backends as alive
	if tenantRouter := gateway.GetTenantRouter("concurrent-lb-bench-tenant"); tenantRouter != nil {
		for _, backend := range tenantRouter.Backends {
			backend.Alive.Store(true)
		}
	}

	handler := createBenchmarkHandler(gateway)
	req := httptest.NewRequest("GET", "/concurrent-lb/test", nil)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			
			if w.Code != http.StatusOK {
				b.Fatalf("Expected status 200, got %d", w.Code)
			}
		}
	})
}

// BenchmarkMultiTenantRouting benchmarks multi-tenant routing performance
func BenchmarkMultiTenantRouting(b *testing.B) {
	// Setup backends for different tenants
	apiBackend := createBenchmarkBackend()
	defer apiBackend.Close()

	webBackend := createBenchmarkBackend()
	defer webBackend.Close()

	adminBackend := createBenchmarkBackend()
	defer adminBackend.Close()

	// Create multi-tenant config
	cfg := &config.Config{
		Tenants: []config.Tenant{
			{
				Name:     "api-bench-tenant",
				Domains:  []string{"api.example.com"},
				Interval: 30,
				Services: []config.Service{
					{Name: "api-service", URL: apiBackend.URL, Health: "/health"},
				},
			},
			{
				Name:       "web-bench-tenant",
				PathPrefix: "/web/",
				Interval:   30,
				Services: []config.Service{
					{Name: "web-service", URL: webBackend.URL, Health: "/health"},
				},
			},
			{
				Name:       "admin-bench-tenant",
				PathPrefix: "/admin/",
				Interval:   30,
				Services: []config.Service{
					{Name: "admin-service", URL: adminBackend.URL, Health: "/health"},
				},
			},
		},
	}

	router := chi.NewRouter()
	gateway := routing.NewGatewayWithRouter(cfg, router)

	// Mark all backends as alive
	tenantNames := []string{"api-bench-tenant", "web-bench-tenant", "admin-bench-tenant"}
	for _, tenantName := range tenantNames {
		if tenantRouter := gateway.GetTenantRouter(tenantName); tenantRouter != nil {
			for _, backend := range tenantRouter.Backends {
				backend.Alive.Store(true)
			}
		}
	}

	handler := createBenchmarkHandler(gateway)

	// Benchmark different routing scenarios
	testCases := []struct {
		name string
		req  *http.Request
	}{
		{"host_routing", createHostRequest("GET", "/users", "api.example.com")},
		{"path_routing_web", httptest.NewRequest("GET", "/web/home", nil)},
		{"path_routing_admin", httptest.NewRequest("GET", "/admin/dashboard", nil)},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, tc.req)
				
				if w.Code != http.StatusOK {
					b.Fatalf("Expected status 200, got %d", w.Code)
				}
			}
		})
	}
}

// BenchmarkLuaScriptExecution benchmarks Lua script execution performance
func BenchmarkLuaScriptExecution(b *testing.B) {
	// Setup backend for Lua testing
	backend := createBenchmarkBackend()
	defer backend.Close()

	// Create config that would use Lua processing
	cfg := fixtures.CreateTestConfig("lua-bench-tenant", "/lua/")
	cfg.Tenants[0].Services[0].URL = backend.URL

	router := chi.NewRouter()
	gateway := routing.NewGatewayWithRouter(cfg, router)

	// Mark backend as alive
	if tenantRouter := gateway.GetTenantRouter("lua-bench-tenant"); tenantRouter != nil {
		for _, gtwBackend := range tenantRouter.Backends {
			gtwBackend.Alive.Store(true)
		}
	}

	handler := createBenchmarkHandler(gateway)
	req := httptest.NewRequest("GET", "/lua/script", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		
		if w.Code != http.StatusOK {
			b.Fatalf("Expected status 200, got %d", w.Code)
		}
	}
}

// BenchmarkLuaScriptExecutionConcurrent benchmarks concurrent Lua execution
func BenchmarkLuaScriptExecutionConcurrent(b *testing.B) {
	// Setup backend for concurrent Lua testing
	backend := createBenchmarkBackend()
	defer backend.Close()

	cfg := fixtures.CreateTestConfig("concurrent-lua-bench-tenant", "/concurrent-lua/")
	cfg.Tenants[0].Services[0].URL = backend.URL

	router := chi.NewRouter()
	gateway := routing.NewGatewayWithRouter(cfg, router)

	// Mark backend as alive
	if tenantRouter := gateway.GetTenantRouter("concurrent-lua-bench-tenant"); tenantRouter != nil {
		for _, gtwBackend := range tenantRouter.Backends {
			gtwBackend.Alive.Store(true)
		}
	}

	handler := createBenchmarkHandler(gateway)
	req := httptest.NewRequest("GET", "/concurrent-lua/script", nil)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			
			if w.Code != http.StatusOK {
				b.Fatalf("Expected status 200, got %d", w.Code)
			}
		}
	})
}

// BenchmarkProxyCreation benchmarks proxy creation performance
func BenchmarkProxyCreation(b *testing.B) {
	// Setup backend
	backend := createBenchmarkBackend()
	defer backend.Close()

	cfg := fixtures.CreateTestConfig("proxy-bench-tenant", "/proxy/")
	cfg.Tenants[0].Services[0].URL = backend.URL

	router := chi.NewRouter()
	gateway := routing.NewGatewayWithRouter(cfg, router)

	// Get tenant router
	tenantRouter := gateway.GetTenantRouter("proxy-bench-tenant")
	if tenantRouter == nil {
		b.Fatal("Expected tenant router to be initialized")
	}

	// Mark backend as alive
	if len(tenantRouter.Backends) > 0 {
		tenantRouter.Backends[0].Alive.Store(true)
	}

	backend_obj := tenantRouter.NextBackend()
	if backend_obj == nil {
		b.Fatal("Expected backend to be available")
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		proxy := gateway.CreateProxy(backend_obj, "/proxy/")
		if proxy == nil {
			b.Fatal("Failed to create proxy")
		}
	}
}

// BenchmarkBackendSelection benchmarks backend selection performance
func BenchmarkBackendSelection(b *testing.B) {
	// Setup multiple backends
	backends := make([]*httptest.Server, 10)
	for i := 0; i < 10; i++ {
		backends[i] = createBenchmarkBackend()
		defer backends[i].Close()
	}

	// Create config with many backends
	services := make([]config.Service, 10)
	for i, backend := range backends {
		services[i] = config.Service{
			Name:   fmt.Sprintf("service%d", i+1),
			URL:    backend.URL,
			Health: "/health",
		}
	}

	cfg := &config.Config{
		Tenants: []config.Tenant{
			{
				Name:       "selection-bench-tenant",
				PathPrefix: "/selection/",
				Interval:   30,
				Services:   services,
			},
		},
	}

	router := chi.NewRouter()
	gateway := routing.NewGatewayWithRouter(cfg, router)

	tenantRouter := gateway.GetTenantRouter("selection-bench-tenant")
	if tenantRouter == nil {
		b.Fatal("Expected tenant router to be initialized")
	}

	// Mark all backends as alive
	for _, backend := range tenantRouter.Backends {
		backend.Alive.Store(true)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		backend := tenantRouter.NextBackend()
		if backend == nil {
			b.Fatal("Expected backend to be selected")
		}
	}
}

// BenchmarkConfigurationParsing benchmarks configuration parsing performance
func BenchmarkConfigurationParsing(b *testing.B) {
	// Create complex configuration
	cfg := fixtures.CreateMultiTenantConfig()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		router := chi.NewRouter()
		gateway := routing.NewGatewayWithRouter(cfg, router)
		
		if gateway == nil {
			b.Fatal("Failed to create gateway from config")
		}
	}
}

// BenchmarkMemoryUsage benchmarks memory usage patterns
func BenchmarkMemoryUsage(b *testing.B) {
	// Setup environment
	backend := createBenchmarkBackend()
	defer backend.Close()

	cfg := fixtures.CreateTestConfig("memory-bench-tenant", "/memory/")
	cfg.Tenants[0].Services[0].URL = backend.URL

	router := chi.NewRouter()
	gateway := routing.NewGatewayWithRouter(cfg, router)
	handler := createBenchmarkHandler(gateway)

	// Create request data for varying sizes
	mediumData := strings.Repeat("data", 100)
	largeData := strings.Repeat("data", 1000)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		var req *http.Request
		
		switch i % 3 {
		case 0:
			req = httptest.NewRequest("GET", "/memory/small", nil)
		case 1:
			req = httptest.NewRequest("POST", "/memory/medium", strings.NewReader(mediumData))
			req.Header.Set("Content-Type", "text/plain")
			req.ContentLength = int64(len(mediumData))
		case 2:
			req = httptest.NewRequest("POST", "/memory/large", strings.NewReader(largeData))
			req.Header.Set("Content-Type", "text/plain")
			req.ContentLength = int64(len(largeData))
		}
		
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		
		if w.Code != http.StatusOK {
			b.Fatalf("Expected status 200, got %d", w.Code)
		}
	}
}

// Helper function to create benchmark handler
func createBenchmarkHandler(gateway *routing.Gateway) http.Handler {
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

// Helper function to create request with Host header
func createHostRequest(method, path, host string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	req.Host = host
	return req
}

// Helper function to create simple backend for benchmarks
func createBenchmarkBackend() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
}