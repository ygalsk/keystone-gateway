#!/bin/bash

# Test script for host-based routing implementation
set -e

GATEWAY_PORT=9010
GATEWAY_URL="http://localhost:$GATEWAY_PORT"

echo "🧪 Testing Keystone Gateway Host-Based Routing Implementation"
echo "============================================================"

# Function to test routing
test_route() {
    local description="$1"
    local host_header="$2" 
    local path="$3"
    local expected_to_work="$4"
    
    echo -n "Testing: $description... "
    
    if [ -n "$host_header" ]; then
        response=$(curl -s -H "Host: $host_header" "$GATEWAY_URL$path" | head -1)
    else
        response=$(curl -s "$GATEWAY_URL$path" | head -1)
    fi
    
    if [[ "$response" == *"404"* ]] && [[ "$expected_to_work" == "false" ]]; then
        echo "✅ PASS (correctly returned 404)"
    elif [[ "$response" == "" ]] && [[ "$expected_to_work" == "false" ]]; then
        echo "✅ PASS (correctly rejected - no response)"
    elif [[ "$response" != *"404"* ]] && [[ "$response" != "" ]] && [[ "$expected_to_work" == "true" ]]; then
        echo "✅ PASS (routed successfully)"
    else
        echo "❌ FAIL"
        echo "   Response: $response"
        echo "   Expected to work: $expected_to_work"
    fi
}

echo ""
echo "📍 Testing Legacy Path-Based Routing (backward compatibility):"
test_route "Legacy API (/api/ prefix)" "" "/api/test" "true"
test_route "Legacy API (wrong path)" "" "/wrong/test" "false"

echo ""
echo "🌐 Testing Host-Based Routing:"
test_route "Modern app (app.example.com)" "app.example.com" "/test" "true"
test_route "Modern app (www.app.example.com)" "www.app.example.com" "/test" "true"
test_route "Wrong domain" "wrong.example.com" "/test" "false"

echo ""
echo "🔗 Testing Hybrid Routing (host + path):"
test_route "API v2 (correct host + path)" "api.example.com" "/v2/test" "true"
test_route "API v2 (correct host, wrong path)" "api.example.com" "/v1/test" "false"
test_route "API v2 (wrong host, correct path)" "wrong.example.com" "/v2/test" "false"

echo ""
echo "📊 Testing Routing Priority:"
echo "   Priority 1: Host + Path (hybrid) - highest"
echo "   Priority 2: Host-only"  
echo "   Priority 3: Path-only (legacy) - lowest"

echo ""
echo "✨ All tests completed!"
