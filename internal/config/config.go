// Package config provides configuration management for Keystone Gateway.
// It handles loading, parsing, and validating YAML configuration files.
package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// LuaEngineConfig represents lua-stone service configuration
type LuaEngineConfig struct {
	Enabled bool   `yaml:"enabled"`
	URL     string `yaml:"url,omitempty"`
	Timeout string `yaml:"timeout,omitempty"`
}

// LuaRoutingConfig represents embedded Lua routing configuration
type LuaRoutingConfig struct {
	Enabled    bool   `yaml:"enabled"`
	ScriptsDir string `yaml:"scripts_dir,omitempty"`
	Timeout    string `yaml:"timeout,omitempty"`
}

// Config represents the main configuration structure for the gateway,
// containing tenant definitions and admin settings.
type Config struct {
	Tenants       []Tenant          `yaml:"tenants"`
	AdminBasePath string            `yaml:"admin_base_path,omitempty"`
	LuaEngine     *LuaEngineConfig  `yaml:"lua_engine,omitempty"`
	LuaRouting    *LuaRoutingConfig `yaml:"lua_routing,omitempty"` // New: Embedded Lua routing
}

// Tenant represents a routing configuration for a specific application or service,
// supporting host-based, path-based, or hybrid routing strategies.
type Tenant struct {
	Name       string    `yaml:"name"`
	PathPrefix string    `yaml:"path_prefix,omitempty"`
	Domains    []string  `yaml:"domains,omitempty"`
	Interval   int       `yaml:"health_interval"`
	LuaScript  string    `yaml:"lua_script,omitempty"`
	LuaRoutes  string    `yaml:"lua_routes,omitempty"` // New: Script for route definition
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
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	for _, tenant := range cfg.Tenants {
		if err := validateTenant(tenant); err != nil {
			return nil, fmt.Errorf("invalid tenant %s: %w", tenant.Name, err)
		}
	}

	return &cfg, nil
}

// validateTenant validates a tenant configuration for correctness.
func validateTenant(t Tenant) error {
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
	return domain != "" && !strings.Contains(domain, " ") && strings.Contains(domain, ".")
}
