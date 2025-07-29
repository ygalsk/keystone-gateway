#!/bin/bash
echo "🧪 Running Real-World Load Tests"
echo "================================"

# Install wrk if not present
if ! command -v wrk &> /dev/null; then
    echo "Installing wrk..."
    apt update
    apt install -y build-essential libssl-dev git
    sudo apt install wrk
fi

echo ""
echo "🌐 Testing HTTPS endpoints with SSL overhead..."

echo "Test 1: Health endpoint (HTTPS)"
wrk -t4 -c50 -d30s --latency https://keystone-gateway.dev/admin/health

echo ""
echo "Test 2: API endpoint (HTTPS + subdomain)"
wrk -t4 -c100 -d30s --latency https://api.keystone-gateway.dev/admin/health

echo ""
echo "Test 3: Load balancing (HTTPS)"
wrk -t4 -c150 -d30s --latency https://keystone-gateway.dev/lb/status/200

echo ""
echo "Test 4: Sustained load (HTTPS, 2 minutes)"
wrk -t6 -c200 -d120s --latency https://api.keystone-gateway.dev/admin/health

echo ""
echo "✅ Real-world load tests completed!"
