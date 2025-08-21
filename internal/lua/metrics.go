package lua

import (
	"log/slog"
	"sync/atomic"
	"time"
)

// LuaMetrics tracks unified execution and router operation statistics atomically
// Consolidates metrics from LuaMetrics, ChiControllerMetrics, and ChiBindingsMetrics
type LuaMetrics struct {
	// Route operations (from ChiControllerMetrics)
	RouteAdds        atomic.Int64
	RouteRemoves     atomic.Int64
	MiddlewareAdds   atomic.Int64
	GroupCreates     atomic.Int64
	RouteErrors      atomic.Int64
	MiddlewareErrors atomic.Int64
	GroupErrors      atomic.Int64

	// Lua execution metrics (from ChiBindingsMetrics + existing)
	LuaExecutions      atomic.Int64
	LuaErrors          atomic.Int64
	ScriptCompilations atomic.Int64
	StatePoolGets      atomic.Int64
	StatePoolPuts      atomic.Int64

	// Performance metrics
	TotalExecutionTime   atomic.Int64
	AvgExecutionTime     atomic.Int64
	TotalOperationTime   atomic.Int64
	TotalOperations      atomic.Int64
	SuccessfulOperations atomic.Int64
	FailedOperations     atomic.Int64
	AvgOperationTime     atomic.Int64

	// Memory and security
	MemoryUsage        atomic.Int64
	PeakMemoryUsage    atomic.Int64
	SecurityViolations atomic.Int64

	// Active counters
	ActiveExecutions atomic.Int32
	CompiledScripts  atomic.Int32

	// Cache metrics
	CacheHits   atomic.Int64
	CacheMisses atomic.Int64
}

func NewLuaMetrics() *LuaMetrics {
	return &LuaMetrics{}
}

// Lua execution methods
func (m *LuaMetrics) RecordLuaExecution(duration time.Duration) {
	m.LuaExecutions.Add(1)
	durationNanos := duration.Nanoseconds()
	m.TotalExecutionTime.Add(durationNanos)

	// Update rolling average
	totalExec := m.LuaExecutions.Load()
	if totalExec > 0 {
		avgNanos := m.TotalExecutionTime.Load() / totalExec
		m.AvgExecutionTime.Store(avgNanos)
	}
}

func (m *LuaMetrics) RecordLuaError() {
	m.LuaErrors.Add(1)
}

func (m *LuaMetrics) RecordScriptCompilation() {
	m.ScriptCompilations.Add(1)
}

func (m *LuaMetrics) RecordStatePoolGet() {
	m.StatePoolGets.Add(1)
}

func (m *LuaMetrics) RecordStatePoolPut() {
	m.StatePoolPuts.Add(1)
}

// Performance tracking
func (m *LuaMetrics) RecordOperationTime(duration time.Duration) {
	m.TotalOperationTime.Add(duration.Nanoseconds())
}

// TrackExecution returns a completion function (defer pattern) - maintains compatibility
func (m *LuaMetrics) TrackExecution() func(error) {
	m.ActiveExecutions.Add(1)
	m.LuaExecutions.Add(1)
	start := time.Now()

	return func(err error) {
		// Update execution counts
		m.ActiveExecutions.Add(-1)

		// Track success/error
		if err != nil {
			m.LuaErrors.Add(1)
		}

		// Track execution time
		duration := time.Since(start)
		m.TotalExecutionTime.Add(duration.Nanoseconds())

		// Update rolling average
		totalExec := m.LuaExecutions.Load()
		if totalExec > 0 {
			avgNanos := m.TotalExecutionTime.Load() / totalExec
			m.AvgExecutionTime.Store(avgNanos)
		}
	}
}

// TrackMemoryUsage updates memory usage estimates
func (m *LuaMetrics) TrackMemoryUsage(bytes int64) {
	m.MemoryUsage.Store(bytes)

	// Update peak if necessary
	for {
		current := m.PeakMemoryUsage.Load()
		if bytes <= current || m.PeakMemoryUsage.CompareAndSwap(current, bytes) {
			break
		}
	}
}

// TrackCacheHit records bytecode cache statistics
func (m *LuaMetrics) TrackCacheHit() {
	m.CacheHits.Add(1)
}

func (m *LuaMetrics) TrackCacheMiss() {
	m.CacheMisses.Add(1)
}

func (m *LuaMetrics) RecordSecurityViolation() {
	m.SecurityViolations.Add(1)
}

// Router operation methods
func (m *LuaMetrics) RecordRouteAdd() {
	m.RouteAdds.Add(1)
	m.TotalOperations.Add(1)
}

func (m *LuaMetrics) RecordRouteRemove() {
	m.RouteRemoves.Add(1)
	m.TotalOperations.Add(1)
}

func (m *LuaMetrics) RecordMiddlewareAdd() {
	m.MiddlewareAdds.Add(1)
	m.TotalOperations.Add(1)
}

func (m *LuaMetrics) RecordGroupCreate() {
	m.GroupCreates.Add(1)
	m.TotalOperations.Add(1)
}

func (m *LuaMetrics) RecordRouteError() {
	m.RouteErrors.Add(1)
}

func (m *LuaMetrics) RecordMiddlewareError() {
	m.MiddlewareErrors.Add(1)
}

func (m *LuaMetrics) RecordGroupError() {
	m.GroupErrors.Add(1)
}

// Operation tracking with success/failure
func (m *LuaMetrics) RecordSuccessfulOperation(duration time.Duration) {
	m.SuccessfulOperations.Add(1)
	m.TotalOperationTime.Add(duration.Nanoseconds())
	m.updateAverageOperationTime()
}

func (m *LuaMetrics) RecordFailedOperation(duration time.Duration) {
	m.FailedOperations.Add(1)
	m.TotalOperationTime.Add(duration.Nanoseconds())
	m.updateAverageOperationTime()
}

func (m *LuaMetrics) updateAverageOperationTime() {
	totalOps := m.TotalOperations.Load()
	if totalOps > 0 {
		avgNanos := m.TotalOperationTime.Load() / totalOps
		m.AvgOperationTime.Store(avgNanos)
	}
}

// TrackOperation records metrics for router operations - moved from ChiRouter
func (m *LuaMetrics) TrackOperation(operation string, start time.Time, err error, logger *slog.Logger) {
	duration := time.Since(start)

	if err != nil {
		// Track error based on the operation type
		switch operation {
		case "route_add", "route_remove":
			m.RecordRouteError()
		case "middleware_add":
			m.RecordMiddlewareError()
		case "group_create":
			m.RecordGroupError()
		}
		m.RecordFailedOperation(duration)
		logger.Error("chi router operation failed",
			"operation", operation,
			"duration_ms", duration.Milliseconds(),
			"error", err.Error())
	} else {
		m.RecordSuccessfulOperation(duration)
		logger.Debug("chi router operation completed",
			"operation", operation,
			"duration_ms", duration.Milliseconds())
	}
}

// GetStats returns unified metrics combining all previous implementations
// Accepts router state counts to provide complete statistics for Prometheus
func (m *LuaMetrics) GetStats(routesCount, middlewaresCount, groupsCount int) map[string]int64 {
	return map[string]int64{
		// Route operations
		"route_adds":        m.RouteAdds.Load(),
		"route_removes":     m.RouteRemoves.Load(),
		"middleware_adds":   m.MiddlewareAdds.Load(),
		"group_creates":     m.GroupCreates.Load(),
		"route_errors":      m.RouteErrors.Load(),
		"middleware_errors": m.MiddlewareErrors.Load(),
		"group_errors":      m.GroupErrors.Load(),

		// Current router state
		"routes_registered":      int64(routesCount),
		"middlewares_registered": int64(middlewaresCount),
		"groups_created":         int64(groupsCount),

		// Lua execution
		"lua_executions":      m.LuaExecutions.Load(),
		"lua_errors":          m.LuaErrors.Load(),
		"script_compilations": m.ScriptCompilations.Load(),
		"state_pool_gets":     m.StatePoolGets.Load(),
		"state_pool_puts":     m.StatePoolPuts.Load(),

		// Performance
		"avg_execution_time_ms":   m.AvgExecutionTime.Load() / 1_000_000, // Convert to ms
		"total_operation_time":    m.TotalOperationTime.Load(),
		"total_operations":        m.TotalOperations.Load(),
		"successful_operations":   m.SuccessfulOperations.Load(),
		"failed_operations":       m.FailedOperations.Load(),
		"avg_operation_time_ms":   m.AvgOperationTime.Load() / 1_000_000, // Convert to ms

		// Memory and security
		"memory_usage_bytes":  m.MemoryUsage.Load(),
		"peak_memory_bytes":   m.PeakMemoryUsage.Load(),
		"security_violations": m.SecurityViolations.Load(),

		// Active counters
		"active_executions": int64(m.ActiveExecutions.Load()),
		"compiled_scripts":  int64(m.CompiledScripts.Load()),

		// Cache
		"cache_hits":   m.CacheHits.Load(),
		"cache_misses": m.CacheMisses.Load(),
	}
}

// GetErrorRate calculates current error rate
func (m *LuaMetrics) GetErrorRate() float64 {
	total := m.LuaExecutions.Load()
	if total == 0 {
		return 0.0
	}
	errors := m.LuaErrors.Load()
	return float64(errors) / float64(total)
}
