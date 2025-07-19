# Getting Started

**Get Keystone Gateway running in under 5 minutes**

## Prerequisites

- **Go 1.21+** (for building from source)
- **Docker** (for containerized deployment)
- **Basic tools**: `make`, `curl`

## Installation

### Option 1: Build from Source
```bash
git clone https://github.com/your-org/keystone-gateway
cd keystone-gateway
make build
```

### Option 2: Using Docker
```bash
git clone https://github.com/your-org/keystone-gateway
cd keystone-gateway
make docker
```

## Quick Setup

### 1. Basic Configuration
Create or edit `configs/config.yaml`:

```yaml
tenants:
  - name: demo
    domains:
      - demo.example.com
    health_interval: 15
    services:
      - name: demo-service
        url: http://localhost:3001
        health: /health
```

### 2. Start the Gateway
```bash
# Run locally
make run

# Or with Docker
make run-docker
```

### 3. Test Your Setup
```bash
# Health check
curl http://localhost:8080/admin/health

# Test routing (with host header)
curl -H "Host: demo.example.com" http://localhost:8080/
```

## Next Steps

- **[Configuration Guide](configuration.md)** - Learn all configuration options
- **[Deployment Guide](deployment.md)** - Deploy to production
- **[Development Guide](../DEVELOPMENT.md)** - Contribute to the project

## Troubleshooting

**Gateway won't start?**
- Check if port 8080 is available: `lsof -i :8080`
- Verify configuration syntax: `make lint`

**Can't reach backends?**
- Ensure backend services are running
- Check health endpoint URLs in config
- Review gateway logs: `make logs`

Need more help? See the **[Troubleshooting Guide](troubleshooting.md)**.