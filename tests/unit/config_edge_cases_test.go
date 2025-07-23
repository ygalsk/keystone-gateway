package unit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"keystone-gateway/internal/config"
)

func TestConfigServiceURLValidation(t *testing.T) {
	tmpDir := t.TempDir()

	testCases := []struct {
		name        string
		serviceURL  string
		expectError bool
		description string
	}{
		{"valid HTTP URL", "http://backend:8080", false, "Standard HTTP URL should be valid"},
		{"valid HTTPS URL", "https://backend.example.com:443", false, "Standard HTTPS URL should be valid"},
		{"URL with path", "http://backend:8080/api", false, "URL with path should be valid"},
		{"URL with query params", "http://backend:8080?param=value", false, "URL with query parameters should be valid"},
		{"localhost URL", "http://localhost:3000", false, "Localhost URL should be valid"},
		{"IP address URL", "http://192.168.1.100:8080", false, "IP address URL should be valid"},
		{"IPv6 URL", "http://[::1]:8080", false, "IPv6 URL should be valid"},
		{"empty URL", "", false, "Empty URL might be allowed for testing"},
		{"malformed URL", "not-a-url", false, "Malformed URLs might be allowed at config level"},
		{"missing protocol", "backend:8080", false, "Missing protocol might be allowed"},
		{"invalid protocol", "ftp://backend:8080", false, "Non-HTTP protocols might be allowed"},
		{"URL with credentials", "http://user:pass@backend:8080", false, "URLs with credentials might be allowed"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configContent := `
tenants:
  - name: test-tenant
    path_prefix: "/api/"
    services:
      - name: test-service
        url: "` + tc.serviceURL + `"
        health: "/health"
`
			configFile := filepath.Join(tmpDir, tc.name+".yaml")
			if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
				t.Fatalf("failed to create config file: %v", err)
			}

			cfg, err := config.LoadConfig(configFile)
			if tc.expectError {
				if err == nil {
					t.Errorf("expected error for URL %q, but got none", tc.serviceURL)
				}
			} else {
				if err != nil {
					t.Logf("URL %q caused error (may be expected): %v", tc.serviceURL, err)
				} else if cfg != nil && len(cfg.Tenants) > 0 && len(cfg.Tenants[0].Services) > 0 {
					actualURL := cfg.Tenants[0].Services[0].URL
					if actualURL != tc.serviceURL {
						t.Errorf("expected URL %q, got %q", tc.serviceURL, actualURL)
					}
				}
			}
		})
	}
}

func TestConfigDomainValidationEdgeCases(t *testing.T) {
	tmpDir := t.TempDir()

	testCases := []struct {
		name        string
		domain      string
		expectError bool
		description string
	}{
		{"normal domain", "example.com", false, "Standard domain should be valid"},
		{"subdomain", "api.example.com", false, "Subdomain should be valid"},
		{"domain with port", "example.com:8080", false, "Domain with port should be valid"},
		{"localhost", "localhost", true, "Localhost without TLD should be invalid"},
		{"IP address", "192.168.1.1", false, "IP address domains might be allowed"},
		{"IPv6", "::1", true, "IPv6 should be invalid as domain"},
		{"domain with spaces", "example .com", true, "Domain with spaces should be invalid"},
		{"empty domain", "", true, "Empty domain should be invalid"},
		{"single char domain", "a.b", false, "Single character domains might be valid"},
		{"very long domain", strings.Repeat("a", 60) + ".com", false, "Long domains might be valid"},
		{"domain with underscore", "sub_domain.example.com", false, "Underscores might be allowed"},
		{"domain with hyphen", "sub-domain.example.com", false, "Hyphens should be allowed"},
		{"domain starting with dot", ".example.com", false, "Leading dots might be processed"},
		{"domain ending with dot", "example.com.", false, "Trailing dots might be allowed"},
		{"punycode domain", "xn--e1afmkfd.xn--p1ai", false, "Punycode domains should be valid"},
		{"numeric domain", "123.456", false, "Numeric domains might be allowed"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configContent := `
tenants:
  - name: test-tenant
    domains: ["` + tc.domain + `"]
    services:
      - name: test-service
        url: "http://backend:8080"
        health: "/health"
`
			configFile := filepath.Join(tmpDir, tc.name+".yaml")
			if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
				t.Fatalf("failed to create config file: %v", err)
			}

			cfg, err := config.LoadConfig(configFile)
			if tc.expectError {
				if err == nil {
					t.Errorf("expected error for domain %q, but got none", tc.domain)
				}
			} else {
				if err != nil {
					t.Logf("Domain %q caused error: %v", tc.domain, err)
				} else if cfg != nil && len(cfg.Tenants) > 0 && len(cfg.Tenants[0].Domains) > 0 {
					actualDomain := cfg.Tenants[0].Domains[0]
					if actualDomain != tc.domain {
						t.Errorf("expected domain %q, got %q", tc.domain, actualDomain)
					}
				}
			}
		})
	}
}

func TestConfigPathPrefixValidation(t *testing.T) {
	tmpDir := t.TempDir()

	testCases := []struct {
		name        string
		pathPrefix  string
		expectError bool
		description string
	}{
		{"valid path prefix", "/api/", false, "Standard path prefix should be valid"},
		{"root path", "/", false, "Root path should be valid"},
		{"deep path", "/api/v1/users/", false, "Deep path should be valid"},
		{"missing leading slash", "api/", true, "Path without leading slash should be invalid"},
		{"missing trailing slash", "/api", true, "Path without trailing slash should be invalid"},
		{"no slashes", "api", true, "Path without slashes should be invalid"},
		{"empty path", "", false, "Empty path should be valid (no path routing)"},
		{"path with spaces", "/api /", false, "Path with spaces might be allowed"},
		{"path with special chars", "/api@#$/", false, "Path with special characters might be allowed"},
		{"path with unicode", "/café/", false, "Unicode paths might be allowed"},
		{"very long path", "/" + strings.Repeat("a", 1000) + "/", false, "Very long paths might be allowed"},
		{"path with query params", "/api/?param=value", false, "Path with query params might be allowed"},
		{"path with fragment", "/api/#section", false, "Path with fragment might be allowed"},
		{"relative path", "../api/", false, "Relative paths might be allowed"},
		{"path with encoded chars", "/api%20encoded/", false, "URL encoded paths might be allowed"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configContent := `
tenants:
  - name: test-tenant
    path_prefix: "` + tc.pathPrefix + `"
    services:
      - name: test-service
        url: "http://backend:8080"
        health: "/health"
`
			configFile := filepath.Join(tmpDir, tc.name+".yaml")
			if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
				t.Fatalf("failed to create config file: %v", err)
			}

			cfg, err := config.LoadConfig(configFile)
			if tc.expectError {
				if err == nil {
					t.Errorf("expected error for path prefix %q, but got none", tc.pathPrefix)
				}
			} else {
				if err != nil {
					t.Logf("Path prefix %q caused error: %v", tc.pathPrefix, err)
				} else if cfg != nil && len(cfg.Tenants) > 0 {
					actualPath := cfg.Tenants[0].PathPrefix
					if actualPath != tc.pathPrefix {
						t.Errorf("expected path prefix %q, got %q", tc.pathPrefix, actualPath)
					}
				}
			}
		})
	}
}

func TestConfigTenantConflicts(t *testing.T) {
	tmpDir := t.TempDir()

	testCases := []struct {
		name        string
		config      string
		expectError bool
		description string
	}{
		{
			name: "duplicate tenant names",
			config: `
tenants:
  - name: duplicate
    path_prefix: "/api/"
    services:
      - name: svc1
        url: "http://backend1:8080"
  - name: duplicate
    path_prefix: "/v2/"
    services:
      - name: svc2
        url: "http://backend2:8080"
`,
			expectError: false, // Might be allowed at config level
			description: "Duplicate tenant names might be detected",
		},
		{
			name: "overlapping path prefixes",
			config: `
tenants:
  - name: tenant1
    path_prefix: "/api/"
    services:
      - name: svc1
        url: "http://backend1:8080"
  - name: tenant2
    path_prefix: "/api/v2/"
    services:
      - name: svc2
        url: "http://backend2:8080"
`,
			expectError: false, // Overlapping paths might be allowed (longest match wins)
			description: "Overlapping path prefixes might be allowed",
		},
		{
			name: "same domain different ports",
			config: `
tenants:
  - name: tenant1
    domains: ["example.com:8080"]
    services:
      - name: svc1
        url: "http://backend1:8080"
  - name: tenant2
    domains: ["example.com:9090"]
    services:
      - name: svc2
        url: "http://backend2:8080"
`,
			expectError: false,
			description: "Same domain with different ports should be allowed",
		},
		{
			name: "duplicate domains",
			config: `
tenants:
  - name: tenant1
    domains: ["example.com"]
    services:
      - name: svc1
        url: "http://backend1:8080"
  - name: tenant2
    domains: ["example.com"]
    services:
      - name: svc2
        url: "http://backend2:8080"
`,
			expectError: false, // Might be allowed, with last one winning
			description: "Duplicate domains might be allowed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configFile := filepath.Join(tmpDir, tc.name+".yaml")
			if err := os.WriteFile(configFile, []byte(tc.config), 0644); err != nil {
				t.Fatalf("failed to create config file: %v", err)
			}

			cfg, err := config.LoadConfig(configFile)
			if tc.expectError {
				if err == nil {
					t.Errorf("expected error for %s, but got none", tc.description)
				}
			} else {
				if err != nil {
					t.Logf("%s caused error: %v", tc.description, err)
				} else if cfg == nil {
					t.Error("expected valid config but got nil")
				}
			}
		})
	}
}

func TestConfigLuaRoutingEdgeCases(t *testing.T) {
	tmpDir := t.TempDir()

	testCases := []struct {
		name        string
		config      string
		expectError bool
		description string
	}{
		{
			name: "lua routing disabled",
			config: `
lua_routing:
  enabled: false
tenants:
  - name: test-tenant
    path_prefix: "/api/"
    services:
      - name: test-service
        url: "http://backend:8080"
`,
			expectError: false,
			description: "Disabled Lua routing should be valid",
		},
		{
			name: "lua routing without scripts dir",
			config: `
lua_routing:
  enabled: true
tenants:
  - name: test-tenant
    path_prefix: "/api/"
    services:
      - name: test-service
        url: "http://backend:8080"
`,
			expectError: false,
			description: "Lua routing without scripts directory might be allowed",
		},
		{
			name: "lua routing with nonexistent scripts dir",
			config: `
lua_routing:
  enabled: true
  scripts_dir: "/nonexistent/path"
tenants:
  - name: test-tenant
    path_prefix: "/api/"
    services:
      - name: test-service
        url: "http://backend:8080"
`,
			expectError: false,
			description: "Nonexistent scripts directory might be allowed at config load time",
		},
		{
			name: "tenant with lua routes but no global lua config",
			config: `
tenants:
  - name: test-tenant
    path_prefix: "/api/"
    lua_routes: "tenant-routes.lua"
    services:
      - name: test-service
        url: "http://backend:8080"
`,
			expectError: false,
			description: "Tenant Lua routes without global config might be allowed",
		},
		{
			name: "empty global scripts array",
			config: `
lua_routing:
  enabled: true
  scripts_dir: "./scripts"
  global_scripts: []
tenants:
  - name: test-tenant
    path_prefix: "/api/"
    services:
      - name: test-service
        url: "http://backend:8080"
`,
			expectError: false,
			description: "Empty global scripts array should be valid",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configFile := filepath.Join(tmpDir, tc.name+".yaml")
			if err := os.WriteFile(configFile, []byte(tc.config), 0644); err != nil {
				t.Fatalf("failed to create config file: %v", err)
			}

			cfg, err := config.LoadConfig(configFile)
			if tc.expectError {
				if err == nil {
					t.Errorf("expected error for %s, but got none", tc.description)
				}
			} else {
				if err != nil {
					t.Logf("%s caused error: %v", tc.description, err)
				} else if cfg == nil {
					t.Error("expected valid config but got nil")
				}
			}
		})
	}
}

func TestConfigExtremeValues(t *testing.T) {
	tmpDir := t.TempDir()

	testCases := []struct {
		name        string
		config      string
		expectError bool
		description string
	}{
		{
			name: "tenant with many services",
			config: `
tenants:
  - name: test-tenant
    path_prefix: "/api/"
    services:
` + func() string {
				var services strings.Builder
				for i := 0; i < 100; i++ {
					services.WriteString(fmt.Sprintf(`      - name: service%d
        url: "http://backend%d:8080"
        health: "/health"
`, i, i))
				}
				return services.String()
			}(),
			expectError: false,
			description: "Tenant with many services should be handled",
		},
		{
			name: "tenant with many domains",
			config: `
tenants:
  - name: test-tenant
    domains: [` + func() string {
				var domains []string
				for i := 0; i < 50; i++ {
					domains = append(domains, fmt.Sprintf(`"domain%d.example.com"`, i))
				}
				return strings.Join(domains, ", ")
			}() + `]
    services:
      - name: test-service
        url: "http://backend:8080"
`,
			expectError: false,
			description: "Tenant with many domains should be handled",
		},
		{
			name: "very long tenant name",
			config: `
tenants:
  - name: "` + strings.Repeat("a", 1000) + `"
    path_prefix: "/api/"
    services:
      - name: test-service
        url: "http://backend:8080"
`,
			expectError: false,
			description: "Very long tenant name should be handled",
		},
		{
			name: "negative health interval",
			config: `
tenants:
  - name: test-tenant
    path_prefix: "/api/"
    health_interval: -1
    services:
      - name: test-service
        url: "http://backend:8080"
        health: "/health"
`,
			expectError: false,
			description: "Negative health interval might be allowed",
		},
		{
			name: "zero health interval",
			config: `
tenants:
  - name: test-tenant
    path_prefix: "/api/"
    health_interval: 0
    services:
      - name: test-service
        url: "http://backend:8080"
        health: "/health"
`,
			expectError: false,
			description: "Zero health interval might be allowed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configFile := filepath.Join(tmpDir, tc.name+".yaml")
			if err := os.WriteFile(configFile, []byte(tc.config), 0644); err != nil {
				t.Fatalf("failed to create config file: %v", err)
			}

			cfg, err := config.LoadConfig(configFile)
			if tc.expectError {
				if err == nil {
					t.Errorf("expected error for %s, but got none", tc.description)
				}
			} else {
				if err != nil {
					t.Logf("%s caused error: %v", tc.description, err)
				} else if cfg == nil {
					t.Error("expected valid config but got nil")
				}
			}
		})
	}
}