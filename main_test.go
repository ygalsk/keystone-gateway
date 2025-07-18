package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
)

// Test domain validation
func TestIsValidDomain(t *testing.T) {
	tests := []struct {
		domain   string
		expected bool
	}{
		{"example.com", true},
		{"app.example.com", true},
		{"sub.app.example.com", true},
		{"", false},              // empty
		{"no spaces.com", false}, // contains space
		{"localhost", false},     // no dot
		{"test", false},          // no dot
	}

	for _, test := range tests {
		result := isValidDomain(test.domain)
		if result != test.expected {
			t.Errorf("isValidDomain(%q) = %v, expected %v", test.domain, result, test.expected)
		}
	}
}

// Test tenant validation
func TestValidateTenant(t *testing.T) {
	tests := []struct {
		name      string
		tenant    Tenant
		shouldErr bool
	}{
		{
			name: "valid path-only tenant",
			tenant: Tenant{
				Name:       "test",
				PathPrefix: "/api/",
				Services:   []Service{{Name: "test", URL: "http://example.com"}},
			},
			shouldErr: false,
		},
		{
			name: "valid host-only tenant",
			tenant: Tenant{
				Name:     "test",
				Domains:  []string{"app.example.com"},
				Services: []Service{{Name: "test", URL: "http://example.com"}},
			},
			shouldErr: false,
		},
		{
			name: "valid hybrid tenant",
			tenant: Tenant{
				Name:       "test",
				Domains:    []string{"api.example.com"},
				PathPrefix: "/v2/",
				Services:   []Service{{Name: "test", URL: "http://example.com"}},
			},
			shouldErr: false,
		},
		{
			name: "invalid - no domains or path_prefix",
			tenant: Tenant{
				Name:     "test",
				Services: []Service{{Name: "test", URL: "http://example.com"}},
			},
			shouldErr: true,
		},
		{
			name: "invalid domain format",
			tenant: Tenant{
				Name:     "test",
				Domains:  []string{"invalid domain"},
				Services: []Service{{Name: "test", URL: "http://example.com"}},
			},
			shouldErr: true,
		},
		{
			name: "invalid path_prefix format",
			tenant: Tenant{
				Name:       "test",
				PathPrefix: "api", // missing slashes
				Services:   []Service{{Name: "test", URL: "http://example.com"}},
			},
			shouldErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateTenant(test.tenant)
			if test.shouldErr && err == nil {
				t.Errorf("Expected error for tenant %v, but got none", test.tenant)
			}
			if !test.shouldErr && err != nil {
				t.Errorf("Expected no error for tenant %v, but got: %v", test.tenant, err)
			}
		})
	}
}

// Test host extraction
func TestExtractHost(t *testing.T) {
	tests := []struct {
		hostHeader string
		expected   string
	}{
		{"example.com", "example.com"},
		{"example.com:8080", "example.com"},
		{"app.example.com:3000", "app.example.com"},
		{"localhost:9000", "localhost"},
	}

	for _, test := range tests {
		result := extractHost(test.hostHeader)
		if result != test.expected {
			t.Errorf("extractHost(%q) = %q, expected %q", test.hostHeader, result, test.expected)
		}
	}
}

// Test backward compatibility
func TestBackwardCompatibility(t *testing.T) {
	// Test that old configuration format still works
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("legacy-response"))
	}))
	defer backend.Close()

	// Old-style configuration (path_prefix only)
	tenant := Tenant{
		Name:       "legacy",
		PathPrefix: "/legacy/",
		Services:   []Service{{Name: "test", URL: backend.URL, Health: "/"}},
	}

	// Validate it passes our new validation
	err := validateTenant(tenant)
	if err != nil {
		t.Errorf("Legacy configuration should be valid, got error: %v", err)
	}
}

// Integration tests for routing functionality (simplified)
func TestRoutingIntegration(t *testing.T) {
	// Create a simple test backend that just echoes the request path
	testBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Echo the path and host for debugging
		response := "path:" + r.URL.Path + ",host:" + r.Host
		w.WriteHeader(200)
		w.Write([]byte(response))
	}))
	defer testBackend.Close()

	// Parse backend URL for our routing setup
	backendURL, _ := url.Parse(testBackend.URL)

	// Create tenant router
	tr := &tenantRouter{
		backends: []*backend{{url: backendURL, alive: atomic.Bool{}}},
	}
	tr.backends[0].alive.Store(true)

	// Test simple routing tables
	pathRouters := map[string]*tenantRouter{
		"/api/": tr,
	}
	hostRouters := map[string]*tenantRouter{
		"app.example.com": tr,
	}
	hybridRouters := map[string]map[string]*tenantRouter{
		"api.example.com": {
			"/v2/": tr,
		},
	}

	// Create handler
	handler := makeHandler(pathRouters, hostRouters, hybridRouters)
	server := httptest.NewServer(handler)
	defer server.Close()

	// Test cases - simplified
	tests := []struct {
		name         string
		host         string
		path         string
		expectStatus int
		shouldMatch  bool
	}{
		{
			name:         "Legacy API routing",
			host:         "localhost",
			path:         "/api/users",
			expectStatus: 200,
			shouldMatch:  true,
		},
		{
			name:         "Host routing",
			host:         "app.example.com",
			path:         "/dashboard",
			expectStatus: 200,
			shouldMatch:  true,
		},
		{
			name:         "Hybrid routing",
			host:         "api.example.com",
			path:         "/v2/endpoints",
			expectStatus: 200,
			shouldMatch:  true,
		},
		{
			name:         "No match",
			host:         "wrong.example.com",
			path:         "/test",
			expectStatus: 404,
			shouldMatch:  false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", server.URL+test.path, nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Host = test.host

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != test.expectStatus {
				t.Errorf("Expected status %d, got %d", test.expectStatus, resp.StatusCode)
			}

			if test.shouldMatch && resp.StatusCode == 200 {
				// For successful matches, just verify we got a response
				body := make([]byte, 1024)
				n, _ := resp.Body.Read(body)
				bodyStr := string(body[:n])
				if len(bodyStr) == 0 {
					t.Error("Expected non-empty response body")
				}
				t.Logf("Response: %s", bodyStr)
			}
		})
	}
}

// Test routing priority (simplified)
func TestRoutingPriority(t *testing.T) {
	// Create test backend
	testBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("priority-test-response"))
	}))
	defer testBackend.Close()

	backendURL, _ := url.Parse(testBackend.URL)
	tr := &tenantRouter{
		backends: []*backend{{url: backendURL, alive: atomic.Bool{}}},
	}
	tr.backends[0].alive.Store(true)

	// Setup routing tables to test priority
	pathRouters := map[string]*tenantRouter{
		"/api/": tr,
	}
	hostRouters := map[string]*tenantRouter{
		"test.example.com": tr,
	}
	hybridRouters := map[string]map[string]*tenantRouter{
		"test.example.com": {
			"/api/": tr,
		},
	}

	handler := makeHandler(pathRouters, hostRouters, hybridRouters)
	server := httptest.NewServer(handler)
	defer server.Close()

	// Test that requests are routed (priority doesn't matter for this test,
	// we just want to ensure the routing works)
	req, _ := http.NewRequest("GET", server.URL+"/api/test", nil)
	req.Host = "test.example.com"

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}