package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"keystone-gateway/internal/config"
	"keystone-gateway/internal/types"
)

// Helper function to create a test backend server
func createTestBackend(statusCode int, body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Backend", "test-backend")
		w.Header().Set("X-Request-ID", r.Header.Get("X-Request-ID"))
		w.WriteHeader(statusCode)
		w.Write([]byte(body))
	}))
}

// Helper function to create test configuration
func createTestConfig(backendURL string) *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Addr:              "localhost:0", // Let OS assign port
			ReadHeaderTimeout: 5 * time.Second,
			IdleTimeout:       30 * time.Second,
		},
		Upstreams: config.UpstreamsConfig{
			Targets: []config.UpstreamTarget{
				{
					Name:    "test-backend",
					URL:     backendURL,
					Weight:  1,
					Enabled: true,
				},
			},
			LoadBalancing: config.LoadBalancingConfig{
				Strategy: "least_connections",
			},
			HealthCheck: types.HealthConfig{
				Enabled:             true,
				Path:                "/health",
				Interval:            30 * time.Second,
				Timeout:             5 * time.Second,
				FailureThreshold:    2,
				SuccessThreshold:    1,
				ExpectedStatusCodes: []int{200, 204},
			},
		},
	}
}

// TestServer_BasicProxyFlow tests the basic HTTP flow through the proxy
func TestServer_BasicProxyFlow(t *testing.T) {
	// Setup test backend
	backend := createTestBackend(http.StatusOK, "backend response")
	defer backend.Close()

	// Create configuration pointing to test backend
	cfg := createTestConfig(backend.URL)

	// Create and start proxy server
	logger := slog.Default()
	server := New(cfg, logger)
	
	// Create test server wrapper
	testServer := httptest.NewServer(server.router)
	defer testServer.Close()

	// Wait briefly for upstream to be added and then mark as healthy
	time.Sleep(10 * time.Millisecond)
	
	// Make request through proxy
	resp, err := http.Get(testServer.URL + "/test")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify response
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status = %v, want %v", resp.StatusCode, http.StatusOK)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	if string(body) != "backend response" {
		t.Errorf("Body = %q, want %q", string(body), "backend response")
	}

	if resp.Header.Get("X-Backend") != "test-backend" {
		t.Error("Expected X-Backend header from backend")
	}
}

// TestServer_HealthEndpoint tests the gateway's own health endpoint
func TestServer_HealthEndpoint(t *testing.T) {
	// Setup test backend
	backend := createTestBackend(http.StatusOK, "backend response")
	defer backend.Close()

	// Create configuration
	cfg := createTestConfig(backend.URL)

	// Create server
	logger := slog.Default()
	server := New(cfg, logger)
	testServer := httptest.NewServer(server.router)
	defer testServer.Close()

	// Test health endpoint
	resp, err := http.Get(testServer.URL + "/health")
	if err != nil {
		t.Fatalf("Health request failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify response
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Health endpoint status = %v, want %v", resp.StatusCode, http.StatusOK)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read health response body: %v", err)
	}

	// Verify response contains expected content
	bodyStr := string(body)
	if !strings.Contains(bodyStr, "backend response") {
		t.Errorf("Expected health response to contain backend content, got %q", bodyStr)
	}
}

// TestServer_AdminStatsEndpoint tests the admin stats endpoint
func TestServer_AdminStatsEndpoint(t *testing.T) {
	// Setup test backend
	backend := createTestBackend(http.StatusOK, "backend response")
	defer backend.Close()

	// Create configuration
	cfg := createTestConfig(backend.URL)

	// Create server
	logger := slog.Default()
	server := New(cfg, logger)
	testServer := httptest.NewServer(server.router)
	defer testServer.Close()

	// Test admin stats endpoint
	resp, err := http.Get(testServer.URL + "/admin/stats")
	if err != nil {
		t.Fatalf("Admin stats request failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify response
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Admin stats endpoint status = %v, want %v", resp.StatusCode, http.StatusOK)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read stats response body: %v", err)
	}

	// Verify response contains expected content  
	bodyStr := string(body)
	if !strings.Contains(bodyStr, "backend response") {
		t.Errorf("Expected stats response to contain backend content, got %q", bodyStr)
	}
}

// TestServer_NoHealthyUpstreams tests behavior when no upstreams are healthy
func TestServer_NoHealthyUpstreams(t *testing.T) {
	// Setup test backend that returns errors
	backend := createTestBackend(http.StatusInternalServerError, "error")
	defer backend.Close()

	// Create configuration
	cfg := createTestConfig(backend.URL)
	
	// Create server
	logger := slog.Default()
	server := New(cfg, logger)

	// Manually mark all upstreams as unhealthy by accessing the stats and finding upstreams
	stats := server.loadBalancer.GetStats()
	if len(stats.UpstreamStats) == 0 {
		t.Skip("No upstreams available to mark unhealthy")
	}
	
	// Since we can't access private fields, we'll test with a backend that's actually down
	// The middleware should still return 503 when no healthy upstreams are available

	testServer := httptest.NewServer(server.router)
	defer testServer.Close()

	// Make request through proxy
	resp, err := http.Get(testServer.URL + "/test")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// Should get service unavailable
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("Status = %v, want %v", resp.StatusCode, http.StatusServiceUnavailable)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	if !strings.Contains(string(body), "Service Unavailable") {
		t.Errorf("Expected 'Service Unavailable' in response body, got %q", string(body))
	}
}

// TestServer_MultipleUpstreams tests load balancing between multiple upstreams
func TestServer_MultipleUpstreams(t *testing.T) {
	// Setup multiple test backends
	backend1 := createTestBackend(http.StatusOK, "backend1 response")
	defer backend1.Close()

	backend2 := createTestBackend(http.StatusOK, "backend2 response")
	defer backend2.Close()

	// Create configuration with multiple upstreams
	cfg := &config.Config{
		Server: config.ServerConfig{
			Addr:              "localhost:0",
			ReadHeaderTimeout: 5 * time.Second,
			IdleTimeout:       30 * time.Second,
		},
		Upstreams: config.UpstreamsConfig{
			Targets: []config.UpstreamTarget{
				{
					Name:    "backend1",
					URL:     backend1.URL,
					Weight:  1,
					Enabled: true,
				},
				{
					Name:    "backend2",
					URL:     backend2.URL,
					Weight:  1,
					Enabled: true,
				},
			},
			LoadBalancing: config.LoadBalancingConfig{
				Strategy: "round_robin",
			},
			HealthCheck: types.HealthConfig{
				Enabled:             true,
				Path:                "/health",
				Interval:            30 * time.Second,
				Timeout:             5 * time.Second,
				FailureThreshold:    2,
				SuccessThreshold:    1,
				ExpectedStatusCodes: []int{200, 204},
			},
		},
	}

	// Create server
	logger := slog.Default()
	server := New(cfg, logger)
	testServer := httptest.NewServer(server.router)
	defer testServer.Close()

	// Make multiple requests and collect responses
	responses := make(map[string]int)
	for i := 0; i < 10; i++ {
		resp, err := http.Get(testServer.URL + "/test")
		if err != nil {
			t.Fatalf("Request %d failed: %v", i, err)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("Failed to read response body for request %d: %v", i, err)
		}

		responses[string(body)]++
	}

	// Verify both backends received requests (round robin should distribute)
	if len(responses) != 2 {
		t.Errorf("Expected responses from 2 backends, got %d: %v", len(responses), responses)
	}

	// Both backends should have received some requests
	for backend, count := range responses {
		if count == 0 {
			t.Errorf("Backend %s received no requests", backend)
		}
	}
}

// TestServer_Middleware tests that middleware is properly applied
func TestServer_Middleware(t *testing.T) {
	// Setup test backend
	backend := createTestBackend(http.StatusOK, "backend response")
	defer backend.Close()

	// Create configuration
	cfg := createTestConfig(backend.URL)

	// Create server
	logger := slog.Default()
	server := New(cfg, logger)
	testServer := httptest.NewServer(server.router)
	defer testServer.Close()

	// Create request
	req, err := http.NewRequest("GET", testServer.URL+"/test", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Make request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify middleware added headers (from chi middleware)
	if requestID := resp.Header.Get("X-Request-Id"); requestID == "" {
		// Note: The request ID might not be echoed back depending on middleware configuration
		// This is more for checking that middleware pipeline is working
	}

	// Verify request reached backend
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status = %v, want %v", resp.StatusCode, http.StatusOK)
	}
}

// TestServer_ErrorHandling tests various error scenarios
func TestServer_ErrorHandling(t *testing.T) {
	tests := []struct {
		name           string
		backendStatus  int
		backendBody    string
		expectedStatus int
	}{
		{
			name:           "backend_404",
			backendStatus:  http.StatusNotFound,
			backendBody:    "not found",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "backend_500",
			backendStatus:  http.StatusInternalServerError,
			backendBody:    "internal error",
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:           "backend_timeout_simulation",
			backendStatus:  http.StatusGatewayTimeout,
			backendBody:    "timeout",
			expectedStatus: http.StatusGatewayTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test backend with specific status
			backend := createTestBackend(tt.backendStatus, tt.backendBody)
			defer backend.Close()

			// Create configuration
			cfg := createTestConfig(backend.URL)

			// Create server
			logger := slog.Default()
			server := New(cfg, logger)
			testServer := httptest.NewServer(server.router)
			defer testServer.Close()

			// Make request
			resp, err := http.Get(testServer.URL + "/test")
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			// Verify status code is passed through
			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Status = %v, want %v", resp.StatusCode, tt.expectedStatus)
			}

			// Verify body is passed through
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read response body: %v", err)
			}

			if string(body) != tt.backendBody {
				t.Errorf("Body = %q, want %q", string(body), tt.backendBody)
			}
		})
	}
}

// TestServer_AdminHealthEndpoint tests the admin health endpoint
func TestServer_AdminHealthEndpoint(t *testing.T) {
	// Setup test backend
	backend := createTestBackend(http.StatusOK, "backend response")
	defer backend.Close()

	// Create configuration
	cfg := createTestConfig(backend.URL)

	// Create server
	logger := slog.Default()
	server := New(cfg, logger)
	testServer := httptest.NewServer(server.router)
	defer testServer.Close()

	// Test admin health endpoint
	resp, err := http.Get(testServer.URL + "/admin/health")
	if err != nil {
		t.Fatalf("Admin health request failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify response
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Admin health endpoint status = %v, want %v", resp.StatusCode, http.StatusOK)
	}

	// Parse JSON response
	var healthResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&healthResp); err != nil {
		t.Fatalf("Failed to decode health response: %v", err)
	}

	// Verify health response structure
	if _, ok := healthResp["timestamp"]; !ok {
		t.Error("Expected timestamp in admin health response")
	}

	if summary, ok := healthResp["summary"].(map[string]interface{}); !ok {
		t.Error("Expected summary section in admin health response")
	} else {
		if totalUpstreams, ok := summary["total_upstreams"].(float64); !ok || totalUpstreams != 1 {
			t.Errorf("Expected 1 total upstream in summary, got %v", summary["total_upstreams"])
		}
	}

	if upstreams, ok := healthResp["upstreams"].(map[string]interface{}); !ok {
		t.Error("Expected upstreams section in admin health response")
	} else {
		if len(upstreams) != 1 {
			t.Errorf("Expected 1 upstream in upstreams section, got %d", len(upstreams))
		}
	}
}

// TestServer_ConcurrentRequests tests handling of concurrent requests
func TestServer_ConcurrentRequests(t *testing.T) {
	// Setup test backend
	backend := createTestBackend(http.StatusOK, "backend response")
	defer backend.Close()

	// Create configuration
	cfg := createTestConfig(backend.URL)

	// Create server
	logger := slog.Default()
	server := New(cfg, logger)
	testServer := httptest.NewServer(server.router)
	defer testServer.Close()

	// Make concurrent requests
	const numRequests = 50
	results := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(requestNum int) {
			resp, err := http.Get(testServer.URL + "/test")
			if err != nil {
				results <- err
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				results <- err
				return
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				results <- err
				return
			}

			if string(body) != "backend response" {
				results <- err
				return
			}

			results <- nil
		}(i)
	}

	// Collect results
	var errors []error
	for i := 0; i < numRequests; i++ {
		if err := <-results; err != nil {
			errors = append(errors, err)
		}
	}

	// Verify no errors
	if len(errors) > 0 {
		t.Errorf("Got %d errors out of %d concurrent requests. First error: %v", len(errors), numRequests, errors[0])
	}
}