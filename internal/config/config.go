// Package config provides configuration management for Keystone Gateway.
// It handles loading, parsing, and validating YAML configuration files.
package config

import (
	"fmt"
	"net"
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

// Config represents the main configuration structure for the gateway,
// containing tenant definitions and admin settings.
type Config struct {
	Tenants       []Tenant          `yaml:"tenants"`
	AdminBasePath string            `yaml:"admin_base_path,omitempty"`
	LuaRouting    *LuaRoutingConfig `yaml:"lua_routing,omitempty"` // Embedded Lua routing only
	TLS           *TLSConfig        `yaml:"tls,omitempty"`
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
		if !strings.HasPrefix(t.PathPrefix, "/") || !strings.HasSuffix(t.PathPrefix, "/") {
			return fmt.Errorf("path_prefix must start and end with '/'")
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
