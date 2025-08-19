package lua

import (
	"sync/atomic"
	"time"
)

// LuaMetrics tracks execution statistics atomically (consistent with upstream patterns)
type LuaMetrics struct {
	// Execution metrics
	activeExecutions atomic.Int32 // Currently running scripts
	totalExecutions  atomic.Int64 // Total script executions
	successCount     atomic.Int64 // Successful executions
	errorCount       atomic.Int64 // Failed executions

	// Performance metrics
	executionTime   atomic.Int64 // Total execution time (nanoseconds)
	avgResponseTime atomic.Int64 // Rolling average response time

	// Resource metrics
	memoryUsage     atomic.Int64 // Current memory usage estimate
	peakMemoryUsage atomic.Int64 // Peak memory usage

	// Script metrics
	compiledScripts atomic.Int32 // Number of compiled scripts
	cacheHits       atomic.Int64 // Bytecode cache hits
	cacheMisses     atomic.Int64 // Bytecode cache misses
}

func NewLuaMetrics() *LuaMetrics {
	return &LuaMetrics{}
}

// TrackExecution returns a completion function (defer pattern)
func (m *LuaMetrics) TrackExecution() func(error) {
	m.activeExecutions.Add(1)
	m.totalExecutions.Add(1)
	start := time.Now()

	return func(err error) {
		// Update execution counts
		m.activeExecutions.Add(-1)

		// Track success/error
		if err != nil {
			m.errorCount.Add(1)
		} else {
			m.successCount.Add(1)
		}

		// Track execution time
		duration := time.Since(start)
		m.executionTime.Add(duration.Nanoseconds())

		// Update rolling average (simple implementation)
		totalExec := m.totalExecutions.Load()
		if totalExec > 0 {
			avgNanos := m.executionTime.Load() / totalExec
			m.avgResponseTime.Store(avgNanos)
		}
	}
}

// TrackMemoryUsage updates memory usage estimates
func (m *LuaMetrics) TrackMemoryUsage(bytes int64) {
	m.memoryUsage.Store(bytes)

	// Update peak if necessary
	for {
		current := m.peakMemoryUsage.Load()
		if bytes <= current || m.peakMemoryUsage.CompareAndSwap(current, bytes) {
			break
		}
	}
}

// TrackCacheHit records bytecode cache statistics
func (m *LuaMetrics) TrackCacheHit() {
	m.cacheHits.Add(1)
}

func (m *LuaMetrics) TrackCacheMiss() {
	m.cacheMisses.Add(1)
}

// GetStats returns current metrics (atomic reads)
func (m *LuaMetrics) GetStats() map[string]int64 {
	return map[string]int64{
		"active_executions":    int64(m.activeExecutions.Load()),
		"total_executions":     m.totalExecutions.Load(),
		"success_count":        m.successCount.Load(),
		"error_count":          m.errorCount.Load(),
		"avg_response_time_ms": m.avgResponseTime.Load() / 1_000_000, // Convert to ms
		"memory_usage_bytes":   m.memoryUsage.Load(),
		"peak_memory_bytes":    m.peakMemoryUsage.Load(),
		"compiled_scripts":     int64(m.compiledScripts.Load()),
		"cache_hits":           m.cacheHits.Load(),
		"cache_misses":         m.cacheMisses.Load(),
	}
}

// GetErrorRate calculates current error rate
func (m *LuaMetrics) GetErrorRate() float64 {
	total := m.totalExecutions.Load()
	if total == 0 {
		return 0.0
	}
	errors := m.errorCount.Load()
	return float64(errors) / float64(total)
}
