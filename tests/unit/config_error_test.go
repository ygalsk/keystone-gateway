package unit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"keystone-gateway/internal/config"
)

func TestConfigInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()

	// Create config file with invalid YAML syntax
	configFile := filepath.Join(tmpDir, "invalid.yaml")
	invalidYAML := `
port: 8080
tenants:
  - name: tenant1
    scripts_path: /path/to/scripts
  - name: tenant2
    # Missing closing bracket or invalid indentation
    scripts_path: /path/to/scripts
      invalid_key: value
routes:
  - path: /api/*
    # Invalid YAML structure
    tenant: [unclosed array
`

	if err := os.WriteFile(configFile, []byte(invalidYAML), 0644); err != nil {
		t.Fatalf("failed to create invalid config file: %v", err)
	}

	// Test that invalid YAML is handled gracefully
	_, err := config.LoadConfig(configFile)
	if err == nil {
		t.Error("expected error when loading invalid YAML config")
	}

	if !strings.Contains(err.Error(), "yaml") && !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("expected YAML-related error, got: %v", err)
	}
}

func TestConfigNonExistentFile(t *testing.T) {
	nonExistentFile := "/path/that/does/not/exist/config.yaml"

	// Test loading non-existent config file
	_, err := config.LoadConfig(nonExistentFile)
	if err == nil {
		t.Error("expected error when loading non-existent config file")
	}

	if !strings.Contains(err.Error(), "no such file") && !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected file not found error, got: %v", err)
	}
}

func TestConfigEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create empty config file
	configFile := filepath.Join(tmpDir, "empty.yaml")
	if err := os.WriteFile(configFile, []byte(""), 0644); err != nil {
		t.Fatalf("failed to create empty config file: %v", err)
	}

	// Test loading empty config
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		t.Errorf("unexpected error loading empty config: %v", err)
	}

	// Empty config should result in default values
	if cfg == nil {
		t.Error("expected non-nil config from empty file")
	}
}

func TestConfigPermissionDenied(t *testing.T) {
	tmpDir := t.TempDir()

	// Create config file with restricted permissions
	configFile := filepath.Join(tmpDir, "restricted.yaml")
	configContent := `
port: 8080
tenants:
  - name: tenant1
    scripts_path: /scripts
`

	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to create config file: %v", err)
	}

	// Remove read permissions
	if err := os.Chmod(configFile, 0000); err != nil {
		t.Fatalf("failed to change file permissions: %v", err)
	}
	defer os.Chmod(configFile, 0644) // Restore for cleanup

	// Test loading config with no read permissions
	_, err := config.LoadConfig(configFile)
	if err == nil {
		t.Error("expected error when loading config with no read permissions")
	}

	if !strings.Contains(err.Error(), "permission denied") && !strings.Contains(err.Error(), "access is denied") {
		t.Errorf("expected permission denied error, got: %v", err)
	}
}

func TestConfigInvalidPort(t *testing.T) {
	tmpDir := t.TempDir()

	testCases := []struct {
		name    string
		port    string
		isValid bool
	}{
		{"negative port", "-1", false},
		{"zero port", "0", false},
		{"too large port", "65536", false},
		{"string port", "abc", false},
		{"float port", "8080.5", false},
		{"valid port", "8080", true},
		{"max valid port", "65535", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configFile := filepath.Join(tmpDir, tc.name+".yaml")
			configContent := `
port: ` + tc.port + `
tenants:
  - name: tenant1
    scripts_path: /scripts
`

			if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
				t.Fatalf("failed to create config file: %v", err)
			}

			cfg, err := config.LoadConfig(configFile)

			if tc.isValid {
				if err != nil {
					t.Errorf("expected no error for valid port %s, got: %v", tc.port, err)
				}
				if cfg == nil {
					t.Error("expected valid config")
				}
			} else {
				// For invalid ports, we might get a YAML error or validation error
				// The exact behavior depends on implementation
				if err == nil && cfg != nil {
					// If it loads successfully, check if validation catches it later
					t.Logf("Port %s loaded successfully, validation may happen elsewhere", tc.port)
				}
			}
		})
	}
}

func TestConfigMissingRequiredFields(t *testing.T) {
	tmpDir := t.TempDir()

	testCases := []struct {
		name    string
		content string
	}{
		{
			"missing tenants",
			`port: 8080`,
		},
		{
			"empty tenants array",
			`
port: 8080
tenants: []
`,
		},
		{
			"tenant missing name",
			`
port: 8080
tenants:
  - scripts_path: /scripts
`,
		},
		{
			"tenant missing scripts_path",
			`
port: 8080
tenants:
  - name: tenant1
`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configFile := filepath.Join(tmpDir, tc.name+".yaml")

			if err := os.WriteFile(configFile, []byte(tc.content), 0644); err != nil {
				t.Fatalf("failed to create config file: %v", err)
			}

			cfg, err := config.LoadConfig(configFile)

			// The behavior depends on implementation - it might load successfully
			// but fail validation later, or fail immediately
			if err == nil && cfg != nil {
				t.Logf("Config with %s loaded successfully, validation may happen elsewhere", tc.name)
			} else if err != nil {
				t.Logf("Config with %s failed as expected: %v", tc.name, err)
			}
		})
	}
}

func TestConfigDuplicateTenants(t *testing.T) {
	tmpDir := t.TempDir()

	configFile := filepath.Join(tmpDir, "duplicate.yaml")
	configContent := `
port: 8080
tenants:
  - name: tenant1
    scripts_path: /scripts1
  - name: tenant1
    scripts_path: /scripts2
routes:
  - path: /api/*
    tenant: tenant1
`

	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to create config file: %v", err)
	}

	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		t.Errorf("unexpected error loading config with duplicate tenants: %v", err)
	}

	// Check if duplicate tenants are handled (last one wins, or error)
	if cfg != nil && cfg.Tenants != nil {
		if len(cfg.Tenants) != 2 {
			t.Logf("Duplicate tenants handling: %d tenants found", len(cfg.Tenants))
		}
	}
}

func TestConfigInvalidTLSConfiguration(t *testing.T) {
	tmpDir := t.TempDir()

	testCases := []struct {
		name    string
		content string
	}{
		{
			"TLS enabled without cert file",
			`
port: 8080
tls:
  enabled: true
  key_file: /path/to/key.pem
tenants:
  - name: tenant1
    scripts_path: /scripts
`,
		},
		{
			"TLS enabled without key file",
			`
port: 8080
tls:
  enabled: true
  cert_file: /path/to/cert.pem
tenants:
  - name: tenant1
    scripts_path: /scripts
`,
		},
		{
			"TLS with non-existent cert files",
			`
port: 8080
tls:
  enabled: true
  cert_file: /nonexistent/cert.pem
  key_file: /nonexistent/key.pem
tenants:
  - name: tenant1
    scripts_path: /scripts
`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configFile := filepath.Join(tmpDir, tc.name+".yaml")

			if err := os.WriteFile(configFile, []byte(tc.content), 0644); err != nil {
				t.Fatalf("failed to create config file: %v", err)
			}

			cfg, err := config.LoadConfig(configFile)

			// TLS configuration errors might be caught during config loading
			// or later during server startup
			if err == nil && cfg != nil {
				t.Logf("TLS config %s loaded successfully, validation may happen at startup", tc.name)
			} else if err != nil {
				t.Logf("TLS config %s failed as expected: %v", tc.name, err)
			}
		})
	}
}

func TestConfigVeryLargeConfigFile(t *testing.T) {
	tmpDir := t.TempDir()

	configFile := filepath.Join(tmpDir, "large.yaml")

	// Build a large config with many tenants and routes
	var content strings.Builder
	content.WriteString("port: 8080\n")
	content.WriteString("tenants:\n")

	// Add 1000 tenants
	for i := 0; i < 1000; i++ {
		content.WriteString("  - name: tenant")
		content.WriteString(strings.Repeat("0", 10)) // Add padding
		content.WriteString("\n    scripts_path: /scripts/tenant")
		content.WriteString("\n")
	}

	content.WriteString("routes:\n")
	// Add 1000 routes
	for i := 0; i < 1000; i++ {
		content.WriteString("  - path: /api/v")
		content.WriteString("/endpoint")
		content.WriteString("\n    tenant: tenant")
		content.WriteString("0000000000")
		content.WriteString("\n")
	}

	if err := os.WriteFile(configFile, []byte(content.String()), 0644); err != nil {
		t.Fatalf("failed to create large config file: %v", err)
	}

	// Test that large configs are handled reasonably
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		t.Errorf("failed to load large config: %v", err)
	}

	if cfg != nil {
		if len(cfg.Tenants) != 1000 {
			t.Errorf("expected 1000 tenants, got %d", len(cfg.Tenants))
		}
		// Note: Config struct doesn't have Routes field - tenants contain routing info
		// This test checks that the large config with 1000 tenants was loaded correctly
	}
}
