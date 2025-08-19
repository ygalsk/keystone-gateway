package types

import (
	"context"
	"github.com/go-chi/chi/v5"
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

// ChiRouterController defines the interface for dynamic Chi router control
// Follows Go best practices: context-aware, proper error handling, atomic operations
type ChiRouterController interface {
	// AddRoute and RemoveRoute = Route Management - context-aware for cancellation safety
	AddRoute(ctx context.Context, route RouteDefinition) error
	RemoveRoute(ctx context.Context, method, pattern string) error

	// AddMiddleware and RemoveMiddleware = Middleware Management - following Chi's middleware patterns
	AddMiddleware(ctx context.Context, middleware MiddlewareDefinition) error
	RemoveMiddleware(ctx context.Context, pattern string) error

	// CreateGroup and RemoveGroup = Group Management - exposes Chi's native grouping logic setup function receives chi.Router just like native Chi
	CreateGroup(ctx context.Context, pattern string, setup func(chi.Router)) error
	RemoveGroup(ctx context.Context, pattern string) error

	// ListRoutes ListMiddlewares and ListGroups = Query Operations - for observability and debugging
	ListRoutes() []RouteInfo
	ListMiddlewares() []MiddlewareInfo
	ListGroups() []GroupInfo

	// GetRoute GetMiddleware and GetGroup Specific Lookups - returns a pointer to allow nil for "not found"
	GetRoute(method, pattern string) (*RouteInfo, error)
	GetMiddleware(pattern string) (*MiddlewareInfo, error)
	GetGroup(pattern string) (*GroupInfo, error)

	// GetStats = Atomic Metrics - consistent with other components
	GetStats() map[string]int64

	// GetRouter = Thread-safe router access - for advanced use cases
	GetRouter() chi.Router
}
