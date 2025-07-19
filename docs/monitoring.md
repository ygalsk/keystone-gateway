# Monitoring Guide

**Health checks, observability, and monitoring for Keystone Gateway**

## Built-in Monitoring

### Health Endpoint

Monitor gateway and backend health:

```bash
# Basic health check
curl http://localhost:8080/admin/health

# Response format
{
  "status": "healthy",
  "tenants": {
    "api": "2/2 healthy",
    "frontend": "1/2 healthy",
    "worker": "0/1 healthy"
  },
  "uptime": "2h34m12s",
  "version": "1.2.1"
}
```

### Tenant Status

Get detailed tenant and backend information:

```bash
curl http://localhost:8080/admin/tenants
```

## External Monitoring

### Nagios/Icinga

Health check script:
```bash
#!/bin/bash
# /usr/local/bin/check_keystone_gateway

HOSTNAME="localhost"
PORT="8080"
WARNING_THRESHOLD=1000  # ms
CRITICAL_THRESHOLD=3000 # ms

start_time=$(date +%s%3N)
status=$(curl -s -o /dev/null -w "%{http_code}" http://$HOSTNAME:$PORT/admin/health)
end_time=$(date +%s%3N)
response_time=$((end_time - start_time))

if [ "$status" != "200" ]; then
    echo "CRITICAL - Gateway unhealthy (HTTP $status)"
    exit 2
elif [ "$response_time" -gt "$CRITICAL_THRESHOLD" ]; then
    echo "CRITICAL - Response time ${response_time}ms > ${CRITICAL_THRESHOLD}ms"
    exit 2
elif [ "$response_time" -gt "$WARNING_THRESHOLD" ]; then
    echo "WARNING - Response time ${response_time}ms > ${WARNING_THRESHOLD}ms"
    exit 1
else
    echo "OK - Gateway healthy (${response_time}ms)"
    exit 0
fi
```

### Prometheus

Basic monitoring with Prometheus:

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'keystone-gateway'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/admin/health'
    scrape_interval: 30s
    scrape_timeout: 10s
```

Custom exporter script:
```bash
#!/bin/bash
# Simple metrics exporter

while true; do
    # Get health data
    health=$(curl -s http://localhost:8080/admin/health)
    
    # Extract metrics
    status=$(echo $health | jq -r '.status')
    uptime=$(echo $health | jq -r '.uptime')
    
    # Export as Prometheus format
    echo "# HELP keystone_gateway_up Gateway availability"
    echo "# TYPE keystone_gateway_up gauge"
    if [ "$status" = "healthy" ]; then
        echo "keystone_gateway_up 1"
    else
        echo "keystone_gateway_up 0"
    fi
    
    echo "# HELP keystone_gateway_uptime_seconds Gateway uptime"
    echo "# TYPE keystone_gateway_uptime_seconds counter"
    # Convert uptime to seconds (simplified)
    echo "keystone_gateway_uptime_seconds $(date +%s)"
    
    sleep 30
done > /var/lib/prometheus/keystone-gateway.prom
```

### Grafana Dashboard

JSON dashboard configuration:
```json
{
  "dashboard": {
    "title": "Keystone Gateway",
    "panels": [
      {
        "title": "Gateway Status",
        "type": "stat",
        "targets": [
          {
            "expr": "keystone_gateway_up",
            "legendFormat": "Gateway"
          }
        ]
      },
      {
        "title": "Response Time",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(http_request_duration_seconds_sum[5m]) / rate(http_request_duration_seconds_count[5m])",
            "legendFormat": "Avg Response Time"
          }
        ]
      }
    ]
  }
}
```

## Log Monitoring

### Structured Logging

Gateway logs in standard format:
```
2025-07-19T14:30:45Z INFO Gateway started on :8080
2025-07-19T14:30:46Z INFO Initialized tenant api with 2 backends
2025-07-19T14:30:47Z WARN Health check failed for service api-backup: connection refused
2025-07-19T14:30:48Z INFO "GET /admin/health HTTP/1.1" from 192.168.1.100 - 200 156B in 2ms
```

### Log Analysis

**Error Detection:**
```bash
# Watch for errors
tail -f gateway.log | grep ERROR

# Count errors per hour
grep ERROR gateway.log | awk '{print $1}' | cut -d'T' -f2 | cut -d':' -f1 | sort | uniq -c
```

**Performance Analysis:**
```bash
# Extract response times
grep '"GET\|POST\|PUT\|DELETE' gateway.log | grep -o 'in [0-9]*ms' | 
  awk '{sum+=$2; count++} END {print "Average:", sum/count "ms"}'

# Find slow requests (>1000ms)
grep '"GET\|POST\|PUT\|DELETE' gateway.log | grep 'in [0-9][0-9][0-9][0-9]ms'
```

### ELK Stack Integration

**Filebeat configuration:**
```yaml
filebeat.inputs:
- type: log
  enabled: true
  paths:
    - /var/log/keystone-gateway/*.log
  fields:
    service: keystone-gateway
    environment: production

output.elasticsearch:
  hosts: ["elasticsearch:9200"]
  index: "keystone-gateway-%{+yyyy.MM.dd}"
```

**Logstash pipeline:**
```ruby
input {
  beats {
    port => 5044
  }
}

filter {
  if [fields][service] == "keystone-gateway" {
    grok {
      match => { 
        "message" => "%{TIMESTAMP_ISO8601:timestamp} %{LOGLEVEL:level} %{GREEDYDATA:message}" 
      }
    }
    
    if [message] =~ /"[A-Z]+ .* HTTP\/1\.1"/ {
      grok {
        match => {
          "message" => '"(?<method>[A-Z]+) (?<path>[^ ]*) HTTP/1\.1" from (?<client_ip>[^ ]*) - (?<status_code>[0-9]+) (?<response_size>[^ ]*) in (?<response_time>[0-9]+)ms'
        }
      }
    }
  }
}

output {
  elasticsearch {
    hosts => ["elasticsearch:9200"]
    index => "keystone-gateway-%{+YYYY.MM.dd}"
  }
}
```

## Application Performance Monitoring

### Custom Metrics Collection

Add metrics collection to your application:

```go
type Metrics struct {
    RequestCount    int64
    ErrorCount      int64
    TotalResponseTime int64
    BackendHealth   map[string]bool
}

func (gw *Gateway) collectMetrics() Metrics {
    return Metrics{
        RequestCount: atomic.LoadInt64(&gw.requestCount),
        ErrorCount: atomic.LoadInt64(&gw.errorCount),
        TotalResponseTime: atomic.LoadInt64(&gw.totalResponseTime),
        BackendHealth: gw.getBackendHealthMap(),
    }
}

func (gw *Gateway) MetricsHandler(w http.ResponseWriter, r *http.Request) {
    metrics := gw.collectMetrics()
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(metrics)
}
```

### Load Testing

**Apache Bench:**
```bash
# Basic load test
ab -n 1000 -c 10 http://localhost:8080/admin/health

# With host header
ab -n 1000 -c 10 -H "Host: api.example.com" http://localhost:8080/api/test

# POST requests
ab -n 100 -c 5 -p postdata.json -T application/json http://localhost:8080/api/users
```

**wrk (if available):**
```bash
# More advanced load testing
wrk -t12 -c400 -d30s --script=test.lua http://localhost:8080/
```

## Alerting

### Basic Alerting Rules

**Health Check Failure:**
```bash
#!/bin/bash
# Cron job: */5 * * * * /usr/local/bin/alert_gateway_health

if ! curl -f http://localhost:8080/admin/health >/dev/null 2>&1; then
    echo "ALERT: Keystone Gateway health check failed" | 
    mail -s "Gateway Down" ops@company.com
fi
```

**High Error Rate:**
```bash
#!/bin/bash
# Check error rate in last 5 minutes

error_count=$(tail -n 1000 /var/log/keystone-gateway.log | 
             grep "$(date -d '5 minutes ago' '+%Y-%m-%d %H:%M')" | 
             grep -c ERROR)

if [ "$error_count" -gt 10 ]; then
    echo "ALERT: High error rate detected: $error_count errors in 5 minutes" |
    mail -s "Gateway Errors" ops@company.com
fi
```

### Prometheus Alerting

```yaml
# alerts.yml
groups:
- name: keystone-gateway
  rules:
  - alert: GatewayDown
    expr: keystone_gateway_up == 0
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: "Keystone Gateway is down"
      
  - alert: HighErrorRate
    expr: rate(keystone_gateway_errors_total[5m]) > 0.1
    for: 2m
    labels:
      severity: warning
    annotations:
      summary: "High error rate detected"
      
  - alert: SlowResponseTime
    expr: keystone_gateway_response_time_seconds > 1
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "Slow response times detected"
```

## Dashboard Examples

### Simple HTML Dashboard

```html
<!DOCTYPE html>
<html>
<head>
    <title>Keystone Gateway Status</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
</head>
<body>
    <h1>Gateway Status</h1>
    <div id="status"></div>
    <canvas id="responseTimeChart"></canvas>
    
    <script>
        async function updateStatus() {
            try {
                const response = await fetch('/admin/health');
                const data = await response.json();
                
                document.getElementById('status').innerHTML = 
                    `<h2>Status: ${data.status}</h2>
                     <p>Uptime: ${data.uptime}</p>
                     <p>Version: ${data.version}</p>`;
                     
                // Update chart with response time data
                updateChart(data);
            } catch (error) {
                document.getElementById('status').innerHTML = 
                    '<h2 style="color: red;">Gateway Unavailable</h2>';
            }
        }
        
        // Update every 30 seconds
        setInterval(updateStatus, 30000);
        updateStatus();
    </script>
</body>
</html>
```

## Best Practices

### Monitoring Strategy

1. **Layer monitoring**: Gateway, backends, infrastructure
2. **Multiple metrics**: Availability, performance, errors
3. **Proactive alerting**: Alert before users notice issues
4. **Historical data**: Keep metrics for trend analysis

### Alert Fatigue Prevention

1. **Meaningful thresholds**: Avoid false positives
2. **Escalation levels**: Warning → Critical → Emergency
3. **Alert grouping**: Combine related alerts
4. **Runbook links**: Include troubleshooting guides

### Performance Monitoring

1. **Baseline establishment**: Know normal performance
2. **Trend analysis**: Watch for gradual degradation
3. **Capacity planning**: Monitor resource usage trends
4. **User experience**: Monitor end-to-end performance

### Security Monitoring

1. **Access logs**: Monitor admin endpoint access
2. **Unusual patterns**: Watch for suspicious traffic
3. **Error patterns**: Monitor for potential attacks
4. **Configuration changes**: Track config modifications

## Troubleshooting Monitoring

**Monitoring not working?**
- Verify network connectivity to monitoring systems
- Check firewall rules for monitoring ports
- Validate monitoring configuration syntax
- Test monitoring endpoints manually

**False alerts?**
- Review alert thresholds
- Check for network issues
- Validate monitoring scripts
- Consider temporal patterns (load spikes, maintenance)

**Missing data?**
- Check log file permissions and rotation
- Verify data retention settings
- Monitor disk space on monitoring systems
- Check for clock synchronization issues