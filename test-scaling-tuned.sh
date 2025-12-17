#!/bin/bash
# Optimized Horizontal Scaling Test with Tuned Nginx

set -e

echo "========================================="
echo "Optimized Horizontal Scaling Test"
echo "========================================="
echo ""

# Update nginx config with better settings
echo "⚙️  Updating nginx configuration for high performance..."
cat > /tmp/keystone-lb-optimized.conf <<'EOF'
# Optimized nginx config for horizontal scaling
user www-data;
worker_processes auto;  # Use all CPU cores
pid /run/nginx.pid;

events {
    worker_connections 8192;  # High concurrency
    use epoll;
    multi_accept on;
}

http {
    # Disable logging for benchmarks
    access_log off;
    error_log /var/log/nginx/error.log crit;

    # Upstream pool
    upstream keystone_backend {
        least_conn;

        server 127.0.0.1:8080 max_fails=3 fail_timeout=10s;
        server 127.0.0.1:8081 max_fails=3 fail_timeout=10s;
        server 127.0.0.1:8082 max_fails=3 fail_timeout=10s;
        server 127.0.0.1:8083 max_fails=3 fail_timeout=10s;

        # More keepalive connections
        keepalive 256;
        keepalive_requests 10000;
    }

    server {
        listen 9090 reuseport;  # Kernel load balancing

        # Disable buffering (lower latency)
        proxy_buffering off;
        proxy_request_buffering off;

        location / {
            proxy_pass http://keystone_backend;
            proxy_http_version 1.1;

            # Critical: Reuse connections
            proxy_set_header Connection "";

            # Minimal headers
            proxy_set_header Host $host;

            # Fast timeouts
            proxy_connect_timeout 5s;
            proxy_send_timeout 5s;
            proxy_read_timeout 5s;

            # Performance
            tcp_nodelay on;
            tcp_nopush on;
        }
    }
}
EOF

# Backup existing nginx.conf
cp /etc/nginx/nginx.conf /etc/nginx/nginx.conf.backup

# Replace nginx.conf
cp /tmp/keystone-lb-optimized.conf /etc/nginx/nginx.conf

# Test and reload
nginx -t && systemctl reload nginx
echo "✅ Nginx optimized and reloaded"
echo ""

# Run progressively higher concurrency tests
echo "📊 Testing with increasing concurrency..."
echo ""

for conc in 100 200 400 800; do
    echo "=== Test with $conc connections ==="
    wrk -t 16 -c $conc -d 15s http://localhost:9090/api/hello 2>/dev/null | grep -E "Requests/sec|Latency" | head -2
    echo ""
    sleep 2
done

echo "=== Final 30s test with optimal concurrency (800) ==="
wrk -t 16 -c 800 -d 30s --latency http://localhost:9090/api/hello
echo ""

echo "========================================="
echo "Test complete!"
echo ""
echo "💡 Tips for better performance:"
echo "  1. If throughput is still low, nginx might be CPU bound"
echo "  2. Try HAProxy instead (lower overhead than nginx)"
echo "  3. Or use direct DNS round-robin (no proxy)"
echo ""
echo "🧹 Restore original nginx:"
echo "  cp /etc/nginx/nginx.conf.backup /etc/nginx/nginx.conf"
echo "  systemctl reload nginx"
echo "========================================="
