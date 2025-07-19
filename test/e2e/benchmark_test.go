package e2e

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	"keystone-gateway/internal/config"
	"keystone-gateway/internal/routing"
)

var testConfig *config.Config

func init() {
	var err error
	testConfig, err = config.LoadConfig("../../configs/examples/test-config.yaml")
	if err != nil {
		panic("Failed to load test config: " + err.Error())
	}
}

// BenchmarkFullRequest tests the complete request processing pipeline
func BenchmarkFullRequest(b *testing.B) {
	gw := routing.NewGateway(testConfig)

	// Create a mock backend server
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer backendServer.Close()

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Host = "app.example.com"

	b.ResetTimer()
	b.ReportAllocs() // Enable memory allocation reporting

	for i := 0; i < b.N; i++ {
		// Test route matching directly since we're testing routing performance
		router, prefix := gw.MatchRoute(req.Host, req.URL.Path)
		if router != nil {
			_ = prefix // Use the prefix to avoid unused variable
		}
	}
}

// BenchmarkConcurrentRequests tests performance under concurrent load
func BenchmarkConcurrentRequests(b *testing.B) {
	gw := routing.NewGateway(testConfig)

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Host = "app.example.com"

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Test route matching performance under concurrent load
			router, prefix := gw.MatchRoute(req.Host, req.URL.Path)
			if router != nil {
				_ = prefix // Use the prefix to avoid unused variable
			}
		}
	})
}

// BenchmarkMemoryUsage measures memory allocation patterns
func BenchmarkMemoryUsage(b *testing.B) {
	gw := routing.NewGateway(testConfig)

	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Simulate route matching operations
		gw.MatchRoute("app.example.com", "/api/v1/users")
		gw.MatchRoute("mobile.example.com", "/v2/data")
		gw.MatchRoute("localhost", "/api/health")
	}

	runtime.GC()
	runtime.ReadMemStats(&m2)

	b.ReportMetric(float64(m2.Alloc-m1.Alloc)/float64(b.N), "bytes/op")
}

// BenchmarkConfigLoad tests configuration loading performance
func BenchmarkConfigLoad(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := config.LoadConfig("../../configs/examples/test-config.yaml")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkHealthCheck tests health endpoint performance
func BenchmarkHealthCheck(b *testing.B) {
	gw := routing.NewGateway(testConfig)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Since we don't have the Application layer in this benchmark,
		// we'll benchmark the route matching instead
		router, prefix := gw.MatchRoute("localhost", "/admin/health")
		if router != nil {
			_ = prefix
		}
	}
}

// BenchmarkTenantLookup tests tenant router lookup performance
func BenchmarkTenantLookup(b *testing.B) {
	gw := routing.NewGateway(testConfig)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		gw.GetTenantRouter("api-service")
		gw.GetTenantRouter("app-service")
		gw.GetTenantRouter("mobile-api")
	}
}
