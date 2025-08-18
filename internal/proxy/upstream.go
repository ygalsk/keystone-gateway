package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync/atomic"
	"time"

	"keystone-gateway/internal/types"
)

// Upstream represents a backend server with connection tracking and health status
type Upstream struct {
	// Configuration
	Name       string
	URL        *url.URL
	Weight     int32  // For weighted load balancing
	HealthPath string // Path for health checks (e.g., "/health")

	// Connection tracking for load balancing
	ActiveConnections atomic.Int32 // Current active connections
	TotalRequests     atomic.Int64 // Total requests sent (for metrics)

	// Health status tracking - uses types.HealthTracker for 2-strike rule
	HealthTracker *types.HealthTracker

	// Performance metrics
	AvgResponseTime  atomic.Int64 // Average response time in microseconds
	LastResponseTime atomic.Int64 // Last response time in microseconds

	// HTTP reverse proxy instance
	Proxy *httputil.ReverseProxy
}

// NewUpstream creates a new upstream with proper initialization
func NewUpstream(name, rawURL string, weight int32, healthPath string) (*Upstream, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	upstream := &Upstream{
		Name:       name,
		URL:        parsedURL,
		Weight:     weight,
		HealthPath: healthPath,
	}

	// Initialize health tracker
	upstream.HealthTracker = types.NewHealthTracker()

	// Create the reverse proxy
	upstream.Proxy = httputil.NewSingleHostReverseProxy(parsedURL)

	// Customize the proxy to handle our connection tracking
	originalDirector := upstream.Proxy.Director
	upstream.Proxy.Director = func(req *http.Request) {
		// Call the original director first
		originalDirector(req)

		// Add custom headers for traceability
		req.Header.Set("X-Proxy-Upstream", name)
		req.Header.Set("X-Proxy-Timestamp", time.Now().Format(time.RFC3339))
	}

	return upstream, nil
}

// IsHealthy returns whether this upstream is currently healthy
func (u *Upstream) IsHealthy() bool {
	return u.HealthTracker.IsHealthy()
}

// MarkHealthy marks this upstream as healthy using the health tracker
func (u *Upstream) MarkHealthy() {
	u.HealthTracker.MarkHealthy()
}

// MarkUnhealthy marks this upstream as unhealthy using the 2-strike rule
func (u *Upstream) MarkUnhealthy() {
	u.HealthTracker.MarkUnhealthy()
}

// MarkDegraded marks this upstream as degraded (responding but slowly)
func (u *Upstream) MarkDegraded() {
	u.HealthTracker.MarkDegraded()
}

// GetHealthStats returns comprehensive health statistics
func (u *Upstream) GetHealthStats() types.HealthStats {
	return u.HealthTracker.Stats()
}

// IncrementConnections increments the active connection count
// Returns the new connection count
func (u *Upstream) IncrementConnections() int32 {
	return u.ActiveConnections.Add(1)
}

// DecrementConnections decrements the active connection count
// Returns the new connection count
func (u *Upstream) DecrementConnections() int32 {
	return u.ActiveConnections.Add(-1)
}

// GetConnectionCount returns the current active connection count
func (u *Upstream) GetConnectionCount() int32 {
	return u.ActiveConnections.Load()
}

// RecordRequest records metrics for a completed request
func (u *Upstream) RecordRequest(responseTime time.Duration, success bool) {
	u.TotalRequests.Add(1)

	// Update response time metrics
	responseTimeMicros := responseTime.Microseconds()
	u.LastResponseTime.Store(responseTimeMicros)

	// Simple moving average calculation
	// In production, you might want a more sophisticated approach
	currentAvg := u.AvgResponseTime.Load()
	if currentAvg == 0 {
		u.AvgResponseTime.Store(responseTimeMicros)
	} else {
		// Weighted average: 90% old, 10% new
		newAvg := (currentAvg*9 + responseTimeMicros) / 10
		u.AvgResponseTime.Store(newAvg)
	}
}

// GetStats returns current statistics for this upstream
func (u *Upstream) GetStats() UpstreamStats {
	return UpstreamStats{
		Name:                u.Name,
		URL:                 u.URL.String(),
		Healthy:             u.IsHealthy(),
		ActiveConnections:   u.GetConnectionCount(),
		TotalRequests:       u.TotalRequests.Load(),
		ConsecutiveFailures: u.HealthTracker.FailureCount(),
		AvgResponseTime:     time.Duration(u.AvgResponseTime.Load()) * time.Microsecond,
		LastHealthCheck:     u.HealthTracker.LastCheck(),
	}
}

// UpstreamStats represents the current statistics for an upstream
type UpstreamStats struct {
	Name                string        `json:"name"`
	URL                 string        `json:"url"`
	Healthy             bool          `json:"healthy"`
	ActiveConnections   int32         `json:"active_connections"`
	TotalRequests       int64         `json:"total_requests"`
	ConsecutiveFailures int32         `json:"consecutive_failures"`
	AvgResponseTime     time.Duration `json:"avg_response_time"`
	LastHealthCheck     time.Time     `json:"last_health_check"`
}
