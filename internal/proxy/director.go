// Package proxy provides load balancing and request direction for upstream servers.
// This file implements the director that selects upstream servers using various algorithms.
package proxy

import (
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// LoadBalancer manages upstream selection using configurable algorithms.
type LoadBalancer struct {
	// strategy defines the load balancing algorithm
	strategy string
	
	// upstreams holds all registered upstreams
	upstreams []*Upstream
	mu        sync.RWMutex
	
	// round robin state (for round_robin strategy)
	rrIndex atomic.Uint64
	
	// logger for load balancer events
	logger *slog.Logger
	
	// random source for weighted algorithms
	rand *rand.Rand
}

// NewLoadBalancer creates a new load balancer with the specified strategy.
func NewLoadBalancer(strategy string, logger *slog.Logger) *LoadBalancer {
	return &LoadBalancer{
		strategy:  strategy,
		upstreams: make([]*Upstream, 0),
		logger:    logger,
		rand:      rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// AddUpstream adds an upstream to the load balancer.
func (lb *LoadBalancer) AddUpstream(upstream *Upstream) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	
	lb.upstreams = append(lb.upstreams, upstream)
	lb.logger.Info("added upstream to load balancer",
		"name", upstream.Name,
		"url", upstream.URL.String(),
		"weight", upstream.Weight,
		"strategy", lb.strategy)
}

// RemoveUpstream removes an upstream from the load balancer.
func (lb *LoadBalancer) RemoveUpstream(name string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	
	for i, upstream := range lb.upstreams {
		if upstream.Name == name {
			// Remove by swapping with last element and truncating
			lb.upstreams[i] = lb.upstreams[len(lb.upstreams)-1]
			lb.upstreams = lb.upstreams[:len(lb.upstreams)-1]
			lb.logger.Info("removed upstream from load balancer", "name", name)
			return
		}
	}
}

// SelectUpstream selects the best upstream server based on the configured strategy.
// Returns nil if no healthy upstreams are available.
func (lb *LoadBalancer) SelectUpstream() *Upstream {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	
	// Filter to only healthy upstreams
	healthy := lb.getHealthyUpstreams()
	if len(healthy) == 0 {
		lb.logger.Warn("no healthy upstreams available")
		return nil
	}
	
	switch lb.strategy {
	case "least_connections":
		return lb.selectLeastConnections(healthy)
	case "round_robin":
		return lb.selectRoundRobin(healthy)
	case "weighted_round_robin":
		return lb.selectWeightedRoundRobin(healthy)
	default:
		lb.logger.Error("unknown load balancing strategy", "strategy", lb.strategy)
		return lb.selectLeastConnections(healthy) // Fallback to least connections
	}
}

// getHealthyUpstreams returns only the upstreams that are currently healthy.
func (lb *LoadBalancer) getHealthyUpstreams() []*Upstream {
	healthy := make([]*Upstream, 0, len(lb.upstreams))
	for _, upstream := range lb.upstreams {
		if upstream.IsHealthy() {
			healthy = append(healthy, upstream)
		}
	}
	return healthy
}

// selectLeastConnections selects the upstream with the fewest active connections.
// In case of ties, it selects randomly among the tied upstreams.
func (lb *LoadBalancer) selectLeastConnections(upstreams []*Upstream) *Upstream {
	if len(upstreams) == 0 {
		return nil
	}
	
	if len(upstreams) == 1 {
		return upstreams[0]
	}
	
	// Find the minimum connection count
	minConnections := upstreams[0].GetConnectionCount()
	for _, upstream := range upstreams[1:] {
		if connections := upstream.GetConnectionCount(); connections < minConnections {
			minConnections = connections
		}
	}
	
	// Collect all upstreams with minimum connections
	candidates := make([]*Upstream, 0, len(upstreams))
	for _, upstream := range upstreams {
		if upstream.GetConnectionCount() == minConnections {
			candidates = append(candidates, upstream)
		}
	}
	
	// Select randomly among candidates to avoid always picking the first one
	if len(candidates) == 1 {
		return candidates[0]
	}
	
	return candidates[lb.rand.Intn(len(candidates))]
}

// selectRoundRobin selects upstreams in round-robin fashion.
func (lb *LoadBalancer) selectRoundRobin(upstreams []*Upstream) *Upstream {
	if len(upstreams) == 0 {
		return nil
	}
	
	index := lb.rrIndex.Add(1) - 1
	return upstreams[index%uint64(len(upstreams))]
}

// selectWeightedRoundRobin selects upstreams based on their weights.
func (lb *LoadBalancer) selectWeightedRoundRobin(upstreams []*Upstream) *Upstream {
	if len(upstreams) == 0 {
		return nil
	}
	
	// Calculate total weight
	var totalWeight int32
	for _, upstream := range upstreams {
		totalWeight += upstream.Weight
	}
	
	if totalWeight == 0 {
		// If all weights are 0, fall back to round robin
		return lb.selectRoundRobin(upstreams)
	}
	
	// Select based on weight
	target := lb.rand.Int31n(totalWeight)
	var current int32
	
	for _, upstream := range upstreams {
		current += upstream.Weight
		if current > target {
			return upstream
		}
	}
	
	// Fallback (should not happen with correct weights)
	return upstreams[len(upstreams)-1]
}

// GetStats returns load balancer statistics.
func (lb *LoadBalancer) GetStats() LoadBalancerStats {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	
	stats := LoadBalancerStats{
		Strategy:      lb.strategy,
		TotalUpstreams: len(lb.upstreams),
		HealthyUpstreams: len(lb.getHealthyUpstreams()),
		UpstreamStats: make([]UpstreamStats, 0, len(lb.upstreams)),
	}
	
	for _, upstream := range lb.upstreams {
		stats.UpstreamStats = append(stats.UpstreamStats, upstream.GetStats())
	}
	
	return stats
}

// LoadBalancerStats provides statistics about the load balancer state.
type LoadBalancerStats struct {
	Strategy         string          `json:"strategy"`
	TotalUpstreams   int             `json:"total_upstreams"`
	HealthyUpstreams int             `json:"healthy_upstreams"`
	UpstreamStats    []UpstreamStats `json:"upstream_stats"`
}

// ProxyDirector creates an HTTP director function that selects upstreams and tracks connections.
func (lb *LoadBalancer) ProxyDirector() func(*http.Request) (*Upstream, error) {
	return func(req *http.Request) (*Upstream, error) {
		upstream := lb.SelectUpstream()
		if upstream == nil {
			return nil, fmt.Errorf("no healthy upstreams available")
		}
		
		// Increment connection count
		upstream.IncrementConnections()
		
		// Set up the request to go to the selected upstream
		req.URL.Scheme = upstream.URL.Scheme
		req.URL.Host = upstream.URL.Host
		
		// Preserve the original path and query
		if upstream.URL.Path != "" && upstream.URL.Path != "/" {
			req.URL.Path = upstream.URL.Path + req.URL.Path
		}
		
		// Add tracing headers
		req.Header.Set("X-Proxy-Upstream", upstream.Name)
		req.Header.Set("X-Proxy-Timestamp", time.Now().Format(time.RFC3339))
		
		// Ensure we have proper host header for the upstream
		if req.Header.Get("Host") == "" {
			req.Header.Set("Host", upstream.URL.Host)
		}
		
		return upstream, nil
	}
}

// RequestWrapper wraps an HTTP request with connection tracking.
type RequestWrapper struct {
	upstream   *Upstream
	startTime  time.Time
	logger     *slog.Logger
}

// NewRequestWrapper creates a new request wrapper for connection tracking.
func NewRequestWrapper(upstream *Upstream, logger *slog.Logger) *RequestWrapper {
	return &RequestWrapper{
		upstream:  upstream,
		startTime: time.Now(),
		logger:    logger,
	}
}

// Finish should be called when the request is complete to update metrics.
func (rw *RequestWrapper) Finish(success bool) {
	if rw.upstream == nil {
		return
	}
	
	// Decrement connection count
	connections := rw.upstream.DecrementConnections()
	
	// Record request metrics
	duration := time.Since(rw.startTime)
	rw.upstream.RecordRequest(duration, success)
	
	rw.logger.Debug("request completed",
		"upstream", rw.upstream.Name,
		"duration", duration,
		"success", success,
		"active_connections", connections)
}

// GetUpstreamCount returns the total number of upstreams in the load balancer.
func (lb *LoadBalancer) GetUpstreamCount() int {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	return len(lb.upstreams)
}

// GetHealthyUpstreamCount returns the number of healthy upstreams.
func (lb *LoadBalancer) GetHealthyUpstreamCount() int {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	return len(lb.getHealthyUpstreams())
}

// HasHealthyUpstreams returns true if there are any healthy upstreams available.
func (lb *LoadBalancer) HasHealthyUpstreams() bool {
	return lb.GetHealthyUpstreamCount() > 0
}