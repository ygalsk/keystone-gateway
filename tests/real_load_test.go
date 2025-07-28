package tests

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"keystone-gateway/internal/config"
	"keystone-gateway/internal/routing"
)

// RealisticTestResult contains metrics from realistic production-like tests
type RealisticTestResult struct {
	TotalRequests      int64
	SuccessfulRequests int64
	FailedRequests     int64
	TimeoutErrors      int64
	NetworkErrors      int64
	Duration           time.Duration
	RequestsPerSecond  float64
	SuccessRate        float64

	// Realistic latency metrics (milliseconds)
	LatencyP50    float64
	LatencyP95    float64
	LatencyP99    float64
	LatencyP999   float64
	LatencyMean   float64
	LatencyStdDev float64

	// Memory efficiency
	MemoryUsedMB     float64
	MemoryPerRequest float64
	GCPausesTotal    time.Duration
	GCCount          uint32

	// Connection metrics
	ConnectionsActive   int64
	ConnectionsCreated  int64
	ConnectionsReused   int64
	ConnectionPoolUsage float64

	// Throughput metrics
	BytesPerSecond  float64
	RequestsPerCore float64

	// Error distribution
	StatusCode2xx int64
	StatusCode4xx int64
	StatusCode5xx int64

	// Backend distribution (for load balancing validation)
	BackendDistribution map[string]int64
}

// RealisticBackend simulates a real API backend
type RealisticBackend struct {
	server *http.Server
	port   int
	id     string
	stats  BackendStats
}

type BackendStats struct {
	RequestCount int64
	TotalLatency time.Duration
	ErrorCount   int64
}

// TestRealisticProductionLoadTesting runs production-like load tests
func TestRealisticProductionLoadTesting(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping realistic production tests in short mode")
	}

	t.Run("realistic_baseline_performance", func(t *testing.T) {
		result := runRealisticLoadTest(t, RealisticTestConfig{
			Name:                  "realistic_baseline",
			Concurrency:           20,
			TotalRequests:         2000,
			Duration:              0,
			BackendLatencyMin:     10 * time.Millisecond, // Real API latency
			BackendLatencyMax:     50 * time.Millisecond,
			BackendErrorRate:      0.01, // 1% error rate
			RequestBodySizeBytes:  512,  // 512 byte requests
			ResponseBodySizeBytes: 1024, // 1KB responses
			ConnectionPoolSize:    50,
			KeepAliveEnabled:      true,
			RequestRate:           0, // No rate limiting for baseline
		})

		logRealisticTestResult(t, result)

		// Realistic baseline expectations
		if result.SuccessRate < 0.95 {
			t.Errorf("Realistic baseline success rate too low: %.2f%% (expected >= 95%%)", result.SuccessRate*100)
		}

		// Should handle at least 200 RPS with realistic backend latency
		if result.RequestsPerSecond < 200 {
			t.Errorf("Realistic baseline RPS too low: %.2f (expected >= 200)", result.RequestsPerSecond)
		}

		// P95 should be reasonable with real backend latency
		if result.LatencyP95 > 200 {
			t.Errorf("Realistic baseline P95 latency too high: %.2fms (expected <= 200ms)", result.LatencyP95)
		}

		// Connection pooling should work
		if result.ConnectionsReused == 0 {
			t.Error("No connection reuse detected - connection pooling not working")
		}

		t.Logf("✅ Realistic baseline: %.0f RPS, P95: %.1fms, %.1f%% conn reuse",
			result.RequestsPerSecond, result.LatencyP95,
			float64(result.ConnectionsReused)/float64(result.ConnectionsCreated+result.ConnectionsReused)*100)
	})

	t.Run("realistic_sustained_load", func(t *testing.T) {
		result := runRealisticLoadTest(t, RealisticTestConfig{
			Name:                  "realistic_sustained",
			Concurrency:           30,
			TotalRequests:         0,                // Use duration instead
			Duration:              60 * time.Second, // 1 minute sustained
			BackendLatencyMin:     15 * time.Millisecond,
			BackendLatencyMax:     80 * time.Millisecond,
			BackendErrorRate:      0.02, // 2% error rate
			RequestBodySizeBytes:  1024, // 1KB requests
			ResponseBodySizeBytes: 2048, // 2KB responses
			ConnectionPoolSize:    100,
			KeepAliveEnabled:      true,
			RequestRate:           20, // 20 req/sec per worker = 600 req/sec total
		})

		logRealisticTestResult(t, result)

		// Sustained load expectations
		if result.SuccessRate < 0.95 {
			t.Errorf("Sustained load success rate too low: %.2f%% (expected >= 95%%)", result.SuccessRate*100)
		}

		// Should maintain performance over time
		if result.RequestsPerSecond < 400 {
			t.Errorf("Sustained load RPS too low: %.2f (expected >= 400)", result.RequestsPerSecond)
		}

		// Memory should be stable over time
		if result.MemoryPerRequest > 2000 { // 2KB per request max
			t.Errorf("High memory usage: %.0f bytes per request (expected <= 2000)", result.MemoryPerRequest)
		}

		t.Logf("✅ Sustained load: %.0f RPS over %.0fs, %.1f%% success",
			result.RequestsPerSecond, result.Duration.Seconds(), result.SuccessRate*100)
	})

	t.Run("realistic_high_concurrency", func(t *testing.T) {
		result := runRealisticLoadTest(t, RealisticTestConfig{
			Name:                  "realistic_high_concurrency",
			Concurrency:           100, // High concurrency
			TotalRequests:         5000,
			Duration:              0,
			BackendLatencyMin:     5 * time.Millisecond,
			BackendLatencyMax:     100 * time.Millisecond,
			BackendErrorRate:      0.03, // 3% error rate under load
			RequestBodySizeBytes:  256,
			ResponseBodySizeBytes: 512,
			ConnectionPoolSize:    200,
			KeepAliveEnabled:      true,
			RequestRate:           0, // No rate limiting
		})

		logRealisticTestResult(t, result)

		// High concurrency expectations - more lenient
		if result.SuccessRate < 0.90 {
			t.Errorf("High concurrency success rate too low: %.2f%% (expected >= 90%%)", result.SuccessRate*100)
		}

		// Should still achieve reasonable throughput
		if result.RequestsPerSecond < 500 {
			t.Errorf("High concurrency RPS too low: %.2f (expected >= 500)", result.RequestsPerSecond)
		}

		// P99 latency should be reasonable
		if result.LatencyP99 > 500 {
			t.Errorf("High concurrency P99 latency too high: %.2fms (expected <= 500ms)", result.LatencyP99)
		}

		t.Logf("✅ High concurrency: %.0f RPS with %d concurrent workers",
			result.RequestsPerSecond, 100)
	})

	t.Run("realistic_load_balancing", func(t *testing.T) {
		result := runRealisticLoadTest(t, RealisticTestConfig{
			Name:                  "realistic_load_balancing",
			Concurrency:           40,
			TotalRequests:         4000,
			Duration:              0,
			BackendCount:          4, // Multiple backends
			BackendLatencyMin:     20 * time.Millisecond,
			BackendLatencyMax:     60 * time.Millisecond,
			BackendErrorRate:      0.01,
			RequestBodySizeBytes:  512,
			ResponseBodySizeBytes: 1024,
			ConnectionPoolSize:    150,
			KeepAliveEnabled:      true,
			RequestRate:           0,
			ValidateLoadBalancing: true,
		})

		logRealisticTestResult(t, result)

		// Load balancing should maintain performance
		if result.SuccessRate < 0.95 {
			t.Errorf("Load balancing success rate too low: %.2f%% (expected >= 95%%)", result.SuccessRate*100)
		}

		// Check load distribution
		if len(result.BackendDistribution) > 1 {
			minRequests := int64(math.MaxInt64)
			maxRequests := int64(0)

			for _, count := range result.BackendDistribution {
				if count < minRequests {
					minRequests = count
				}
				if count > maxRequests {
					maxRequests = count
				}
			}

			// Load should be reasonably balanced (within 20% difference)
			imbalanceRatio := float64(maxRequests-minRequests) / float64(maxRequests)
			if imbalanceRatio > 0.3 {
				t.Errorf("Load balancing too uneven: %.1f%% imbalance (expected <= 30%%)",
					imbalanceRatio*100)
			}

			t.Logf("✅ Load balancing distribution: %v", result.BackendDistribution)
		}
	})

	t.Run("realistic_stress_test", func(t *testing.T) {
		result := runRealisticLoadTest(t, RealisticTestConfig{
			Name:                  "realistic_stress",
			Concurrency:           200, // Very high concurrency
			TotalRequests:         10000,
			Duration:              0,
			BackendLatencyMin:     1 * time.Millisecond,
			BackendLatencyMax:     200 * time.Millisecond,
			BackendErrorRate:      0.05, // 5% error rate under stress
			RequestBodySizeBytes:  128,
			ResponseBodySizeBytes: 256,
			ConnectionPoolSize:    500,
			KeepAliveEnabled:      true,
			RequestRate:           0,
		})

		logRealisticTestResult(t, result)

		// Stress test - ensure graceful degradation
		if result.SuccessRate < 0.80 {
			t.Errorf("Stress test success rate too low: %.2f%% (expected >= 80%%)", result.SuccessRate*100)
		}

		// Should still maintain some throughput under stress
		if result.RequestsPerSecond < 200 {
			t.Errorf("Stress test RPS too low: %.2f (expected >= 200)", result.RequestsPerSecond)
		}

		// Memory usage shouldn't explode under stress
		if result.MemoryPerRequest > 5000 {
			t.Errorf("High memory usage under stress: %.0f bytes per request", result.MemoryPerRequest)
		}

		t.Logf("✅ Stress test: %.0f RPS, %.1f%% success under extreme load",
			result.RequestsPerSecond, result.SuccessRate*100)
	})
}

// RealisticTestConfig defines parameters for realistic production tests
type RealisticTestConfig struct {
	Name                  string
	Concurrency           int
	TotalRequests         int64
	Duration              time.Duration
	BackendCount          int
	BackendLatencyMin     time.Duration
	BackendLatencyMax     time.Duration
	BackendErrorRate      float64 // 0.0 to 1.0
	RequestBodySizeBytes  int
	ResponseBodySizeBytes int
	ConnectionPoolSize    int
	KeepAliveEnabled      bool
	RequestRate           int // Requests per second per worker (0 = unlimited)
	ValidateLoadBalancing bool
}

// runRealisticLoadTest executes a realistic production-like load test
func runRealisticLoadTest(t *testing.T, cfg RealisticTestConfig) *RealisticTestResult {
	// Default backend count
	if cfg.BackendCount == 0 {
		cfg.BackendCount = 2
	}

	// Create realistic backends
	backends := make([]*RealisticBackend, cfg.BackendCount)
	services := make([]config.Service, cfg.BackendCount)

	for i := 0; i < cfg.BackendCount; i++ {
		backend := createRealisticBackend(t, i, cfg)
		backends[i] = backend

		services[i] = config.Service{
			Name:   fmt.Sprintf("backend-%d", i),
			URL:    fmt.Sprintf("http://localhost:%d", backend.port),
			Health: "/health",
		}
	}

	// Cleanup backends
	defer func() {
		for _, backend := range backends {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			backend.server.Shutdown(ctx)
			cancel()
		}
	}()

	// Create gateway
	gatewayConfig := &config.Config{
		Tenants: []config.Tenant{
			{
				Name:       fmt.Sprintf("%s-tenant", cfg.Name),
				PathPrefix: "/api/",
				Interval:   30,
				Services:   services,
			},
		},
	}

	router := chi.NewRouter()
	gateway := routing.NewGatewayWithRouter(gatewayConfig, router)

	// Mark backends as alive
	if tenantRouter := gateway.GetTenantRouter(fmt.Sprintf("%s-tenant", cfg.Name)); tenantRouter != nil {
		for _, backend := range tenantRouter.Backends {
			backend.Alive.Store(true)
		}
	}

	// Start gateway server
	gatewayPort := startGatewayServer(t, gateway)
	defer func() {
		// Gateway cleanup handled by test framework
	}()

	// Initialize metrics
	var totalRequests, successCount, errorCount int64
	var timeoutErrors, networkErrors int64
	var status2xx, status4xx, status5xx int64
	var connectionsCreated, connectionsReused int64

	latencies := make([]time.Duration, 0, 100000)
	var latencyMutex sync.Mutex

	backendDistribution := make(map[string]int64)
	var distributionMutex sync.Mutex

	// Memory tracking
	var beforeStats, afterStats runtime.MemStats
	runtime.GC()
	runtime.GC()
	runtime.ReadMemStats(&beforeStats)

	// HTTP client with realistic settings
	transport := &http.Transport{
		MaxIdleConns:        cfg.ConnectionPoolSize,
		MaxIdleConnsPerHost: cfg.ConnectionPoolSize / cfg.BackendCount,
		IdleConnTimeout:     90 * time.Second,
		DisableKeepAlives:   !cfg.KeepAliveEnabled,
		DisableCompression:  false,

		// Realistic timeouts
		DialTimeout:           10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second, // Generous timeout for realistic test
	}

	// Test execution context
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

	// Rate limiting
	var rateLimiter <-chan time.Time
	if cfg.RequestRate > 0 {
		rateLimiter = time.Tick(time.Second / time.Duration(cfg.RequestRate))
	}

	// Worker function
	worker := func(wg *sync.WaitGroup, workerID int) {
		defer wg.Done()
		requestNum := int64(0)

		for {
			// Termination check
			if useTimeBasedTest {
				select {
				case <-ctx.Done():
					return
				default:
				}
			} else {
				if requestNum >= cfg.TotalRequests/int64(cfg.Concurrency) {
					return
				}
			}

			// Rate limiting
			if rateLimiter != nil {
				select {
				case <-rateLimiter:
				case <-ctx.Done():
					return
				}
			}

			atomic.AddInt64(&totalRequests, 1)
			requestNum++

			// Create realistic request
			gatewayURL := fmt.Sprintf("http://localhost:%d/api/test/worker-%d/req-%d",
				gatewayPort, workerID, requestNum)

			// Generate request body if needed
			var body io.Reader
			if cfg.RequestBodySizeBytes > 0 {
				bodyData := make([]byte, cfg.RequestBodySizeBytes)
				rand.Read(bodyData)
				body = bytes.NewReader(bodyData)
			}

			start := time.Now()
			req, err := http.NewRequestWithContext(ctx, "POST", gatewayURL, body)
			if err != nil {
				atomic.AddInt64(&errorCount, 1)
				continue
			}

			if cfg.RequestBodySizeBytes > 0 {
				req.Header.Set("Content-Type", "application/octet-stream")
			}

			// Make request
			resp, err := client.Do(req)
			latency := time.Since(start)

			// Record latency
			latencyMutex.Lock()
			latencies = append(latencies, latency)
			latencyMutex.Unlock()

			if err != nil {
				atomic.AddInt64(&errorCount, 1)
				if ctx.Err() != nil {
					atomic.AddInt64(&timeoutErrors, 1)
				} else {
					atomic.AddInt64(&networkErrors, 1)
				}
				continue
			}

			// Read response
			responseBody, err := io.ReadAll(resp.Body)
			resp.Body.Close()

			if err != nil {
				atomic.AddInt64(&errorCount, 1)
				atomic.AddInt64(&networkErrors, 1)
				continue
			}

			// Record backend distribution
			if cfg.ValidateLoadBalancing {
				backendID := resp.Header.Get("X-Backend-ID")
				if backendID != "" {
					distributionMutex.Lock()
					backendDistribution[backendID]++
					distributionMutex.Unlock()
				}
			}

			// Classify response
			switch {
			case resp.StatusCode >= 200 && resp.StatusCode < 300:
				atomic.AddInt64(&successCount, 1)
				atomic.AddInt64(&status2xx, 1)
			case resp.StatusCode >= 400 && resp.StatusCode < 500:
				atomic.AddInt64(&errorCount, 1)
				atomic.AddInt64(&status4xx, 1)
			case resp.StatusCode >= 500:
				atomic.AddInt64(&errorCount, 1)
				atomic.AddInt64(&status5xx, 1)
			}

			// Track connection reuse (approximation)
			if resp.Header.Get("X-Connection-Reused") == "true" {
				atomic.AddInt64(&connectionsReused, 1)
			} else {
				atomic.AddInt64(&connectionsCreated, 1)
			}

			_ = responseBody // Use response body to prevent optimization
		}
	}

	// Execute test
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

	// Calculate latency percentiles
	latencyMutex.Lock()
	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	var p50, p95, p99, p999, mean, stddev float64
	n := len(latencies)

	if n > 0 {
		p50 = float64(latencies[n*50/100]) / float64(time.Millisecond)
		p95 = float64(latencies[n*95/100]) / float64(time.Millisecond)
		p99 = float64(latencies[n*99/100]) / float64(time.Millisecond)
		p999 = float64(latencies[n*999/1000]) / float64(time.Millisecond)

		// Calculate mean
		var sum time.Duration
		for _, lat := range latencies {
			sum += lat
		}
		mean = float64(sum) / float64(n) / float64(time.Millisecond)

		// Calculate standard deviation
		var variance float64
		for _, lat := range latencies {
			diff := float64(lat)/float64(time.Millisecond) - mean
			variance += diff * diff
		}
		stddev = math.Sqrt(variance / float64(n))
	}
	latencyMutex.Unlock()

	// Compile results
	total := atomic.LoadInt64(&totalRequests)
	success := atomic.LoadInt64(&successCount)
	failed := atomic.LoadInt64(&errorCount)

	result := &RealisticTestResult{
		TotalRequests:      total,
		SuccessfulRequests: success,
		FailedRequests:     failed,
		TimeoutErrors:      atomic.LoadInt64(&timeoutErrors),
		NetworkErrors:      atomic.LoadInt64(&networkErrors),
		Duration:           duration,
		RequestsPerSecond:  float64(total) / duration.Seconds(),
		SuccessRate:        float64(success) / float64(total),

		LatencyP50:    p50,
		LatencyP95:    p95,
		LatencyP99:    p99,
		LatencyP999:   p999,
		LatencyMean:   mean,
		LatencyStdDev: stddev,

		MemoryUsedMB:     float64(afterStats.Alloc-beforeStats.Alloc) / 1024 / 1024,
		MemoryPerRequest: float64(int64(afterStats.Alloc)-int64(beforeStats.Alloc)) / float64(total),
		GCCount:          afterStats.NumGC - beforeStats.NumGC,

		ConnectionsCreated: atomic.LoadInt64(&connectionsCreated),
		ConnectionsReused:  atomic.LoadInt64(&connectionsReused),

		BytesPerSecond:  0, // TODO: Calculate if needed
		RequestsPerCore: float64(total) / duration.Seconds() / float64(runtime.NumCPU()),

		StatusCode2xx: atomic.LoadInt64(&status2xx),
		StatusCode4xx: atomic.LoadInt64(&status4xx),
		StatusCode5xx: atomic.LoadInt64(&status5xx),

		BackendDistribution: backendDistribution,
	}

	return result
}

// createRealisticBackend creates a backend that simulates real API behavior
func createRealisticBackend(t *testing.T, id int, cfg RealisticTestConfig) *RealisticBackend {
	// Find available port
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to find available port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	backend := &RealisticBackend{
		port: port,
		id:   fmt.Sprintf("backend-%d", id),
	}

	mux := http.NewServeMux()

	// Main handler with realistic behavior
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		atomic.AddInt64(&backend.stats.RequestCount, 1)

		// Simulate realistic backend processing time
		processingTime := cfg.BackendLatencyMin +
			time.Duration(rand.Int63n(int64(cfg.BackendLatencyMax-cfg.BackendLatencyMin)))
		time.Sleep(processingTime)

		// Simulate errors based on error rate
		if rand.Float64() < cfg.BackendErrorRate {
			atomic.AddInt64(&backend.stats.ErrorCount, 1)
			http.Error(w, "Backend Error", http.StatusInternalServerError)
			return
		}

		// Read request body if present
		if r.ContentLength > 0 {
			io.ReadAll(r.Body)
			r.Body.Close()
		}

		// Generate realistic response
		response := map[string]interface{}{
			"backend_id":    backend.id,
			"request_path":  r.URL.Path,
			"timestamp":     time.Now().Unix(),
			"processing_ms": processingTime.Milliseconds(),
		}

		// Add padding to reach desired response size
		if cfg.ResponseBodySizeBytes > 100 {
			paddingSize := cfg.ResponseBodySizeBytes - 100 // Account for JSON overhead
			if paddingSize > 0 {
				padding := make([]byte, paddingSize)
				rand.Read(padding)
				response["padding"] = string(padding)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Backend-ID", backend.id)
		w.Header().Set("X-Processing-Time", fmt.Sprintf("%.2fms",
			float64(processingTime)/float64(time.Millisecond)))

		json.NewEncoder(w).Encode(response)

		// Update stats
		latency := time.Since(start)
		atomic.AddInt64((*int64)(&backend.stats.TotalLatency), int64(latency))
	})

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Stats endpoint
	mux.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		stats := map[string]interface{}{
			"backend_id":    backend.id,
			"request_count": atomic.LoadInt64(&backend.stats.RequestCount),
			"error_count":   atomic.LoadInt64(&backend.stats.ErrorCount),
			"avg_latency_ms": float64(atomic.LoadInt64((*int64)(&backend.stats.TotalLatency))) /
				float64(atomic.LoadInt64(&backend.stats.RequestCount)) /
				float64(time.Millisecond),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	})

	backend.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	// Start server
	go func() {
		if err := backend.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Logf("Backend %s error: %v", backend.id, err)
		}
	}()

	// Wait for server to start
	time.Sleep(50 * time.Millisecond)

	return backend
}

// startGatewayServer starts the gateway server and returns the port
func startGatewayServer(t *testing.T, gateway *routing.Gateway) int {
	// Find available port
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to find available port for gateway: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	// Create gateway handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Input validation (same as production)
		for name := range r.Header {
			for _, char := range name {
				if char == 0 {
					http.Error(w, "Bad Request: Invalid header name", http.StatusBadRequest)
					return
				}
			}
		}

		if len(r.URL.Path) > 1024 {
			http.NotFound(w, r)
			return
		}

		for _, char := range r.URL.Path {
			if char == 0 {
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

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: handler,

		// Production-like server settings
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB
	}

	// Start gateway server
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Logf("Gateway server error: %v", err)
		}
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	return port
}

// logRealisticTestResult logs comprehensive realistic test results
func logRealisticTestResult(t *testing.T, result *RealisticTestResult) {
	t.Logf("=== 🚀 REALISTIC PRODUCTION TEST RESULTS ===")

	// Performance Summary
	t.Logf("🔥 PERFORMANCE SUMMARY:")
	t.Logf("  Requests/sec:    %.1f RPS", result.RequestsPerSecond)
	t.Logf("  Success Rate:    %.2f%% (%d/%d)", result.SuccessRate*100, result.SuccessfulRequests, result.TotalRequests)
	t.Logf("  Duration:        %v", result.Duration)
	t.Logf("  RPS per CPU:     %.1f", result.RequestsPerCore)

	// Latency Distribution
	t.Logf("⚡ LATENCY DISTRIBUTION (ms):")
	t.Logf("  Mean:     %.2f", result.LatencyMean)
	t.Logf("  P50:      %.2f", result.LatencyP50)
	t.Logf("  P95:      %.2f", result.LatencyP95)
	t.Logf("  P99:      %.2f", result.LatencyP99)
	t.Logf("  P99.9:    %.2f", result.LatencyP999)
	t.Logf("  Std Dev:  %.2f", result.LatencyStdDev)

	// Connection Efficiency
	totalConnections := result.ConnectionsCreated + result.ConnectionsReused
	var reuseRatio float64
	if totalConnections > 0 {
		reuseRatio = float64(result.ConnectionsReused) / float64(totalConnections) * 100
	}

	t.Logf("🔗 CONNECTION EFFICIENCY:")
	t.Logf("  Created:         %d", result.ConnectionsCreated)
	t.Logf("  Reused:          %d", result.ConnectionsReused)
	t.Logf("  Reuse Ratio:     %.1f%%", reuseRatio)
	t.Logf("  Pool Usage:      %.1f%%", result.ConnectionPoolUsage)

	// Memory Efficiency
	t.Logf("💾 MEMORY EFFICIENCY:")
	t.Logf("  Total Used:      %.2f MB", result.MemoryUsedMB)
	t.Logf("  Per Request:     %.0f bytes", result.MemoryPerRequest)
	t.Logf("  GC Cycles:       %d", result.GCCount)
	if result.GCPausesTotal > 0 {
		t.Logf("  GC Pause Total:  %v", result.GCPausesTotal)
	}

	// Error Distribution
	if result.FailedRequests > 0 {
		t.Logf("❌ ERROR BREAKDOWN:")
		t.Logf("  Total Failures:  %d", result.FailedRequests)
		t.Logf("  Network Errors:  %d", result.NetworkErrors)
		t.Logf("  Timeouts:        %d", result.TimeoutErrors)
		t.Logf("  4xx Responses:   %d", result.StatusCode4xx)
		t.Logf("  5xx Responses:   %d", result.StatusCode5xx)
	}

	// Load Balancing Distribution
	if len(result.BackendDistribution) > 1 {
		t.Logf("⚖️  LOAD BALANCING:")
		for backend, count := range result.BackendDistribution {
			percentage := float64(count) / float64(result.TotalRequests) * 100
			t.Logf("  %s: %d requests (%.1f%%)", backend, count, percentage)
		}
	}

	// Performance Rating
	rating := "🔥 EXCELLENT"
	if result.RequestsPerSecond < 500 {
		rating = "⚠️  NEEDS IMPROVEMENT"
	} else if result.RequestsPerSecond < 1000 {
		rating = "✅ GOOD"
	} else if result.RequestsPerSecond < 2000 {
		rating = "🚀 VERY GOOD"
	}

	t.Logf("📊 OVERALL RATING: %s", rating)
	t.Logf("==========================================")
}
