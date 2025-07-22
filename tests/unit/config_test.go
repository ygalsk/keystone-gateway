package unit

import (
	"os"
	"path/filepath"
	"testing"

	"keystone-gateway/internal/config"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name        string
		configPath  string
		expectError bool
	}{
		{
			name:        "valid config",
			configPath:  "../../testdata/configs/valid.yaml",
			expectError: false,
		},
		{
			name:        "invalid config",
			configPath:  "../../testdata/configs/invalid.yaml",
			expectError: true,
		},
		{
			name:        "nonexistent file",
			configPath:  "nonexistent.yaml",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := config.LoadConfig(tt.configPath)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if cfg == nil {
				t.Errorf("expected config but got nil")
			}
		})
	}
}

func TestValidateTenant(t *testing.T) {
	tests := []struct {
		name        string
		tenant      config.Tenant
		expectError bool
	}{
		{
			name: "valid tenant with path prefix",
			tenant: config.Tenant{
				Name:       "test",
				PathPrefix: "/api/",
			},
			expectError: false,
		},
		{
			name: "valid tenant with domains",
			tenant: config.Tenant{
				Name:    "test",
				Domains: []string{"example.com"},
			},
			expectError: false,
		},
		{
			name: "invalid tenant - no routing config",
			tenant: config.Tenant{
				Name: "test",
			},
			expectError: true,
		},
		{
			name: "invalid tenant - bad path prefix",
			tenant: config.Tenant{
				Name:       "test",
				PathPrefix: "api",
			},
			expectError: true,
		},
		{
			name: "invalid tenant - bad domain",
			tenant: config.Tenant{
				Name:    "test",
				Domains: []string{"invalid domain"},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := config.ValidateTenant(tt.tenant)
			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestTLSConfig(t *testing.T) {
	// Create a temporary config file with TLS settings
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "tls_config.yaml")

	configContent := `
lua_routing:
  enabled: true

tls:
  enabled: true
  cert_file: "/path/to/cert.pem"
  key_file: "/path/to/key.pem"

tenants:
  - name: "test-tenant"
    path_prefix: "/api/"
    services:
      - name: "backend1"
        url: "http://localhost:8081"
        health: "/health"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.TLS == nil {
		t.Fatal("expected TLS config but got nil")
	}

	if !cfg.TLS.Enabled {
		t.Error("expected TLS to be enabled")
	}

	if cfg.TLS.CertFile != "/path/to/cert.pem" {
		t.Errorf("expected cert file '/path/to/cert.pem', got %q", cfg.TLS.CertFile)
	}

	if cfg.TLS.KeyFile != "/path/to/key.pem" {
		t.Errorf("expected key file '/path/to/key.pem', got %q", cfg.TLS.KeyFile)
	}
}
