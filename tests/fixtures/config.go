package fixtures

import "keystone-gateway/internal/config"

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
			{
				Name:       "tenant-a",
				PathPrefix: "/api/a/",
				Interval:   30,
				Services: []config.Service{
					{Name: "backend-a", URL: "http://backend-a:8080", Health: "/health"},
				},
			},
			{
				Name:       "tenant-b",
				PathPrefix: "/api/b/",
				Interval:   30,
				Services: []config.Service{
					{Name: "backend-b", URL: "http://backend-b:8080", Health: "/health"},
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

// CreateAdminConfig creates a config with admin endpoints enabled
func CreateAdminConfig(adminBasePath string) *config.Config {
	cfg := CreateTestConfig("test-tenant", "/api/")
	cfg.AdminBasePath = adminBasePath
	return cfg
}