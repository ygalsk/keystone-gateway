package unit

import (
	"os"
	"path/filepath"
	"testing"

	"keystone-gateway/internal/config"
)

// TestConfigCore tests essential configuration functionality for 80%+ coverage
func TestConfigCore(t *testing.T) {
	t.Run("load_valid_config", func(t *testing.T) {
		configYAML := `
tenants:
  - name: api
    path_prefix: /api/
    services:
      - name: backend
        url: http://localhost:8080
        health: /health
lua_routing:
  enabled: true
  scripts_dir: ./scripts
`
		tmpDir := t.TempDir()
		configFile := filepath.Join(tmpDir, "config.yaml")
		err := os.WriteFile(configFile, []byte(configYAML), 0644)
		if err != nil {
			t.Fatalf("Write config failed: %v", err)
		}

		cfg, err := config.LoadConfig(configFile)
		if err != nil {
			t.Fatalf("LoadConfig failed: %v", err)
		}

		if len(cfg.Tenants) != 1 {
			t.Errorf("Expected 1 tenant, got %d", len(cfg.Tenants))
		}

		if cfg.Tenants[0].Name != "api" {
			t.Errorf("Expected tenant name 'api', got '%s'", cfg.Tenants[0].Name)
		}
	})

	t.Run("load_multi_tenant_config", func(t *testing.T) {
		configYAML := `
tenants:
  - name: api
    domains: ["api.example.com"]
    services:
      - name: api-backend
        url: http://api:8080
        health: /health
  - name: web
    path_prefix: /web/
    services:
      - name: web-backend
        url: http://web:8080
        health: /health
`
		tmpDir := t.TempDir()
		configFile := filepath.Join(tmpDir, "config.yaml")
		err := os.WriteFile(configFile, []byte(configYAML), 0644)
		if err != nil {
			t.Fatalf("Write config failed: %v", err)
		}

		cfg, err := config.LoadConfig(configFile)
		if err != nil {
			t.Fatalf("LoadConfig failed: %v", err)
		}

		if len(cfg.Tenants) != 2 {
			t.Errorf("Expected 2 tenants, got %d", len(cfg.Tenants))
		}
	})

	t.Run("validation_errors", func(t *testing.T) {
		testCases := []struct {
			name   string
			config string
		}{
			{
				name: "no_routing_config",
				config: `
tenants:
  - name: invalid
    services:
      - name: backend
        url: http://localhost:8080
`,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				tmpDir := t.TempDir()
				configFile := filepath.Join(tmpDir, "config.yaml")
				err := os.WriteFile(configFile, []byte(tc.config), 0644)
				if err != nil {
					t.Fatalf("Write config failed: %v", err)
				}

				_, err = config.LoadConfig(configFile)
				if err == nil {
					t.Error("Expected validation error, got nil")
				}
			})
		}
	})

	t.Run("compression_defaults", func(t *testing.T) {
		configYAML := `
tenants:
  - name: simple
    path_prefix: /simple/
    services:
      - name: backend
        url: http://localhost:8080
`
		tmpDir := t.TempDir()
		configFile := filepath.Join(tmpDir, "config.yaml")
		err := os.WriteFile(configFile, []byte(configYAML), 0644)
		if err != nil {
			t.Fatalf("Write config failed: %v", err)
		}

		cfg, err := config.LoadConfig(configFile)
		if err != nil {
			t.Fatalf("LoadConfig failed: %v", err)
		}

		compression := cfg.GetCompressionConfig()
		if !compression.Enabled {
			t.Error("Expected compression enabled by default")
		}
		if compression.Level != 5 {
			t.Errorf("Expected compression level 5, got %d", compression.Level)
		}
	})

	t.Run("file_not_found", func(t *testing.T) {
		_, err := config.LoadConfig("/nonexistent/config.yaml")
		if err == nil {
			t.Error("Expected error for nonexistent file")
		}
	})

	t.Run("invalid_yaml", func(t *testing.T) {
		invalidYAML := `invalid: yaml: content: [`
		tmpDir := t.TempDir()
		configFile := filepath.Join(tmpDir, "invalid.yaml")
		err := os.WriteFile(configFile, []byte(invalidYAML), 0644)
		if err != nil {
			t.Fatalf("Failed to write test config: %v", err)
		}

		_, err = config.LoadConfig(configFile)
		if err == nil {
			t.Error("Expected error for invalid YAML")
		}
	})
}

// TestConfigDirect tests config package functions directly for better coverage
func TestConfigDirect(t *testing.T) {
	t.Run("validate_tenant_valid", func(t *testing.T) {
		tenant := config.Tenant{
			Name:       "api",
			PathPrefix: "/api/",
			Services: []config.Service{
				{Name: "backend", URL: "http://localhost:8080"},
			},
		}

		err := config.ValidateTenant(tenant)
		if err != nil {
			t.Errorf("Valid tenant failed validation: %v", err)
		}
	})

	t.Run("validate_tenant_invalid", func(t *testing.T) {
		tenant := config.Tenant{
			Name: "invalid",
			Services: []config.Service{
				{Name: "backend", URL: "http://localhost:8080"},
			},
		}

		err := config.ValidateTenant(tenant)
		if err == nil {
			t.Error("Expected validation error for tenant without routing config")
		}
	})

	t.Run("get_compression_config_default", func(t *testing.T) {
		cfg := &config.Config{}
		compression := cfg.GetCompressionConfig()

		if !compression.Enabled {
			t.Error("Expected compression enabled by default")
		}

		if compression.Level != 5 {
			t.Errorf("Expected compression level 5, got %d", compression.Level)
		}

		if len(compression.ContentTypes) == 0 {
			t.Error("Expected default content types")
		}
	})

	t.Run("get_compression_config_custom", func(t *testing.T) {
		cfg := &config.Config{
			Compression: &config.CompressionConfig{
				Enabled: false,
				Level:   3,
			},
		}

		compression := cfg.GetCompressionConfig()

		if compression.Enabled {
			t.Error("Expected compression disabled")
		}

		if compression.Level != 3 {
			t.Errorf("Expected compression level 3, got %d", compression.Level)
		}
	})
}
