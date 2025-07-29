#!/bin/bash
echo "🧪 Running Local HTTP Load Tests"
echo "================================"

# Install wrk if not present
if ! command -v wrk &> /dev/null; then
    echo "Installing wrk..."
    sudo apt update
    sudo apt install -y wrk
fi

echo ""
echo "🌐 Testing HTTP endpoints for local development..."

echo "Test 1: Health endpoint (HTTP)"
wrk -t4 -c50 -d30s --latency http://localhost/admin/health

echo ""
echo "Test 2: API endpoint (HTTP + Host header routing)"
wrk -t4 -c100 -d30s --latency -H "Host: api.keystone-gateway.dev" http://localhost/admin/health

echo ""
echo "Test 3: Load balancing (HTTP)"
wrk -t4 -c150 -d30s --latency http://localhost/lb/users

echo ""
echo "Test 4: Sustained load (HTTP, 2 minutes)"
wrk -t6 -c200 -d120s --latency http://localhost/admin/health

echo ""
echo "✅ Local HTTP load tests completed!"