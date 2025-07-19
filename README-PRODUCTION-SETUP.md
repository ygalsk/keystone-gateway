# 🚀 Keystone Gateway Production Setup Guide

This guide will help you deploy Keystone Gateway to production with proper SSL certificates, monitoring, and security.

## 📋 Prerequisites

1. **Server Requirements:**
   - Linux server with Docker and Docker Compose
   - 2+ CPU cores, 4GB+ RAM recommended
   - 20GB+ storage
   - Public IP address

2. **Domain Requirements:**
   - DNS control for `*.keystone-gateway.dev` domains
   - Ability to point A records to your server's IP

3. **Software Dependencies:**
   - Docker 20.10+
   - Docker Compose 2.0+
   - Git

## 🏗️ Quick Deployment

### 1. Clone and Prepare

```bash
git clone <your-repo>
cd keystone-gateway
```

### 2. Configure DNS

Point these domains to your server's public IP:

```
demo.keystone-gateway.dev     → YOUR_SERVER_IP
api.keystone-gateway.dev      → YOUR_SERVER_IP  
auth.keystone-gateway.dev     → YOUR_SERVER_IP
status.keystone-gateway.dev   → YOUR_SERVER_IP
grafana.keystone-gateway.dev  → YOUR_SERVER_IP
traefik.keystone-gateway.dev  → YOUR_SERVER_IP
```

### 3. Update Production Configuration

**CRITICAL: Change all default passwords and secrets!**

Edit `configs/production.yaml`:

```bash
# Update these BEFORE deploying:
- Admin password
- Redis password  
- Database password
- JWT secrets
- Session secrets
```

Edit `docker-compose.production.yml`:

```bash
# Update these environment variables:
- GF_SECURITY_ADMIN_PASSWORD
- POSTGRES_PASSWORD
- Redis requirepass
- JWT_SECRET
- SESSION_SECRET
```

### 4. Deploy

```bash
./deploy.sh production deploy
```

The deployment script will:
- ✅ Build the gateway
- ✅ Start all services with Docker Compose
- ✅ Configure Traefik for SSL termination
- ✅ Set up Let's Encrypt certificates automatically
- ✅ Run health checks
- ✅ Show deployment status

## 🔐 SSL Certificates

SSL certificates are automatically managed by Traefik + Let's Encrypt:

1. **Automatic Certificate Issuance:** Traefik requests certificates from Let's Encrypt
2. **Auto-renewal:** Certificates auto-renew before expiration
3. **HTTP to HTTPS Redirect:** All traffic automatically redirected to HTTPS

**Certificate Storage:** `/var/lib/docker/volumes/keystone-gateway_letsencrypt/_data/acme.json`

## 📊 Monitoring & Observability

### Access Points

- **Traefik Dashboard:** `https://traefik.keystone-gateway.dev:8888`
- **Grafana:** `https://grafana.keystone-gateway.dev`
- **Prometheus:** `http://YOUR_SERVER_IP:9090` (internal)

### Default Credentials

- **Grafana:** `admin` / `change-this-password-in-production`
- **Traefik:** No authentication (configure if needed)

### Monitoring Stack

- **Prometheus:** Metrics collection
- **Grafana:** Visualization and dashboards  
- **Traefik:** SSL termination, load balancing, metrics
- **Gateway:** Application metrics via `/metrics` endpoint

## 🛠️ Management Commands

### Deployment Management

```bash
# Deploy/update
./deploy.sh production deploy

# Stop all services
./deploy.sh production stop

# Restart services
./deploy.sh production restart

# View logs
./deploy.sh production logs

# Check status
./deploy.sh production status

# Cleanup old images
./deploy.sh production cleanup

# Create backup
./deploy.sh production backup
```

### Service-Specific Commands

```bash
# View specific service logs
docker-compose -f docker-compose.production.yml logs -f keystone-gateway

# Scale services (if needed)
docker-compose -f docker-compose.production.yml up -d --scale api-backend=3

# Execute commands in containers
docker-compose -f docker-compose.production.yml exec keystone-gateway sh
```

## 🔒 Security Considerations

### Required Security Updates

1. **Change all default passwords** in production configs
2. **Update JWT and session secrets** with strong random values
3. **Configure firewall** to allow only necessary ports (80, 443, 22)
4. **Enable automatic security updates** on your server
5. **Regular backups** of data volumes

### Recommended Security Measures

```bash
# Update server packages
sudo apt update && sudo apt upgrade -y

# Configure UFW firewall
sudo ufw allow ssh
sudo ufw allow http
sudo ufw allow https
sudo ufw enable

# Set up fail2ban for SSH protection
sudo apt install fail2ban
```

## 🔧 Performance Tuning

### Production Optimizations

The production configuration includes:

- **Redis caching** for sessions and application data
- **Connection pooling** for database connections
- **Health checks** for all services
- **Resource limits** and restart policies
- **Log rotation** to prevent disk space issues

### Scaling Considerations

```bash
# Scale API backends
docker-compose -f docker-compose.production.yml up -d --scale api-backend=3

# Scale auth backends  
docker-compose -f docker-compose.production.yml up -d --scale auth-backend=2

# Monitor resource usage
docker stats
```

## 📱 Endpoints After Deployment

| Service | URL | Purpose |
|---------|-----|---------|
| Demo App | `https://demo.keystone-gateway.dev` | Demo application |
| API | `https://api.keystone-gateway.dev` | REST API endpoints |
| Auth | `https://auth.keystone-gateway.dev` | Authentication service |
| Status | `https://status.keystone-gateway.dev` | Status monitoring |
| Grafana | `https://grafana.keystone-gateway.dev` | Monitoring dashboards |
| Traefik | `https://traefik.keystone-gateway.dev:8888` | Proxy dashboard |
| Gateway Admin | `https://demo.keystone-gateway.dev/admin` | Gateway admin panel |

## 🚨 Troubleshooting

### Common Issues

**1. SSL Certificate Issues:**
```bash
# Check certificate status
docker-compose -f docker-compose.production.yml logs traefik

# Force certificate renewal
docker-compose -f docker-compose.production.yml restart traefik
```

**2. Service Not Starting:**
```bash
# Check service logs
docker-compose -f docker-compose.production.yml logs [service-name]

# Check service health
docker-compose -f docker-compose.production.yml ps
```

**3. Domain Not Resolving:**
```bash
# Test DNS resolution
nslookup demo.keystone-gateway.dev

# Test direct connection
curl -I http://YOUR_SERVER_IP:80
```

**4. Performance Issues:**
```bash
# Monitor resource usage
docker stats

# Check gateway metrics
curl http://localhost:8080/metrics
```

### Emergency Procedures

**Rollback Deployment:**
```bash
# Stop current deployment
./deploy.sh production stop

# Restore from backup
# (restore data volumes from backup)

# Deploy previous version
git checkout [previous-tag]
./deploy.sh production deploy
```

**Quick Health Check:**
```bash
# Test all endpoints
curl -f https://demo.keystone-gateway.dev/health
curl -f https://api.keystone-gateway.dev/health  
curl -f https://auth.keystone-gateway.dev/health
```

## 📞 Support

- **Documentation:** See `docs/` directory
- **Issues:** Create GitHub issues for bugs/features
- **Monitoring:** Check Grafana dashboards for service health
- **Logs:** Use `./deploy.sh production logs` for debugging

## 🎯 Next Steps

1. ✅ Deploy to production
2. ✅ Verify all endpoints work with SSL
3. ✅ Set up monitoring alerts in Grafana
4. ✅ Configure automated backups
5. ✅ Set up CI/CD pipeline for automatic deployments
6. ✅ Load test the production deployment
7. ✅ Documentation for your team