package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file for testing
	configContent := `
tenants:
  - name: "test-service"
    path_prefix: "/api/"
    services:
      - name: "backend"
        url: "http://localhost:8081"
        health: "/health"
admin_base_path: "/admin"
`

	tmpFile, err := os.CreateTemp("", "test-config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	tmpFile.Close()

	// Test loading the config
	cfg, err := LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Validate the loaded config
	if len(cfg.Tenants) != 1 {
		t.Errorf("Expected 1 tenant, got %d", len(cfg.Tenants))
	}

	tenant := cfg.Tenants[0]
	if tenant.Name != "test-service" {
		t.Errorf("Expected tenant name 'test-service', got '%s'", tenant.Name)
	}

	if tenant.PathPrefix != "/api/" {
		t.Errorf("Expected path prefix '/api/', got '%s'", tenant.PathPrefix)
	}

	if len(tenant.Services) != 1 {
		t.Errorf("Expected 1 service, got %d", len(tenant.Services))
	}
}

func TestValidateTenant(t *testing.T) {
	tests := []struct {
		name    string
		tenant  Tenant
		wantErr bool
	}{
		{
			name: "valid path-based tenant",
			tenant: Tenant{
				Name:       "test",
				PathPrefix: "/api/",
				Services:   []Service{{Name: "svc", URL: "http://localhost:8080", Health: "/health"}},
			},
			wantErr: false,
		},
		{
			name: "valid domain-based tenant",
			tenant: Tenant{
				Name:     "test",
				Domains:  []string{"api.example.com"},
				Services: []Service{{Name: "svc", URL: "http://localhost:8080", Health: "/health"}},
			},
			wantErr: false,
		},
		{
			name: "invalid tenant - no routing config",
			tenant: Tenant{
				Name:     "test",
				Services: []Service{{Name: "svc", URL: "http://localhost:8080", Health: "/health"}},
			},
			wantErr: true,
		},
		{
			name: "invalid tenant - bad path prefix",
			tenant: Tenant{
				Name:       "test",
				PathPrefix: "api", // Should start and end with /
				Services:   []Service{{Name: "svc", URL: "http://localhost:8080", Health: "/health"}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTenant(tt.tenant)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateTenant() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
