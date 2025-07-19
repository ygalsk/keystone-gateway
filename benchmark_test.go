package main

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
)

// BenchmarkFullRequest tests the complete request processing pipeline
func BenchmarkFullRequest(b *testing.B) {
	gw := NewGateway(testConfig)
	router := gw.SetupRouter()

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
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

// BenchmarkConcurrentRequests tests performance under concurrent load
func BenchmarkConcurrentRequests(b *testing.B) {
	gw := NewGateway(testConfig)
	router := gw.SetupRouter()

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Host = "app.example.com"

	b.ResetTimer()
	b.ReportAllocs()
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
		}
	})
}

// BenchmarkMemoryUsage measures memory allocation patterns
func BenchmarkMemoryUsage(b *testing.B) {
	gw := NewGateway(testConfig)
	
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
		_, err := LoadConfig("./configs/test-config.yaml")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkHealthCheck tests health endpoint performance
func BenchmarkHealthCheck(b *testing.B) {
	gw := NewGateway(testConfig)
	
	req := httptest.NewRequest("GET", "/admin/health", nil)
	
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		gw.HealthHandler(w, req)
	}
}

// BenchmarkTenantLookup tests tenant router lookup performance  
func BenchmarkTenantLookup(b *testing.B) {
	gw := NewGateway(testConfig)
	
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		gw.getTenantRouter("api-service")
		gw.getTenantRouter("app-service")
		gw.getTenantRouter("mobile-api")
	}
}