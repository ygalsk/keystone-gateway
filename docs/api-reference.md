# API Reference

**HTTP endpoints and responses for Keystone Gateway**

## Admin Endpoints

All admin endpoints are prefixed with the configured `admin_base_path` (default: `/admin`).

### Health Check

Check gateway and backend health status.

**Endpoint:** `GET /admin/health`

**Response:**
```json
{
  "status": "healthy",
  "tenants": {
    "api": "1/1 healthy",
    "frontend": "2/2 healthy", 
    "worker": "0/1 healthy"
  },
  "uptime": "2h34m12s",
  "version": "1.2.1"
}
```

**Status Codes:**
- `200 OK` - Gateway is healthy
- `503 Service Unavailable` - One or more critical backends are down

**Example:**
```bash
curl http://localhost:8080/admin/health
```

### Tenant Information

Get detailed tenant and backend configuration.

**Endpoint:** `GET /admin/tenants`

**Response:**
```json
[
  {
    "name": "api",
    "path_prefix": "/api/v1/",
    "domains": ["api.example.com"],
    "health_interval": 15,
    "services": [
      {
        "name": "api-primary",
        "url": "http://api1:8080",
        "health": "/health"
      },
      {
        "name": "api-backup", 
        "url": "http://api2:8080",
        "health": "/health"
      }
    ]
  }
]
```

**Status Codes:**
- `200 OK` - Successfully retrieved tenant information

**Example:**
```bash
curl http://localhost:8080/admin/tenants
```

## Proxy Endpoints

All other requests are handled by the proxy based on routing configuration.

### Request Flow

1. **Host-based Routing**: Matches `Host` header against `domains`
2. **Path-based Routing**: Matches URL path against `path_prefix`  
3. **Hybrid Routing**: Matches both host and path
4. **Backend Selection**: Round-robin among healthy backends
5. **Proxy Request**: Forwards to selected backend

### Headers

**Request Headers Added:**
- `X-Forwarded-For`: Client IP address
- `X-Forwarded-Proto`: Original protocol (http/https)
- `X-Real-IP`: Client IP address

**Response Headers Added:**
- Standard proxy headers from Chi middleware

### Error Responses

**404 Not Found**
```json
{
  "error": "No route found",
  "path": "/unknown/path",
  "host": "unknown.example.com"
}
```

**502 Bad Gateway**
```json
{
  "error": "All backends unavailable", 
  "tenant": "api",
  "backends_checked": 2
}
```

**503 Service Unavailable**
```json
{
  "error": "Backend health check failed",
  "tenant": "api",
  "backend": "api-primary"
}
```

## Health Check Protocol

Backend services must implement a health check endpoint that:

1. **Returns HTTP status < 400 for healthy**
2. **Returns HTTP status >= 400 for unhealthy**
3. **Responds quickly (< 1 second recommended)**

### Example Backend Health Endpoint

```javascript
// Node.js example
app.get('/health', (req, res) => {
  // Check dependencies (database, external APIs, etc.)
  if (database.isConnected() && externalAPI.isReachable()) {
    res.status(200).json({ status: 'healthy' });
  } else {
    res.status(503).json({ status: 'unhealthy' });
  }
});
```

```go
// Go example
func healthHandler(w http.ResponseWriter, r *http.Request) {
    if database.Ping() == nil && externalAPI.Check() {
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
    } else {
        w.WriteHeader(http.StatusServiceUnavailable)
        json.NewEncoder(w).Encode(map[string]string{"status": "unhealthy"})
    }
}
```

## Response Times

**Typical Response Times:**
- Health endpoint: < 5ms
- Tenant endpoint: < 10ms  
- Proxy requests: Backend response time + 1-3ms overhead

**Timeouts:**
- Health checks: 3 seconds
- Proxy requests: 60 seconds (configurable)
- Admin endpoints: 10 seconds

## Rate Limiting

Currently no built-in rate limiting. Use upstream reverse proxy (nginx, traefik) or implement in backend services.

## Logging

**Request Logging Format:**
```
"GET /api/users HTTP/1.1" from 192.168.1.100 - 200 1.2KB in 45ms
```

**Log Levels:**
- `ERROR`: Backend failures, configuration errors
- `WARN`: Health check failures, fallback usage
- `INFO`: Startup, configuration changes  
- `DEBUG`: Request routing decisions

## Monitoring Integration

### Prometheus Metrics (Future)
- Request counts by tenant/status
- Response time histograms
- Backend health status
- Error rates

### Custom Monitoring
Use the `/admin/health` endpoint for external monitoring:

```bash
# Nagios/Icinga check
#!/bin/bash
status=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/admin/health)
if [ "$status" = "200" ]; then
  echo "OK - Gateway healthy"
  exit 0
else
  echo "CRITICAL - Gateway unhealthy (HTTP $status)"
  exit 2
fi
```

## Configuration Reload

Currently requires restart. Future versions may support graceful configuration reload via API.

## Security Considerations

### Admin Endpoint Security
- **Network Restriction**: Limit access to admin endpoints by IP
- **Authentication**: Use reverse proxy for authentication if needed
- **Monitoring**: Log all admin endpoint access

### Request Security
- **Header Validation**: Basic validation of Host headers
- **Backend Communication**: Use HTTPS for backend communication in production
- **Input Sanitization**: Relies on backend services for input validation

## Examples

### Complete curl Examples

```bash
# Health check
curl -v http://localhost:8080/admin/health

# Tenant info
curl -v http://localhost:8080/admin/tenants

# Test host-based routing
curl -H "Host: api.example.com" http://localhost:8080/users

# Test path-based routing  
curl http://localhost:8080/api/v1/users

# Test with JSON
curl -H "Content-Type: application/json" \
     -H "Host: api.example.com" \
     -d '{"name": "test"}' \
     http://localhost:8080/users
```

### Integration Testing

```bash
# Test script for CI/CD
#!/bin/bash
set -e

# Wait for gateway to start
timeout 30s bash -c 'until curl -f http://localhost:8080/admin/health; do sleep 1; done'

# Test all configured routes
curl -f -H "Host: app.example.com" http://localhost:8080/
curl -f http://localhost:8080/api/v1/status
curl -f http://localhost:8080/admin/tenants

echo "All tests passed"
```