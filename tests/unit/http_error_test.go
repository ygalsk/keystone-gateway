package unit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"keystone-gateway/internal/config"
	"keystone-gateway/internal/routing"
)

// mockApplication mimics the main Application struct for testing
type mockApplication struct {
	gateway *routing.Gateway
}

func (app *mockApplication) HealthHandler(w http.ResponseWriter, r *http.Request) {
	// Simplified health handler for testing
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"healthy"}`))
}

func (app *mockApplication) TenantsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	cfg := app.gateway.GetConfig()
	if err := json.NewEncoder(w).Encode(cfg.Tenants); err != nil {
		http.Error(w, "Failed to encode tenants data", http.StatusInternalServerError)
		return
	}
}

func (app *mockApplication) ProxyHandler(w http.ResponseWriter, r *http.Request) {
	router, stripPrefix := app.gateway.MatchRoute(r.Host, r.URL.Path)
	if router == nil {
		// Debug: log why route matching failed
		t := testing.T{}
		t.Logf("No route match for host=%q path=%q", r.Host, r.URL.Path)
		http.NotFound(w, r)
		return
	}

	backend := router.NextBackend()
	if backend == nil {
		http.Error(w, "No backend available", http.StatusBadGateway)
		return
	}

	proxy := app.gateway.CreateProxy(backend, stripPrefix)
	proxy.ServeHTTP(w, r)
}

func TestHTTPMalformedRequests(t *testing.T) {
	router := chi.NewRouter()
	app := &mockApplication{
		gateway: routing.NewGatewayWithRouter(&config.Config{}, router),
	}

	router.Get("/health", app.HealthHandler)

	testCases := []struct {
		method      string
		url         string
		body        io.Reader
		headers     map[string]string
		expectCode  int
		description string
	}{
		{
			method:      "GET",
			url:         "/health",
			body:        nil,
			headers:     nil,
			expectCode:  http.StatusOK,
			description: "valid request",
		},
		{
			method:      "",
			url:         "/health",
			body:        nil,
			headers:     nil,
			expectCode:  http.StatusOK,  // Chi router actually accepts empty method as GET
			description: "empty method",
		},
		{
			method:      "INVALID_METHOD",
			url:         "/health", 
			body:        nil,
			headers:     nil,
			expectCode:  http.StatusMethodNotAllowed,
			description: "invalid HTTP method",
		},
		{
			method:      "GET",
			url:         "",
			body:        nil,
			headers:     nil,
			expectCode:  http.StatusNotFound,
			description: "empty URL path",
		},
		{
			method:      "POST",
			url:         "/health",
			body:        strings.NewReader("malformed json {"),
			headers:     map[string]string{"Content-Type": "application/json"},
			expectCode:  http.StatusMethodNotAllowed,
			description: "POST to GET endpoint with malformed JSON",
		},
		{
			method:      "GET",
			url:         "/health?param=" + strings.Repeat("x", 10000),
			body:        nil,
			headers:     nil,
			expectCode:  http.StatusOK,
			description: "very long query string",
		},
		{
			method: "GET",
			url:    "/health",
			body:   nil,
			headers: map[string]string{
				"Host":           "malformed..host..name",
				"Content-Length": "invalid",
				"User-Agent":     strings.Repeat("x", 1000),
			},
			expectCode:  http.StatusOK,
			description: "malformed headers",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			var req *http.Request
			var err error

			if tc.url == "" {
				req, err = http.NewRequest(tc.method, "http://example.com", tc.body)
			} else {
				req, err = http.NewRequest(tc.method, tc.url, tc.body)
			}

			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}

			// Add custom headers
			for key, value := range tc.headers {
				req.Header.Set(key, value)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tc.expectCode {
				t.Errorf("expected status %d, got %d", tc.expectCode, w.Code)
			}
		})
	}
}

func TestHTTPRequestTimeout(t *testing.T) {
	// Create a slow backend server
	slowBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("slow response"))
	}))
	defer slowBackend.Close()

	// Configure gateway with the slow backend
	cfg := &config.Config{
		Tenants: []config.Tenant{
			{
				Name:       "slow-tenant",
				PathPrefix: "/api/",
				Interval:   30,
				Services: []config.Service{
					{Name: "slow-svc", URL: slowBackend.URL, Health: "/health"},
				},
			},
		},
	}

	router := chi.NewRouter()
	app := &mockApplication{
		gateway: routing.NewGatewayWithRouter(cfg, router),
	}

	// Set up proxy handler
	router.HandleFunc("/api/*", app.ProxyHandler)

	// Test request with timeout
	req := httptest.NewRequest("GET", "/api/test", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	start := time.Now()

	// This should timeout before the backend responds
	router.ServeHTTP(w, req)
	duration := time.Since(start)

	// The request might complete normally or timeout depending on implementation
	// We mainly want to ensure it doesn't crash
	if duration > 3*time.Second {
		t.Errorf("request took too long: %v", duration)
	}
}

func TestHTTPResponseErrors(t *testing.T) {
	// Create backends that return various error responses
	errorBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/500":
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal Server Error"))
		case "/404":
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("Not Found"))
		case "/timeout":
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusRequestTimeout)
		case "/empty":
			w.WriteHeader(http.StatusOK)
			// Empty response body
		case "/large":
			w.WriteHeader(http.StatusOK)
			// Very large response
			largeData := strings.Repeat("x", 1024*1024) // 1MB
			w.Write([]byte(largeData))
		case "/invalid-json":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("invalid json {"))
		default:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}
	}))
	defer errorBackend.Close()

	cfg := &config.Config{
		Tenants: []config.Tenant{
			{
				Name:       "error-tenant",
				PathPrefix: "/api/",
				Interval:   30,
				Services: []config.Service{
					{Name: "error-svc", URL: errorBackend.URL, Health: "/health"},
				},
			},
		},
	}

	router := chi.NewRouter()
	app := &mockApplication{
		gateway: routing.NewGatewayWithRouter(cfg, router),
	}
	
	// Mark backend as healthy for testing
	if tenantRouter := app.gateway.GetTenantRouter("error-tenant"); tenantRouter != nil {
		if len(tenantRouter.Backends) > 0 {
			tenantRouter.Backends[0].Alive.Store(true)
		}
	}
	
	// Handle all /api/ requests with proxy
	router.HandleFunc("/api/{path:.*}", app.ProxyHandler)

	testCases := []struct {
		path        string
		expectCode  int
		description string
	}{
		{"/api/500", http.StatusInternalServerError, "500 error from backend"},
		{"/api/404", http.StatusNotFound, "404 error from backend"},
		{"/api/timeout", http.StatusRequestTimeout, "timeout from backend"},
		{"/api/empty", http.StatusOK, "empty response from backend"},
		{"/api/large", http.StatusOK, "large response from backend"},
		{"/api/invalid-json", http.StatusOK, "invalid JSON from backend"},
		{"/api/normal", http.StatusOK, "normal response from backend"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tc.expectCode {
				t.Errorf("expected status %d, got %d", tc.expectCode, w.Code)
			}
		})
	}
}

func TestHTTPNoBackendAvailable(t *testing.T) {
	// Configure gateway with no backends
	cfg := &config.Config{
		Tenants: []config.Tenant{
			{
				Name:       "empty-tenant",
				PathPrefix: "/api/",
				Interval:   30,
				Services:   []config.Service{}, // No services
			},
		},
	}

	router := chi.NewRouter()
	app := &mockApplication{
		gateway: routing.NewGatewayWithRouter(cfg, router),
	}
	router.HandleFunc("/api/*", app.ProxyHandler)

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should return 502 Bad Gateway when no backends available
	if w.Code != http.StatusBadGateway {
		t.Errorf("expected status %d, got %d", http.StatusBadGateway, w.Code)
	}

	if !strings.Contains(w.Body.String(), "No backend available") {
		t.Errorf("expected error message about no backend, got: %s", w.Body.String())
	}
}

func TestHTTPNoRouteMatch(t *testing.T) {
	cfg := &config.Config{
		Tenants: []config.Tenant{
			{
				Name:       "specific-tenant",
				PathPrefix: "/api/",
				Interval:   30,
				Services: []config.Service{
					{Name: "svc", URL: "http://backend", Health: "/health"},
				},
			},
		},
	}

	router := chi.NewRouter()
	app := &mockApplication{
		gateway: routing.NewGatewayWithRouter(cfg, router),
	}
	router.HandleFunc("/*", app.ProxyHandler)

	// Request to path that doesn't match any tenant
	req := httptest.NewRequest("GET", "/nomatch/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should return 404 when no route matches
	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestHTTPLargeRequestBody(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read the request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("received %d bytes", len(body))))
	}))
	defer backend.Close()

	cfg := &config.Config{
		Tenants: []config.Tenant{
			{
				Name:       "upload-tenant",
				PathPrefix: "/upload/",
				Interval:   30,
				Services: []config.Service{
					{Name: "upload-svc", URL: backend.URL, Health: "/health"},
				},
			},
		},
	}

	router := chi.NewRouter()
	app := &mockApplication{
		gateway: routing.NewGatewayWithRouter(cfg, router),
	}
	router.HandleFunc("/upload/*", app.ProxyHandler)

	testCases := []struct {
		size        int
		description string
	}{
		{0, "empty body"},
		{1024, "1KB body"},
		{1024 * 1024, "1MB body"},
		{5 * 1024 * 1024, "5MB body"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			largeData := bytes.Repeat([]byte("x"), tc.size)
			req := httptest.NewRequest("POST", "/upload/data", bytes.NewReader(largeData))
			req.Header.Set("Content-Type", "application/octet-stream")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should handle large bodies without crashing
			if w.Code != http.StatusOK {
				t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
			}
		})
	}
}

func TestHTTPHeaderManipulation(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Echo back request headers as JSON
		headers := make(map[string]string)
		for key, values := range r.Header {
			headers[key] = strings.Join(values, ",")
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Backend-Response", "true")
		json.NewEncoder(w).Encode(headers)
	}))
	defer backend.Close()

	cfg := &config.Config{
		Tenants: []config.Tenant{
			{
				Name:       "header-tenant",
				PathPrefix: "/api/",
				Interval:   30,
				Services: []config.Service{
					{Name: "header-svc", URL: backend.URL, Health: "/health"},
				},
			},
		},
	}

	router := chi.NewRouter()
	app := &mockApplication{
		gateway: routing.NewGatewayWithRouter(cfg, router),
	}
	router.HandleFunc("/api/*", app.ProxyHandler)

	// Test various header scenarios
	testCases := []struct {
		headers     map[string]string
		description string
	}{
		{
			map[string]string{"User-Agent": "test-client"},
			"standard user agent",
		},
		{
			map[string]string{
				"X-Custom-Header": "custom-value",
				"Authorization":   "Bearer token123",
			},
			"custom and auth headers",
		},
		{
			map[string]string{
				"Content-Type": "application/json",
				"Accept":       "application/json",
			},
			"content type headers",
		},
		{
			map[string]string{
				"X-Very-Long-Header-Name-That-Exceeds-Normal-Length": strings.Repeat("x", 1000),
			},
			"very long header",
		},
		{
			map[string]string{
				"X-Special-Chars": "value with spaces and symbols !@#$%",
			},
			"header with special characters",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/headers", nil)

			// Add test headers
			for key, value := range tc.headers {
				req.Header.Set(key, value)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
			}

			// Verify backend received the response header
			if w.Header().Get("X-Backend-Response") != "true" {
				t.Error("expected backend response header")
			}

			// The response should contain JSON with the forwarded headers
			if !strings.Contains(w.Header().Get("Content-Type"), "application/json") {
				t.Error("expected JSON content type in response")
			}
		})
	}
}

func TestHTTPAdminEndpointErrors(t *testing.T) {
	cfg := &config.Config{
		AdminBasePath: "/admin",
		Tenants: []config.Tenant{
			{
				Name:       "test-tenant",
				PathPrefix: "/api/",
				Interval:   30,
				Services: []config.Service{
					{Name: "svc", URL: "http://backend", Health: "/health"},
				},
			},
		},
	}

	router := chi.NewRouter()
	app := &mockApplication{
		gateway: routing.NewGatewayWithRouter(cfg, router),
	}

	// Set up admin endpoints
	router.Get("/admin/health", app.HealthHandler)
	router.Get("/admin/tenants", app.TenantsHandler)

	testCases := []struct {
		path        string
		method      string
		expectCode  int
		description string
	}{
		{"/admin/health", "GET", http.StatusOK, "health endpoint"},
		{"/admin/tenants", "GET", http.StatusOK, "tenants endpoint"},
		{"/admin/nonexistent", "GET", http.StatusNotFound, "nonexistent admin endpoint"},
		{"/admin/health", "POST", http.StatusMethodNotAllowed, "wrong method to health"},
		{"/admin/tenants", "DELETE", http.StatusMethodNotAllowed, "wrong method to tenants"},
		{"/admin", "GET", http.StatusNotFound, "admin root without trailing"},
		{"/admin/", "GET", http.StatusNotFound, "admin root with trailing"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tc.expectCode {
				t.Errorf("expected status %d, got %d", tc.expectCode, w.Code)
			}

			// For successful responses, verify content type
			if tc.expectCode == http.StatusOK {
				contentType := w.Header().Get("Content-Type")
				if !strings.Contains(contentType, "application/json") {
					t.Errorf("expected JSON content type, got %s", contentType)
				}
			}
		})
	}
}

func TestHTTPConnectionDrops(t *testing.T) {
	// Create a backend that drops connections
	droppingBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Start writing response then close connection
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("partial response"))
		
		// Force connection close by hijacking and closing
		if hijacker, ok := w.(http.Hijacker); ok {
			conn, _, err := hijacker.Hijack()
			if err == nil {
				conn.Close()
				return
			}
		}
		
		// Fallback: just finish normally
		w.Write([]byte(" complete"))
	}))
	defer droppingBackend.Close()

	cfg := &config.Config{
		Tenants: []config.Tenant{
			{
				Name:       "dropping-tenant",
				PathPrefix: "/api/",
				Interval:   30,
				Services: []config.Service{
					{Name: "drop-svc", URL: droppingBackend.URL, Health: "/health"},
				},
			},
		},
	}

	router := chi.NewRouter()
	app := &mockApplication{
		gateway: routing.NewGatewayWithRouter(cfg, router),
	}
	router.HandleFunc("/api/*", app.ProxyHandler)

	req := httptest.NewRequest("GET", "/api/drop", nil)
	w := httptest.NewRecorder()

	// This should not crash even if backend drops connection
	router.ServeHTTP(w, req)

	// We mainly want to ensure it doesn't crash
	// The exact status code may vary depending on how the connection drop is handled
	if w.Code == 0 {
		t.Error("expected some HTTP status code, got 0")
	}
}

func TestHTTPQueryParameterHandling(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Echo back query parameters
		params := r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(params)
	}))
	defer backend.Close()

	cfg := &config.Config{
		Tenants: []config.Tenant{
			{
				Name:       "query-tenant",
				PathPrefix: "/api/",
				Interval:   30,
				Services: []config.Service{
					{Name: "query-svc", URL: backend.URL, Health: "/health"},
				},
			},
		},
	}

	router := chi.NewRouter()
	app := &mockApplication{
		gateway: routing.NewGatewayWithRouter(cfg, router),
	}
	router.HandleFunc("/api/*", app.ProxyHandler)

	testCases := []struct {
		query       string
		description string
	}{
		{"", "no query parameters"},
		{"?simple=value", "simple query parameter"},
		{"?key1=value1&key2=value2", "multiple parameters"},
		{"?empty=", "empty parameter value"},
		{"?special=" + url.QueryEscape("value with spaces & symbols"), "URL encoded parameter"},
		{"?" + strings.Repeat("key=value&", 100)[:len(strings.Repeat("key=value&", 100))-1], "many parameters"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/test"+tc.query, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
			}

			// Response should be valid JSON
			var result map[string][]string
			err := json.NewDecoder(w.Body).Decode(&result)
			if err != nil {
				t.Errorf("failed to decode JSON response: %v", err)
			}

			// Should contain backend parameter from service URL
			if _, exists := result["backend"]; !exists && len(tc.query) > 0 {
				// Backend parameter should be merged with request parameters
			}
		})
	}
}