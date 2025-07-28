package tests

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"keystone-gateway/internal/config"
	"keystone-gateway/internal/routing"
	"keystone-gateway/tests/fixtures"
)

// LoadTestResult contains metrics from a load test run
type LoadTestResult struct {
	TotalRequests      int64
	SuccessfulRequests int64
	FailedRequests     int64
	Duration           time.Duration
	RequestsPerSecond  float64
	SuccessRate        float64

	// Latency metrics (in milliseconds)
	LatencyP50  float64
	LatencyP95  float64
	LatencyP99  float64
	LatencyMean float64
	LatencyMax  float64
	LatencyMin  float64

	// Memory metrics
	MemoryBefore uint64
	MemoryAfter  uint64
	MemoryDelta  int64
	GCCount      uint32

	// Connection metrics (when available)
	ConnectionsCreated  int64
	ConnectionsReused   int64
	ConnectionPoolStats map[string]interface{}
}

// LatencyTracker tracks request latencies efficiently
type LatencyTracker struct {
	latencies []time.Duration
	mutex     sync.Mutex
}

func NewLatencyTracker() *LatencyTracker {
	return &LatencyTracker{
		latencies: make([]time.Duration, 0, 10000), // Pre-allocate for 10k samples
	}
}

func (lt *LatencyTracker) Record(latency time.Duration) {
	lt.mutex.Lock()
	lt.latencies = append(lt.latencies, latency)
	lt.mutex.Unlock()
}

func (lt *LatencyTracker) GetPercentiles() (p50, p95, p99, mean, min, max float64) {
	lt.mutex.Lock()
	defer lt.mutex.Unlock()

	if len(lt.latencies) == 0 {
		return 0, 0, 0, 0, 0, 0
	}

	// Sort latencies for percentile calculation
	sort.Slice(lt.latencies, func(i, j int) bool {
		return lt.latencies[i] < lt.latencies[j]
	})

	n := len(lt.latencies)

	// Calculate percentiles
	p50 = float64(lt.latencies[n*50/100]) / float64(time.Millisecond)
	p95 = float64(lt.latencies[n*95/100]) / float64(time.Millisecond)
	p99 = float64(lt.latencies[n*99/100]) / float64(time.Millisecond)
	min = float64(lt.latencies[0]) / float64(time.Millisecond)
	max = float64(lt.latencies[n-1]) / float64(time.Millisecond)

	// Calculate mean
	var sum time.Duration
	for _, lat := range lt.latencies {
		sum += lat
	}
	mean = float64(sum) / float64(n) / float64(time.Millisecond)

	return p50, p95, p99, mean, min, max
}

// TestImprovedLoadTesting contains the new comprehensive load tests
func TestImprovedLoadTesting(t *testing.T) {

	t.Run("baseline_performance_test", func(t *testing.T) {
		result := runLoadTest(t, LoadTestConfig{
			Name:                  "baseline",
			Concurrency:           10,
			RequestsPerWorker:     100,
			Duration:              0, // Fixed number of requests
			PathPrefix:            "/baseline/",
			BackendCount:          1,
			EnableLatencyTracking: true,
		})

		logLoadTestResult(t, result)

		// Baseline expectations
		if result.SuccessRate < 0.99 {
			t.Errorf("Baseline success rate too low: %.2f%% (expected >= 99%%)", result.SuccessRate*100)
		}

		if result.RequestsPerSecond < 500 {
			t.Errorf("Baseline RPS too low: %.2f (expected >= 500)", result.RequestsPerSecond)
		}

		if result.LatencyP95 > 50 {
			t.Errorf("Baseline P95 latency too high: %.2fms (expected <= 50ms)", result.LatencyP95)
		}
	})

	t.Run("moderate_load_test", func(t *testing.T) {
		result := runLoadTest(t, LoadTestConfig{
			Name:                  "moderate_load",
			Concurrency:           50,
			RequestsPerWorker:     50,
			Duration:              0,
			PathPrefix:            "/moderate/",
			BackendCount:          2,
			EnableLatencyTracking: true,
		})

		logLoadTestResult(t, result)

		// Moderate load expectations
		if result.SuccessRate < 0.95 {
			t.Errorf("Moderate load success rate too low: %.2f%% (expected >= 95%%)", result.SuccessRate*100)
		}

		if result.RequestsPerSecond < 1000 {
			t.Errorf("Moderate load RPS too low: %.2f (expected >= 1000)", result.RequestsPerSecond)
		}
	})

	t.Run("high_concurrency_test", func(t *testing.T) {
		result := runLoadTest(t, LoadTestConfig{
			Name:                  "high_concurrency",
			Concurrency:           200,
			RequestsPerWorker:     25,
			Duration:              0,
			PathPrefix:            "/high/",
			BackendCount:          3,
			EnableLatencyTracking: true,
		})

		logLoadTestResult(t, result)

		// High concurrency expectations - more lenient
		if result.SuccessRate < 0.90 {
			t.Errorf("High concurrency success rate too low: %.2f%% (expected >= 90%%)", result.SuccessRate*100)
		}

		if result.LatencyP99 > 500 {
			t.Errorf("High concurrency P99 latency too high: %.2fms (expected <= 500ms)", result.LatencyP99)
		}
	})

	t.Run("sustained_load_test", func(t *testing.T) {
		result := runLoadTest(t, LoadTestConfig{
			Name:                  "sustained",
			Concurrency:           30,
			RequestsPerWorker:     0, // Use duration instead
			Duration:              10 * time.Second,
			PathPrefix:            "/sustained/",
			BackendCount:          2,
			EnableLatencyTracking: true,
			RequestDelay:          5 * time.Millisecond, // Steady rate
		})

		logLoadTestResult(t, result)

		// Sustained load expectations
		if result.SuccessRate < 0.95 {
			t.Errorf("Sustained load success rate too low: %.2f%% (expected >= 95%%)", result.SuccessRate*100)
		}

		expectedMinRequests := int64(1000) // Should handle at least 100 RPS
		if result.TotalRequests < expectedMinRequests {
			t.Errorf("Sustained load processed too few requests: %d (expected >= %d)",
				result.TotalRequests, expectedMinRequests)
		}
	})

	t.Run("connection_pooling_validation", func(t *testing.T) {
		// Test that validates connection pooling is working
		result := runLoadTest(t, LoadTestConfig{
			Name:                  "connection_pooling",
			Concurrency:           100,
			RequestsPerWorker:     10,
			Duration:              0,
			PathPrefix:            "/pooling/",
			BackendCount:          1,
			EnableLatencyTracking: true,
			ValidateConnections:   true,
		})

		logLoadTestResult(t, result)

		// With connection pooling, latency should be consistent
		latencyVariation := result.LatencyP95 - result.LatencyP50
		if latencyVariation > 100 { // 100ms variation
			t.Errorf("High latency variation suggests connection issues: P95-P50 = %.2fms (expected <= 100ms)",
				latencyVariation)
		}

		t.Logf("Connection pooling metrics: %+v", result.ConnectionPoolStats)
	})

	t.Run("memory_efficiency_test", func(t *testing.T) {
		result := runLoadTest(t, LoadTestConfig{
			Name:                  "memory_efficiency",
			Concurrency:           50,
			RequestsPerWorker:     100,
			Duration:              0,
			PathPrefix:            "/memory/",
			BackendCount:          2,
			EnableLatencyTracking: false, // Reduce memory overhead for this test
			TrackMemory:           true,
		})

		logLoadTestResult(t, result)

		// Memory efficiency checks
		memoryPerRequest := float64(result.MemoryDelta) / float64(result.TotalRequests)
		maxMemoryPerRequest := 10000.0 // 10KB per request max

		if result.MemoryDelta > 0 && memoryPerRequest > maxMemoryPerRequest {
			t.Errorf("High memory usage per request: %.2f bytes (expected <= %.2f)",
				memoryPerRequest, maxMemoryPerRequest)
		}

		t.Logf("Memory efficiency: %.2f bytes per request", memoryPerRequest)
	})

	t.Run("error_handling_resilience", func(t *testing.T) {
		// Test with backends that return errors
		result := runLoadTestWithErrorBackend(t, LoadTestConfig{
			Name:                  "error_resilience",
			Concurrency:           30,
			RequestsPerWorker:     50,
			Duration:              0,
			PathPrefix:            "/errors/",
			BackendCount:          1,
			EnableLatencyTracking: true,
		})

		logLoadTestResult(t, result)

		// With error backend, we expect failures but gateway should handle gracefully
		if result.SuccessRate > 0.10 { // Error backend should fail most requests
			t.Logf("Note: Error backend had %.2f%% success rate (expected low)", result.SuccessRate*100)
		}

		// More importantly, check that errors don't cause excessive memory usage
		if result.MemoryDelta > 0 {
			memoryPerRequest := float64(result.MemoryDelta) / float64(result.TotalRequests)
			if memoryPerRequest > 15000 { // 15KB per request max during errors
				t.Errorf("High memory usage during errors: %.2f bytes per request", memoryPerRequest)
			}
		}
	})

	t.Run("load_balancing_efficiency", func(t *testing.T) {
		result := runLoadTest(t, LoadTestConfig{
			Name:                  "load_balancing",
			Concurrency:           40,
			RequestsPerWorker:     25,
			Duration:              0,
			PathPrefix:            "/lb/",
			BackendCount:          4, // Multiple backends to test distribution
			EnableLatencyTracking: true,
		})

		logLoadTestResult(t, result)

		// Load balancing should maintain good performance
		if result.SuccessRate < 0.95 {
			t.Errorf("Load balancing success rate too low: %.2f%% (expected >= 95%%)", result.SuccessRate*100)
		}

		// With multiple backends, average latency should be reasonable
		if result.LatencyMean > 100 {
			t.Errorf("Load balancing mean latency too high: %.2fms (expected <= 100ms)", result.LatencyMean)
		}
	})
}

// LoadTestConfig defines parameters for a load test
type LoadTestConfig struct {
	Name                  string
	Concurrency           int
	RequestsPerWorker     int
	Duration              time.Duration // If > 0, use time-based instead of request count
	PathPrefix            string
	BackendCount          int
	EnableLatencyTracking bool
	TrackMemory           bool
	RequestDelay          time.Duration
	ValidateConnections   bool
}

// runLoadTest executes a load test with the given configuration
func runLoadTest(t *testing.T, cfg LoadTestConfig) *LoadTestResult {
	// Setup backends
	backends := make([]*httptest.Server, cfg.BackendCount)
	services := make([]config.Service, cfg.BackendCount)

	for i := 0; i < cfg.BackendCount; i++ {
		backends[i] = fixtures.CreateSimpleBackend(t)
		defer backends[i].Close()

		services[i] = config.Service{
			Name:   fmt.Sprintf("service%d", i+1),
			URL:    backends[i].URL,
			Health: "/health",
		}
	}

	// Create gateway configuration
	gatewayConfig := &config.Config{
		Tenants: []config.Tenant{
			{
				Name:       fmt.Sprintf("%s-tenant", cfg.Name),
				PathPrefix: cfg.PathPrefix,
				Interval:   30,
				Services:   services,
			},
		},
	}

	router := chi.NewRouter()
	gateway := routing.NewGatewayWithRouter(gatewayConfig, router)

	// Mark all backends as alive
	if tenantRouter := gateway.GetTenantRouter(fmt.Sprintf("%s-tenant", cfg.Name)); tenantRouter != nil {
		for _, backend := range tenantRouter.Backends {
			backend.Alive.Store(true)
		}
	}

	// Initialize metrics
	var totalRequests, successCount, errorCount int64
	var latencyTracker *LatencyTracker
	if cfg.EnableLatencyTracking {
		latencyTracker = NewLatencyTracker()
	}

	// Memory tracking
	var beforeStats, afterStats runtime.MemStats
	if cfg.TrackMemory {
		runtime.GC()
		runtime.GC()
		runtime.ReadMemStats(&beforeStats)
	}

	// Determine test duration and worker behavior
	var ctx context.Context
	var cancel context.CancelFunc
	useTimeBasedTest := cfg.Duration > 0

	if useTimeBasedTest {
		ctx, cancel = context.WithTimeout(context.Background(), cfg.Duration)
		defer cancel()
	} else {
		ctx, cancel = context.WithCancel(context.Background())
		defer cancel()
	}

	// Worker function
	worker := func(wg *sync.WaitGroup, workerID int) {
		defer wg.Done()
		requestNum := 0

		for {
			// Check termination conditions
			if useTimeBasedTest {
				select {
				case <-ctx.Done():
					return
				default:
				}
			} else {
				if requestNum >= cfg.RequestsPerWorker {
					return
				}
			}

			atomic.AddInt64(&totalRequests, 1)
			requestNum++

			// Create request
			path := fmt.Sprintf("%sworker-%d-req-%d", cfg.PathPrefix, workerID, requestNum)
			req, err := http.NewRequest("GET", path, nil)
			if err != nil {
				atomic.AddInt64(&errorCount, 1)
				continue
			}

			// Measure latency
			start := time.Now()
			resp := &optimizedResponseRecorder{}

			// Use the actual handler logic inline for performance
			// Validate path length
			if len(req.URL.Path) > 1024 {
				resp.WriteHeader(http.StatusNotFound)
			} else {
				tenantRouter, stripPrefix := gateway.MatchRoute(req.Host, req.URL.Path)
				if tenantRouter == nil {
					resp.WriteHeader(http.StatusNotFound)
				} else {
					backend := tenantRouter.NextBackend()
					if backend == nil {
						resp.WriteHeader(http.StatusBadGateway)
					} else {
						proxy := gateway.CreateProxy(backend, stripPrefix)
						proxy.ServeHTTP(resp, req)
					}
				}
			}

			latency := time.Since(start)

			// Record metrics
			if cfg.EnableLatencyTracking {
				latencyTracker.Record(latency)
			}

			if resp.statusCode == 200 {
				atomic.AddInt64(&successCount, 1)
			} else {
				atomic.AddInt64(&errorCount, 1)
			}

			// Optional delay between requests
			if cfg.RequestDelay > 0 {
				time.Sleep(cfg.RequestDelay)
			}
		}
	}

	// Execute load test
	start := time.Now()
	var wg sync.WaitGroup

	for i := 0; i < cfg.Concurrency; i++ {
		wg.Add(1)
		go worker(&wg, i)
	}

	wg.Wait()
	duration := time.Since(start)

	// Final memory measurement
	if cfg.TrackMemory {
		runtime.GC()
		runtime.GC()
		runtime.ReadMemStats(&afterStats)
	}

	// Compile results
	result := &LoadTestResult{
		TotalRequests:      atomic.LoadInt64(&totalRequests),
		SuccessfulRequests: atomic.LoadInt64(&successCount),
		FailedRequests:     atomic.LoadInt64(&errorCount),
		Duration:           duration,
		RequestsPerSecond:  float64(atomic.LoadInt64(&totalRequests)) / duration.Seconds(),
	}

	if result.TotalRequests > 0 {
		result.SuccessRate = float64(result.SuccessfulRequests) / float64(result.TotalRequests)
	}

	// Latency metrics
	if cfg.EnableLatencyTracking && latencyTracker != nil {
		result.LatencyP50, result.LatencyP95, result.LatencyP99,
			result.LatencyMean, result.LatencyMin, result.LatencyMax = latencyTracker.GetPercentiles()
	}

	// Memory metrics
	if cfg.TrackMemory {
		result.MemoryBefore = beforeStats.Alloc
		result.MemoryAfter = afterStats.Alloc
		if afterStats.Alloc >= beforeStats.Alloc {
			result.MemoryDelta = int64(afterStats.Alloc - beforeStats.Alloc)
		} else {
			result.MemoryDelta = -int64(beforeStats.Alloc - afterStats.Alloc)
		}
		result.GCCount = afterStats.NumGC - beforeStats.NumGC
	}

	// Connection metrics (if supported by gateway)
	if cfg.ValidateConnections {
		result.ConnectionPoolStats = gateway.GetTransportStats()
	}

	return result
}

// runLoadTestWithErrorBackend runs a load test with backends that return errors
func runLoadTestWithErrorBackend(t *testing.T, cfg LoadTestConfig) *LoadTestResult {
	// Create error backend
	backend := fixtures.CreateErrorBackend(t)
	defer backend.Close()

	// Create gateway configuration
	gatewayConfig := &config.Config{
		Tenants: []config.Tenant{
			{
				Name:       fmt.Sprintf("%s-tenant", cfg.Name),
				PathPrefix: cfg.PathPrefix,
				Interval:   30,
				Services: []config.Service{
					{Name: "error-service", URL: backend.URL, Health: "/health"},
				},
			},
		},
	}

	router := chi.NewRouter()
	gateway := routing.NewGatewayWithRouter(gatewayConfig, router)

	// Initialize metrics
	var totalRequests, successCount, errorCount int64

	// Memory tracking
	var beforeStats, afterStats runtime.MemStats
	runtime.GC()
	runtime.GC()
	runtime.ReadMemStats(&beforeStats)

	// Worker function for error testing
	worker := func(wg *sync.WaitGroup, workerID int) {
		defer wg.Done()

		for i := 0; i < cfg.RequestsPerWorker; i++ {
			atomic.AddInt64(&totalRequests, 1)

			// Create request
			path := fmt.Sprintf("%serror-worker-%d-req-%d", cfg.PathPrefix, workerID, i)
			req, err := http.NewRequest("GET", path, nil)
			if err != nil {
				atomic.AddInt64(&errorCount, 1)
				continue
			}

			resp := &optimizedResponseRecorder{}

			// Handle request inline
			tenantRouter, stripPrefix := gateway.MatchRoute(req.Host, req.URL.Path)
			if tenantRouter == nil {
				resp.WriteHeader(http.StatusNotFound)
				atomic.AddInt64(&errorCount, 1)
			} else {
				gtwBackend := tenantRouter.NextBackend()
				if gtwBackend == nil {
					resp.WriteHeader(http.StatusBadGateway)
					atomic.AddInt64(&errorCount, 1)
				} else {
					proxy := gateway.CreateProxy(gtwBackend, stripPrefix)
					proxy.ServeHTTP(resp, req)

					if resp.statusCode == 200 {
						atomic.AddInt64(&successCount, 1)
					} else {
						atomic.AddInt64(&errorCount, 1)
					}
				}
			}
		}
	}

	// Execute load test
	start := time.Now()
	var wg sync.WaitGroup

	for i := 0; i < cfg.Concurrency; i++ {
		wg.Add(1)
		go worker(&wg, i)
	}

	wg.Wait()
	duration := time.Since(start)

	// Final memory measurement
	runtime.GC()
	runtime.GC()
	runtime.ReadMemStats(&afterStats)

	// Compile results
	total := atomic.LoadInt64(&totalRequests)
	success := atomic.LoadInt64(&successCount)
	failed := atomic.LoadInt64(&errorCount)

	result := &LoadTestResult{
		TotalRequests:      total,
		SuccessfulRequests: success,
		FailedRequests:     failed,
		Duration:           duration,
		RequestsPerSecond:  float64(total) / duration.Seconds(),
	}

	if total > 0 {
		result.SuccessRate = float64(success) / float64(total)
	}

	// Memory metrics
	result.MemoryBefore = beforeStats.Alloc
	result.MemoryAfter = afterStats.Alloc
	if afterStats.Alloc >= beforeStats.Alloc {
		result.MemoryDelta = int64(afterStats.Alloc - beforeStats.Alloc)
	} else {
		result.MemoryDelta = -int64(beforeStats.Alloc - afterStats.Alloc)
	}
	result.GCCount = afterStats.NumGC - beforeStats.NumGC

	return result
}

// optimizedResponseRecorder is a more efficient response recorder for load testing
type optimizedResponseRecorder struct {
	statusCode int
	written    int
}

func (r *optimizedResponseRecorder) Header() http.Header {
	// Return minimal header for performance
	return make(http.Header)
}

func (r *optimizedResponseRecorder) Write(data []byte) (int, error) {
	if r.statusCode == 0 {
		r.statusCode = 200
	}
	r.written += len(data)
	return len(data), nil
}

func (r *optimizedResponseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
}

// logLoadTestResult logs comprehensive test results
func logLoadTestResult(t *testing.T, result *LoadTestResult) {
	t.Logf("=== %s Load Test Results ===", "Load Test")
	t.Logf("Performance Metrics:")
	t.Logf("  Total Requests: %d", result.TotalRequests)
	t.Logf("  Successful: %d (%.2f%%)", result.SuccessfulRequests, result.SuccessRate*100)
	t.Logf("  Failed: %d", result.FailedRequests)
	t.Logf("  Duration: %v", result.Duration)
	t.Logf("  Requests/sec: %.2f", result.RequestsPerSecond)

	if result.LatencyP50 > 0 {
		t.Logf("Latency Metrics (ms):")
		t.Logf("  Mean: %.2f", result.LatencyMean)
		t.Logf("  P50:  %.2f", result.LatencyP50)
		t.Logf("  P95:  %.2f", result.LatencyP95)
		t.Logf("  P99:  %.2f", result.LatencyP99)
		t.Logf("  Min:  %.2f", result.LatencyMin)
		t.Logf("  Max:  %.2f", result.LatencyMax)
	}

	if result.MemoryBefore > 0 {
		t.Logf("Memory Metrics:")
		t.Logf("  Before: %d bytes", result.MemoryBefore)
		t.Logf("  After:  %d bytes", result.MemoryAfter)
		t.Logf("  Delta:  %d bytes", result.MemoryDelta)
		t.Logf("  GC Count: %d", result.GCCount)
		if result.TotalRequests > 0 {
			memPerReq := float64(result.MemoryDelta) / float64(result.TotalRequests)
			t.Logf("  Memory/Request: %.2f bytes", memPerReq)
		}
	}

	if len(result.ConnectionPoolStats) > 0 {
		t.Logf("Connection Pool Stats: %+v", result.ConnectionPoolStats)
	}

	t.Logf("=====================================")
}
