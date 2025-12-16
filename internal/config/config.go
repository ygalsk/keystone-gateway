// Package config provides configuration management for Keystone Gateway.
// It handles loading, parsing, and validating YAML configuration files.
package config

import (
	"fmt"
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
	ModulePaths   []string `yaml:"module_paths,omitempty"`   // Lua module paths for LuaRocks
	ModuleCPaths  []string `yaml:"module_cpaths,omitempty"`  // C module paths for LuaRocks
	StatePoolSize int      `yaml:"state_pool_size,omitempty"` // Lua VM pool size (default: 10)
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

// MiddlewareConfig controls which middleware components are enabled.
type MiddlewareConfig struct {
	RequestID bool `yaml:"request_id"` // Generate request IDs (default: true)
	RealIP    bool `yaml:"real_ip"`    // Parse real IP from headers (default: true)
	Logging   bool `yaml:"logging"`    // Log requests (default: true)
	Recovery  bool `yaml:"recovery"`   // Recover from panics (default: true)
	Timeout   int  `yaml:"timeout"`    // Request timeout in seconds (default: 10)
	Throttle  int  `yaml:"throttle"`   // Max concurrent requests (default: 100)
}

// Config represents the main configuration structure for the gateway,
// containing tenant definitions and configuration sections.
type Config struct {
	Tenants       []Tenant            `yaml:"tenants"`
	LuaRouting    LuaRoutingConfig    `yaml:"lua_routing"`    // Embedded Lua routing only
	Middleware    MiddlewareConfig    `yaml:"middleware"`     // Middleware configuration
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
		Middleware: MiddlewareConfig{
			RequestID: true,
			RealIP:    true,
			Logging:   true,
			Recovery:  true,
			Timeout:   10,
			Throttle:  100,
		},
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
	if raw.Middleware.Timeout == 0 {
		raw.Middleware.Timeout = 10
	}
	if raw.Middleware.Throttle == 0 {
		raw.Middleware.Throttle = 100
	}
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

// Route represents a single HTTP route (Go-owned routing)
type Route struct {
	Method     string   `yaml:"method"`               // "GET", "POST", etc.
	Pattern    string   `yaml:"pattern"`              // "/users/{id}"
	Handler    string   `yaml:"handler,omitempty"`    // Lua function name
	Backend    string   `yaml:"backend,omitempty"`    // Backend service name (proxy)
	Middleware []string `yaml:"middleware,omitempty"` // Lua middleware function names
}

// RouteGroup represents Chi route group with nested routes
type RouteGroup struct {
	Pattern    string   `yaml:"pattern"`              // "/articles"
	Middleware []string `yaml:"middleware,omitempty"` // Group-level middleware
	Routes     []Route  `yaml:"routes"`               // Nested routes
}

// ErrorHandlers represents custom error handlers
type ErrorHandlers struct {
	NotFound         string `yaml:"not_found,omitempty"`           // 404 handler (Lua function)
	MethodNotAllowed string `yaml:"method_not_allowed,omitempty"` // 405 handler (Lua function)
}

// Tenant represents a routing configuration for a specific application or service,
// using path-based routing. For domain-based routing, use an external reverse proxy
// (Nginx, HAProxy) or ingress controller to route different domains to different path prefixes.
type Tenant struct {
	Name          string         `yaml:"name"`
	PathPrefix    string         `yaml:"path_prefix,omitempty"`
	Routes        []Route        `yaml:"routes,omitempty"`         // Go-owned explicit routes
	RouteGroups   []RouteGroup   `yaml:"route_groups,omitempty"`  // Chi route groups
	ErrorHandlers ErrorHandlers  `yaml:"error_handlers,omitempty"`
	Services      []Service      `yaml:"services"`

	// DEPRECATED: Remove in v7.0.0 - use Routes instead
	LuaRoutes []string `yaml:"lua_routes,omitempty"`
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
// PathPrefix is optional - if not specified, tenant will use catch-all route.
func ValidateTenant(t Tenant) error {
	if t.PathPrefix != "" {
		if !strings.HasPrefix(t.PathPrefix, "/") {
			return fmt.Errorf("path_prefix must start with '/'")
		}
	}

	// Require at least one route configuration
	hasRoutes := len(t.Routes) > 0 || len(t.RouteGroups) > 0 || len(t.LuaRoutes) > 0 || len(t.Services) > 0
	if !hasRoutes {
		return fmt.Errorf("tenant must have at least one route, route_group, lua_routes, or service configured")
	}

	// Validate services
	for _, s := range t.Services {
		u, err := url.Parse(s.URL)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("service %q has invalid url: %q", s.Name, s.URL)
		}
	}

	// Validate routes
	for i, route := range t.Routes {
		if route.Method == "" {
			return fmt.Errorf("route %d: method is required", i)
		}
		if route.Pattern == "" {
			return fmt.Errorf("route %d: pattern is required", i)
		}
		if route.Handler == "" && route.Backend == "" {
			return fmt.Errorf("route %d: either handler or backend is required", i)
		}
		if route.Handler != "" && route.Backend != "" {
			return fmt.Errorf("route %d: cannot specify both handler and backend", i)
		}
	}

	// Validate route groups
	for i, group := range t.RouteGroups {
		if group.Pattern == "" {
			return fmt.Errorf("route_group %d: pattern is required", i)
		}
		if len(group.Routes) == 0 {
			return fmt.Errorf("route_group %d: must have at least one route", i)
		}
	}

	return nil
}
