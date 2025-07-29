#!/bin/bash
set -e

echo "🚀 Deploying Keystone Gateway to Production"
echo "=========================================="

# Pull latest images
echo "📦 Pulling latest images..."
docker compose -f docker-compose.production.yml pull

# Build application
echo "🔨 Building application..."
docker compose -f docker-compose.production.yml build

# Start services
echo "🌟 Starting services..."
docker compose -f docker-compose.production.yml up -d

# Wait for services
echo "⏳ Waiting for services to be ready..."
sleep 30

# Health check
echo "🏥 Running health checks..."
if curl -f https://keystone-gateway.dev/admin/health; then
    echo "✅ Gateway is healthy"
else
    echo "❌ Gateway health check failed"
    exit 1
fi

if curl -f https://api.keystone-gateway.dev/admin/health; then
    echo "✅ API subdomain is healthy"
else
    echo "❌ API subdomain health check failed"
    exit 1
fi

echo "🎉 Deployment completed successfully!"
echo ""
echo "Available endpoints:"
echo "  • Main site: https://keystone-gateway.dev/admin/health"
echo "  • API: https://api.keystone-gateway.dev/admin/health"
echo "  • Load balancer: https://keystone-gateway.dev/lb/status/200"
echo "  • Monitoring: http://localhost:9090 (server only)"
