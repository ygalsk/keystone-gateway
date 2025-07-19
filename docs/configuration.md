# Configuration Guide

**Complete YAML configuration reference for Keystone Gateway**

## Configuration File Structure

```yaml
# Admin settings (optional)
admin_base_path: "/admin"  # Default: "/"

# Tenant definitions
tenants:
  - name: "service-name"
    # Routing configuration (choose one or both)
    domains: ["example.com", "api.example.com"]  # Host-based routing
    path_prefix: "/api/v1/"                      # Path-based routing
    
    # Health checking
    health_interval: 15  # Seconds between health checks
    
    # Backend services
    services:
      - name: "primary"
        url: "http://backend1:8080"
        health: "/health"
      - name: "secondary"  
        url: "http://backend2:8080"
        health: "/status"
```

## Routing Modes

### 1. Host-based Routing
Routes requests based on the `Host` header:

```yaml
tenants:
  - name: api
    domains: ["api.example.com", "api-v2.example.com"]
    services:
      - name: api-server
        url: http://api-backend:3000
        health: /health
```

### 2. Path-based Routing
Routes requests based on URL path prefix:

```yaml
tenants:
  - name: api-v1
    path_prefix: "/api/v1/"
    services:
      - name: api-v1-server
        url: http://api-v1:3000
        health: /health
```

### 3. Hybrid Routing
Combines both host and path matching:

```yaml
tenants:
  - name: api-admin
    domains: ["api.example.com"]
    path_prefix: "/admin/"
    services:
      - name: admin-service
        url: http://admin-backend:3000
        health: /health
```

## Configuration Options

### Admin Settings
```yaml
admin_base_path: "/admin"  # Base path for /health and /tenants endpoints
```

### Tenant Settings
```yaml
name: "unique-service-name"        # Required: Unique identifier
domains: ["host1.com", "host2.com"]  # Optional: Host-based routing
path_prefix: "/api/v2/"            # Optional: Path-based routing  
health_interval: 30                # Optional: Health check interval (seconds, default: 10)
```

### Service Settings
```yaml
name: "backend-name"               # Required: Service identifier
url: "http://backend:8080"         # Required: Backend URL
health: "/health"                  # Required: Health check endpoint path
```

## Example Configurations

### Multi-service Setup
```yaml
admin_base_path: "/admin"

tenants:
  # Frontend app
  - name: frontend
    domains: ["example.com", "www.example.com"]
    health_interval: 30
    services:
      - name: frontend-primary
        url: http://frontend1:3000
        health: /health
      - name: frontend-backup
        url: http://frontend2:3000
        health: /health

  # API services
  - name: api-v1
    path_prefix: "/api/v1/"
    health_interval: 10
    services:
      - name: api-v1-main
        url: http://api-v1:8080
        health: /status

  # Admin panel (hybrid routing)
  - name: admin
    domains: ["admin.example.com"]
    path_prefix: "/dashboard/"
    health_interval: 20
    services:
      - name: admin-service
        url: http://admin:4000
        health: /health
```

### Development Setup
```yaml
tenants:
  - name: dev-app
    domains: ["localhost", "127.0.0.1"]
    health_interval: 5
    services:
      - name: dev-server
        url: http://localhost:3001
        health: /health
```

## Health Checks

The gateway performs health checks on all backend services:

- **Interval**: Configurable per tenant (default: 10 seconds)
- **Method**: HTTP GET to the health endpoint
- **Success**: HTTP status code < 400
- **Failure Handling**: Removes unhealthy backends from rotation
- **Recovery**: Automatically re-adds backends when they become healthy

## Load Balancing

- **Algorithm**: Round-robin across healthy backends
- **Failover**: Automatic removal of unhealthy backends
- **Fallback**: Uses first backend if none are healthy (to avoid complete failure)

## Best Practices

### Health Check Endpoints
- Keep health checks lightweight (< 100ms response time)
- Return HTTP 200 for healthy, 503 for unhealthy
- Include dependency checks (database, external APIs)

### Configuration Management
- Use environment-specific config files
- Validate configuration before deployment: `make test`
- Keep sensitive data in environment variables, not config files

### Performance Tuning
- Set appropriate health check intervals (balance responsiveness vs. load)
- Use shorter intervals for critical services (5-10s)
- Use longer intervals for stable services (30-60s)

## Troubleshooting

**Service not routing correctly?**
- Check domain/path_prefix configuration
- Verify DNS/Host headers match exactly
- Test with curl: `curl -H "Host: your-domain.com" http://localhost:8080/`

**Health checks failing?**
- Verify backend health endpoint returns < 400 status
- Check backend accessibility from gateway
- Review health_interval setting

**Load balancing not working?**
- Ensure multiple services are configured
- Check that backends are healthy: `curl http://localhost:8080/admin/health`
- Verify service URLs are correct and accessible