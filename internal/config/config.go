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

// TLSConfig represents TLS configuration for the gateway
type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

// CompressionConfig represents HTTP response compression configuration
type CompressionConfig struct {
	Enabled      bool     `yaml:"enabled"`
	Level        int      `yaml:"level,omitempty"`         // Compression level (1-9, default: 5)
	ContentTypes []string `yaml:"content_types,omitempty"` // MIME types to compress
}

// ServerConfig represents server configuration
type ServerConfig struct {
	Port string `yaml:"port,omitempty"` // Server port (default: 8080)
}

// GetCompressionConfig returns compression configuration with defaults
func (c *Config) GetCompressionConfig() CompressionConfig {
	if c.Compression == nil {
		// Return default compression settings when not configured
		return CompressionConfig{
			Enabled: true, // Enable compression by default
			Level:   5,    // Balanced compression level
			ContentTypes: []string{
				"text/html",
				"text/css",
				"text/javascript",
				"application/json",
				"application/xml",
				"text/plain",
			},
		}
	}

	config := *c.Compression

	// Apply defaults for missing values
	if config.Level == 0 {
		config.Level = 5 // Default to balanced compression
	}
	if len(config.ContentTypes) == 0 {
		config.ContentTypes = []string{
			"text/html",
			"text/css",
			"text/javascript",
			"application/json",
			"application/xml",
			"text/plain",
		}
	}

	return config
}

// GetPort returns the configured port or default port (8080)
func (c *Config) GetPort() string {
	if c.Server != nil && c.Server.Port != "" {
		return c.Server.Port
	}
	return "8080"
}

// Config represents the main configuration structure for the gateway,
// containing tenant definitions and admin settings.
type Config struct {
	Tenants       []Tenant           `yaml:"tenants"`
	AdminBasePath string             `yaml:"admin_base_path,omitempty"`
	Server        *ServerConfig      `yaml:"server,omitempty"`
	LuaRouting    *LuaRoutingConfig  `yaml:"lua_routing,omitempty"` // Embedded Lua routing only
	TLS           *TLSConfig         `yaml:"tls,omitempty"`
	Compression   *CompressionConfig `yaml:"compression,omitempty"`
}

// Tenant represents a routing configuration for a specific application or service,
// supporting host-based, path-based, or hybrid routing strategies.
type Tenant struct {
	Name       string    `yaml:"name"`
	PathPrefix string    `yaml:"path_prefix,omitempty"`
	Domains    []string  `yaml:"domains,omitempty"`
	Interval   int       `yaml:"health_interval"`
	LuaRoutes  string    `yaml:"lua_routes,omitempty"` // Script for route definition
	Services   []Service `yaml:"services"`
}

// Service represents a backend service endpoint with health check configuration.
type Service struct {
	Name   string `yaml:"name"`
	URL    string `yaml:"url"`
	Health string `yaml:"health"`
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

	// Require at least one service and validate service URLs
	if len(t.Services) == 0 {
		return fmt.Errorf("tenant must have at least one service")
	}
	for _, s := range t.Services {
		u, err := url.Parse(s.URL)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("service %q has invalid url: %q", s.Name, s.URL)
		}
		if s.Health != "" && !strings.HasPrefix(s.Health, "/") {
			return fmt.Errorf("service %q health path must start with '/': %q", s.Name, s.Health)
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
