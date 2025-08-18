// Package proxy provides health checking functionality for upstream servers.
// This file implements the health checker with configurable intervals and 2-strike rule.
package proxy

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"keystone-gateway/internal/types"
)

// HealthChecker manages health checks for multiple upstreams.
// It runs periodic health checks and maintains health status using the 2-strike rule.
type HealthChecker struct {
	// config holds the health check configuration
	config types.HealthConfig
	
	// upstreams tracks all upstreams being monitored
	upstreams map[string]*Upstream
	mu        sync.RWMutex
	
	// httpClient for performing health checks
	httpClient *http.Client
	
	// logger for health check events
	logger *slog.Logger
	
	// ctx and cancel for controlling the health check lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewHealthChecker creates a new health checker with the given configuration.
func NewHealthChecker(config types.HealthConfig, logger *slog.Logger) *HealthChecker {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &HealthChecker{
		config:     config,
		upstreams:  make(map[string]*Upstream),
		httpClient: &http.Client{
			Timeout: config.Timeout,
			// Don't follow redirects for health checks
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}
}

// AddUpstream adds an upstream to be health checked.
func (hc *HealthChecker) AddUpstream(upstream *Upstream) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	
	hc.upstreams[upstream.Name] = upstream
	hc.logger.Info("added upstream for health checking",
		"name", upstream.Name,
		"url", upstream.URL.String(),
		"health_path", upstream.HealthPath)
}

// RemoveUpstream removes an upstream from health checking.
func (hc *HealthChecker) RemoveUpstream(name string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	
	delete(hc.upstreams, name)
	hc.logger.Info("removed upstream from health checking", "name", name)
}

// Start begins the health checking process for all registered upstreams.
func (hc *HealthChecker) Start() {
	if !hc.config.Enabled {
		hc.logger.Info("health checking is disabled")
		return
	}
	
	hc.logger.Info("starting health checker",
		"interval", hc.config.Interval,
		"timeout", hc.config.Timeout,
		"path", hc.config.Path)
	
	hc.wg.Add(1)
	go hc.run()
}

// Stop gracefully stops the health checking process.
func (hc *HealthChecker) Stop() {
	hc.logger.Info("stopping health checker")
	hc.cancel()
	hc.wg.Wait()
	hc.logger.Info("health checker stopped")
}

// run is the main health checking loop.
func (hc *HealthChecker) run() {
	defer hc.wg.Done()
	
	ticker := time.NewTicker(hc.config.Interval)
	defer ticker.Stop()
	
	// Perform initial health check
	hc.checkAllUpstreams()
	
	for {
		select {
		case <-hc.ctx.Done():
			return
		case <-ticker.C:
			hc.checkAllUpstreams()
		}
	}
}

// checkAllUpstreams performs health checks on all registered upstreams.
func (hc *HealthChecker) checkAllUpstreams() {
	hc.mu.RLock()
	upstreams := make([]*Upstream, 0, len(hc.upstreams))
	for _, upstream := range hc.upstreams {
		upstreams = append(upstreams, upstream)
	}
	hc.mu.RUnlock()
	
	// Check all upstreams concurrently
	var wg sync.WaitGroup
	for _, upstream := range upstreams {
		wg.Add(1)
		go func(u *Upstream) {
			defer wg.Done()
			hc.checkUpstream(u)
		}(upstream)
	}
	wg.Wait()
}

// checkUpstream performs a health check on a single upstream.
func (hc *HealthChecker) checkUpstream(upstream *Upstream) {
	start := time.Now()
	
	// Determine health check URL
	healthURL := hc.buildHealthURL(upstream)
	
	// Create request with context for timeout
	ctx, cancel := context.WithTimeout(hc.ctx, hc.config.Timeout)
	defer cancel()
	
	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		hc.handleCheckError(upstream, start, fmt.Errorf("failed to create request: %w", err))
		return
	}
	
	// Add headers for identification
	req.Header.Set("User-Agent", "Keystone-Gateway-HealthCheck/1.0")
	req.Header.Set("X-Health-Check", "true")
	
	// Perform the health check
	resp, err := hc.httpClient.Do(req)
	if err != nil {
		hc.handleCheckError(upstream, start, fmt.Errorf("request failed: %w", err))
		return
	}
	defer resp.Body.Close()
	
	// Evaluate response
	duration := time.Since(start)
	if hc.isHealthyResponse(resp.StatusCode) {
		hc.handleCheckSuccess(upstream, duration, resp.StatusCode)
	} else {
		hc.handleCheckError(upstream, start, fmt.Errorf("unhealthy status code: %d", resp.StatusCode))
	}
}

// buildHealthURL constructs the health check URL for an upstream.
func (hc *HealthChecker) buildHealthURL(upstream *Upstream) string {
	healthPath := upstream.HealthPath
	if healthPath == "" {
		healthPath = hc.config.Path
	}
	
	healthURL := &url.URL{
		Scheme: upstream.URL.Scheme,
		Host:   upstream.URL.Host,
		Path:   healthPath,
	}
	
	return healthURL.String()
}

// isHealthyResponse checks if the status code indicates a healthy response.
func (hc *HealthChecker) isHealthyResponse(statusCode int) bool {
	for _, expected := range hc.config.ExpectedStatusCodes {
		if statusCode == expected {
			return true
		}
	}
	return false
}

// handleCheckSuccess processes a successful health check.
func (hc *HealthChecker) handleCheckSuccess(upstream *Upstream, duration time.Duration, statusCode int) {
	wasUnhealthy := !upstream.IsHealthy()
	
	upstream.MarkHealthy()
	upstream.RecordRequest(duration, true)
	
	if wasUnhealthy {
		hc.logger.Info("upstream recovered",
			"name", upstream.Name,
			"duration", duration,
			"status_code", statusCode)
	} else {
		hc.logger.Debug("upstream healthy",
			"name", upstream.Name,
			"duration", duration,
			"status_code", statusCode)
	}
}

// handleCheckError processes a failed health check.
func (hc *HealthChecker) handleCheckError(upstream *Upstream, start time.Time, err error) {
	duration := time.Since(start)
	wasHealthy := upstream.IsHealthy()
	
	upstream.MarkUnhealthy()
	upstream.RecordRequest(duration, false)
	
	stats := upstream.GetHealthStats()
	
	if wasHealthy && !upstream.IsHealthy() {
		hc.logger.Warn("upstream became unhealthy",
			"name", upstream.Name,
			"error", err.Error(),
			"failure_count", stats.FailureCount,
			"duration", duration)
	} else {
		hc.logger.Debug("upstream check failed",
			"name", upstream.Name,
			"error", err.Error(),
			"failure_count", stats.FailureCount,
			"duration", duration)
	}
}

// GetUpstreamHealth returns the health status of a specific upstream.
func (hc *HealthChecker) GetUpstreamHealth(name string) (types.HealthStats, bool) {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	
	upstream, exists := hc.upstreams[name]
	if !exists {
		return types.HealthStats{}, false
	}
	
	return upstream.GetHealthStats(), true
}

// GetAllUpstreamHealth returns health status for all upstreams.
func (hc *HealthChecker) GetAllUpstreamHealth() map[string]types.HealthStats {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	
	result := make(map[string]types.HealthStats, len(hc.upstreams))
	for name, upstream := range hc.upstreams {
		result[name] = upstream.GetHealthStats()
	}
	
	return result
}

// GetHealthyUpstreams returns a list of currently healthy upstream names.
func (hc *HealthChecker) GetHealthyUpstreams() []string {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	
	var healthy []string
	for name, upstream := range hc.upstreams {
		if upstream.IsHealthy() {
			healthy = append(healthy, name)
		}
	}
	
	return healthy
}

// CheckUpstreamNow performs an immediate health check on a specific upstream.
func (hc *HealthChecker) CheckUpstreamNow(name string) error {
	hc.mu.RLock()
	upstream, exists := hc.upstreams[name]
	hc.mu.RUnlock()
	
	if !exists {
		return fmt.Errorf("upstream '%s' not found", name)
	}
	
	hc.checkUpstream(upstream)
	return nil
}