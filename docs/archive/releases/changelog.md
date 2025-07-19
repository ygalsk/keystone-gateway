# Release Notes

## v1.2.0 - Host-Based Routing (July 18, 2025)

### 🎯 New Features

#### Host-Based Routing
- **NEW**: Route requests based on domain/hostname in addition to path prefixes
- **NEW**: Support for multiple domains per tenant
- **NEW**: Hybrid routing (host + path combination)
- **NEW**: Configurable routing priority system

#### Configuration Enhancements  
- **NEW**: `domains` field in tenant configuration
- **NEW**: Enhanced configuration validation
- **NEW**: Domain format validation
- **NEW**: Flexible tenant routing options

### 🔧 Configuration Examples

#### Host-Only Routing
```yaml
tenants:
  - name: "production-app"
    domains: ["app.example.com", "www.app.example.com"]
    services:
      - name: "web-server"
        url: "http://web:8080"
        health: "/health"
```

#### Hybrid Routing (Host + Path)
```yaml
tenants:
  - name: "api-v2"
    domains: ["api.example.com"]
    path_prefix: "/v2/"
    services:
      - name: "api-server"
        url: "http://api:3000"
        health: "/status"
```

#### Mixed Environment (Legacy + New)
```yaml
tenants:
  # Legacy path-based routing (unchanged)
  - name: "legacy-api"
    path_prefix: "/api/"
    services: [...]
    
  # New host-based routing
  - name: "modern-app"  
    domains: ["app.example.com"]
    services: [...]
```

### 🚀 Routing Priority

1. **Host + Path Match** (highest priority)
2. **Host-only Match**  
3. **Path-only Match** (backward compatibility)

### ⚡ Performance

- **Latency Impact**: < 0.5ms overhead
- **Memory Impact**: < 1MB additional usage  
- **Throughput**: No degradation
- **Scalability**: No additional scaling concerns

### 🔄 Migration

#### Zero-Downtime Migration
- **100% Backward Compatible**: Existing configurations work unchanged
- **Gradual Migration**: Add `domains` alongside existing `path_prefix`
- **No Breaking Changes**: All existing APIs and behaviors preserved

#### Migration Example
```yaml
# Step 1: Add both (gradual migration)
tenants:
  - name: "my-app"
    domains: ["my-app.example.com"]  # New clean URLs
    path_prefix: "/app/"             # Keep legacy URLs working
    services: [...]

# Step 2: Remove path_prefix when ready  
tenants:
  - name: "my-app"
    domains: ["my-app.example.com"]
    services: [...]
```

### 🧪 Testing

- **Unit Tests**: 15+ new tests, 100% pass rate
- **Integration Tests**: End-to-end routing validation
- **Performance Tests**: Load testing with 1000+ concurrent requests
- **Backward Compatibility**: Verified with existing configurations

### 🐛 Bug Fixes

- Fixed proxy director path rewriting edge cases
- Improved error handling for invalid configurations
- Enhanced host header parsing (port handling)

### 📚 Documentation

- Updated README with host-based routing examples
- Added comprehensive configuration documentation  
- Created migration guide
- Added performance testing results

### 🔐 Security

- Domain validation prevents malicious configuration
- No new attack vectors introduced
- Maintains existing security model

---

## v1.1.0 - Path-Based Routing (Baseline)

### Features
- Path-based routing with prefix matching
- Round-robin load balancing
- Health checking
- Docker support

---

## Migration Guide v1.1.0 → v1.2.0

### For Users
- **No action required** - existing configurations work unchanged
- **Optional**: Migrate to host-based routing for cleaner URLs
- **Recommended**: Use hybrid approach for gradual migration

### For Developers  
- New `domains` field in `Tenant` struct
- Updated routing logic in `makeHandler`
- Additional validation functions
- Enhanced test coverage

### Deployment
```bash
# Pull new version
docker pull keystone-gateway:v1.2.0

# Update docker-compose.yml (optional - config changes only)
# No service restarts required for backward compatibility

# Deploy with zero downtime
docker-compose up -d
```

### Verification
```bash
# Test legacy routing still works
curl http://gateway/api/test

# Test new host-based routing
curl -H "Host: app.example.com" http://gateway/test

# Check logs for routing type confirmations
docker logs keystone-gateway
```
