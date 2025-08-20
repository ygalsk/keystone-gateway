package types

import (
	"net/http"
	"time"
)

// RouteResult represents the result of the Lua routing decision
type RouteResult struct {
	ShouldRoute    bool              `json:"should_route"`
	TargetUpstream string            `json:"target_upstream,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`
	StatusCode     int               `json:"status_code,omitempty"`
}

// RouteDefinition represents a dynamic route registration
type RouteDefinition struct {
	TenantName   string
	Method       string
	Pattern      string
	GroupPattern string
	Handler      http.HandlerFunc
	RegisteredAt time.Time
}

// MiddlewareDefinition represents a dynamic middleware registration
type MiddlewareDefinition struct {
	TenantName   string
	Pattern      string
	GroupPattern string
	Middleware   func(http.Handler) http.Handler
	RegisteredAt time.Time
}

// RouteInfo provides information about registered routes
type RouteInfo struct {
	Method       string    `json:"method"`
	Pattern      string    `json:"pattern"`
	TenantName   string    `json:"tenant_name"`
	GroupPattern string    `json:"group_pattern,omitempty"`
	Registered   time.Time `json:"registered"`
}

// MiddlewareInfo provides information about registered middleware
type MiddlewareInfo struct {
	Pattern      string    `json:"pattern"`
	TenantName   string    `json:"tenant_name"`
	GroupPattern string    `json:"group_pattern,omitempty"`
	Registered   time.Time `json:"registered"`
}

// GroupInfo provides information about route groups
type GroupInfo struct {
	Pattern     string    `json:"pattern"`
	TenantName  string    `json:"tenant_name"`
	ParentGroup string    `json:"parent_group,omitempty"`
	Registered  time.Time `json:"registered"`
}

// ChiOperationResult represents the result of a Chi router operation
type ChiOperationResult struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   error  `json:"-"` // Don't serialize errors
}
