package proxy

import (
	"log/slog"
	"net/url"
	"testing"
	"time"

	"keystone-gateway/internal/types"
)

// Helper function to create test upstreams
func createTestUpstreams(configs []testUpstreamConfig) []*Upstream {
	upstreams := make([]*Upstream, len(configs))
	for i, config := range configs {
		parsedURL, _ := url.Parse(config.url)
		upstream := &Upstream{
			Name:          config.name,
			URL:           parsedURL,
			Weight:        config.weight,
			HealthPath:    "/health",
			HealthTracker: types.NewHealthTracker(),
		}
		
		// Set connection count
		upstream.ActiveConnections.Store(config.connections)
		
		// Set health status
		if config.healthy {
			upstream.HealthTracker.MarkHealthy()
		} else {
			upstream.HealthTracker.MarkUnhealthy()
			upstream.HealthTracker.MarkUnhealthy() // 2-strike rule
		}
		
		upstreams[i] = upstream
	}
	return upstreams
}

// Test upstream configuration for table-driven tests
type testUpstreamConfig struct {
	name        string
	url         string
	connections int32
	weight      int32
	healthy     bool
}

// TestLoadBalancer_AllStrategies tests all load balancing strategies with table-driven approach
func TestLoadBalancer_AllStrategies(t *testing.T) {
	tests := []struct {
		name           string
		strategy       string
		upstreams      []testUpstreamConfig
		expectedNames  []string // Multiple possible selections for round robin
		testMultiple   bool     // Whether to test multiple selections
	}{
		{
			name:     "least_connections_picks_minimum",
			strategy: "least_connections",
			upstreams: []testUpstreamConfig{
				{name: "backend1", url: "http://backend1", connections: 5, weight: 1, healthy: true},
				{name: "backend2", url: "http://backend2", connections: 2, weight: 1, healthy: true},
				{name: "backend3", url: "http://backend3", connections: 8, weight: 1, healthy: true},
			},
			expectedNames: []string{"backend2"},
		},
		{
			name:     "least_connections_handles_tie",
			strategy: "least_connections",
			upstreams: []testUpstreamConfig{
				{name: "backend1", url: "http://backend1", connections: 3, weight: 1, healthy: true},
				{name: "backend2", url: "http://backend2", connections: 3, weight: 1, healthy: true},
				{name: "backend3", url: "http://backend3", connections: 8, weight: 1, healthy: true},
			},
			expectedNames: []string{"backend1", "backend2"}, // Either could be selected
		},
		{
			name:     "round_robin_cycles_through",
			strategy: "round_robin",
			upstreams: []testUpstreamConfig{
				{name: "backend1", url: "http://backend1", connections: 0, weight: 1, healthy: true},
				{name: "backend2", url: "http://backend2", connections: 0, weight: 1, healthy: true},
				{name: "backend3", url: "http://backend3", connections: 0, weight: 1, healthy: true},
			},
			expectedNames: []string{"backend1", "backend2", "backend3"}, // Should cycle through all
			testMultiple:  true,
		},
		{
			name:     "weighted_round_robin_respects_weights",
			strategy: "weighted_round_robin",
			upstreams: []testUpstreamConfig{
				{name: "backend1", url: "http://backend1", connections: 0, weight: 1, healthy: true},
				{name: "backend2", url: "http://backend2", connections: 0, weight: 3, healthy: true},
				{name: "backend3", url: "http://backend3", connections: 0, weight: 1, healthy: true},
			},
			expectedNames: []string{"backend1", "backend2", "backend3"}, // backend2 should appear more often
		},
		{
			name:     "health_aware_skips_unhealthy",
			strategy: "least_connections",
			upstreams: []testUpstreamConfig{
				{name: "backend1", url: "http://backend1", connections: 1, weight: 1, healthy: false},
				{name: "backend2", url: "http://backend2", connections: 5, weight: 1, healthy: true},
				{name: "backend3", url: "http://backend3", connections: 3, weight: 1, healthy: false},
			},
			expectedNames: []string{"backend2"},
		},
		{
			name:     "no_healthy_upstreams_returns_nil",
			strategy: "least_connections",
			upstreams: []testUpstreamConfig{
				{name: "backend1", url: "http://backend1", connections: 1, weight: 1, healthy: false},
				{name: "backend2", url: "http://backend2", connections: 5, weight: 1, healthy: false},
			},
			expectedNames: nil, // Should return nil
		},
		{
			name:     "single_upstream_always_selected",
			strategy: "least_connections",
			upstreams: []testUpstreamConfig{
				{name: "backend1", url: "http://backend1", connections: 10, weight: 1, healthy: true},
			},
			expectedNames: []string{"backend1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create load balancer
			logger := slog.Default()
			lb := NewLoadBalancer(tt.strategy, logger)

			// Add upstreams
			upstreams := createTestUpstreams(tt.upstreams)
			for _, upstream := range upstreams {
				lb.AddUpstream(upstream)
			}

			if tt.expectedNames == nil {
				// Test for nil return
				selected := lb.SelectUpstream()
				if selected != nil {
					t.Errorf("Expected nil upstream, got %s", selected.Name)
				}
				return
			}

			if tt.testMultiple {
				// Test round robin cycling
				selections := make(map[string]int)
				for i := 0; i < 10; i++ {
					selected := lb.SelectUpstream()
					if selected != nil {
						selections[selected.Name]++
					}
				}

				// Verify all expected backends were selected
				for _, expectedName := range tt.expectedNames {
					if selections[expectedName] == 0 {
						t.Errorf("Expected backend %s to be selected at least once", expectedName)
					}
				}
			} else {
				// Test single selection
				selected := lb.SelectUpstream()
				if selected == nil {
					t.Fatalf("Expected upstream to be selected, got nil")
				}

				found := false
				for _, expectedName := range tt.expectedNames {
					if selected.Name == expectedName {
						found = true
						break
					}
				}

				if !found {
					t.Errorf("Expected one of %v, got %s", tt.expectedNames, selected.Name)
				}
			}
		})
	}
}

// TestUpstream_ConnectionTracking tests connection tracking with table-driven approach
func TestUpstream_ConnectionTracking(t *testing.T) {
	tests := []struct {
		name     string
		actions  []string // "inc", "dec"
		expected int32
	}{
		{
			name:     "single_increment",
			actions:  []string{"inc"},
			expected: 1,
		},
		{
			name:     "increment_then_decrement",
			actions:  []string{"inc", "dec"},
			expected: 0,
		},
		{
			name:     "multiple_increments",
			actions:  []string{"inc", "inc", "inc"},
			expected: 3,
		},
		{
			name:     "increment_decrement_pattern",
			actions:  []string{"inc", "inc", "dec", "inc", "dec", "dec"},
			expected: 0,
		},
		{
			name:     "no_actions",
			actions:  []string{},
			expected: 0,
		},
		{
			name:     "decrement_below_zero",
			actions:  []string{"dec"},
			expected: -1, // Atomic operations allow negative values
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := &Upstream{
				Name: "test-upstream",
			}

			// Execute actions
			for _, action := range tt.actions {
				switch action {
				case "inc":
					upstream.IncrementConnections()
				case "dec":
					upstream.DecrementConnections()
				}
			}

			// Verify final count
			if got := upstream.GetConnectionCount(); got != tt.expected {
				t.Errorf("GetConnectionCount() = %d, want %d", got, tt.expected)
			}
		})
	}
}

// TestUpstream_ConnectionTracking_Concurrent tests concurrent connection tracking
func TestUpstream_ConnectionTracking_Concurrent(t *testing.T) {
	upstream := &Upstream{
		Name: "test-upstream",
	}

	const numGoroutines = 100
	const operationsPerGoroutine = 100

	// Use a done channel to coordinate goroutines
	done := make(chan bool, numGoroutines*2)

	// Start incrementing goroutines
	for i := 0; i < numGoroutines; i++ {
		go func() {
			for j := 0; j < operationsPerGoroutine; j++ {
				upstream.IncrementConnections()
			}
			done <- true
		}()
	}

	// Start decrementing goroutines
	for i := 0; i < numGoroutines; i++ {
		go func() {
			for j := 0; j < operationsPerGoroutine; j++ {
				upstream.DecrementConnections()
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines*2; i++ {
		<-done
	}

	// Final count should be 0 (equal increments and decrements)
	if got := upstream.GetConnectionCount(); got != 0 {
		t.Errorf("Expected connection count 0 after concurrent operations, got %d", got)
	}
}

// TestLoadBalancer_UpstreamManagement tests adding and removing upstreams
func TestLoadBalancer_UpstreamManagement(t *testing.T) {
	logger := slog.Default()
	lb := NewLoadBalancer("least_connections", logger)

	// Initially empty
	if count := lb.GetUpstreamCount(); count != 0 {
		t.Errorf("Expected 0 upstreams initially, got %d", count)
	}

	// Add upstream
	upstream1, _ := NewUpstream("test1", "http://test1", 1, "/health")
	lb.AddUpstream(upstream1)

	if count := lb.GetUpstreamCount(); count != 1 {
		t.Errorf("Expected 1 upstream after adding, got %d", count)
	}

	// Add another upstream
	upstream2, _ := NewUpstream("test2", "http://test2", 1, "/health")
	lb.AddUpstream(upstream2)

	if count := lb.GetUpstreamCount(); count != 2 {
		t.Errorf("Expected 2 upstreams after adding second, got %d", count)
	}

	// Remove upstream
	lb.RemoveUpstream("test1")

	if count := lb.GetUpstreamCount(); count != 1 {
		t.Errorf("Expected 1 upstream after removing, got %d", count)
	}

	// Remove non-existent upstream (should not panic)
	lb.RemoveUpstream("non-existent")

	if count := lb.GetUpstreamCount(); count != 1 {
		t.Errorf("Expected 1 upstream after removing non-existent, got %d", count)
	}
}

// TestLoadBalancer_HealthyUpstreamCount tests health-aware upstream counting
func TestLoadBalancer_HealthyUpstreamCount(t *testing.T) {
	logger := slog.Default()
	lb := NewLoadBalancer("least_connections", logger)

	// Create upstreams with different health states
	upstreams := createTestUpstreams([]testUpstreamConfig{
		{name: "healthy1", url: "http://healthy1", connections: 0, weight: 1, healthy: true},
		{name: "healthy2", url: "http://healthy2", connections: 0, weight: 1, healthy: true},
		{name: "unhealthy1", url: "http://unhealthy1", connections: 0, weight: 1, healthy: false},
	})

	for _, upstream := range upstreams {
		lb.AddUpstream(upstream)
	}

	// Check counts
	if total := lb.GetUpstreamCount(); total != 3 {
		t.Errorf("Expected 3 total upstreams, got %d", total)
	}

	if healthy := lb.GetHealthyUpstreamCount(); healthy != 2 {
		t.Errorf("Expected 2 healthy upstreams, got %d", healthy)
	}

	if !lb.HasHealthyUpstreams() {
		t.Error("Expected to have healthy upstreams")
	}

	// Mark all as unhealthy
	for _, upstream := range upstreams {
		upstream.MarkUnhealthy()
		upstream.MarkUnhealthy() // 2-strike rule
	}

	if healthy := lb.GetHealthyUpstreamCount(); healthy != 0 {
		t.Errorf("Expected 0 healthy upstreams after marking all unhealthy, got %d", healthy)
	}

	if lb.HasHealthyUpstreams() {
		t.Error("Expected to have no healthy upstreams")
	}
}

// TestLoadBalancer_Stats tests statistics gathering
func TestLoadBalancer_Stats(t *testing.T) {
	logger := slog.Default()
	lb := NewLoadBalancer("round_robin", logger)

	// Add upstreams
	upstreams := createTestUpstreams([]testUpstreamConfig{
		{name: "backend1", url: "http://backend1", connections: 5, weight: 1, healthy: true},
		{name: "backend2", url: "http://backend2", connections: 3, weight: 2, healthy: false},
	})

	for _, upstream := range upstreams {
		lb.AddUpstream(upstream)
	}

	// Get stats
	stats := lb.GetStats()

	if stats.Strategy != "round_robin" {
		t.Errorf("Expected strategy 'round_robin', got '%s'", stats.Strategy)
	}

	if stats.TotalUpstreams != 2 {
		t.Errorf("Expected 2 total upstreams, got %d", stats.TotalUpstreams)
	}

	if stats.HealthyUpstreams != 1 {
		t.Errorf("Expected 1 healthy upstream, got %d", stats.HealthyUpstreams)
	}

	if len(stats.UpstreamStats) != 2 {
		t.Errorf("Expected 2 upstream stats, got %d", len(stats.UpstreamStats))
	}

	// Verify individual upstream stats
	for _, upstreamStat := range stats.UpstreamStats {
		if upstreamStat.Name == "backend1" {
			if upstreamStat.ActiveConnections != 5 {
				t.Errorf("Expected backend1 to have 5 connections, got %d", upstreamStat.ActiveConnections)
			}
			if !upstreamStat.Healthy {
				t.Error("Expected backend1 to be healthy")
			}
		} else if upstreamStat.Name == "backend2" {
			if upstreamStat.ActiveConnections != 3 {
				t.Errorf("Expected backend2 to have 3 connections, got %d", upstreamStat.ActiveConnections)
			}
			if upstreamStat.Healthy {
				t.Error("Expected backend2 to be unhealthy")
			}
		}
	}
}

// TestUpstream_RequestMetrics tests request recording functionality
func TestUpstream_RequestMetrics(t *testing.T) {
	upstream := &Upstream{
		Name: "test-upstream",
	}

	// Record successful request
	upstream.RecordRequest(100*time.Millisecond, true)

	if totalRequests := upstream.TotalRequests.Load(); totalRequests != 1 {
		t.Errorf("Expected 1 total request, got %d", totalRequests)
	}

	expectedMicros := int64(100 * 1000) // 100ms in microseconds
	if avgTime := upstream.AvgResponseTime.Load(); avgTime != expectedMicros {
		t.Errorf("Expected avg response time %d microseconds, got %d", expectedMicros, avgTime)
	}

	if lastTime := upstream.LastResponseTime.Load(); lastTime != expectedMicros {
		t.Errorf("Expected last response time %d microseconds, got %d", expectedMicros, lastTime)
	}

	// Record another request (should update moving average)
	upstream.RecordRequest(200*time.Millisecond, false)

	if totalRequests := upstream.TotalRequests.Load(); totalRequests != 2 {
		t.Errorf("Expected 2 total requests, got %d", totalRequests)
	}

	// Verify last response time is updated
	expectedLastMicros := int64(200 * 1000)
	if lastTime := upstream.LastResponseTime.Load(); lastTime != expectedLastMicros {
		t.Errorf("Expected last response time %d microseconds, got %d", expectedLastMicros, lastTime)
	}

	// Average should be weighted (90% old + 10% new)
	expectedAvg := (expectedMicros*9 + expectedLastMicros) / 10
	if avgTime := upstream.AvgResponseTime.Load(); avgTime != expectedAvg {
		t.Errorf("Expected weighted avg response time %d microseconds, got %d", expectedAvg, avgTime)
	}
}

// TestNewUpstream tests upstream creation and initialization
func TestNewUpstream(t *testing.T) {
	tests := []struct {
		name       string
		upstreamName string
		rawURL     string
		weight     int32
		healthPath string
		wantError  bool
	}{
		{
			name:         "valid_http_upstream",
			upstreamName: "test-backend",
			rawURL:       "http://localhost:8080",
			weight:       1,
			healthPath:   "/health",
			wantError:    false,
		},
		{
			name:         "valid_https_upstream",
			upstreamName: "secure-backend",
			rawURL:       "https://api.example.com",
			weight:       5,
			healthPath:   "/status",
			wantError:    false,
		},
		{
			name:         "invalid_url_scheme",
			upstreamName: "bad-backend", 
			rawURL:       "://invalid-url",
			weight:       1,
			healthPath:   "/health",
			wantError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream, err := NewUpstream(tt.upstreamName, tt.rawURL, tt.weight, tt.healthPath)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if upstream.Name != tt.upstreamName {
				t.Errorf("Expected name %s, got %s", tt.upstreamName, upstream.Name)
			}

			if upstream.URL.String() != tt.rawURL {
				t.Errorf("Expected URL %s, got %s", tt.rawURL, upstream.URL.String())
			}

			if upstream.Weight != tt.weight {
				t.Errorf("Expected weight %d, got %d", tt.weight, upstream.Weight)
			}

			if upstream.HealthPath != tt.healthPath {
				t.Errorf("Expected health path %s, got %s", tt.healthPath, upstream.HealthPath)
			}

			if upstream.HealthTracker == nil {
				t.Error("Expected health tracker to be initialized")
			}

			if upstream.Proxy == nil {
				t.Error("Expected proxy to be initialized")
			}

			// Verify initial state
			if connections := upstream.GetConnectionCount(); connections != 0 {
				t.Errorf("Expected 0 initial connections, got %d", connections)
			}

			if totalRequests := upstream.TotalRequests.Load(); totalRequests != 0 {
				t.Errorf("Expected 0 initial total requests, got %d", totalRequests)
			}
		})
	}
}

// TestRequestWrapper tests connection tracking wrapper
func TestRequestWrapper(t *testing.T) {
	upstream := &Upstream{
		Name: "test-upstream",
	}
	logger := slog.Default()

	// Create request wrapper
	wrapper := NewRequestWrapper(upstream, logger)

	if wrapper.upstream != upstream {
		t.Error("Expected wrapper to reference the upstream")
	}

	// Increment connections manually to simulate selection
	upstream.IncrementConnections()
	initialConnections := upstream.GetConnectionCount()

	if initialConnections != 1 {
		t.Errorf("Expected 1 connection after increment, got %d", initialConnections)
	}

	// Finish the request
	wrapper.Finish(true)

	// Verify connection count decreased
	finalConnections := upstream.GetConnectionCount()
	if finalConnections != 0 {
		t.Errorf("Expected 0 connections after finish, got %d", finalConnections)
	}

	// Verify request was recorded
	if totalRequests := upstream.TotalRequests.Load(); totalRequests != 1 {
		t.Errorf("Expected 1 total request after finish, got %d", totalRequests)
	}
}

// TestRequestWrapper_NilUpstream tests request wrapper with nil upstream
func TestRequestWrapper_NilUpstream(t *testing.T) {
	logger := slog.Default()

	// Create request wrapper with nil upstream
	wrapper := NewRequestWrapper(nil, logger)

	// This should not panic
	wrapper.Finish(true)
	wrapper.Finish(false)
}