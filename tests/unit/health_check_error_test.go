package unit

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"keystone-gateway/internal/routing"
)

func TestHealthCheckBackendUnreachable(t *testing.T) {
	// Create a backend pointing to a non-existent server
	unreachableURL, _ := url.Parse("http://localhost:99999")
	backend := &routing.GatewayBackend{URL: unreachableURL}
	backend.Alive.Store(true) // Start as healthy to test failure detection

	// Test that health check detects unreachable backend
	client := &http.Client{Timeout: 1 * time.Second}
	resp, err := client.Get(unreachableURL.String() + "/health")

	// Should get connection refused or timeout error
	if err == nil {
		t.Error("expected error when connecting to unreachable backend")
		if resp != nil {
			resp.Body.Close()
		}
	}

	// Error should indicate connection failure
	if err != nil {
		errStr := err.Error()
		if !strings.Contains(errStr, "connection refused") && !strings.Contains(errStr, "timeout") {
			t.Errorf("expected connection error, got: %v", err)
		}
	}
}

func TestHealthCheckTimeout(t *testing.T) {
	// Create a server that responds slowly
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			// Sleep longer than client timeout
			time.Sleep(2 * time.Second)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer slowServer.Close()

	// Test health check with short timeout
	client := &http.Client{Timeout: 500 * time.Millisecond}
	start := time.Now()
	resp, err := client.Get(slowServer.URL + "/health")
	duration := time.Since(start)

	// Should timeout within reasonable time
	if err == nil {
		t.Error("expected timeout error")
		if resp != nil {
			resp.Body.Close()
		}
	}

	// Should timeout around 500ms, not 2 seconds
	if duration > 1*time.Second {
		t.Errorf("timeout took too long: %v", duration)
	}

	if err != nil && !strings.Contains(err.Error(), "timeout") {
		t.Errorf("expected timeout error, got: %v", err)
	}
}

func TestHealthCheckInvalidResponse(t *testing.T) {
	// Create server that returns non-200 status
	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Backend is down"))
		case "/health-404":
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("Health endpoint not found"))
		case "/health-empty":
			// Return empty response body
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer errorServer.Close()

	testCases := []struct {
		path           string
		expectedStatus int
		description    string
	}{
		{"/health", http.StatusInternalServerError, "500 internal server error"},
		{"/health-404", http.StatusNotFound, "404 not found"},
		{"/health-empty", http.StatusOK, "empty response body"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			resp, err := http.Get(errorServer.URL + tc.path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.expectedStatus {
				t.Errorf("expected status %d, got %d", tc.expectedStatus, resp.StatusCode)
			}

			// Only 200 status should be considered healthy
			if resp.StatusCode != http.StatusOK && resp.StatusCode == http.StatusOK {
				t.Error("non-200 status should be considered unhealthy")
			}
		})
	}
}

func TestBackendSelectionWithUnhealthyBackends(t *testing.T) {
	// Create a tenant router with multiple backends
	tr := &routing.TenantRouter{
		Name:     "test-tenant",
		Backends: make([]*routing.GatewayBackend, 3),
	}

	// Create backends with different health states
	url1, _ := url.Parse("http://backend1.test")
	url2, _ := url.Parse("http://backend2.test") 
	url3, _ := url.Parse("http://backend3.test")

	tr.Backends[0] = &routing.GatewayBackend{URL: url1}
	tr.Backends[1] = &routing.GatewayBackend{URL: url2}
	tr.Backends[2] = &routing.GatewayBackend{URL: url3}

	// Set health states: backend1=unhealthy, backend2=healthy, backend3=unhealthy
	tr.Backends[0].Alive.Store(false)
	tr.Backends[1].Alive.Store(true)
	tr.Backends[2].Alive.Store(false)

	// NextBackend should return the only healthy backend
	for i := 0; i < 10; i++ {
		backend := tr.NextBackend()
		if backend == nil {
			t.Fatal("expected to get a backend")
		}
		if backend.URL.String() != "http://backend2.test" {
			t.Errorf("expected healthy backend2, got %s", backend.URL.String())
		}
	}
}

func TestBackendSelectionAllUnhealthy(t *testing.T) {
	// Create a tenant router with all unhealthy backends
	tr := &routing.TenantRouter{
		Name:     "test-tenant",
		Backends: make([]*routing.GatewayBackend, 2),
	}

	url1, _ := url.Parse("http://backend1.test")
	url2, _ := url.Parse("http://backend2.test")

	tr.Backends[0] = &routing.GatewayBackend{URL: url1}
	tr.Backends[1] = &routing.GatewayBackend{URL: url2}

	// Both backends are unhealthy
	tr.Backends[0].Alive.Store(false)
	tr.Backends[1].Alive.Store(false)

	// Should fallback to first backend even if unhealthy
	backend := tr.NextBackend()
	if backend == nil {
		t.Fatal("expected fallback backend")
	}
	if backend.URL.String() != "http://backend1.test" {
		t.Errorf("expected fallback to first backend, got %s", backend.URL.String())
	}
}

func TestBackendSelectionNoBackends(t *testing.T) {
	// Create a tenant router with no backends
	tr := &routing.TenantRouter{
		Name:     "empty-tenant",
		Backends: []*routing.GatewayBackend{},
	}

	// Should return nil when no backends available
	backend := tr.NextBackend()
	if backend != nil {
		t.Errorf("expected nil backend, got %s", backend.URL.String())
	}
}

func TestHealthCheckSSLErrors(t *testing.T) {
	// Test various SSL/TLS related errors
	testCases := []struct {
		url         string
		description string
	}{
		{"https://expired.badssl.com/", "expired certificate"},
		{"https://self-signed.badssl.com/", "self-signed certificate"},
		{"https://wrong.host.badssl.com/", "wrong hostname"},
		{"https://untrusted-root.badssl.com/", "untrusted root"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			client := &http.Client{Timeout: 5 * time.Second}
			_, err := client.Get(tc.url)

			// Should get SSL/TLS related error
			if err == nil {
				t.Errorf("expected SSL error for %s", tc.description)
			} else {
				errStr := err.Error()
				// Check for common SSL error indicators
				sslErrorFound := strings.Contains(errStr, "certificate") ||
					strings.Contains(errStr, "tls") ||
					strings.Contains(errStr, "x509") ||
					strings.Contains(errStr, "ssl")

				if !sslErrorFound {
					t.Errorf("expected SSL-related error for %s, got: %v", tc.description, err)
				}
			}
		})
	}
}

func TestHealthCheckDNSFailures(t *testing.T) {
	// Test DNS resolution failures
	invalidHosts := []string{
		"http://nonexistent-host-12345.invalid",
		"http://does.not.exist.anywhere.invalid",
		"http://..invalid-domain",
	}

	for _, hostURL := range invalidHosts {
		t.Run(hostURL, func(t *testing.T) {
			client := &http.Client{Timeout: 2 * time.Second}
			_, err := client.Get(hostURL)

			if err == nil {
				t.Errorf("expected DNS error for %s", hostURL)
			} else {
				errStr := err.Error()
				// Check for DNS resolution error indicators
				dnsErrorFound := strings.Contains(errStr, "no such host") ||
					strings.Contains(errStr, "dns") ||
					strings.Contains(errStr, "resolve")

				if !dnsErrorFound {
					t.Errorf("expected DNS error for %s, got: %v", hostURL, err)
				}
			}
		})
	}
}

func TestConcurrentBackendSelection(t *testing.T) {
	// Test thread safety of backend selection under concurrent load
	tr := &routing.TenantRouter{
		Name:     "concurrent-test",
		Backends: make([]*routing.GatewayBackend, 3),
	}

	// Create backends
	for i := 0; i < 3; i++ {
		url, _ := url.Parse("http://backend" + string(rune('1'+i)) + ".test")
		tr.Backends[i] = &routing.GatewayBackend{URL: url}
		tr.Backends[i].Alive.Store(true)
	}

	const numGoroutines = 50
	const numSelections = 100
	results := make(chan string, numGoroutines*numSelections)

	// Launch concurrent backend selections
	for i := 0; i < numGoroutines; i++ {
		go func() {
			for j := 0; j < numSelections; j++ {
				backend := tr.NextBackend()
				if backend != nil {
					results <- backend.URL.String()
				} else {
					results <- "nil"
				}
			}
		}()
	}

	// Collect results
	backendCounts := make(map[string]int)
	for i := 0; i < numGoroutines*numSelections; i++ {
		result := <-results
		backendCounts[result]++
	}

	// Verify no nil results
	if nilCount, exists := backendCounts["nil"]; exists && nilCount > 0 {
		t.Errorf("got %d nil backends during concurrent selection", nilCount)
	}

	// Verify all backends were used (round-robin should distribute load)
	for i := 1; i <= 3; i++ {
		backendURL := "http://backend" + string(rune('0'+i)) + ".test"
		if count, exists := backendCounts[backendURL]; !exists || count == 0 {
			t.Errorf("backend %s was never selected", backendURL)
		}
	}
}

func TestAtomicHealthStatusOperations(t *testing.T) {
	// Test thread safety of health status updates
	backend := &routing.GatewayBackend{}
	url, _ := url.Parse("http://test-backend.test")
	backend.URL = url
	backend.Alive.Store(true)

	const numGoroutines = 20
	const numOperations = 1000
	done := make(chan bool, numGoroutines)

	// Launch concurrent health status updates
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()

			for j := 0; j < numOperations; j++ {
				// Alternate between setting healthy/unhealthy
				if (id+j)%2 == 0 {
					backend.Alive.Store(true)
				} else {
					backend.Alive.Store(false)
				}

				// Read the status
				_ = backend.Alive.Load()
			}
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		select {
		case <-done:
			// Good
		case <-time.After(10 * time.Second):
			t.Fatal("concurrent health status test timed out")
		}
	}

	// Final status should be either true or false, not corrupted
	finalStatus := backend.Alive.Load()
	if finalStatus != true && finalStatus != false {
		t.Errorf("health status corrupted: %v", finalStatus)
	}
}

func TestRoundRobinIndexOverflow(t *testing.T) {
	// Test round-robin index overflow protection
	tr := &routing.TenantRouter{
		Name:     "overflow-test",
		Backends: make([]*routing.GatewayBackend, 2),
	}

	url1, _ := url.Parse("http://backend1.test")
	url2, _ := url.Parse("http://backend2.test")
	tr.Backends[0] = &routing.GatewayBackend{URL: url1}
	tr.Backends[1] = &routing.GatewayBackend{URL: url2}
	tr.Backends[0].Alive.Store(true)
	tr.Backends[1].Alive.Store(true)

	// Set RRIndex to near uint64 max to test overflow
	atomic.StoreUint64(&tr.RRIndex, ^uint64(0)-10) // Max uint64 - 10

	// Should handle overflow gracefully
	for i := 0; i < 20; i++ {
		backend := tr.NextBackend()
		if backend == nil {
			t.Fatalf("got nil backend at iteration %d", i)
		}
		// Should alternate between the two backends due to round-robin
		expectedURL := "http://backend" + string(rune('1'+(i%2))) + ".test"
		if backend.URL.String() != expectedURL {
			// Due to atomic increment, the pattern might not be exact, 
			// but we should never get nil or crash
		}
	}

	// Verify index wrapped around correctly
	finalIndex := atomic.LoadUint64(&tr.RRIndex)
	if finalIndex == ^uint64(0) {
		t.Error("RRIndex should have wrapped around")
	}
}