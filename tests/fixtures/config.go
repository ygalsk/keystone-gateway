package fixtures

import (
	"fmt"
	"keystone-gateway/internal/config"
)

// CreateTestConfig creates a basic test configuration
func CreateTestConfig(tenantName, pathPrefix string) *config.Config {
	return &config.Config{
		Tenants: []config.Tenant{
			{
				Name:       tenantName,
				PathPrefix: pathPrefix,
				Interval:   30,
				Services: []config.Service{
					{Name: "test-service", URL: "http://test-backend:8080", Health: "/health"},
				},
			},
		},
	}
}

// CreateMultiTenantConfig creates a multi-tenant test configuration
func CreateMultiTenantConfig() *config.Config {
	return &config.Config{
		Tenants: []config.Tenant{
			// Host-based tenants
			{
				Name:     "api-tenant",
				Domains:  []string{"api.example.com"},
				Interval: 30,
				Services: []config.Service{
					{Name: "api-backend", URL: "http://api-backend:8080", Health: "/health"},
				},
			},
			{
				Name:     "web-tenant",
				Domains:  []string{"web.example.com"},
				Interval: 30,
				Services: []config.Service{
					{Name: "web-backend", URL: "http://web-backend:8080", Health: "/health"},
				},
			},
			{
				Name:     "mobile-tenant",
				Domains:  []string{"mobile.example.com"},
				Interval: 30,
				Services: []config.Service{
					{Name: "mobile-backend", URL: "http://mobile-backend:8080", Health: "/health"},
				},
			},
			// Path-based tenants
			{
				Name:       "admin-tenant",
				PathPrefix: "/admin/",
				Interval:   30,
				Services: []config.Service{
					{Name: "admin-backend", URL: "http://admin-backend:8080", Health: "/health"},
				},
			},
			{
				Name:       "api-path-tenant",
				PathPrefix: "/api/v1/",
				Interval:   30,
				Services: []config.Service{
					{Name: "api-path-backend", URL: "http://api-path-backend:8080", Health: "/health"},
				},
			},
			// Hybrid tenant (host + path)
			{
				Name:       "hybrid-tenant",
				Domains:    []string{"api.example.com"},
				PathPrefix: "/v2/",
				Interval:   30,
				Services: []config.Service{
					{Name: "hybrid-backend", URL: "http://hybrid-backend:8080", Health: "/health"},
				},
			},
		},
	}
}

// CreateConfigWithBackend creates a config with a specific backend URL
func CreateConfigWithBackend(tenantName, pathPrefix, backendURL string) *config.Config {
	return &config.Config{
		Tenants: []config.Tenant{
			{
				Name:       tenantName,
				PathPrefix: pathPrefix,
				Interval:   30,
				Services: []config.Service{
					{Name: "test-service", URL: backendURL, Health: "/health"},
				},
			},
		},
	}
}

// CreateConfigWithMultipleBackends creates a config with multiple backend URLs for load balancing
func CreateConfigWithMultipleBackends(tenantName, pathPrefix string, backendURLs []string) *config.Config {
	services := make([]config.Service, len(backendURLs))
	for i, url := range backendURLs {
		services[i] = config.Service{
			Name:   fmt.Sprintf("backend-%d", i),
			URL:    url,
			Health: "/health",
		}
	}
	
	return &config.Config{
		Tenants: []config.Tenant{
			{
				Name:       tenantName,
				PathPrefix: pathPrefix,
				Interval:   30,
				Services:   services,
			},
		},
	}
}

// CreateAdminConfig creates a config with admin endpoints enabled
func CreateAdminConfig(adminBasePath string) *config.Config {
	cfg := CreateTestConfig("test-tenant", "/api/")
	cfg.AdminBasePath = adminBasePath
	return cfg
}

// CreateHealthAndAPIConfig creates a config that handles both health and API endpoints
func CreateHealthAndAPIConfig(tenantName, backendURL string) *config.Config {
	return &config.Config{
		Tenants: []config.Tenant{
			// Health endpoint tenant (root level)
			{
				Name:       tenantName + "-health",
				PathPrefix: "/health",
				Interval:   30,
				Services: []config.Service{
					{Name: "health-service", URL: backendURL, Health: "/health"},
				},
			},
			// API endpoint tenant
			{
				Name:       tenantName,
				PathPrefix: "/api/",
				Interval:   30,
				Services: []config.Service{
					{Name: "api-service", URL: backendURL, Health: "/health"},
				},
			},
		},
	}
}