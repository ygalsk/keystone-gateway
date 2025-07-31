# Keystone Gateway Deployment Guide

This directory contains Docker-based deployment configurations for Keystone Gateway. We use a **clean, streamlined Docker-only approach** with unified **Makefile** automation, focusing purely on the gateway without external infrastructure dependencies.

## 📁 Directory Structure

```
deployments/
├── docker/                           # Docker deployment configurations
│   └── docker-compose.staging.yml   # Staging environment
└── README.md                         # This file
```

**Note**: We use a **simplified Docker approach** focused purely on the gateway. Production deployment is managed through the root `docker-compose.production.yml` file.

## 🚀 Quick Start

### Using the Makefile System (Recommended)

```bash
# Start development environment
make dev

# Deploy to staging environment  
make staging

# Deploy to production environment
make production

# View all available commands
make help
```

### Direct Docker Compose (Advanced)

```bash
# Deploy to staging environment
docker-compose -f deployments/docker/docker-compose.staging.yml up -d

# Deploy to production
docker-compose -f docker-compose.production.yml up -d
```

## 🌍 Environment Configurations

We support three deployment environments, all focused purely on the gateway:

### Development Environment (`make dev`)
- **Purpose**: Local development and testing
- **Port**: 8082
- **Configuration**: Uses staging config for consistency
- **Resources**: Lightweight single container
- **Features**: Quick startup, development logging

### Staging Environment (`make staging`)
- **Purpose**: Pre-production testing and validation
- **Port**: 8081
- **Configuration**: `configs/environments/staging.yaml`
- **Compose File**: `deployments/docker/docker-compose.staging.yml`
- **Features**: 
  - Gateway with load balancing
  - Mock backend for testing
  - Health checks and logging

### Production Environment (`make production`)
- **Purpose**: Live production workloads
- **Port**: 8080
- **Configuration**: `configs/environments/production-high-load.yaml`
- **Compose File**: `docker-compose.production.yml` (root directory)
- **Features**:
  - High-performance configuration
  - Resource optimization
  - Production logging

## 🔧 Makefile Commands

All deployments are managed through the unified Makefile system:

### Environment Management
```bash
make dev            # Start development environment
make staging        # Deploy to staging
make production     # Deploy to production (with confirmation)

make dev-stop       # Stop development
make staging-stop   # Stop staging
make production-stop # Stop production (with confirmation)
```

### Monitoring and Health
```bash
make health         # Check all environments
make status         # Show environment status  
make logs           # Show available log commands

make dev-health     # Check development health
make staging-health # Check staging health
make production-health # Check production health
```

### Development Workflow
```bash
make feature-start FEATURE=my-feature  # Start new feature
make hotfix-start HOTFIX=my-hotfix     # Start emergency hotfix
```

### Build and Test
```bash
make build          # Build Docker image
make test           # Run comprehensive test suite
make lint           # Code quality checks
make fmt            # Format code
```

## 📊 Health Checks

### Health Check Endpoints

| Environment | Port | Endpoint | Purpose |
|-------------|------|----------|---------|
| Development | 8082 | `/admin/health` | Development health status |
| Staging | 8081 | `/admin/health` | Staging health status |
| Production | 8080 | `/admin/health` | Production health status |

### Quick Health Check
```bash
# Check all environments at once
make health

# Check specific environment
curl http://localhost:8080/admin/health  # Production
curl http://localhost:8081/admin/health  # Staging
curl http://localhost:8082/admin/health  # Development
```

## 🐳 Docker Configuration

### Images

- **Gateway**: Built from project Dockerfile with multi-stage build
- **Mock Backend**: httpbin for staging testing

### Networks

- **Staging**: Internal Docker Compose network
- **Production**: Single-service deployment

### Volumes

- Configuration files (read-only mounts)
- Lua scripts (read-only mounts)
- Log management through Docker logging drivers

## 🔄 CI/CD Integration

### GitHub Actions Triggers

```yaml
# Staging deployment
on:
  push:
    branches: [ staging ]

# Production deployment  
on:
  push:
    branches: [ main ]
```

### Deployment Pipeline

1. **Build Phase**: Docker image creation
2. **Test Phase**: Automated testing
3. **Deploy Phase**: Environment-specific deployment using Makefile
4. **Verify Phase**: Health checks

## 🔐 Security Considerations

### Container Security

- Non-root user in containers
- Read-only configuration mounts
- Resource limits to prevent resource exhaustion
- Minimal base images

### Configuration Security

- Configuration files mounted read-only
- No secrets in version control
- Environment-specific configurations

## 📝 Troubleshooting

### Common Issues

**Gateway not starting:**
```bash
# Check logs
make dev-logs        # Development logs
make staging-logs    # Staging logs
make production-logs # Production logs
```

**Health checks failing:**
```bash
# Direct health check
curl -v http://localhost:8080/admin/health

# Check container status
docker ps
```

**Configuration issues:**
```bash
# Validate configuration files
make validate

# Check mounted configs
docker exec -it keystone-gateway cat /app/config.yaml
```

### Log Access

**View logs for specific environment:**
```bash
# Development
make dev-logs

# Staging
make staging-logs

# Production
make production-logs
```

**Direct Docker commands:**
```bash
# View all containers
docker ps

# Specific container logs
docker logs keystone-gateway-staging
docker logs keystone-gateway
```

## 🔄 Rollback Procedures

### Simple Rollback

```bash
# Stop current deployment
make staging-stop  # or make production-stop

# Use previous image
docker tag keystone-gateway:previous keystone-gateway:latest

# Restart
make staging  # or make production
```

### Using Git for Rollback

```bash
# Rollback to previous commit
git checkout HEAD~1

# Rebuild and deploy
make staging  # or make production
```

## 📈 Scaling

### Manual Scaling (Production)

Production scaling can be achieved by:

1. **Horizontal scaling**: Deploy multiple instances behind a load balancer
2. **Vertical scaling**: Increase container resource limits
3. **Configuration tuning**: Optimize GOGC and GOMAXPROCS settings

### Docker Compose Scaling (Staging)

```bash
# Scale gateway instances (staging only)
docker-compose -f deployments/docker/docker-compose.staging.yml up -d --scale keystone-gateway=3
```

## 🛠️ Maintenance

### Regular Tasks

1. **Image Updates**: Keep base images up to date with `make build`
2. **Log Management**: Monitor container log sizes
3. **Health Monitoring**: Regular `make health` checks
4. **Performance Monitoring**: Review gateway metrics

### Update Procedures

1. Test changes in development: `make dev`
2. Deploy to staging: `make staging`
3. Verify functionality and performance
4. Deploy to production: `make production`
5. Monitor post-deployment health

## 📞 Support

For deployment issues:

1. Check service health: `make health`
2. Review service logs: `make logs`
3. Verify configuration: `make validate`
4. Test connectivity to admin endpoints
5. Check this README for solutions

## 🔗 Related Documentation

- [Main README](../README.md) - Project overview and quick start
- [Configuration Examples](../configs/examples/) - Sample configurations
- [Lua Scripting Guide](../docs/lua-scripting.md) - Route scripting
- [Contributing Guidelines](../CONTRIBUTING.md) - Development workflow