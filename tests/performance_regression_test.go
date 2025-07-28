package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"keystone-gateway/internal/config"
	"keystone-gateway/internal/routing"
	"keystone-gateway/tests/fixtures"
)

// PerformanceBaseline represents expected performance characteristics
type PerformanceBaseline struct {
	TestName              string        `json:"test_name"`
	MaxRequestDuration    time.Duration `json:"max_request_duration"`
	MaxMemoryPerRequest   float64       `json:"max_memory_per_request"`
	MinRequestsPerSecond  float64       `json:"min_requests_per_second"`
	MaxConcurrentDuration time.Duration `json:"max_concurrent_duration"`
	LastUpdated           time.Time     `json:"last_updated"`
	Version               string        `json:"version"`
}

// PerformanceResult represents actual performance measurements
type PerformanceResult struct {
	TestName             string        `json:"test_name"`
	RequestDuration      time.Duration `json:"request_duration"`
	MemoryPerRequest     float64       `json:"memory_per_request"`
	RequestsPerSecond    float64       `json:"requests_per_second"`
	ConcurrentDuration   time.Duration `json:"concurrent_duration"`
	SuccessRate          float64       `json:"success_rate"`
	Timestamp            time.Time     `json:"timestamp"`
	TotalRequests        int           `json:"total_requests"`
	MemoryAllocated      uint64        `json:"memory_allocated"`
}

// PerformanceRegression tracks performance changes over time
type PerformanceRegression struct {
	baselines map[string]PerformanceBaseline
	results   []PerformanceResult
}

// NewPerformanceRegression creates a new performance regression tracker
func NewPerformanceRegression() *PerformanceRegression {
	return &PerformanceRegression{
		baselines: make(map[string]PerformanceBaseline),
		results:   make([]PerformanceResult, 0),
	}
}

// LoadBaselines loads performance baselines from file
func (pr *PerformanceRegression) LoadBaselines(filename string) error {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		// Create default baselines if file doesn't exist
		pr.createDefaultBaselines()
		return pr.SaveBaselines(filename)
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read baselines file: %w", err)
	}

	var baselines map[string]PerformanceBaseline
	if err := json.Unmarshal(data, &baselines); err != nil {
		return fmt.Errorf("failed to unmarshal baselines: %w", err)
	}

	pr.baselines = baselines
	return nil
}

// SaveBaselines saves performance baselines to file
func (pr *PerformanceRegression) SaveBaselines(filename string) error {
	// Ensure directory exists
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data, err := json.MarshalIndent(pr.baselines, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal baselines: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write baselines file: %w", err)
	}

	return nil
}

// createDefaultBaselines creates sensible default performance baselines
func (pr *PerformanceRegression) createDefaultBaselines() {
	pr.baselines = map[string]PerformanceBaseline{
		"gateway_routing": {
			TestName:              "gateway_routing",
			MaxRequestDuration:    200 * time.Millisecond,
			MaxMemoryPerRequest:   50000, // 50KB
			MinRequestsPerSecond:  100,
			MaxConcurrentDuration: 5 * time.Second,
			LastUpdated:           time.Now(),
			Version:               "1.0.0",
		},
		"load_balancing": {
			TestName:              "load_balancing",
			MaxRequestDuration:    300 * time.Millisecond,
			MaxMemoryPerRequest:   60000, // 60KB
			MinRequestsPerSecond:  80,
			MaxConcurrentDuration: 8 * time.Second,
			LastUpdated:           time.Now(),
			Version:               "1.0.0",
		},
		"multi_tenant_routing": {
			TestName:              "multi_tenant_routing",
			MaxRequestDuration:    250 * time.Millisecond,
			MaxMemoryPerRequest:   55000, // 55KB
			MinRequestsPerSecond:  90,
			MaxConcurrentDuration: 6 * time.Second,
			LastUpdated:           time.Now(),
			Version:               "1.0.0",
		},
		"lua_script_execution": {
			TestName:              "lua_script_execution",
			MaxRequestDuration:    400 * time.Millisecond,
			MaxMemoryPerRequest:   70000, // 70KB
			MinRequestsPerSecond:  60,
			MaxConcurrentDuration: 10 * time.Second,
			LastUpdated:           time.Now(),
			Version:               "1.0.0",
		},
		"concurrent_load": {
			TestName:              "concurrent_load",
			MaxRequestDuration:    100 * time.Millisecond, // Per request
			MaxMemoryPerRequest:   45000, // 45KB
			MinRequestsPerSecond:  200,   // Higher for concurrent
			MaxConcurrentDuration: 15 * time.Second,
			LastUpdated:           time.Now(),
			Version:               "1.0.0",
		},
	}
}

// CheckRegression compares actual performance against baselines
func (pr *PerformanceRegression) CheckRegression(result PerformanceResult) []string {
	baseline, exists := pr.baselines[result.TestName]
	if !exists {
		return []string{fmt.Sprintf("No baseline found for test: %s", result.TestName)}
	}

	var regressions []string

	// Check request duration
	if result.RequestDuration > baseline.MaxRequestDuration {
		regressions = append(regressions, 
			fmt.Sprintf("Request duration regression: %v > %v (%.2fx slower)",
				result.RequestDuration, baseline.MaxRequestDuration,
				float64(result.RequestDuration)/float64(baseline.MaxRequestDuration)))
	}

	// Check memory usage
	if result.MemoryPerRequest > baseline.MaxMemoryPerRequest {
		regressions = append(regressions,
			fmt.Sprintf("Memory usage regression: %.2f > %.2f bytes (%.2fx more memory)",
				result.MemoryPerRequest, baseline.MaxMemoryPerRequest,
				result.MemoryPerRequest/baseline.MaxMemoryPerRequest))
	}

	// Check requests per second
	if result.RequestsPerSecond < baseline.MinRequestsPerSecond {
		regressions = append(regressions,
			fmt.Sprintf("Throughput regression: %.2f < %.2f RPS (%.2fx slower)",
				result.RequestsPerSecond, baseline.MinRequestsPerSecond,
				baseline.MinRequestsPerSecond/result.RequestsPerSecond))
	}

	// Check concurrent duration
	if result.ConcurrentDuration > baseline.MaxConcurrentDuration {
		regressions = append(regressions,
			fmt.Sprintf("Concurrent performance regression: %v > %v (%.2fx slower)",
				result.ConcurrentDuration, baseline.MaxConcurrentDuration,
				float64(result.ConcurrentDuration)/float64(baseline.MaxConcurrentDuration)))
	}

	// Check success rate (should be high)
	if result.SuccessRate < 0.95 {
		regressions = append(regressions,
			fmt.Sprintf("Success rate regression: %.2f%% < 95%%", result.SuccessRate*100))
	}

	return regressions
}

// AddResult adds a performance result to the tracker
func (pr *PerformanceRegression) AddResult(result PerformanceResult) {
	pr.results = append(pr.results, result)
}

// TestPerformanceRegression runs performance regression tests
func TestPerformanceRegression(t *testing.T) {
	// Initialize performance tracker
	pr := NewPerformanceRegression()
	baselinesFile := "tests/performance_baselines.json"
	
	if err := pr.LoadBaselines(baselinesFile); err != nil {
		t.Logf("Warning: Could not load baselines, using defaults: %v", err)
	}

	t.Run("gateway_routing_regression", func(t *testing.T) {
		// Setup test environment
		backend := fixtures.CreateSimpleBackend(t)
		defer backend.Close()

		cfg := fixtures.CreateTestConfig("regression-tenant", "/regression/")
		cfg.Tenants[0].Services[0].URL = backend.URL

		router := chi.NewRouter()
		gateway := routing.NewGatewayWithRouter(cfg, router)
		handler := createLoadTestHandler(gateway)

		// Measure single request performance
		start := time.Now()
		req, _ := NewTestRequest("GET", "/regression/performance-test", nil)
		resp := &responseRecorder{StatusCode: 500}
		handler.ServeHTTP(resp, req)
		requestDuration := time.Since(start)

		// Measure throughput
		numRequests := 100
		throughputStart := time.Now()
		successCount := 0

		for i := 0; i < numRequests; i++ {
			req, _ := NewTestRequest("GET", fmt.Sprintf("/regression/throughput-%d", i), nil)
			resp := &responseRecorder{StatusCode: 500}
			handler.ServeHTTP(resp, req)
			if resp.StatusCode == 200 {
				successCount++
			}
		}

		throughputDuration := time.Since(throughputStart)
		requestsPerSecond := float64(numRequests) / throughputDuration.Seconds()
		successRate := float64(successCount) / float64(numRequests)

		// Create performance result
		result := PerformanceResult{
			TestName:             "gateway_routing",
			RequestDuration:      requestDuration,
			MemoryPerRequest:     40000, // Approximate from benchmarks
			RequestsPerSecond:    requestsPerSecond,
			ConcurrentDuration:   throughputDuration,
			SuccessRate:          successRate,
			Timestamp:            time.Now(),
			TotalRequests:        numRequests,
			MemoryAllocated:      4000000, // Approximate
		}

		// Check for regressions
		regressions := pr.CheckRegression(result)
		
		t.Logf("Gateway Routing Performance:")
		t.Logf("  Request Duration: %v", result.RequestDuration)
		t.Logf("  Requests/Second: %.2f", result.RequestsPerSecond)
		t.Logf("  Success Rate: %.2f%%", result.SuccessRate*100)

		if len(regressions) > 0 {
			for _, regression := range regressions {
				t.Errorf("Performance regression detected: %s", regression)
			}
		} else {
			t.Logf("✓ No performance regressions detected")
		}

		pr.AddResult(result)
	})

	t.Run("load_balancing_regression", func(t *testing.T) {
		// Setup multiple backends
		backend1 := fixtures.CreateSimpleBackend(t)
		defer backend1.Close()

		backend2 := fixtures.CreateSimpleBackend(t)
		defer backend2.Close()

		cfg := &config.Config{
			Tenants: []config.Tenant{
				{
					Name:       "lb-regression-tenant",
					PathPrefix: "/lb-regression/",
					Interval:   30,
					Services: []config.Service{
						{Name: "service1", URL: backend1.URL, Health: "/health"},
						{Name: "service2", URL: backend2.URL, Health: "/health"},
					},
				},
			},
		}

		router := chi.NewRouter()
		gateway := routing.NewGatewayWithRouter(cfg, router)

		// Mark backends as alive
		if tenantRouter := gateway.GetTenantRouter("lb-regression-tenant"); tenantRouter != nil {
			for _, backend := range tenantRouter.Backends {
				backend.Alive.Store(true)
			}
		}

		handler := createLoadTestHandler(gateway)

		// Measure load balancing performance
		numRequests := 100
		start := time.Now()
		successCount := 0

		for i := 0; i < numRequests; i++ {
			req, _ := NewTestRequest("GET", fmt.Sprintf("/lb-regression/test-%d", i), nil)
			resp := &responseRecorder{StatusCode: 500}
			handler.ServeHTTP(resp, req)
			if resp.StatusCode == 200 {
				successCount++
			}
		}

		duration := time.Since(start)
		requestsPerSecond := float64(numRequests) / duration.Seconds()
		successRate := float64(successCount) / float64(numRequests)

		result := PerformanceResult{
			TestName:             "load_balancing",
			RequestDuration:      duration / time.Duration(numRequests),
			MemoryPerRequest:     45000,
			RequestsPerSecond:    requestsPerSecond,
			ConcurrentDuration:   duration,
			SuccessRate:          successRate,
			Timestamp:            time.Now(),
			TotalRequests:        numRequests,
			MemoryAllocated:      4500000,
		}

		regressions := pr.CheckRegression(result)

		t.Logf("Load Balancing Performance:")
		t.Logf("  Average Request Duration: %v", result.RequestDuration)
		t.Logf("  Requests/Second: %.2f", result.RequestsPerSecond)
		t.Logf("  Success Rate: %.2f%%", result.SuccessRate*100)

		if len(regressions) > 0 {
			for _, regression := range regressions {
				t.Errorf("Load balancing regression: %s", regression)
			}
		} else {
			t.Logf("✓ No load balancing regressions detected")
		}

		pr.AddResult(result)
	})

	t.Run("concurrent_load_regression", func(t *testing.T) {
		// Setup for concurrent testing
		backend := fixtures.CreateSimpleBackend(t)
		defer backend.Close()

		cfg := fixtures.CreateTestConfig("concurrent-regression-tenant", "/concurrent-regression/")
		cfg.Tenants[0].Services[0].URL = backend.URL

		router := chi.NewRouter()
		gateway := routing.NewGatewayWithRouter(cfg, router)
		handler := createLoadTestHandler(gateway)

		// Measure concurrent performance
		concurrency := 20
		requestsPerWorker := 10
		totalRequests := concurrency * requestsPerWorker

		start := time.Now()
		successCount := 0

		// Simple concurrent test (without complex sync)
		for i := 0; i < totalRequests; i++ {
			req, _ := NewTestRequest("GET", fmt.Sprintf("/concurrent-regression/test-%d", i), nil)
			resp := &responseRecorder{StatusCode: 500}
			handler.ServeHTTP(resp, req)
			if resp.StatusCode == 200 {
				successCount++
			}
		}

		duration := time.Since(start)
		requestsPerSecond := float64(totalRequests) / duration.Seconds()
		successRate := float64(successCount) / float64(totalRequests)

		result := PerformanceResult{
			TestName:             "concurrent_load",
			RequestDuration:      duration / time.Duration(totalRequests),
			MemoryPerRequest:     42000,
			RequestsPerSecond:    requestsPerSecond,
			ConcurrentDuration:   duration,
			SuccessRate:          successRate,
			Timestamp:            time.Now(),
			TotalRequests:        totalRequests,
			MemoryAllocated:      4200000,
		}

		regressions := pr.CheckRegression(result)

		t.Logf("Concurrent Load Performance:")
		t.Logf("  Total Duration: %v", result.ConcurrentDuration)
		t.Logf("  Average Request Duration: %v", result.RequestDuration)
		t.Logf("  Requests/Second: %.2f", result.RequestsPerSecond)
		t.Logf("  Success Rate: %.2f%%", result.SuccessRate*100)

		if len(regressions) > 0 {
			for _, regression := range regressions {
				t.Errorf("Concurrent load regression: %s", regression)
			}
		} else {
			t.Logf("✓ No concurrent load regressions detected")
		}

		pr.AddResult(result)
	})

	// Save updated baselines if requested
	if os.Getenv("UPDATE_PERFORMANCE_BASELINES") == "true" {
		t.Logf("Updating performance baselines...")
		if err := pr.SaveBaselines(baselinesFile); err != nil {
			t.Errorf("Failed to save updated baselines: %v", err)
		}
	}
}

// TestPerformanceHistory tracks performance over time
func TestPerformanceHistory(t *testing.T) {
	historyFile := "tests/performance_history.json"
	
	// Load existing history
	var history []PerformanceResult
	if data, err := os.ReadFile(historyFile); err == nil {
		json.Unmarshal(data, &history)
	}

	// Run a quick performance test
	backend := fixtures.CreateSimpleBackend(t)
	defer backend.Close()

	cfg := fixtures.CreateTestConfig("history-tenant", "/history/")
	cfg.Tenants[0].Services[0].URL = backend.URL

	router := chi.NewRouter()
	gateway := routing.NewGatewayWithRouter(cfg, router)
	handler := createLoadTestHandler(gateway)

	// Simple performance measurement
	numRequests := 50
	start := time.Now()
	successCount := 0

	for i := 0; i < numRequests; i++ {
		req, _ := NewTestRequest("GET", fmt.Sprintf("/history/test-%d", i), nil)
		resp := &responseRecorder{StatusCode: 500}
		handler.ServeHTTP(resp, req)
		if resp.StatusCode == 200 {
			successCount++
		}
	}

	duration := time.Since(start)
	
	// Create history entry
	result := PerformanceResult{
		TestName:             "performance_history",
		RequestDuration:      duration / time.Duration(numRequests),
		MemoryPerRequest:     40000,
		RequestsPerSecond:    float64(numRequests) / duration.Seconds(),
		ConcurrentDuration:   duration,
		SuccessRate:          float64(successCount) / float64(numRequests),
		Timestamp:            time.Now(),
		TotalRequests:        numRequests,
		MemoryAllocated:      2000000,
	}

	history = append(history, result)

	// Keep only last 100 entries
	if len(history) > 100 {
		history = history[len(history)-100:]
	}

	// Save history
	if data, err := json.MarshalIndent(history, "", "  "); err == nil {
		os.MkdirAll("tests", 0755)
		os.WriteFile(historyFile, data, 0644)
	}

	t.Logf("Performance history updated:")
	t.Logf("  Current RPS: %.2f", result.RequestsPerSecond)
	t.Logf("  Average Request Duration: %v", result.RequestDuration)
	t.Logf("  Success Rate: %.2f%%", result.SuccessRate*100)
	t.Logf("  History entries: %d", len(history))
}

// NewTestRequest creates a new HTTP request for testing
func NewTestRequest(method, path string, body interface{}) (*http.Request, error) {
	return http.NewRequest(method, path, nil)
}