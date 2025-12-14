// Package config provides configuration management for Keystone Gateway.
// It handles loading, parsing, and validating YAML configuration files.
package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// LuaRoutingConfig represents embedded Lua routing configuration
type LuaRoutingConfig struct {
	Enabled       bool     `yaml:"enabled"`
	ScriptsDir    string   `yaml:"scripts_dir,omitempty"`
	GlobalScripts []string `yaml:"global_scripts,omitempty"`
}

// CompressionConfig represents HTTP response compression configuration
type CompressionConfig struct {
	Enabled      bool     `yaml:"enabled"`
	Level        int      `yaml:"level,omitempty"`         // Compression level (1-9, default: 5)
	ContentTypes []string `yaml:"content_types,omitempty"` // MIME types to compress
}


// RequestLimitsConfig represents request size limits
type RequestLimitsConfig struct {
	MaxBodySize int64 `yaml:"max_body_size,omitempty"` // Max request body size in bytes (default: 10MB)
}

// ServerConfig represents server configuration
// Note: Port is removed as CLI flag takes precedence
type ServerConfig struct {
}

// Config represents the main configuration structure for the gateway,
// containing tenant definitions and configuration sections.
type Config struct {
	Tenants       []Tenant            `yaml:"tenants"`
	Server        ServerConfig        `yaml:"server,omitempty"`
	LuaRouting    LuaRoutingConfig    `yaml:"lua_routing"` // Embedded Lua routing only
	Compression   CompressionConfig   `yaml:"compression"`
	RequestLimits RequestLimitsConfig `yaml:"request_limits"`
}

// UnmarshalYAML implements custom unmarshaling with automatic defaults.
// This ensures defaults are always applied and it's impossible to create a Config without them.
func (c *Config) UnmarshalYAML(value *yaml.Node) error {
	// Use a type alias to avoid recursion
	type rawConfig Config
	raw := rawConfig{
		// Set defaults before unmarshaling
		Compression: CompressionConfig{
			Level: 5,
			ContentTypes: []string{
				"text/html",
				"text/css",
				"text/javascript",
				"application/json",
				"application/xml",
				"text/plain",
			},
		},
		RequestLimits: RequestLimitsConfig{
			MaxBodySize: 10 << 20, // 10MB
		},
	}

	if err := value.Decode(&raw); err != nil {
		return err
	}

	// Apply defaults for zero values
	if raw.Compression.Level == 0 {
		raw.Compression.Level = 5
	}
	if len(raw.Compression.ContentTypes) == 0 {
		raw.Compression.ContentTypes = []string{
			"text/html",
			"text/css",
			"text/javascript",
			"application/json",
			"application/xml",
			"text/plain",
		}
	}
	if raw.RequestLimits.MaxBodySize <= 0 {
		raw.RequestLimits.MaxBodySize = 10 << 20
	}

	*c = Config(raw)
	return nil
}

// Tenant represents a routing configuration for a specific application or service,
// supporting host-based, path-based, or hybrid routing strategies.
type Tenant struct {
	Name       string    `yaml:"name"`
	PathPrefix string    `yaml:"path_prefix,omitempty"`
	Domains    []string  `yaml:"domains,omitempty"`
	LuaRoutes  []string  `yaml:"lua_routes,omitempty"` // Scripts for route definition
	Services   []Service `yaml:"services"`
}

// Service represents a backend service endpoint.
type Service struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

// LoadConfig reads and parses a YAML configuration file, returning a validated Config instance.
// Returns an error if the file cannot be read, parsed, or contains invalid tenant configurations.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config

	// Handle empty or whitespace-only files gracefully
	if len(strings.TrimSpace(string(data))) == 0 {
		return &cfg, nil // Return empty config for whitespace-only files
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Validate tenants (defaults already applied via UnmarshalYAML)
	for _, tenant := range cfg.Tenants {
		if err := ValidateTenant(tenant); err != nil {
			return nil, fmt.Errorf("invalid tenant %s: %w", tenant.Name, err)
		}
	}

	return &cfg, nil
}

// ValidateTenant validates a tenant configuration for correctness.
func ValidateTenant(t Tenant) error {
	if len(t.Domains) == 0 && t.PathPrefix == "" {
		return fmt.Errorf("must specify either domains or path_prefix")
	}

	for _, domain := range t.Domains {
		if !isValidDomain(domain) {
			return fmt.Errorf("invalid domain: %s", domain)
		}
	}

	if t.PathPrefix != "" {
		if !strings.HasPrefix(t.PathPrefix, "/") {
			return fmt.Errorf("path_prefix must start with '/'")
		}
		// Temporarily removed trailing slash requirement to test Chi mounting
		// if !strings.HasSuffix(t.PathPrefix, "/") {
		//	return fmt.Errorf("path_prefix must end with '/'")
		// }
	}

	// Require at least one service ONLY if no Lua routing is configured
	if len(t.Services) == 0 && len(t.LuaRoutes) == 0 {
		return fmt.Errorf("tenant must have at least one service or lua_routes configured")
	}
	for _, s := range t.Services {
		u, err := url.Parse(s.URL)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("service %q has invalid url: %q", s.Name, s.URL)
		}
	}

	return nil
}

// isValidDomain performs basic domain name validation.
func isValidDomain(domain string) bool {
	if domain == "" || strings.Contains(domain, " ") {
		return false
	}

	// Reject IP addresses (both IPv4 and IPv6)
	if net.ParseIP(domain) != nil {
		return false
	}

	// Basic domain validation: must contain a dot and have valid format
	return strings.Contains(domain, ".")
}
