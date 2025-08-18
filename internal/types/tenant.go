// Package types defines core data types for the Keystone Gateway.
// This file contains tenant configuration types and tenant resolution functionality.
package types

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Tenant represents a configured tenant in the gateway.
type Tenant struct {
	// ID uniquely identifies this tenant.
	ID string `yaml:"id" json:"id"`
	// Name is a human-readable name for this tenant.
	Name string `yaml:"name" json:"name"`
	// Domains lists all domains that should route to this tenant.
	Domains []string `yaml:"domains" json:"domains"`
	// BasePath is the URL path prefix for this tenant (optional).
	BasePath string `yaml:"base_path" json:"base_path"`
	// Upstreams defines the backend services for this tenant.
	Upstreams []Upstream `yaml:"upstreams" json:"upstreams"`
	// LuaScript is the optional Lua script file for dynamic routing.
	LuaScript string `yaml:"lua_script" json:"lua_script"`
	// DynamicRoutingEnabled allows Lua scripts to modify routes at runtime.
	DynamicRoutingEnabled bool `yaml:"dynamic_routing_enabled" json:"dynamic_routing_enabled"`
	// LuaLimits defines resource limits for Lua script execution.
	LuaLimits LuaLimits `yaml:"lua_limits" json:"lua_limits"`
	// HealthConfig overrides global health check settings for this tenant.
	HealthConfig *HealthConfig `yaml:"health_config,omitempty" json:"health_config,omitempty"`
}

// Upstream represents a backend service endpoint.
type Upstream struct {
	// ID uniquely identifies this upstream within the tenant.
	ID string `yaml:"id" json:"id"`
	// Name is a human-readable name for this upstream.
	Name string `yaml:"name" json:"name"`
	// URL is the base URL for this upstream service.
	URL string `yaml:"url" json:"url"`
	// Weight determines the relative load balancing weight (default: 100).
	Weight int `yaml:"weight" json:"weight"`
	// HealthConfig overrides tenant health check settings for this upstream.
	HealthConfig *HealthConfig `yaml:"health_config,omitempty" json:"health_config,omitempty"`
	// Headers are additional headers to add when proxying to this upstream.
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
	// Timeout overrides the default request timeout for this upstream.
	Timeout time.Duration `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	// MaxConnections limits concurrent connections to this upstream.
	MaxConnections int `yaml:"max_connections,omitempty" json:"max_connections,omitempty"`
}

// LuaLimits defines resource limits for Lua script execution.
type LuaLimits struct {
	// MaxRoutes limits the number of routes a Lua script can create.
	MaxRoutes int `yaml:"max_routes" json:"max_routes"`
	// MaxMiddlewares limits the number of middleware a Lua script can register.
	MaxMiddlewares int `yaml:"max_middlewares" json:"max_middlewares"`
	// CPUTimeout limits the CPU time for Lua script execution.
	CPUTimeout time.Duration `yaml:"cpu_timeout" json:"cpu_timeout"`
	// MemoryLimit limits memory usage for Lua script execution.
	MemoryLimit int64 `yaml:"memory_limit" json:"memory_limit"`
	// MaxChangesPerMinute limits dynamic routing changes.
	MaxChangesPerMinute int `yaml:"max_changes_per_minute" json:"max_changes_per_minute"`
}

// DefaultLuaLimits returns sensible defaults for Lua resource limits.
func DefaultLuaLimits() LuaLimits {
	return LuaLimits{
		MaxRoutes:           200,
		MaxMiddlewares:      20,
		CPUTimeout:          30 * time.Millisecond,
		MemoryLimit:         10 * 1024 * 1024, // 10MB
		MaxChangesPerMinute: 60,
	}
}

// Validate checks if the tenant configuration is valid.
func (t *Tenant) Validate() error {
	if t.ID == "" {
		return &ConfigError{Field: "id", Message: "cannot be empty"}
	}
	if t.Name == "" {
		return &ConfigError{Field: "name", Message: "cannot be empty"}
	}
	if len(t.Domains) == 0 {
		return &ConfigError{Field: "domains", Message: "must specify at least one domain"}
	}
	if len(t.Upstreams) == 0 {
		return &ConfigError{Field: "upstreams", Message: "must specify at least one upstream"}
	}

	// Validate domains
	for i, domain := range t.Domains {
		if domain == "" {
			return &ConfigError{
				Field:   fmt.Sprintf("domains[%d]", i),
				Message: "cannot be empty",
			}
		}
		// Basic domain validation - should not contain path or scheme
		if strings.Contains(domain, "/") || strings.Contains(domain, "://") {
			return &ConfigError{
				Field:   fmt.Sprintf("domains[%d]", i),
				Message: "should be domain only, not full URL",
			}
		}
	}

	// Validate base path
	if t.BasePath != "" && !strings.HasPrefix(t.BasePath, "/") {
		return &ConfigError{
			Field:   "base_path",
			Message: "must start with '/'",
		}
	}

	// Validate upstreams
	for i, upstream := range t.Upstreams {
		if err := upstream.Validate(); err != nil {
			return &ConfigError{
				Field:   fmt.Sprintf("upstreams[%d]", i),
				Message: err.Error(),
			}
		}
	}

	// Validate Lua limits if dynamic routing is enabled
	if t.DynamicRoutingEnabled {
		if err := t.LuaLimits.Validate(); err != nil {
			return &ConfigError{
				Field:   "lua_limits",
				Message: err.Error(),
			}
		}
	}

	// Validate health config if provided
	if t.HealthConfig != nil {
		if err := t.HealthConfig.Validate(); err != nil {
			return &ConfigError{
				Field:   "health_config",
				Message: err.Error(),
			}
		}
	}

	return nil
}

// MatchesDomain checks if the given domain matches this tenant.
func (t *Tenant) MatchesDomain(domain string) bool {
	for _, d := range t.Domains {
		if matchDomain(d, domain) {
			return true
		}
	}
	return false
}

// MatchesPath checks if the given path matches this tenant's base path.
func (t *Tenant) MatchesPath(path string) bool {
	if t.BasePath == "" || t.BasePath == "/" {
		return true
	}
	return strings.HasPrefix(path, t.BasePath)
}

// GetHealthConfig returns the effective health config for this tenant.
func (t *Tenant) GetHealthConfig(global *HealthConfig) HealthConfig {
	if t.HealthConfig != nil {
		return *t.HealthConfig
	}
	if global != nil {
		return *global
	}
	return DefaultHealthConfig()
}

// Validate checks if the upstream configuration is valid.
func (u *Upstream) Validate() error {
	if u.ID == "" {
		return &ConfigError{Field: "id", Message: "cannot be empty"}
	}
	if u.Name == "" {
		return &ConfigError{Field: "name", Message: "cannot be empty"}
	}
	if u.URL == "" {
		return &ConfigError{Field: "url", Message: "cannot be empty"}
	}

	// Validate URL format
	parsedURL, err := url.Parse(u.URL)
	if err != nil {
		return &ConfigError{
			Field:   "url",
			Message: fmt.Sprintf("invalid URL format: %v", err),
		}
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return &ConfigError{
			Field:   "url",
			Message: "scheme must be http or https",
		}
	}
	if parsedURL.Host == "" {
		return &ConfigError{
			Field:   "url",
			Message: "must include host",
		}
	}

	// Validate weight
	if u.Weight < 0 {
		return &ConfigError{
			Field:   "weight",
			Message: "cannot be negative",
		}
	}
	if u.Weight == 0 {
		u.Weight = 100 // Set default weight
	}

	// Validate timeout
	if u.Timeout < 0 {
		return &ConfigError{
			Field:   "timeout",
			Message: "cannot be negative",
		}
	}

	// Validate max connections
	if u.MaxConnections < 0 {
		return &ConfigError{
			Field:   "max_connections",
			Message: "cannot be negative",
		}
	}

	// Validate health config if provided
	if u.HealthConfig != nil {
		if err := u.HealthConfig.Validate(); err != nil {
			return &ConfigError{
				Field:   "health_config",
				Message: err.Error(),
			}
		}
	}

	return nil
}

// GetHealthConfig returns the effective health config for this upstream.
func (u *Upstream) GetHealthConfig(tenant *HealthConfig, global *HealthConfig) HealthConfig {
	if u.HealthConfig != nil {
		return *u.HealthConfig
	}
	if tenant != nil {
		return *tenant
	}
	if global != nil {
		return *global
	}
	return DefaultHealthConfig()
}

// ParsedURL returns the parsed URL for this upstream.
func (u *Upstream) ParsedURL() (*url.URL, error) {
	return url.Parse(u.URL)
}

// Validate checks if the Lua limits configuration is valid.
func (ll *LuaLimits) Validate() error {
	if ll.MaxRoutes <= 0 {
		return &ConfigError{Field: "max_routes", Message: "must be positive"}
	}
	if ll.MaxMiddlewares <= 0 {
		return &ConfigError{Field: "max_middlewares", Message: "must be positive"}
	}
	if ll.CPUTimeout <= 0 {
		return &ConfigError{Field: "cpu_timeout", Message: "must be positive"}
	}
	if ll.MemoryLimit <= 0 {
		return &ConfigError{Field: "memory_limit", Message: "must be positive"}
	}
	if ll.MaxChangesPerMinute <= 0 {
		return &ConfigError{Field: "max_changes_per_minute", Message: "must be positive"}
	}
	return nil
}

// TenantResolver provides tenant resolution functionality.
type TenantResolver struct {
	tenants map[string]*Tenant // domain -> tenant mapping
}

// NewTenantResolver creates a new tenant resolver with the given tenants.
func NewTenantResolver(tenants []*Tenant) *TenantResolver {
	tr := &TenantResolver{
		tenants: make(map[string]*Tenant),
	}
	
	// Build domain mapping
	for _, tenant := range tenants {
		for _, domain := range tenant.Domains {
			tr.tenants[domain] = tenant
		}
	}
	
	return tr
}

// Resolve finds the tenant for the given host and path.
func (tr *TenantResolver) Resolve(host, path string) *Tenant {
	// Strip port from host if present
	if colonIndex := strings.Index(host, ":"); colonIndex != -1 {
		host = host[:colonIndex]
	}
	
	// Look for exact domain match first
	if tenant, exists := tr.tenants[host]; exists {
		if tenant.MatchesPath(path) {
			return tenant
		}
	}
	
	// Look for wildcard domain matches
	for domain, tenant := range tr.tenants {
		if matchDomain(domain, host) && tenant.MatchesPath(path) {
			return tenant
		}
	}
	
	return nil
}

// matchDomain checks if a domain pattern matches the given host.
// Supports wildcards like *.example.com
func matchDomain(pattern, host string) bool {
	if pattern == host {
		return true
	}
	
	// Handle wildcard patterns
	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[1:] // Remove the *
		return strings.HasSuffix(host, suffix)
	}
	
	return false
}

// TenantStats provides statistics about a tenant.
type TenantStats struct {
	ID              string                 `json:"id"`
	Name            string                 `json:"name"`
	Domains         []string               `json:"domains"`
	UpstreamCount   int                    `json:"upstream_count"`
	HealthyUpstreams int                   `json:"healthy_upstreams"`
	TotalRequests   int64                  `json:"total_requests"`
	ErrorCount      int64                  `json:"error_count"`
	AvgResponseTime time.Duration          `json:"avg_response_time"`
	UpstreamStats   map[string]HealthStats `json:"upstream_stats"`
}