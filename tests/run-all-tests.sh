#!/bin/sh
echo "🧪 Keystone Gateway Load Tests"
echo "=============================="

mkdir -p /results

# Test 1: Health Check Performance
echo "Test 1: Health endpoint"
wrk -t2 -c10 -d30s --latency http://keystone-gateway:8080/admin/health > /results/health.txt

# Test 2: API Performance
echo "Test 2: API endpoints"
wrk -t4 -c50 -d30s --latency http://nginx/api/time > /results/api.txt

# Test 3: Load Balancing
echo "Test 3: Load balancing"
wrk -t4 -c100 -d30s --latency http://nginx/lb/status/200 > /results/loadbalancing.txt

# Test 4: Stress Test
echo "Test 4: Stress test"
wrk -t8 -c200 -d30s --latency http://nginx/api/fast > /results/stress.txt

echo "✅ All tests completed!"

# Generate summary
echo "Performance Summary:" > /results/summary.txt
echo "==================" >> /results/summary.txt
grep "Requests/sec" /results/*.txt >> /results/summary.txt
echo "" >> /results/summary.txt
echo "Latency Summary:" >> /results/summary.txt
grep "Latency" /results/*.txt >> /results/summary.txt

cat /results/summary.txt