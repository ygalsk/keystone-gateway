package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// Test just the routing logic without reverse proxy
func TestRoutingLogicOnly(t *testing.T) {
	// Create mock tenant routers
	tr := &tenantRouter{
		backends: []*backend{{url: nil, alive: atomic.Bool{}}}, // URL doesn't matter for this test
	}
	tr.backends[0].alive.Store(true)

	// Set up routing tables
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

	// Custom handler that just reports which router was matched
	handler := func(w http.ResponseWriter, r *http.Request) {
		host := extractHost(r.Host)
		path := r.URL.Path
		
		var matched *tenantRouter
		var matchedPrefix string
		var routingType string
		
		// Priority 1: Host + Path combination (hybrid routing)
		if hostPathMap, exists := hybridRouters[host]; exists {
			for prefix, rt := range hostPathMap {
				if strings.HasPrefix(path, prefix) {
					if len(prefix) > len(matchedPrefix) {
						matchedPrefix = prefix
						matched = rt
						routingType = "hybrid"
					}
				}
			}
		}
		
		// Priority 2: Host-only routing
		if matched == nil {
			if rt, exists := hostRouters[host]; exists {
				matched = rt
				matchedPrefix = ""
				routingType = "host"
			}
		}
		
		// Priority 3: Path-only routing (backward compatibility)
		if matched == nil {
			for prefix, rt := range pathRouters {
				if strings.HasPrefix(path, prefix) {
					if len(prefix) > len(matchedPrefix) {
						matchedPrefix = prefix
						matched = rt
						routingType = "path"
					}
				}
			}
		}
		
		if matched == nil {
			http.NotFound(w, r)
			return
		}
		
		// Return info about the match
		w.WriteHeader(200)
		w.Write([]byte("type:" + routingType + ",prefix:" + matchedPrefix))
	}

	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	tests := []struct {
		name     string
		host     string
		path     string
		expected string
	}{
		{
			name:     "Path-based routing",
			host:     "localhost",
			path:     "/api/users",
			expected: "type:path,prefix:/api/",
		},
		{
			name:     "Host-based routing",
			host:     "app.example.com",
			path:     "/dashboard",
			expected: "type:host,prefix:",
		},
		{
			name:     "Hybrid routing",
			host:     "api.example.com",
			path:     "/v2/endpoints",
			expected: "type:hybrid,prefix:/v2/",
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

			if resp.StatusCode != 200 {
				t.Errorf("Expected status 200, got %d", resp.StatusCode)
			}

			body := make([]byte, 1024)
			n, _ := resp.Body.Read(body)
			bodyStr := string(body[:n])

			if bodyStr != test.expected {
				t.Errorf("Expected %q, got %q", test.expected, bodyStr)
			}
		})
	}
}
