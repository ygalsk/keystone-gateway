// Package config provides configuration structures and loading for Keystone Gateway.
// This file contains the minimal configuration model focused on upstream management.
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
	"keystone-gateway/internal/types"
)

// Config represents the minimal configuration for Keystone Gateway.
// Focused on upstream management with extensibility for future features.
type Config struct {
	// Server configuration
	Server ServerConfig `yaml:"server" json:"server"`
	// Upstream configuration
	Upstreams UpstreamsConfig `yaml:"upstreams" json:"upstreams"`
}

// ServerConfig defines basic server settings.
type ServerConfig struct {
	// Addr is the address to bind the server to
	Addr string `yaml:"addr" json:"addr"`
	// ReadHeaderTimeout for HTTP requests
	ReadHeaderTimeout time.Duration `yaml:"read_header_timeout" json:"read_header_timeout"`
	// IdleTimeout for HTTP connections
	IdleTimeout time.Duration `yaml:"idle_timeout" json:"idle_timeout"`
	// TLS configuration (optional)
	TLS *TLSConfig `yaml:"tls,omitempty" json:"tls,omitempty"`
}

// TLSConfig defines TLS/HTTPS settings.
type TLSConfig struct {
	// Enabled determines if TLS should be used
	Enabled bool `yaml:"enabled" json:"enabled"`
	// CertFile path to TLS certificate file
	CertFile string `yaml:"cert_file" json:"cert_file"`
	// KeyFile path to TLS private key file
	KeyFile string `yaml:"key_file" json:"key_file"`
}

// UpstreamsConfig defines upstream server configuration.
type UpstreamsConfig struct {
	// Targets is the list of upstream servers
	Targets []UpstreamTarget `yaml:"targets" json:"targets"`
	// LoadBalancing defines load balancing strategy
	LoadBalancing LoadBalancingConfig `yaml:"load_balancing" json:"load_balancing"`
	// HealthCheck defines health check settings
	HealthCheck types.HealthConfig `yaml:"health_check" json:"health_check"`
}

// UpstreamTarget represents a single upstream server.
type UpstreamTarget struct {
	// Name is a unique identifier for this upstream
	Name string `yaml:"name" json:"name"`
	// URL is the base URL of the upstream server
	URL string `yaml:"url" json:"url"`
	// Weight for weighted load balancing (optional, defaults to 1)
	Weight int32 `yaml:"weight,omitempty" json:"weight,omitempty"`
	// Enabled determines if this upstream should receive traffic
	Enabled bool `yaml:"enabled" json:"enabled"`
}

// LoadBalancingConfig defines load balancing behavior.
type LoadBalancingConfig struct {
	// Strategy defines the load balancing algorithm
	Strategy string `yaml:"strategy" json:"strategy"`
}

// Default returns a minimal default configuration.
func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Addr:              ":8080",
			ReadHeaderTimeout: 5 * time.Second,
			IdleTimeout:       120 * time.Second,
		},
		Upstreams: UpstreamsConfig{
			Targets: []UpstreamTarget{
				{
					Name:    "example-upstream",
					URL:     "http://localhost:8081",
					Weight:  1,
					Enabled: true,
				},
			},
			LoadBalancing: LoadBalancingConfig{
				Strategy: "least_connections",
			},
			HealthCheck: types.DefaultHealthConfig(),
		},
	}
}

// LoadFromFile loads configuration from a YAML file.
func LoadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config YAML: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

// LoadFromEnvOrFile loads configuration from environment variable or file.
// Checks KEYSTONE_CONFIG_PATH environment variable first, then falls back to default path.
func LoadFromEnvOrFile(defaultPath string) (*Config, error) {
	configPath := os.Getenv("KEYSTONE_CONFIG_PATH")
	if configPath == "" {
		configPath = defaultPath
	}

	// If config file doesn't exist, return default config
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return Default(), nil
	}

	return LoadFromFile(configPath)
}

// Validate validates the configuration for correctness.
func (c *Config) Validate() error {
	if c.Server.Addr == "" {
		return fmt.Errorf("server.addr cannot be empty")
	}

	if c.Server.ReadHeaderTimeout <= 0 {
		return fmt.Errorf("server.read_header_timeout must be positive")
	}

	if c.Server.IdleTimeout <= 0 {
		return fmt.Errorf("server.idle_timeout must be positive")
	}

	if len(c.Upstreams.Targets) == 0 {
		return fmt.Errorf("at least one upstream target must be configured")
	}

	// Validate upstream targets
	names := make(map[string]bool)
	enabledCount := 0
	for i, target := range c.Upstreams.Targets {
		if target.Name == "" {
			return fmt.Errorf("upstream target %d: name cannot be empty", i)
		}
		if names[target.Name] {
			return fmt.Errorf("upstream target %d: duplicate name '%s'", i, target.Name)
		}
		names[target.Name] = true

		if target.URL == "" {
			return fmt.Errorf("upstream target '%s': url cannot be empty", target.Name)
		}

		if target.Weight <= 0 {
			return fmt.Errorf("upstream target '%s': weight must be positive", target.Name)
		}

		if target.Enabled {
			enabledCount++
		}
	}

	if enabledCount == 0 {
		return fmt.Errorf("at least one upstream target must be enabled")
	}

	// Validate load balancing strategy
	switch c.Upstreams.LoadBalancing.Strategy {
	case "least_connections", "round_robin", "weighted_round_robin":
		// Valid strategies
	case "":
		c.Upstreams.LoadBalancing.Strategy = "least_connections" // Default
	default:
		return fmt.Errorf("unsupported load balancing strategy: %s", c.Upstreams.LoadBalancing.Strategy)
	}

	// Validate health check configuration
	if err := c.Upstreams.HealthCheck.Validate(); err != nil {
		return fmt.Errorf("health check config: %w", err)
	}

	// Validate TLS configuration if enabled
	if c.Server.TLS != nil && c.Server.TLS.Enabled {
		if c.Server.TLS.CertFile == "" {
			return fmt.Errorf("tls.cert_file cannot be empty when TLS is enabled")
		}
		if c.Server.TLS.KeyFile == "" {
			return fmt.Errorf("tls.key_file cannot be empty when TLS is enabled")
		}
		
		// Check if cert and key files exist
		if _, err := os.Stat(c.Server.TLS.CertFile); os.IsNotExist(err) {
			return fmt.Errorf("tls.cert_file does not exist: %s", c.Server.TLS.CertFile)
		}
		if _, err := os.Stat(c.Server.TLS.KeyFile); os.IsNotExist(err) {
			return fmt.Errorf("tls.key_file does not exist: %s", c.Server.TLS.KeyFile)
		}
	}

	return nil
}

// SaveToFile saves the configuration to a YAML file.
func (c *Config) SaveToFile(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config to YAML: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetEnabledUpstreams returns only the enabled upstream targets.
func (c *Config) GetEnabledUpstreams() []UpstreamTarget {
	var enabled []UpstreamTarget
	for _, target := range c.Upstreams.Targets {
		if target.Enabled {
			enabled = append(enabled, target)
		}
	}
	return enabled
}