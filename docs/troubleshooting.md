# Troubleshooting Guide

**Common issues and solutions for Keystone Gateway**

## Quick Diagnostics

```bash
# Check gateway status
make status

# View logs
make logs

# Test health endpoint
curl http://localhost:8080/admin/health

# Test configuration
make test
```

## Common Issues

### Gateway Won't Start

**Error:** `listen tcp :8080: bind: address already in use`

**Solution:**
```bash
# Find what's using the port
lsof -i :8080
sudo netstat -tulpn | grep :8080

# Kill the process or change port
pkill -f keystone-gateway
# Or start on different port
./keystone-gateway -addr :8081
```

**Error:** `Failed to load config: no such file or directory`

**Solution:**
```bash
# Check config file exists
ls -la configs/config.yaml

# Use correct path
./keystone-gateway -config configs/config.yaml

# Or create config from template
cp configs/config.yaml.example configs/config.yaml
```

### Routing Issues

**Problem:** Requests return 404 "No route found"

**Diagnosis:**
```bash
# Check tenant configuration
curl http://localhost:8080/admin/tenants

# Test with exact host header
curl -H "Host: yourdomain.com" http://localhost:8080/

# Check path prefix matching
curl -v http://localhost:8080/api/v1/test
```

**Solutions:**
1. **Host-based routing**: Ensure `Host` header matches `domains` in config
2. **Path-based routing**: Verify URL starts with exact `path_prefix`
3. **Case sensitivity**: Domains and paths are case-sensitive
4. **Trailing slashes**: `/api/v1` ≠ `/api/v1/`

### Backend Connection Issues

**Problem:** 502 Bad Gateway or "All backends unavailable"

**Diagnosis:**
```bash
# Check backend health
curl http://localhost:8080/admin/health

# Test backend directly
curl http://backend-server:3000/health

# Check gateway logs
make logs | grep ERROR
```

**Solutions:**
1. **Backend down**: Start/restart backend services
2. **Wrong URL**: Verify backend URLs in config are correct
3. **Network issues**: Check connectivity between gateway and backends
4. **Health endpoint**: Ensure backend health endpoints return < 400 status

### Health Check Failures

**Problem:** Backends showing as unhealthy

**Diagnosis:**
```bash
# Test health endpoint directly
curl -v http://backend:3000/health

# Check response code and time
curl -w "Status: %{http_code}, Time: %{time_total}s\n" \
     http://backend:3000/health

# Review gateway logs
journalctl -u keystone-gateway -f
```

**Common Causes:**
1. **Wrong health path**: Check `health` field in config
2. **Slow response**: Health check timeout (3s default)
3. **Authentication**: Health endpoints shouldn't require auth
4. **Dependencies**: Backend health check includes failing dependencies

### Performance Issues

**Problem:** Slow response times

**Diagnosis:**
```bash
# Test gateway directly
time curl http://localhost:8080/admin/health

# Test backend directly
time curl http://backend:3000/health

# Check resource usage
top -p $(pgrep keystone-gateway)
```

**Solutions:**
1. **Backend performance**: Optimize backend services
2. **Health check frequency**: Increase `health_interval` if too aggressive
3. **Resource limits**: Ensure adequate CPU/memory
4. **Network latency**: Check network between gateway and backends

### Docker Issues

**Problem:** Container won't start or crashes

**Diagnosis:**
```bash
# Check container logs
docker logs keystone-gateway

# Check container status
docker ps -a

# Test configuration
docker run --rm -v $(pwd)/configs:/app/configs keystone-gateway:latest -config /app/configs/config.yaml -help
```

**Solutions:**
1. **Volume mounts**: Ensure config file is mounted correctly
2. **Network connectivity**: Check Docker network configuration
3. **Port conflicts**: Verify port mapping doesn't conflict
4. **Image build**: Rebuild image if source changed

### SSL/TLS Issues

**Problem:** HTTPS not working with reverse proxy

**Check nginx/traefik configuration:**
```bash
# Test SSL termination
curl -v https://yourdomain.com

# Check certificate
openssl s_client -connect yourdomain.com:443 -servername yourdomain.com

# Verify proxy headers
curl -H "Host: yourdomain.com" http://localhost:8080/ -H "X-Forwarded-Proto: https"
```

## Debugging Techniques

### Enable Debug Logging

Set environment variable:
```bash
export LOG_LEVEL=debug
./keystone-gateway -config config.yaml
```

### Request Tracing

Add unique request ID for tracing:
```bash
curl -H "X-Request-ID: test-123" \
     -H "Host: yourdomain.com" \
     http://localhost:8080/api/test
```

### Configuration Validation

```bash
# Test configuration syntax
make test

# Dry-run configuration
./keystone-gateway -config config.yaml -test-config

# Validate YAML syntax
python -c "import yaml; yaml.safe_load(open('configs/config.yaml'))"
```

### Network Debugging

```bash
# Check connectivity
nc -zv backend-host 3000

# Trace route
traceroute backend-host

# DNS resolution
nslookup backend-host
dig backend-host
```

## Log Analysis

### Common Log Patterns

**Successful request:**
```
"GET /api/users HTTP/1.1" from 192.168.1.100 - 200 1.2KB in 45ms
```

**Health check failure:**
```
Health check failed for service api-primary: connection refused
```

**Backend unavailable:**
```
All backends unavailable for tenant 'api', using fallback
```

**Configuration error:**
```
Invalid tenant configuration: missing 'name' field
```

### Log Analysis Tools

```bash
# Filter errors only
make logs | grep ERROR

# Count requests by status
make logs | grep -E '"[A-Z]+ .* HTTP/1.1"' | awk '{print $6}' | sort | uniq -c

# Average response times
make logs | grep -E '"[A-Z]+ .* HTTP/1.1"' | grep -o 'in [0-9.]*ms' | awk '{sum+=$2; count++} END {print sum/count "ms average"}'
```

## Performance Monitoring

### Resource Usage

```bash
# Gateway memory usage
ps -o pid,rss,cmd -p $(pgrep keystone-gateway)

# System resources
free -h
df -h
iostat 1 5
```

### Request Metrics

```bash
# Response time monitoring
while true; do
  time curl -s http://localhost:8080/admin/health > /dev/null
  sleep 1
done

# Load testing (if ab available)
ab -n 1000 -c 10 http://localhost:8080/admin/health
```

## Getting Help

### Information to Include

When reporting issues, include:

1. **Gateway version**: `./keystone-gateway -version`
2. **Configuration file**: (sanitized, remove sensitive data)
3. **Error logs**: Recent error messages
4. **Environment**: OS, Docker version, network setup
5. **Steps to reproduce**: Exact commands and expected vs actual behavior

### Useful Commands

```bash
# Collect diagnostic info
cat > debug-info.txt << EOF
Gateway Version: $(./keystone-gateway -version 2>&1)
OS: $(uname -a)
Docker: $(docker --version 2>&1)
Config file: configs/config.yaml
Recent logs: (see attached)
EOF

# Save recent logs
make logs --tail=100 > recent-logs.txt
```

### Support Channels

- **GitHub Issues**: For bugs and feature requests
- **GitHub Discussions**: For questions and community help
- **Documentation**: Check all docs in `/docs` directory

### Emergency Procedures

**Complete service failure:**
```bash
# Quick restart
make stop && make deploy-prod

# Rollback to previous version
docker tag keystone-gateway:previous keystone-gateway:latest
make deploy-prod

# Manual failover
# Point DNS/load balancer directly to backends
```

**Partial service failure:**
```bash
# Identify failing tenant
curl http://localhost:8080/admin/health

# Temporarily disable failing tenant
# (edit config, remove tenant, restart)

# Test remaining services
curl -H "Host: working-service.com" http://localhost:8080/
```