package unit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"keystone-gateway/internal/config"
)

// TestConfigLoading tests YAML configuration loading
func TestConfigLoading(t *testing.T) {
	testCases := []struct {
		name        string
		configYAML  string
		expectError bool
		errorContains string
	}{
		{
			name: "valid basic configuration",
			configYAML: `
tenants:
  - name: test-tenant
    path_prefix: /api/
    services:
      - name: backend1
        url: http://localhost:8080
        health: /health
`,
			expectError: false,
		},
		{
			name: "valid multi-tenant configuration",
			configYAML: `
tenants:
  - name: api-tenant
    domains: ["api.example.com"]
    services:
      - name: api-backend
        url: http://api-server:8080
        health: /health
  - name: web-tenant
    path_prefix: /web/
    services:
      - name: web-backend
        url: http://web-server:8080
        health: /health
admin_base_path: /admin
`,
			expectError: false,
		},
		{
			name: "valid hybrid configuration",
			configYAML: `
tenants:
  - name: hybrid-tenant
    domains: ["api.example.com"]
    path_prefix: /v2/
    services:
      - name: hybrid-backend
        url: http://hybrid-server:8080
        health: /health
`,
			expectError: false,
		},
		{
			name: "configuration with TLS",
			configYAML: `
tenants:
  - name: secure-tenant
    domains: ["secure.example.com"]
    services:
      - name: secure-backend
        url: https://secure-server:8443
        health: /health
tls:
  enabled: true
  cert_file: /etc/ssl/cert.pem
  key_file: /etc/ssl/key.pem
`,
			expectError: false,
		},
		{
			name: "configuration with Lua routing",
			configYAML: `
tenants:
  - name: lua-tenant
    path_prefix: /lua/
    lua_routes: tenant-routes
    services:
      - name: lua-backend
        url: http://lua-server:8080
        health: /health
lua_routing:
  enabled: true
  scripts_dir: /etc/lua/scripts
  global_scripts: ["middleware", "auth"]
`,
			expectError: false,
		},
		{
			name: "invalid YAML syntax",
			configYAML: `
tenants:
  - name: test-tenant
    path_prefix: /api/
    services:
      - name: backend1
        url: http://localhost:8080
        health: /health
      invalid_yaml: [
`,
			expectError: true,
			errorContains: "failed to parse config",
		},
		{
			name: "empty configuration",
			configYAML: ``,
			expectError: false, // Empty config is valid, just no tenants
		},
		{
			name: "tenant without domain or path",
			configYAML: `
tenants:
  - name: invalid-tenant
    services:
      - name: backend1
        url: http://localhost:8080
        health: /health
`,
			expectError: true,
			errorContains: "must specify either domains or path_prefix",
		},
		{
			name: "tenant with invalid domain",
			configYAML: `
tenants:
  - name: invalid-domain-tenant
    domains: ["invalid domain with spaces"]
    services:
      - name: backend1
        url: http://localhost:8080
        health: /health
`,
			expectError: true,
			errorContains: "invalid domain",
		},
		{
			name: "tenant with invalid path prefix",
			configYAML: `
tenants:
  - name: invalid-path-tenant
    path_prefix: "api"
    services:
      - name: backend1
        url: http://localhost:8080
        health: /health
`,
			expectError: true,
			errorContains: "path_prefix must start and end with '/'",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create temporary config file
			tempDir := t.TempDir()
			configPath := filepath.Join(tempDir, "config.yaml")

			err := os.WriteFile(configPath, []byte(tc.configYAML), 0644)
			if err != nil {
				t.Fatalf("Failed to write config file: %v", err)
			}

			// Load configuration
			cfg, err := config.LoadConfig(configPath)

			if tc.expectError {
				if err == nil {
					t.Fatal("Expected error, got nil")
				}
				if tc.errorContains != "" && !strings.Contains(err.Error(), tc.errorContains) {
					t.Errorf("Expected error containing %q, got %q", tc.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
				if cfg == nil {
					t.Error("Expected config, got nil")
				}
			}
		})
	}
}

// TestTenantValidation tests tenant validation logic
func TestTenantValidation(t *testing.T) {
	testCases := []struct {
		name        string
		tenant      config.Tenant
		expectError bool
		errorContains string
	}{
		{
			name: "valid domain-based tenant",
			tenant: config.Tenant{
				Name:    "domain-tenant",
				Domains: []string{"example.com", "api.example.com"},
				Services: []config.Service{{
					Name:   "backend",
					URL:    "http://localhost:8080",
					Health: "/health",
				}},
			},
			expectError: false,
		},
		{
			name: "valid path-based tenant",
			tenant: config.Tenant{
				Name:       "path-tenant",
				PathPrefix: "/api/",
				Services: []config.Service{{
					Name:   "backend",
					URL:    "http://localhost:8080",
					Health: "/health",
				}},
			},
			expectError: false,
		},
		{
			name: "valid hybrid tenant",
			tenant: config.Tenant{
				Name:       "hybrid-tenant",
				Domains:    []string{"api.example.com"},
				PathPrefix: "/v2/",
				Services: []config.Service{{
					Name:   "backend",
					URL:    "http://localhost:8080",
					Health: "/health",
				}},
			},
			expectError: false,
		},
		{
			name: "tenant with no routing configuration",
			tenant: config.Tenant{
				Name: "invalid-tenant",
				Services: []config.Service{{
					Name:   "backend",
					URL:    "http://localhost:8080",
					Health: "/health",
				}},
			},
			expectError: true,
			errorContains: "must specify either domains or path_prefix",
		},
		{
			name: "tenant with invalid domain format",
			tenant: config.Tenant{
				Name:    "invalid-domain-tenant",
				Domains: []string{"invalid domain"},
				Services: []config.Service{{
					Name:   "backend",
					URL:    "http://localhost:8080",
					Health: "/health",
				}},
			},
			expectError: true,
			errorContains: "invalid domain",
		},
		{
			name: "tenant with empty domain",
			tenant: config.Tenant{
				Name:    "empty-domain-tenant",
				Domains: []string{""},
				Services: []config.Service{{
					Name:   "backend",
					URL:    "http://localhost:8080",
					Health: "/health",
				}},
			},
			expectError: true,
			errorContains: "invalid domain",
		},
		{
			name: "tenant with path prefix missing leading slash",
			tenant: config.Tenant{
				Name:       "invalid-path-tenant",
				PathPrefix: "api/",
				Services: []config.Service{{
					Name:   "backend",
					URL:    "http://localhost:8080",
					Health: "/health",
				}},
			},
			expectError: true,
			errorContains: "path_prefix must start and end with '/'",
		},
		{
			name: "tenant with path prefix missing trailing slash",
			tenant: config.Tenant{
				Name:       "invalid-path-tenant",
				PathPrefix: "/api",
				Services: []config.Service{{
					Name:   "backend",
					URL:    "http://localhost:8080",
					Health: "/health",
				}},
			},
			expectError: true,
			errorContains: "path_prefix must start and end with '/'",
		},
		{
			name: "tenant with single character path",
			tenant: config.Tenant{
				Name:       "single-char-tenant",
				PathPrefix: "/",
				Services: []config.Service{{
					Name:   "backend",
					URL:    "http://localhost:8080",
					Health: "/health",
				}},
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := config.ValidateTenant(tc.tenant)

			if tc.expectError {
				if err == nil {
					t.Fatal("Expected error, got nil")
				}
				if tc.errorContains != "" && !strings.Contains(err.Error(), tc.errorContains) {
					t.Errorf("Expected error containing %q, got %q", tc.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}
		})
	}
}

// TestDomainValidation tests domain validation logic
func TestDomainValidation(t *testing.T) {
	testCases := []struct {
		name        string
		domains     []string
		expectError bool
	}{
		{
			name:        "valid domains",
			domains:     []string{"example.com", "api.example.com", "sub.domain.example.org"},
			expectError: false,
		},
		{
			name:        "domain with hyphen",
			domains:     []string{"my-domain.com"},
			expectError: false,
		},
		{
			name:        "domain with numbers",
			domains:     []string{"domain123.com"},
			expectError: false,
		},
		{
			name:        "internationalized domain",
			domains:     []string{"xn--fsq.com"}, // Punycode for Chinese domain
			expectError: false,
		},
		{
			name:        "localhost",
			domains:     []string{"localhost"},
			expectError: true, // No dot in domain
		},
		{
			name:        "IP address",
			domains:     []string{"192.168.1.1"},
			expectError: true, // No dot in valid domain context
		},
		{
			name:        "domain with space",
			domains:     []string{"invalid domain.com"},
			expectError: true,
		},
		{
			name:        "empty domain",
			domains:     []string{""},
			expectError: true,
		},
		{
			name:        "domain without TLD",
			domains:     []string{"nodot"},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tenant := config.Tenant{
				Name:    "test-tenant",
				Domains: tc.domains,
				Services: []config.Service{{
					Name:   "backend",
					URL:    "http://localhost:8080",
					Health: "/health",
				}},
			}

			err := config.ValidateTenant(tenant)

			if tc.expectError {
				if err == nil {
					t.Fatal("Expected error for invalid domain, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for valid domain, got: %v", err)
				}
			}
		})
	}
}

// TestConfigFileHandling tests file system operations
func TestConfigFileHandling(t *testing.T) {
	testCases := []struct {
		name        string
		setupFunc   func(t *testing.T) string
		expectError bool
		errorContains string
	}{
		{
			name: "non-existent file",
			setupFunc: func(t *testing.T) string {
				return "/non/existent/config.yaml"
			},
			expectError: true,
			errorContains: "failed to read config",
		},
		{
			name: "directory instead of file",
			setupFunc: func(t *testing.T) string {
				return t.TempDir()
			},
			expectError: true,
			errorContains: "failed to read config",
		},
		{
			name: "empty file",
			setupFunc: func(t *testing.T) string {
				tempDir := t.TempDir()
				configPath := filepath.Join(tempDir, "empty.yaml")
				err := os.WriteFile(configPath, []byte(""), 0644)
				if err != nil {
					t.Fatalf("Failed to create empty file: %v", err)
				}
				return configPath
			},
			expectError: false,
		},
		{
			name: "file with only whitespace",
			setupFunc: func(t *testing.T) string {
				tempDir := t.TempDir()
				configPath := filepath.Join(tempDir, "whitespace.yaml")
				err := os.WriteFile(configPath, []byte("   \n\t  \n"), 0644)
				if err != nil {
					t.Fatalf("Failed to create whitespace file: %v", err)
				}
				return configPath
			},
			expectError: false,
		},
		{
			name: "very large config file",
			setupFunc: func(t *testing.T) string {
				tempDir := t.TempDir()
				configPath := filepath.Join(tempDir, "large.yaml")
				
				// Generate large config with many tenants
				var configBuilder strings.Builder
				configBuilder.WriteString("tenants:\n")
				for i := 0; i < 100; i++ {
					configBuilder.WriteString("  - name: tenant")
					configBuilder.WriteString(string(rune('0' + i%10)))
					configBuilder.WriteString("\n    path_prefix: /api")
					configBuilder.WriteString(string(rune('0' + i%10)))
					configBuilder.WriteString("/\n    services:\n      - name: backend\n        url: http://localhost:808")
					configBuilder.WriteString(string(rune('0' + i%10)))
					configBuilder.WriteString("\n        health: /health\n")
				}
				
				err := os.WriteFile(configPath, []byte(configBuilder.String()), 0644)
				if err != nil {
					t.Fatalf("Failed to create large file: %v", err)
				}
				return configPath
			},
			expectError: false,
		},
		{
			name: "file with binary content",
			setupFunc: func(t *testing.T) string {
				tempDir := t.TempDir()
				configPath := filepath.Join(tempDir, "binary.yaml")
				binaryData := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE}
				err := os.WriteFile(configPath, binaryData, 0644)
				if err != nil {
					t.Fatalf("Failed to create binary file: %v", err)
				}
				return configPath
			},
			expectError: true,
			errorContains: "failed to parse config",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configPath := tc.setupFunc(t)

			cfg, err := config.LoadConfig(configPath)

			if tc.expectError {
				if err == nil {
					t.Fatal("Expected error, got nil")
				}
				if tc.errorContains != "" && !strings.Contains(err.Error(), tc.errorContains) {
					t.Errorf("Expected error containing %q, got %q", tc.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
				if cfg == nil {
					t.Error("Expected config, got nil")
				}
			}
		})
	}
}

// TestConfigStructureValidation tests configuration structure validation
func TestConfigStructureValidation(t *testing.T) {
	testCases := []struct {
		name       string
		configYAML string
		testFunc   func(t *testing.T, cfg *config.Config)
	}{
		{
			name: "admin base path configuration",
			configYAML: `
tenants:
  - name: test-tenant
    path_prefix: /api/
    services:
      - name: backend
        url: http://localhost:8080
        health: /health
admin_base_path: /admin
`,
			testFunc: func(t *testing.T, cfg *config.Config) {
				if cfg.AdminBasePath != "/admin" {
					t.Errorf("Expected admin base path '/admin', got %q", cfg.AdminBasePath)
				}
			},
		},
		{
			name: "TLS configuration",
			configYAML: `
tenants:
  - name: test-tenant
    path_prefix: /api/
    services:
      - name: backend
        url: http://localhost:8080
        health: /health
tls:
  enabled: true
  cert_file: /etc/ssl/cert.pem
  key_file: /etc/ssl/key.pem
`,
			testFunc: func(t *testing.T, cfg *config.Config) {
				if cfg.TLS == nil {
					t.Fatal("Expected TLS config, got nil")
				}
				if !cfg.TLS.Enabled {
					t.Error("Expected TLS enabled")
				}
				if cfg.TLS.CertFile != "/etc/ssl/cert.pem" {
					t.Errorf("Expected cert file '/etc/ssl/cert.pem', got %q", cfg.TLS.CertFile)
				}
				if cfg.TLS.KeyFile != "/etc/ssl/key.pem" {
					t.Errorf("Expected key file '/etc/ssl/key.pem', got %q", cfg.TLS.KeyFile)
				}
			},
		},
		{
			name: "Lua routing configuration",
			configYAML: `
tenants:
  - name: test-tenant
    path_prefix: /api/
    lua_routes: tenant-script
    services:
      - name: backend
        url: http://localhost:8080
        health: /health
lua_routing:
  enabled: true
  scripts_dir: /etc/lua/scripts
  global_scripts: ["middleware", "auth"]
`,
			testFunc: func(t *testing.T, cfg *config.Config) {
				if cfg.LuaRouting == nil {
					t.Fatal("Expected Lua routing config, got nil")
				}
				if !cfg.LuaRouting.Enabled {
					t.Error("Expected Lua routing enabled")
				}
				if cfg.LuaRouting.ScriptsDir != "/etc/lua/scripts" {
					t.Errorf("Expected scripts dir '/etc/lua/scripts', got %q", cfg.LuaRouting.ScriptsDir)
				}
				expectedGlobals := []string{"middleware", "auth"}
				if len(cfg.LuaRouting.GlobalScripts) != len(expectedGlobals) {
					t.Errorf("Expected %d global scripts, got %d", len(expectedGlobals), len(cfg.LuaRouting.GlobalScripts))
				}
				
				// Check tenant has Lua routes
				if len(cfg.Tenants) == 0 {
					t.Fatal("Expected at least one tenant")
				}
				if cfg.Tenants[0].LuaRoutes != "tenant-script" {
					t.Errorf("Expected tenant Lua routes 'tenant-script', got %q", cfg.Tenants[0].LuaRoutes)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create temporary config file
			tempDir := t.TempDir()
			configPath := filepath.Join(tempDir, "config.yaml")

			err := os.WriteFile(configPath, []byte(tc.configYAML), 0644)
			if err != nil {
				t.Fatalf("Failed to write config file: %v", err)
			}

			// Load configuration
			cfg, err := config.LoadConfig(configPath)
			if err != nil {
				t.Fatalf("Failed to load config: %v", err)
			}

			tc.testFunc(t, cfg)
		})
	}
}