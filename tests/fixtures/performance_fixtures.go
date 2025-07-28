package fixtures

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"keystone-gateway/internal/config"
)

// PerformanceBackendConfig represents configuration for performance testing backends
type PerformanceBackendConfig struct {
	Type           string        // "fast", "slow", "variable", "memory-intensive"
	ResponseSize   int           // Size of response body in bytes
	ProcessingTime time.Duration // Artificial processing delay
	ErrorRate      float64       // Probability of returning an error (0.0 - 1.0)
	MemoryAlloc    int           // Amount of memory to allocate per request
}

// CreatePerformanceBackend creates a backend server configured for performance testing
func CreatePerformanceBackend(t *testing.T, config PerformanceBackendConfig) *httptest.Server {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate processing time
		if config.ProcessingTime > 0 {
			time.Sleep(config.ProcessingTime)
		}

		// Simulate memory allocation
		if config.MemoryAlloc > 0 {
			_ = make([]byte, config.MemoryAlloc)
		}

		// Simulate error rate
		if config.ErrorRate > 0 {
			if time.Now().UnixNano()%1000 < int64(config.ErrorRate*1000) {
				http.Error(w, "Simulated backend error", http.StatusInternalServerError)
				return
			}
		}

		// Generate response based on type
		switch config.Type {
		case "fast":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok","type":"fast","response_time":"minimal"}`))

		case "slow":
			// Additional slow processing
			time.Sleep(50 * time.Millisecond)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok","type":"slow","response_time":"extended"}`))

		case "variable":
			// Variable processing time based on request
			delay := time.Duration(time.Now().UnixNano()%50) * time.Millisecond
			time.Sleep(delay)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(fmt.Sprintf(`{"status":"ok","type":"variable","delay_ms":%d}`, delay.Milliseconds())))

		case "memory-intensive":
			// Allocate and use significant memory
			largeData := make([]byte, 1024*1024) // 1MB
			for i := range largeData {
				largeData[i] = byte(i % 256)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok","type":"memory-intensive","allocated_mb":1}`))

		default:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok","type":"default"}`))
		}

		// Add configured response size padding
		if config.ResponseSize > 0 {
			padding := make([]byte, config.ResponseSize)
			for i := range padding {
				padding[i] = 'x'
			}
			w.Write(padding)
		}
	})

	return httptest.NewServer(handler)
}

// CreatePerfFastBackend creates a backend optimized for speed
func CreatePerfFastBackend(t *testing.T) *httptest.Server {
	return CreatePerformanceBackend(t, PerformanceBackendConfig{
		Type:           "fast",
		ResponseSize:   100,
		ProcessingTime: 0,
		ErrorRate:      0,
		MemoryAlloc:    1024, // 1KB
	})
}

// CreatePerfSlowBackend creates a backend with intentional delays
func CreatePerfSlowBackend(t *testing.T) *httptest.Server {
	return CreatePerformanceBackend(t, PerformanceBackendConfig{
		Type:           "slow",
		ResponseSize:   500,
		ProcessingTime: 100 * time.Millisecond,
		ErrorRate:      0,
		MemoryAlloc:    4096, // 4KB
	})
}

// CreateVariableBackend creates a backend with variable response times
func CreateVariableBackend(t *testing.T) *httptest.Server {
	return CreatePerformanceBackend(t, PerformanceBackendConfig{
		Type:           "variable",
		ResponseSize:   300,
		ProcessingTime: 0, // Variable timing handled in handler
		ErrorRate:      0.05, // 5% error rate
		MemoryAlloc:    2048, // 2KB
	})
}

// CreateMemoryIntensiveBackend creates a backend that uses significant memory
func CreateMemoryIntensiveBackend(t *testing.T) *httptest.Server {
	return CreatePerformanceBackend(t, PerformanceBackendConfig{
		Type:           "memory-intensive",
		ResponseSize:   1000,
		ProcessingTime: 25 * time.Millisecond,
		ErrorRate:      0,
		MemoryAlloc:    1024 * 1024, // 1MB
	})
}

// CreateReliableBackend creates a highly reliable backend for baseline testing
func CreateReliableBackend(t *testing.T) *httptest.Server {
	return CreatePerformanceBackend(t, PerformanceBackendConfig{
		Type:           "fast",
		ResponseSize:   200,
		ProcessingTime: 5 * time.Millisecond,
		ErrorRate:      0, // No errors
		MemoryAlloc:    512, // 512 bytes
	})
}

// CreatePerformanceTestConfig creates a configuration optimized for performance testing
func CreatePerformanceTestConfig(tenantName, pathPrefix string, backends []*httptest.Server) *config.Config {
	services := make([]config.Service, len(backends))
	for i, backend := range backends {
		services[i] = config.Service{
			Name:   fmt.Sprintf("perf-service-%d", i+1),
			URL:    backend.URL,
			Health: "/health",
		}
	}

	return &config.Config{
		Tenants: []config.Tenant{
			{
				Name:       tenantName,
				PathPrefix: pathPrefix,
				Interval:   15, // Fast health checks for performance testing
				Services:   services,
			},
		},
	}
}

// CreateMultiTenantPerformanceConfig creates a multi-tenant configuration for performance testing
func CreateMultiTenantPerformanceConfig() *config.Config {
	return &config.Config{
		Tenants: []config.Tenant{
			{
				Name:     "perf-api-tenant",
				Domains:  []string{"api.perf.example.com"},
				Interval: 30,
				Services: []config.Service{
					{Name: "api-service", URL: "http://localhost:8081", Health: "/health"},
				},
			},
			{
				Name:       "perf-web-tenant",
				PathPrefix: "/web/",
				Interval:   30,
				Services: []config.Service{
					{Name: "web-service", URL: "http://localhost:8082", Health: "/health"},
				},
			},
			{
				Name:       "perf-admin-tenant",
				PathPrefix: "/admin/",
				Interval:   15,
				Services: []config.Service{
					{Name: "admin-service", URL: "http://localhost:8083", Health: "/health"},
				},
			},
		},
	}
}

// PerformanceTestSuite provides utilities for performance testing
type PerformanceTestSuite struct {
	t               *testing.T
	fastBackend     *httptest.Server
	slowBackend     *httptest.Server
	variableBackend *httptest.Server
	memoryBackend   *httptest.Server
}

// NewPerformanceTestSuite creates a new performance test suite with various backend types
func NewPerformanceTestSuite(t *testing.T) *PerformanceTestSuite {
	return &PerformanceTestSuite{
		t:               t,
		fastBackend:     CreatePerfFastBackend(t),
		slowBackend:     CreatePerfSlowBackend(t),
		variableBackend: CreateVariableBackend(t),
		memoryBackend:   CreateMemoryIntensiveBackend(t),
	}
}

// GetFastBackend returns the fast backend for the test suite
func (pts *PerformanceTestSuite) GetFastBackend() *httptest.Server {
	return pts.fastBackend
}

// GetSlowBackend returns the slow backend for the test suite
func (pts *PerformanceTestSuite) GetSlowBackend() *httptest.Server {
	return pts.slowBackend
}

// GetVariableBackend returns the variable backend for the test suite
func (pts *PerformanceTestSuite) GetVariableBackend() *httptest.Server {
	return pts.variableBackend
}

// GetMemoryBackend returns the memory-intensive backend for the test suite
func (pts *PerformanceTestSuite) GetMemoryBackend() *httptest.Server {
	return pts.memoryBackend
}

// GetAllBackends returns all backends in the test suite
func (pts *PerformanceTestSuite) GetAllBackends() []*httptest.Server {
	return []*httptest.Server{
		pts.fastBackend,
		pts.slowBackend,
		pts.variableBackend,
		pts.memoryBackend,
	}
}

// Cleanup closes all backends in the test suite
func (pts *PerformanceTestSuite) Cleanup() {
	if pts.fastBackend != nil {
		pts.fastBackend.Close()
	}
	if pts.slowBackend != nil {
		pts.slowBackend.Close()
	}
	if pts.variableBackend != nil {
		pts.variableBackend.Close()
	}
	if pts.memoryBackend != nil {
		pts.memoryBackend.Close()
	}
}

// CreateLoadTestConfig creates a configuration specifically for load testing
func CreateLoadTestConfig(concurrency int) *config.Config {
	// Create multiple services to distribute load
	services := make([]config.Service, concurrency/10+1) // One service per 10 concurrent requests
	for i := range services {
		services[i] = config.Service{
			Name:   fmt.Sprintf("load-service-%d", i+1),
			URL:    "http://localhost:8080", // Will be replaced by actual backend URLs
			Health: "/health",
		}
	}

	return &config.Config{
		Tenants: []config.Tenant{
			{
				Name:       "load-test-tenant",
				PathPrefix: "/load/",
				Interval:   10, // Very frequent health checks
				Services:   services,
			},
		},
	}
}

// PerformanceMetrics represents performance measurements
type PerformanceMetrics struct {
	RequestsPerSecond   float64
	AverageResponseTime time.Duration
	P95ResponseTime     time.Duration
	P99ResponseTime     time.Duration
	ErrorRate           float64
	MemoryUsageMB       float64
	CPUUsagePercent     float64
}

// BenchmarkResult represents the result of a performance benchmark
type BenchmarkResult struct {
	TestName          string
	TotalRequests     int
	SuccessfulRequests int
	FailedRequests    int
	Duration          time.Duration
	Metrics           PerformanceMetrics
	Timestamp         time.Time
}

// CreateBenchmarkConfig creates a configuration optimized for benchmarking
func CreateBenchmarkConfig(tenantName string) *config.Config {
	return &config.Config{
		Tenants: []config.Tenant{
			{
				Name:       tenantName,
				PathPrefix: fmt.Sprintf("/%s/", tenantName),
				Interval:   60, // Longer intervals for benchmarking
				Services: []config.Service{
					{
						Name:   fmt.Sprintf("%s-service", tenantName),
						URL:    "http://localhost:8080", // Will be replaced
						Health: "/health",
					},
				},
			},
		},
	}
}

// WarmupBackend performs warmup requests to ensure consistent performance measurements
func WarmupBackend(backend *httptest.Server, requests int) error {
	client := &http.Client{Timeout: 5 * time.Second}
	
	for i := 0; i < requests; i++ {
		resp, err := client.Get(backend.URL + "/warmup")
		if err != nil {
			return fmt.Errorf("warmup request %d failed: %w", i, err)
		}
		resp.Body.Close()
		
		// Small delay between warmup requests
		time.Sleep(10 * time.Millisecond)
	}
	
	return nil
}

// CreateScalingTestBackends creates backends for scaling tests
func CreateScalingTestBackends(t *testing.T, count int) []*httptest.Server {
	backends := make([]*httptest.Server, count)
	
	for i := 0; i < count; i++ {
		backends[i] = CreatePerformanceBackend(t, PerformanceBackendConfig{
			Type:           "fast",
			ResponseSize:   100 + (i * 50), // Varying response sizes
			ProcessingTime: time.Duration(i*5) * time.Millisecond, // Varying delays
			ErrorRate:      0,
			MemoryAlloc:    1024 * (i + 1), // Varying memory usage
		})
	}
	
	return backends
}

// CleanupBackends closes multiple backends
func CleanupBackends(backends []*httptest.Server) {
	for _, backend := range backends {
		if backend != nil {
			backend.Close()
		}
	}
}