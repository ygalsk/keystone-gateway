# Admin API

## Endpoints

All under configured `admin_base_path` (default: `/admin`)

### Gateway Health
```bash
GET /admin/health
```
```json
{"status": "healthy", "timestamp": "..."}
```

### List Tenants  
```bash
GET /admin/tenants
```
```json
{
  "tenants": [
    {
      "name": "api",
      "status": "healthy", 
      "services": 2,
      "routing": "host-based"
    }
  ]
}
```

### Tenant Health
```bash
GET /admin/tenants/{name}/health
```
```json
{
  "tenant": "api",
  "status": "healthy",
  "services": [
    {
      "name": "backend-1",
      "url": "http://backend:3001", 
      "status": "healthy",
      "last_check": "..."
    }
  ]
}
```

## Status Codes

- `200` - Healthy
- `503` - Unhealthy (some/all backends down)
- `404` - Tenant not found

That's it.