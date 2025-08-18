// Package types defines core data types for the Keystone Gateway.
// This file contains health check related types and status tracking functionality.
package types

import (
	"sync/atomic"
	"time"
)

// HealthStatus represents the current health state of an upstream.
type HealthStatus int32

const (
	// HealthStatusUnknown indicates the health status has not been determined yet.
	HealthStatusUnknown HealthStatus = iota
	// HealthStatusHealthy indicates the upstream is responding normally.
	HealthStatusHealthy
	// HealthStatusUnhealthy indicates the upstream is not responding or returning errors.
	HealthStatusUnhealthy
	// HealthStatusDegraded indicates the upstream is responding but with high latency or errors.
	HealthStatusDegraded
)

// String returns a human-readable representation of the health status.
func (hs HealthStatus) String() string {
	switch hs {
	case HealthStatusHealthy:
		return "healthy"
	case HealthStatusUnhealthy:
		return "unhealthy"
	case HealthStatusDegraded:
		return "degraded"
	default:
		return "unknown"
	}
}

// HealthCheck represents a single health check result.
type HealthCheck struct {
	// ID uniquely identifies this health check.
	ID string `json:"id"`
	// UpstreamID identifies which upstream this check is for.
	UpstreamID string `json:"upstream_id"`
	// Status indicates whether the check passed or failed.
	Status HealthStatus `json:"status"`
	// Timestamp when the check was performed.
	Timestamp time.Time `json:"timestamp"`
	// Duration how long the check took.
	Duration time.Duration `json:"duration"`
	// StatusCode HTTP status code returned by the upstream (if applicable).
	StatusCode int `json:"status_code,omitempty"`
	// Error describes any error that occurred during the check.
	Error string `json:"error,omitempty"`
}

// HealthTracker tracks health status with 2-strike rule for an upstream.
// It uses atomic operations for thread-safe access without mutex overhead.
type HealthTracker struct {
	// status holds the current health status atomically.
	status atomic.Int32
	// failureCount tracks consecutive failures for 2-strike rule.
	failureCount atomic.Int32
	// lastCheck stores the timestamp of the last health check.
	lastCheck atomic.Int64
	// checkCount tracks total number of health checks performed.
	checkCount atomic.Int64
	// successCount tracks total number of successful health checks.
	successCount atomic.Int64
}

// NewHealthTracker creates a new health tracker with unknown status.
func NewHealthTracker() *HealthTracker {
	ht := &HealthTracker{}
	ht.status.Store(int32(HealthStatusUnknown))
	return ht
}

// Status returns the current health status.
func (ht *HealthTracker) Status() HealthStatus {
	return HealthStatus(ht.status.Load())
}

// IsHealthy returns true if the upstream is currently considered healthy.
func (ht *HealthTracker) IsHealthy() bool {
	return ht.Status() == HealthStatusHealthy
}

// MarkHealthy marks the upstream as healthy and resets failure count.
func (ht *HealthTracker) MarkHealthy() {
	ht.status.Store(int32(HealthStatusHealthy))
	ht.failureCount.Store(0)
	ht.lastCheck.Store(time.Now().UnixNano())
	ht.checkCount.Add(1)
	ht.successCount.Add(1)
}

// MarkUnhealthy marks the upstream as unhealthy using the 2-strike rule.
// Only marks as unhealthy after 2 consecutive failures to reduce noise.
func (ht *HealthTracker) MarkUnhealthy() {
	failures := ht.failureCount.Add(1)
	ht.lastCheck.Store(time.Now().UnixNano())
	ht.checkCount.Add(1)
	
	// 2-strike rule: only mark as unhealthy after 2 consecutive failures
	if failures >= 2 {
		ht.status.Store(int32(HealthStatusUnhealthy))
	}
}

// MarkDegraded marks the upstream as degraded (responding but slowly/errors).
func (ht *HealthTracker) MarkDegraded() {
	ht.status.Store(int32(HealthStatusDegraded))
	ht.failureCount.Store(1) // Degraded counts as a partial failure
	ht.lastCheck.Store(time.Now().UnixNano())
	ht.checkCount.Add(1)
}

// FailureCount returns the current consecutive failure count.
func (ht *HealthTracker) FailureCount() int32 {
	return ht.failureCount.Load()
}

// LastCheck returns the timestamp of the last health check.
func (ht *HealthTracker) LastCheck() time.Time {
	nanos := ht.lastCheck.Load()
	if nanos == 0 {
		return time.Time{}
	}
	return time.Unix(0, nanos)
}

// Stats returns health check statistics.
func (ht *HealthTracker) Stats() HealthStats {
	total := ht.checkCount.Load()
	success := ht.successCount.Load()
	
	var successRate float64
	if total > 0 {
		successRate = float64(success) / float64(total)
	}
	
	return HealthStats{
		Status:         ht.Status(),
		TotalChecks:    total,
		SuccessfulChecks: success,
		SuccessRate:    successRate,
		FailureCount:   ht.FailureCount(),
		LastCheck:      ht.LastCheck(),
	}
}

// HealthStats provides comprehensive health statistics for an upstream.
type HealthStats struct {
	Status           HealthStatus  `json:"status"`
	TotalChecks      int64         `json:"total_checks"`
	SuccessfulChecks int64         `json:"successful_checks"`
	SuccessRate      float64       `json:"success_rate"`
	FailureCount     int32         `json:"failure_count"`
	LastCheck        time.Time     `json:"last_check"`
}

// HealthConfig defines configuration for health checking behavior.
type HealthConfig struct {
	// Enabled determines if health checking is active.
	Enabled bool `yaml:"enabled" json:"enabled"`
	// Path is the HTTP path to check for health.
	Path string `yaml:"path" json:"path"`
	// Interval between health checks.
	Interval time.Duration `yaml:"interval" json:"interval"`
	// Timeout for individual health check requests.
	Timeout time.Duration `yaml:"timeout" json:"timeout"`
	// FailureThreshold number of consecutive failures before marking unhealthy.
	FailureThreshold int `yaml:"failure_threshold" json:"failure_threshold"`
	// SuccessThreshold number of consecutive successes before marking healthy.
	SuccessThreshold int `yaml:"success_threshold" json:"success_threshold"`
	// ExpectedStatusCodes list of HTTP status codes considered healthy.
	ExpectedStatusCodes []int `yaml:"expected_status_codes" json:"expected_status_codes"`
}

// DefaultHealthConfig returns sensible defaults for health checking.
func DefaultHealthConfig() HealthConfig {
	return HealthConfig{
		Enabled:             true,
		Path:                "/health",
		Interval:            30 * time.Second,
		Timeout:             5 * time.Second,
		FailureThreshold:    2, // 2-strike rule
		SuccessThreshold:    1,
		ExpectedStatusCodes: []int{200, 204},
	}
}

// Validate checks if the health configuration is valid.
func (hc *HealthConfig) Validate() error {
	if hc.Interval <= 0 {
		return &ConfigError{
			Field:   "interval",
			Message: "must be positive",
		}
	}
	if hc.Timeout <= 0 {
		return &ConfigError{
			Field:   "timeout",
			Message: "must be positive",
		}
	}
	if hc.Timeout >= hc.Interval {
		return &ConfigError{
			Field:   "timeout",
			Message: "must be less than interval",
		}
	}
	if hc.FailureThreshold <= 0 {
		return &ConfigError{
			Field:   "failure_threshold",
			Message: "must be positive",
		}
	}
	if hc.SuccessThreshold <= 0 {
		return &ConfigError{
			Field:   "success_threshold",
			Message: "must be positive",
		}
	}
	if len(hc.ExpectedStatusCodes) == 0 {
		return &ConfigError{
			Field:   "expected_status_codes",
			Message: "must not be empty",
		}
	}
	return nil
}

// ConfigError represents a configuration validation error.
type ConfigError struct {
	Field   string
	Message string
}

// Error implements the error interface.
func (ce *ConfigError) Error() string {
	return "config error in field '" + ce.Field + "': " + ce.Message
}