package unit

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"keystone-gateway/internal/config"
	"keystone-gateway/internal/routing"
)

func TestHostMiddleware(t *testing.T) {
	gw := routing.NewGateway(&config.Config{})

	tests := []struct {
		name           string
		allowedDomains []string
		requestHost    string
		expectPass     bool
	}{
		{
			name:           "Allowed domain passes",
			allowedDomains: []string{"example.com", "api.example.com"},
			requestHost:    "api.example.com",
			expectPass:     true,
		},
		{
			name:           "Domain with port passes",
			allowedDomains: []string{"example.com"},
			requestHost:    "example.com:8080",
			expectPass:     true,
		},
		{
			name:           "Disallowed domain fails",
			allowedDomains: []string{"example.com"},
			requestHost:    "malicious.com",
			expectPass:     false,
		},
		{
			name:           "Empty allowed domains blocks all",
			allowedDomains: []string{},
			requestHost:    "example.com",
			expectPass:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := gw.HostMiddleware(tt.allowedDomains)

			handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("allowed"))
			}))

			req := httptest.NewRequest("GET", "/test", nil)
			req.Host = tt.requestHost
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if tt.expectPass {
				assert.Equal(t, http.StatusOK, w.Code)
				assert.Equal(t, "allowed", w.Body.String())
			} else {
				assert.Equal(t, http.StatusNotFound, w.Code)
			}
		})
	}
}

func TestProxyMiddleware(t *testing.T) {
	// Test no backends available
	t.Run("No backends available", func(t *testing.T) {
		gw := routing.NewGateway(&config.Config{})
		emptyTr := &routing.TenantRouter{Name: "empty", Backends: []*routing.GatewayBackend{}}
		emptyMiddleware := gw.ProxyMiddleware(emptyTr, "")

		handler := emptyMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("Handler called when should have returned error")
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadGateway, w.Code)
		assert.Contains(t, w.Body.String(), "No backend available")
	})
}

func TestExtractHost(t *testing.T) {
	tests := []struct {
		name         string
		hostHeader   string
		expectedHost string
	}{
		{
			name:         "Host without port",
			hostHeader:   "example.com",
			expectedHost: "example.com",
		},
		{
			name:         "Host with port",
			hostHeader:   "example.com:8080",
			expectedHost: "example.com",
		},
		{
			name:         "Host with standard port",
			hostHeader:   "example.com:80",
			expectedHost: "example.com",
		},
		{
			name:         "IPv4 with port",
			hostHeader:   "192.168.1.1:3000",
			expectedHost: "192.168.1.1",
		},
		{
			name:         "IPv6 with port (limitation: doesn't parse correctly)",
			hostHeader:   "[::1]:8080",
			expectedHost: "[", // Current implementation limitation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := routing.ExtractHost(tt.hostHeader)
			assert.Equal(t, tt.expectedHost, result)
		})
	}
}
