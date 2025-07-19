# Deployment Guide

**Production deployment options for Keystone Gateway**

## Quick Production Deployment

```bash
# Build and deploy with Docker
make deploy-prod

# Or manual steps
make build
make docker
docker-compose -f docker-compose.simple.yml up -d
```

## Deployment Options

### Option 1: Single Binary (Recommended)
```bash
# Build static binary
make build

# Copy to server
scp keystone-gateway user@server:/usr/local/bin/
scp configs/production-simple.yaml user@server:/etc/keystone/config.yaml

# Run with systemd (see systemd section below)
systemctl start keystone-gateway
```

### Option 2: Docker Compose
```bash
# Use simplified production setup
docker-compose -f docker-compose.simple.yml up -d

# Or core services only
docker-compose -f docker-compose.core.yml up -d
```

### Option 3: Docker Swarm
```bash
# Initialize swarm
docker swarm init

# Deploy stack
docker stack deploy -c docker-compose.simple.yml keystone
```

## Production Configuration

### Environment Variables
```bash
export KEYSTONE_CONFIG="/etc/keystone/config.yaml"
export KEYSTONE_ADDR=":8080"
export KEYSTONE_LOG_LEVEL="info"
```

### Production config.yaml
```yaml
admin_base_path: "/admin"

tenants:
  - name: production-app
    domains: 
      - yourdomain.com
      - www.yourdomain.com
    health_interval: 30
    services:
      - name: app-primary
        url: http://app1.internal:8080
        health: /health
      - name: app-secondary
        url: http://app2.internal:8080
        health: /health

  - name: api
    domains:
      - api.yourdomain.com
    health_interval: 15
    services:
      - name: api-server
        url: http://api.internal:3000
        health: /status
```

## Systemd Service (Linux)

Create `/etc/systemd/system/keystone-gateway.service`:

```ini
[Unit]
Description=Keystone Gateway
After=network.target
Wants=network-online.target

[Service]
Type=simple
User=keystone
Group=keystone
ExecStart=/usr/local/bin/keystone-gateway -config /etc/keystone/config.yaml -addr :8080
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/log/keystone

[Install]
WantedBy=multi-user.target
```

Setup:
```bash
# Create user
sudo useradd -r -s /bin/false keystone

# Create directories
sudo mkdir -p /etc/keystone /var/log/keystone
sudo chown keystone:keystone /var/log/keystone

# Install and start
sudo systemctl daemon-reload
sudo systemctl enable keystone-gateway
sudo systemctl start keystone-gateway
```

## Reverse Proxy Setup

### Nginx (Recommended)
```nginx
upstream keystone {
    server 127.0.0.1:8080;
    # Add more instances for HA
    # server 127.0.0.1:8081;
}

server {
    listen 80;
    server_name yourdomain.com *.yourdomain.com;
    
    # Redirect to HTTPS
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name yourdomain.com *.yourdomain.com;
    
    # SSL configuration
    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;
    
    # Proxy to Keystone Gateway
    location / {
        proxy_pass http://keystone;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # Health check timeout
        proxy_connect_timeout 5s;
        proxy_send_timeout 10s;
        proxy_read_timeout 10s;
    }
    
    # Admin endpoints (restrict access)
    location /admin/ {
        allow 10.0.0.0/8;
        allow 172.16.0.0/12;
        allow 192.168.0.0/16;
        deny all;
        
        proxy_pass http://keystone;
        proxy_set_header Host $host;
    }
}
```

### Traefik
```yaml
# docker-compose with Traefik
version: '3.8'
services:
  keystone-gateway:
    image: keystone-gateway:latest
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.keystone.rule=Host(`yourdomain.com`) || Host(`api.yourdomain.com`)"
      - "traefik.http.routers.keystone.entrypoints=websecure"
      - "traefik.http.routers.keystone.tls.certresolver=letsencrypt"
      - "traefik.http.services.keystone.loadbalancer.server.port=8080"
```

## High Availability

### Multiple Gateway Instances
```bash
# Run multiple instances
./keystone-gateway -config config.yaml -addr :8080 &
./keystone-gateway -config config.yaml -addr :8081 &
./keystone-gateway -config config.yaml -addr :8082 &

# Load balance with nginx/haproxy
```

### Health Monitoring
```bash
# Health check script
#!/bin/bash
if ! curl -f http://localhost:8080/admin/health; then
    echo "Gateway unhealthy, restarting..."
    systemctl restart keystone-gateway
fi
```

### Monitoring Integration
```yaml
# Prometheus monitoring
version: '3.8'
services:
  keystone-gateway:
    # ... gateway config
    
  prometheus:
    image: prom/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--web.enable-lifecycle'
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
```

## Security Considerations

### Network Security
```bash
# Firewall rules (iptables)
iptables -A INPUT -p tcp --dport 8080 -s 10.0.0.0/8 -j ACCEPT
iptables -A INPUT -p tcp --dport 8080 -j DROP
```

### Admin Endpoint Protection
- Restrict `/admin/` endpoints to internal networks
- Use authentication proxy if external access needed
- Monitor admin endpoint access

### SSL/TLS Termination
- Use nginx/traefik for SSL termination
- Enable HTTP/2 for better performance
- Use modern TLS configurations

## Performance Tuning

### OS-level Tuning
```bash
# Increase file descriptor limits
echo "keystone soft nofile 65536" >> /etc/security/limits.conf
echo "keystone hard nofile 65536" >> /etc/security/limits.conf

# Network tuning
echo "net.core.somaxconn = 65536" >> /etc/sysctl.conf
echo "net.ipv4.tcp_max_syn_backlog = 65536" >> /etc/sysctl.conf
sysctl -p
```

### Application Tuning
- Adjust health check intervals based on load
- Monitor memory usage and adjust if needed
- Use multiple gateway instances for high traffic

## Deployment Checklist

### Pre-deployment
- [ ] Test configuration locally: `make test`
- [ ] Verify all backend services are accessible
- [ ] Check SSL certificates and domains
- [ ] Review security settings

### Deployment
- [ ] Deploy with rolling updates (zero downtime)
- [ ] Verify health endpoints respond correctly
- [ ] Test routing for all configured domains/paths
- [ ] Monitor logs for errors

### Post-deployment
- [ ] Set up monitoring and alerting
- [ ] Configure log rotation
- [ ] Document rollback procedures
- [ ] Schedule regular health checks

## Troubleshooting

**Deployment fails?**
- Check binary permissions and paths
- Verify configuration file syntax
- Review systemd/docker logs

**Gateway not accessible?**
- Verify firewall rules and port bindings
- Check reverse proxy configuration
- Test direct access to gateway port

**Performance issues?**
- Monitor resource usage (CPU, memory, network)
- Check backend response times
- Review health check intervals

Need help? See **[Troubleshooting Guide](troubleshooting.md)** for detailed solutions.