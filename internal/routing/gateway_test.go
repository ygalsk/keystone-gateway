package routing

import (
	"testing"

	"keystone-gateway/internal/config"
)

func TestGatewayCreation(t *testing.T) {
	cfg := &config.Config{
		Tenants: []config.Tenant{
			{
				Name:       "test-service",
				PathPrefix: "/api/",
				Services: []config.Service{
					{Name: "backend", URL: "http://localhost:8081", Health: "/health"},
				},
			},
		},
		AdminBasePath: "/admin",
	}

	gateway := NewGateway(cfg)

	if gateway == nil {
		t.Fatal("Gateway creation failed")
	}

	if gateway.GetConfig() != cfg {
		t.Error("Gateway config not set correctly")
	}
}

func TestRouteMatching(t *testing.T) {
	cfg := &config.Config{
		Tenants: []config.Tenant{
			{
				Name:       "api-service",
				PathPrefix: "/api/",
				Services: []config.Service{
					{Name: "backend", URL: "http://localhost:8081", Health: "/health"},
				},
			},
			{
				Name:    "app-service",
				Domains: []string{"app.example.com"},
				Services: []config.Service{
					{Name: "backend", URL: "http://localhost:8082", Health: "/health"},
				},
			},
		},
	}

	gateway := NewGateway(cfg)

	tests := []struct {
		name           string
		host           string
		path           string
		expectMatch    bool
		expectedPrefix string
	}{
		{
			name:           "Path-based routing",
			host:           "localhost",
			path:           "/api/users",
			expectMatch:    true,
			expectedPrefix: "/api/",
		},
		{
			name:           "Host-based routing",
			host:           "app.example.com",
			path:           "/dashboard",
			expectMatch:    true,
			expectedPrefix: "",
		},
		{
			name:        "No match",
			host:        "unknown.com",
			path:        "/unknown",
			expectMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, prefix := gateway.MatchRoute(tt.host, tt.path)

			if tt.expectMatch {
				if router == nil {
					t.Errorf("Expected route match, but got none")
				}
				if prefix != tt.expectedPrefix {
					t.Errorf("Expected prefix '%s', got '%s'", tt.expectedPrefix, prefix)
				}
			} else {
				if router != nil {
					t.Errorf("Expected no route match, but got one")
				}
			}
		})
	}
}
