package metrics

import (
	"fmt"
	"io"
	"time"
)

// boolToInt converts boolean to integer for Prometheus metrics
func BoolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// LoadBalancerStats represents stats from the load balancer
type LoadBalancerStats struct {
	Strategy         string
	TotalUpstreams   int
	HealthyUpstreams int
	UpstreamStats    []UpstreamStat
}

// UpstreamStat represents individual upstream statistics
type UpstreamStat struct {
	Name               string
	URL                string
	TotalRequests      int64
	AvgResponseTime    time.Duration
	Healthy            bool
	ActiveConnections  int64
	ConsecutiveFailures int64
}

// FormatPrometheusMetrics writes load balancer metrics in Prometheus format
func FormatPrometheusMetrics(w io.Writer, lbStats LoadBalancerStats, startTime time.Time) error {
	// Server uptime
	uptime := time.Since(startTime)

	// Gateway info
	fmt.Fprintf(w, "# HELP keystone_gateway_info Information about the keystone gateway\n")
	fmt.Fprintf(w, "# TYPE keystone_gateway_info gauge\n")
	fmt.Fprintf(w, "keystone_gateway_info{version=\"v1.0.0\",strategy=\"%s\"} 1\n", lbStats.Strategy)

	// Uptime
	fmt.Fprintf(w, "# HELP keystone_gateway_uptime_seconds Server uptime in seconds\n")
	fmt.Fprintf(w, "# TYPE keystone_gateway_uptime_seconds counter\n")
	fmt.Fprintf(w, "keystone_gateway_uptime_seconds %.2f\n", uptime.Seconds())

	// Total upstreams
	fmt.Fprintf(w, "# HELP keystone_upstreams_total Total number of upstreams\n")
	fmt.Fprintf(w, "# TYPE keystone_upstreams_total gauge\n")
	fmt.Fprintf(w, "keystone_upstreams_total %d\n", lbStats.TotalUpstreams)

	// Healthy upstreams
	fmt.Fprintf(w, "# HELP keystone_healthy_upstreams Total healthy upstreams\n")
	fmt.Fprintf(w, "# TYPE keystone_healthy_upstreams gauge\n")
	fmt.Fprintf(w, "keystone_healthy_upstreams %d\n", lbStats.HealthyUpstreams)

	// Per-upstream metric headers
	fmt.Fprintf(w, "# HELP keystone_upstream_requests_total Total requests sent to upstream\n")
	fmt.Fprintf(w, "# TYPE keystone_upstream_requests_total counter\n")

	fmt.Fprintf(w, "# HELP keystone_upstream_response_time_microseconds Average response time in microseconds\n")
	fmt.Fprintf(w, "# TYPE keystone_upstream_response_time_microseconds gauge\n")

	fmt.Fprintf(w, "# HELP keystone_upstream_healthy Health status of upstream (1=healthy, 0=unhealthy)\n")
	fmt.Fprintf(w, "# TYPE keystone_upstream_healthy gauge\n")

	fmt.Fprintf(w, "# HELP keystone_upstream_active_connections Current active connections to upstream\n")
	fmt.Fprintf(w, "# TYPE keystone_upstream_active_connections gauge\n")

	fmt.Fprintf(w, "# HELP keystone_upstream_consecutive_failures Number of consecutive failures\n")
	fmt.Fprintf(w, "# TYPE keystone_upstream_consecutive_failures gauge\n")

	// Per-upstream metrics
	for _, upstream := range lbStats.UpstreamStats {
		labels := fmt.Sprintf("upstream=\"%s\",url=\"%s\"", upstream.Name, upstream.URL)

		fmt.Fprintf(w, "keystone_upstream_requests_total{%s} %d\n", labels, upstream.TotalRequests)
		fmt.Fprintf(w, "keystone_upstream_response_time_microseconds{%s} %d\n", labels, upstream.AvgResponseTime.Microseconds())
		fmt.Fprintf(w, "keystone_upstream_healthy{%s} %d\n", labels, BoolToInt(upstream.Healthy))
		fmt.Fprintf(w, "keystone_upstream_active_connections{%s} %d\n", labels, upstream.ActiveConnections)
		fmt.Fprintf(w, "keystone_upstream_consecutive_failures{%s} %d\n", labels, upstream.ConsecutiveFailures)
	}

	return nil
}

// FormatLuaMetrics writes Lua metrics in Prometheus format
func FormatLuaMetrics(w io.Writer, luaStats map[string]int64) error {
	// Lua execution metrics
	fmt.Fprintf(w, "# HELP keystone_lua_executions_total Total Lua script executions\n")
	fmt.Fprintf(w, "# TYPE keystone_lua_executions_total counter\n")
	fmt.Fprintf(w, "keystone_lua_executions_total %d\n", luaStats["lua_executions"])

	fmt.Fprintf(w, "# HELP keystone_lua_errors_total Total Lua execution errors\n")
	fmt.Fprintf(w, "# TYPE keystone_lua_errors_total counter\n")
	fmt.Fprintf(w, "keystone_lua_errors_total %d\n", luaStats["lua_errors"])

	// Router operations
	fmt.Fprintf(w, "# HELP keystone_lua_route_adds_total Total route registrations\n")
	fmt.Fprintf(w, "# TYPE keystone_lua_route_adds_total counter\n")
	fmt.Fprintf(w, "keystone_lua_route_adds_total %d\n", luaStats["route_adds"])

	fmt.Fprintf(w, "# HELP keystone_lua_middleware_adds_total Total middleware registrations\n")
	fmt.Fprintf(w, "# TYPE keystone_lua_middleware_adds_total counter\n")
	fmt.Fprintf(w, "keystone_lua_middleware_adds_total %d\n", luaStats["middleware_adds"])

	fmt.Fprintf(w, "# HELP keystone_lua_group_creates_total Total group creations\n")
	fmt.Fprintf(w, "# TYPE keystone_lua_group_creates_total counter\n")
	fmt.Fprintf(w, "keystone_lua_group_creates_total %d\n", luaStats["group_creates"])

	// Current state
	fmt.Fprintf(w, "# HELP keystone_lua_routes_registered Current number of registered routes\n")
	fmt.Fprintf(w, "# TYPE keystone_lua_routes_registered gauge\n")
	fmt.Fprintf(w, "keystone_lua_routes_registered %d\n", luaStats["routes_registered"])

	fmt.Fprintf(w, "# HELP keystone_lua_middlewares_registered Current number of registered middlewares\n")
	fmt.Fprintf(w, "# TYPE keystone_lua_middlewares_registered gauge\n")
	fmt.Fprintf(w, "keystone_lua_middlewares_registered %d\n", luaStats["middlewares_registered"])

	fmt.Fprintf(w, "# HELP keystone_lua_groups_created Current number of created groups\n")
	fmt.Fprintf(w, "# TYPE keystone_lua_groups_created gauge\n")
	fmt.Fprintf(w, "keystone_lua_groups_created %d\n", luaStats["groups_created"])

	// Performance metrics
	fmt.Fprintf(w, "# HELP keystone_lua_avg_execution_time_milliseconds Average Lua execution time\n")
	fmt.Fprintf(w, "# TYPE keystone_lua_avg_execution_time_milliseconds gauge\n")
	fmt.Fprintf(w, "keystone_lua_avg_execution_time_milliseconds %d\n", luaStats["avg_execution_time_ms"])

	fmt.Fprintf(w, "# HELP keystone_lua_active_executions Current active Lua executions\n")
	fmt.Fprintf(w, "# TYPE keystone_lua_active_executions gauge\n")
	fmt.Fprintf(w, "keystone_lua_active_executions %d\n", luaStats["active_executions"])

	// Memory metrics
	fmt.Fprintf(w, "# HELP keystone_lua_memory_usage_bytes Current Lua memory usage\n")
	fmt.Fprintf(w, "# TYPE keystone_lua_memory_usage_bytes gauge\n")
	fmt.Fprintf(w, "keystone_lua_memory_usage_bytes %d\n", luaStats["memory_usage_bytes"])

	fmt.Fprintf(w, "# HELP keystone_lua_security_violations_total Total security violations\n")
	fmt.Fprintf(w, "# TYPE keystone_lua_security_violations_total counter\n")
	fmt.Fprintf(w, "keystone_lua_security_violations_total %d\n", luaStats["security_violations"])

	return nil
}