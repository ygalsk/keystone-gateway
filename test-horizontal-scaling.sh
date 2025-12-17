#!/bin/bash
# Quick Horizontal Scaling Test
# Tests running multiple gateway instances behind nginx on a single server

set -e

echo "========================================="
echo "Keystone Gateway Horizontal Scaling Test"
echo "========================================="
echo ""

# Check if nginx is installed
if ! command -v nginx &> /dev/null; then
    echo "❌ nginx not found. Install with:"
    echo "   apt-get install nginx -y"
    exit 1
fi

# Build the gateway
echo "📦 Building gateway..."
make build-luajit
echo "✅ Build complete"
echo ""

# Kill any existing instances
echo "🧹 Cleaning up existing instances..."
pkill -f keystone-gateway || true
sleep 2
echo ""

# Start 4 gateway instances on ports 8080-8083
echo "🚀 Starting 4 gateway instances..."
for i in 0 1 2 3; do
    port=$((8080 + i))
    echo "  Starting instance on port $port..."
    ./keystone-gateway -config examples/configs/config-golua.yaml -addr :$port > /tmp/gateway-$port.log 2>&1 &
    sleep 1
done
echo "✅ 4 instances running"
echo ""

# Wait for instances to be ready
echo "⏳ Waiting for instances to be ready..."
sleep 3

# Test each instance individually
echo "🔍 Testing individual instances..."
for i in 0 1 2 3; do
    port=$((8080 + i))
    if curl -s http://localhost:$port/health > /dev/null; then
        echo "  ✅ Instance on port $port: healthy"
    else
        echo "  ❌ Instance on port $port: unhealthy"
    fi
done
echo ""

# Create nginx config
echo "⚙️  Configuring nginx load balancer..."
cat > /tmp/keystone-lb.conf <<'EOF'
upstream keystone_backend {
    least_conn;
    server 127.0.0.1:8080 max_fails=3 fail_timeout=30s;
    server 127.0.0.1:8081 max_fails=3 fail_timeout=30s;
    server 127.0.0.1:8082 max_fails=3 fail_timeout=30s;
    server 127.0.0.1:8083 max_fails=3 fail_timeout=30s;
    keepalive 64;
}

server {
    listen 9090;

    location / {
        proxy_pass http://keystone_backend;
        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_buffering off;
    }

    location /health {
        proxy_pass http://keystone_backend/health;
        proxy_http_version 1.1;
        proxy_set_header Connection "";
    }
}
EOF

# Copy config to nginx
cp /tmp/keystone-lb.conf /etc/nginx/sites-available/keystone-lb.conf
ln -sf /etc/nginx/sites-available/keystone-lb.conf /etc/nginx/sites-enabled/keystone-lb.conf

# Test and reload nginx
nginx -t && systemctl reload nginx
echo "✅ Nginx configured and reloaded"
echo ""

# Benchmark
echo "📊 Running benchmarks..."
echo ""
echo "=== Baseline: Single Instance (port 8080) ==="
wrk -t 4 -c 50 -d 10s http://localhost:8080/api/hello 2>/dev/null | grep -E "Requests/sec|Latency"
echo ""

echo "=== Horizontal: 4 Instances via Load Balancer (port 9090) ==="
wrk -t 16 -c 200 -d 30s --latency http://localhost:9090/api/hello
echo ""

echo "========================================="
echo "Test complete!"
echo ""
echo "📈 To compare:"
echo "  Single instance:  ~82k req/sec"
echo "  4 instances:      ~330k req/sec (expected)"
echo ""
echo "🔍 Check instance logs:"
echo "  tail -f /tmp/gateway-808{0,1,2,3}.log"
echo ""
echo "🧹 Cleanup:"
echo "  pkill -f keystone-gateway"
echo "  rm /etc/nginx/sites-enabled/keystone-lb.conf"
echo "  systemctl reload nginx"
echo "========================================="
