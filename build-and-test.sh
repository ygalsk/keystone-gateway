#!/bin/bash
# build-and-test.sh - Build Keystone Gateway and run comprehensive performance tests

set -e

# Configuration
GATEWAY_BINARY="./keystone-gateway"
CONFIG_FILE="configs/production-test.yaml"
GATEWAY_PORT=8080
TEST_DURATION=60
RESULTS_DIR="test-results"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

echo "🚀 Keystone Gateway - Build & Performance Test Suite"
echo "==================================================="
echo "Timestamp: $(date)"
echo "Gateway Port: $GATEWAY_PORT"
echo "Config: $CONFIG_FILE"
echo ""

# Create results directory
mkdir -p "$RESULTS_DIR"
RESULTS_FILE="$RESULTS_DIR/performance_test_$TIMESTAMP.json"
LOG_FILE="$RESULTS_DIR/gateway_$TIMESTAMP.log"

# Function to log with timestamp
log() {
    echo -e "${CYAN}[$(date +'%H:%M:%S')]${NC} $1"
}

# Function to check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Prerequisites check
log "🔍 Checking prerequisites..."
MISSING_DEPS=false

if ! command_exists go; then
    echo -e "${RED}❌ Go is not installed${NC}"
    MISSING_DEPS=true
fi

if ! command_exists docker; then
    echo -e "${RED}❌ Docker is not installed${NC}"
    MISSING_DEPS=true
fi

if ! command_exists docker-compose; then
    echo -e "${RED}❌ Docker Compose is not installed${NC}"
    MISSING_DEPS=true
fi

if ! command_exists ab; then
    echo -e "${RED}❌ Apache Bench (ab) is not installed${NC}"
    echo "Install with: sudo apt-get install apache2-utils"
    MISSING_DEPS=true
fi

if ! command_exists curl; then
    echo -e "${RED}❌ curl is not installed${NC}"
    MISSING_DEPS=true
fi

if [ "$MISSING_DEPS" = true ]; then
    echo -e "${RED}Please install missing dependencies before continuing.${NC}"
    exit 1
fi

echo -e "${GREEN}✅ All prerequisites satisfied${NC}"

# Check if running as root (not recommended)
if [ "$EUID" -eq 0 ]; then
    echo -e "${YELLOW}⚠️  Running as root - consider using a non-root user${NC}"
fi

# Function to cleanup on exit
cleanup() {
    log "🧹 Cleaning up..."
    
    # Stop gateway if running
    if [ -n "$GATEWAY_PID" ] && kill -0 "$GATEWAY_PID" 2>/dev/null; then
        log "Stopping gateway (PID: $GATEWAY_PID)"
        kill -TERM "$GATEWAY_PID" 2>/dev/null || true
        sleep 3
        kill -KILL "$GATEWAY_PID" 2>/dev/null || true
    fi
    
    # Stop Docker services
    log "Stopping Docker services..."
    docker-compose -f docker-compose.backends.yml down --remove-orphans --timeout 10 2>/dev/null || true
    
    log "Cleanup completed"
}

trap cleanup EXIT INT TERM

# Build the gateway
log "🔨 Building Keystone Gateway..."
if [ -f "go.mod" ]; then
    go mod tidy
    go build -o "$GATEWAY_BINARY" -ldflags "-X main.version=1.2.1" .
    echo -e "${GREEN}✅ Gateway built successfully${NC}"
else
    echo -e "${RED}❌ go.mod not found. Make sure you're in the project root directory.${NC}"
    exit 1
fi

# Make sure config file exists
if [ ! -f "$CONFIG_FILE" ]; then
    echo -e "${RED}❌ Config file not found: $CONFIG_FILE${NC}"
    echo "Please create the configuration file first."
    exit 1
fi

# Setup mock backends if not done already
if [ ! -d "mock-backends" ]; then
    log "📁 Setting up mock backends..."
    if [ -f "setup-backends.sh" ]; then
        chmod +x setup-backends.sh
        ./setup-backends.sh
    else
        echo -e "${YELLOW}⚠️  setup-backends.sh not found - please run it first${NC}"
        exit 1
    fi
fi

# Start Docker services
log "🐳 Starting Docker backend services..."
docker-compose -f docker-compose.backends.yml down --remove-orphans 2>/dev/null || true
docker-compose -f docker-compose.backends.yml up -d --build --force-recreate

# Wait for services to be ready
log "⏳ Waiting for backend services to initialize..."
services_ready() {
    local ready=true
    local services=("localhost:3001" "localhost:3002" "localhost:3003" "localhost:3004")
    
    for service in "${services[@]}"; do
        if ! curl -s -f "http://$service/health" >/dev/null 2>&1; then
            ready=false
            break
        fi
    done
    
    [ "$ready" = true ]
}

# Wait up to 60 seconds for services
for i in {1..60}; do
    if services_ready; then
        echo -e "${GREEN}✅ All backend services are ready${NC}"
        break
    fi
    if [ $i -eq 60 ]; then
        echo -e "${RED}❌ Backend services failed to start within 60 seconds${NC}"
        echo "Checking service status..."
        docker-compose ps
        exit 1
    fi
    echo -n "."
    sleep 1
done

# Check port availability
if lsof -Pi :$GATEWAY_PORT -sTCP:LISTEN -t >/dev/null 2>&1; then
    echo -e "${RED}❌ Port $GATEWAY_PORT is already in use${NC}"
    echo "Please stop the service using this port or change GATEWAY_PORT"
    exit 1
fi

# Start the gateway
log "🚀 Starting Keystone Gateway..."
if ./"$GATEWAY_BINARY" -config "$CONFIG_FILE" -addr ":$GATEWAY_PORT" > "$LOG_FILE" 2>&1 &
then
    GATEWAY_PID=$!
    log "Gateway started with PID: $GATEWAY_PID"
    
    # Wait for gateway to be ready
    log "⏳ Waiting for gateway to initialize..."
    for i in {1..30}; do
        if curl -s -f "http://localhost:$GATEWAY_PORT/admin/health" >/dev/null 2>&1; then
            echo -e "${GREEN}✅ Gateway is ready and responding${NC}"
            break
        fi
        if [ $i -eq 30 ]; then
            echo -e "${RED}❌ Gateway failed to start within 30 seconds${NC}"
            echo "Gateway log:"
            tail -20 "$LOG_FILE"
            exit 1
        fi
        sleep 1
    done
else
    echo -e "${RED}❌ Failed to start gateway${NC}"
    exit 1
fi

# Initialize results file
cat > "$RESULTS_FILE" << EOF
{
  "test_run": {
    "timestamp": "$(date -Iseconds)",
    "gateway_version": "1.2.1",
    "test_duration": $TEST_DURATION,
    "config_file": "$CONFIG_FILE"
  },
  "tests": []
}
EOF

# Function to run performance test
run_perf_test() {
    local test_name="$1"
    local url="$2"
    local host_header="$3"
    local requests="$4"
    local concurrency="$5"
    local description="$6"
    local test_type="$7"
    
    log "🔧 Running: $test_name"
    echo "   URL: $url"
    echo "   Description: $description"
    if [ -n "$host_header" ]; then
        echo "   Host Header: $host_header"
    fi
    echo "   Load: $requests requests, $concurrency concurrent"
    echo ""
    
    # Test connectivity first
    local connectivity_check
    if [ -n "$host_header" ]; then
        connectivity_check=$(curl -s -o /dev/null -w "%{http_code}" -H "Host: $host_header" "$url" 2>/dev/null || echo "000")
    else
        connectivity_check=$(curl -s -o /dev/null -w "%{http_code}" "$url" 2>/dev/null || echo "000")
    fi
    
    if [ "$connectivity_check" = "000" ] || [ "${connectivity_check:0:1}" = "5" ]; then
        echo -e "   ${RED}❌ CONNECTIVITY FAILED${NC} - HTTP $connectivity_check"
        # Add failed test to results
        jq --argjson test '{
            "name": "'$test_name'",
            "type": "'$test_type'",
            "status": "failed",
            "error": "connectivity_failed",
            "http_code": "'$connectivity_check'"
        }' '.tests += [$test]' "$RESULTS_FILE" > "${RESULTS_FILE}.tmp" && mv "${RESULTS_FILE}.tmp" "$RESULTS_FILE"
        return
    fi
    
    echo -e "   ${GREEN}✅ Connectivity OK${NC} (HTTP $connectivity_check)"
    
    # Build ab command
    local ab_cmd="ab -n $requests -c $concurrency -g /tmp/ab_data_$$.tsv -q -k"
    if [ -n "$host_header" ]; then
        ab_cmd="$ab_cmd -H 'Host: $host_header'"
    fi
    ab_cmd="$ab_cmd '$url'"
    
    # Run performance test
    echo "   🚀 Running performance test..."
    local ab_output
    if ab_output=$(timeout 120s bash -c "$ab_cmd" 2>&1); then
        # Parse results
        local rps=$(echo "$ab_output" | grep "Requests per second" | awk '{print $4}' | head -1)
        local time_per_req=$(echo "$ab_output" | grep "Time per request:" | head -1 | awk '{print $4}')
        local failed=$(echo "$ab_output" | grep "Failed requests:" | awk '{print $3}' | head -1)
        local transfer_rate=$(echo "$ab_output" | grep "Transfer rate:" | awk '{print $3}' | head -1)
        local total_time=$(echo "$ab_output" | grep "Time taken for tests:" | awk '{print $5}')
        
        # Handle empty values
        rps=${rps:-0}
        time_per_req=${time_per_req:-0}
        failed=${failed:-0}
        transfer_rate=${transfer_rate:-0}
        total_time=${total_time:-0}
        
        # Extract percentile data
        local p50=$(echo "$ab_output" | grep "50%" | awk '{print $2}' | head -1)
        local p95=$(echo "$ab_output" | grep "95%" | awk '{print $2}' | head -1)
        local p99=$(echo "$ab_output" | grep "99%" | awk '{print $2}' | head -1)
        
        # Display results
        echo -e "   📊 Requests/sec: ${GREEN}$rps${NC}"
        echo -e "   ⏱️  Time/request: ${BLUE}${time_per_req}ms${NC}"
        echo -e "   📈 Transfer rate: ${YELLOW}$transfer_rate KB/sec${NC}"
        echo -e "   ❌ Failed requests: $failed"
        echo -e "   ⏰ Total time: ${CYAN}${total_time}s${NC}"
        if [ -n "$p50" ] && [ "$p50" != "0" ]; then
            echo -e "   📊 Latency P50/P95/P99: ${PURPLE}${p50}/${p95}/${p99}ms${NC}"
        fi
        
        # Performance assessment
        local rps_int=$(printf "%.0f" "$rps" 2>/dev/null || echo "0")
        local status="good"
        local assessment=""
        
        if [ "$test_type" = "local" ]; then
            if [ "$rps_int" -gt 5000 ]; then
                assessment="🚀 ${GREEN}EXCEPTIONAL${NC} - Outstanding local performance"
                status="excellent"
            elif [ "$rps_int" -gt 2000 ]; then
                assessment="✅ ${GREEN}EXCELLENT${NC} - Great local performance"
                status="excellent"
            elif [ "$rps_int" -gt 500 ]; then
                assessment="✅ ${GREEN}GOOD${NC} - Solid local performance"
                status="good"
            elif [ "$rps_int" -gt 0 ]; then
                assessment="⚠️  ${YELLOW}CHECK BACKENDS${NC} - Local performance issue"
                status="warning"
            else
                assessment="❌ ${RED}FAILED${NC} - Backend not responding"
                status="failed"
            fi
        else
            if [ "$rps_int" -gt 1000 ]; then
                assessment="🚀 ${GREEN}OUTSTANDING${NC} - Production-ready performance"
                status="excellent"
            elif [ "$rps_int" -gt 500 ]; then
                assessment="✅ ${GREEN}EXCELLENT${NC} - Great production performance"
                status="excellent"
            elif [ "$rps_int" -gt 200 ]; then
                assessment="✅ ${GREEN}GOOD${NC} - Solid production performance"
                status="good"
            elif [ "$rps_int" -gt 0 ]; then
                assessment="⚠️  ${YELLOW}INVESTIGATE${NC} - Production performance concern"
                status="warning"
            else
                assessment="❌ ${RED}FAILED${NC} - No response from production"
                status="failed"
            fi
        fi
        
        echo -e "   $assessment"
        
        # Add to results file
        jq --argjson test '{
            "name": "'$test_name'",
            "type": "'$test_type'",
            "url": "'$url'",
            "host_header": "'$host_header'",
            "description": "'$description'",
            "status": "'$status'",
            "metrics": {
                "requests_per_second": '$rps',
                "time_per_request_ms": '$time_per_req',
                "failed_requests": '$failed',
                "transfer_rate_kb_sec": '$transfer_rate',
                "total_time_seconds": '$total_time',
                "percentiles": {
                    "p50": "'$p50'",
                    "p95": "'$p95'",
                    "p99": "'$p99'"
                }
            },
            "load": {
                "total_requests": '$requests',
                "concurrency": '$concurrency'
            }
        }' '.tests += [$test]' "$RESULTS_FILE" > "${RESULTS_FILE}.tmp" && mv "${RESULTS_FILE}.tmp" "$RESULTS_FILE"
        
    else
        echo -e "   ${RED}❌ TEST FAILED${NC} - Timeout or error"
        # Add failed test to results
        jq --argjson test '{
            "name": "'$test_name'",
            "type": "'$test_type'",
            "status": "failed",
            "error": "timeout_or_error"
        }' '.tests += [$test]' "$RESULTS_FILE" > "${RESULTS_FILE}.tmp" && mv "${RESULTS_FILE}.tmp" "$RESULTS_FILE"
    fi
    
    # Clean up ab data file
    rm -f /tmp/ab_data_$$.tsv
    echo ""
}

# Start performance testing
log "📊 Starting Performance Test Suite..."
echo "===================================="

# Test 1: Gateway health check (baseline)
run_perf_test \
    "Gateway Health Check" \
    "http://localhost:$GATEWAY_PORT/admin/health" \
    "" \
    500 \
    10 \
    "Gateway admin health endpoint baseline test" \
    "local"

# Test 2: Demo service via gateway (host-based routing)
run_perf_test \
    "Demo Service (Host Routing)" \
    "http://localhost:$GATEWAY_PORT/" \
    "demo.keystone-gateway.dev" \
    2000 \
    50 \
    "Demo service through gateway with host-based routing" \
    "local"

# Test 3: API service via gateway (host-based routing)
run_perf_test \
    "API Service (Host Routing)" \
    "http://localhost:$GATEWAY_PORT/users" \
    "api.keystone-gateway.dev" \
    3000 \
    75 \
    "API service through gateway with host-based routing" \
    "local"

# Test 4: Auth service via gateway (host-based routing)
run_perf_test \
    "Auth Service (Host Routing)" \
    "http://localhost:$GATEWAY_PORT/login" \
    "auth.keystone-gateway.dev" \
    1500 \
    25 \
    "Auth service through gateway with host-based routing" \
    "local"

# Test 5: Status service via gateway (host-based routing)
run_perf_test \
    "Status Service (Host Routing)" \
    "http://localhost:$GATEWAY_PORT/" \
    "status.keystone-gateway.dev" \
    1000 \
    20 \
    "Status service through gateway with host-based routing" \
    "local"

# Test 6: Grafana service via gateway (host-based routing)
run_perf_test \
    "Grafana Service (Host Routing)" \
    "http://localhost:$GATEWAY_PORT/" \
    "grafana.keystone-gateway.dev" \
    500 \
    10 \
    "Grafana service through gateway with host-based routing" \
    "local"

# Test 7: API v1 (path-based routing)
run_perf_test \
    "API v1 (Path Routing)" \
    "http://localhost:$GATEWAY_PORT/api/v1/users" \
    "" \
    2000 \
    50 \
    "API v1 through gateway with path-based routing" \
    "local"

# Test 8: API v2 (path-based routing)
run_perf_test \
    "API v2 (Path Routing)" \
    "http://localhost:$GATEWAY_PORT/api/v2/users" \
    "" \
    2000 \
    50 \
    "API v2 through gateway with path-based routing" \
    "local"

# Test 9: Hybrid routing (host + path)
run_perf_test \
    "API Admin (Hybrid Routing)" \
    "http://localhost:$GATEWAY_PORT/admin/stats" \
    "api.keystone-gateway.dev" \
    1000 \
    25 \
    "API admin through gateway with hybrid routing (host + path)" \
    "local"

# Test 10: High concurrency test
run_perf_test \
    "High Concurrency Test" \
    "http://localhost:$GATEWAY_PORT/users" \
    "api.keystone-gateway.dev" \
    5000 \
    200 \
    "High concurrency test to check gateway scalability" \
    "local"

# Test direct backend performance for comparison
log "🏠 Testing Direct Backend Performance (for comparison)..."

# Direct API backend test
run_perf_test \
    "Direct API Backend" \
    "http://localhost:3002/users" \
    "" \
    3000 \
    75 \
    "Direct connection to API backend (bypassing gateway)" \
    "direct"

# Direct demo backend test
run_perf_test \
    "Direct Demo Backend" \
    "http://localhost:3001/" \
    "" \
    2000 \
    50 \
    "Direct connection to demo backend (bypassing gateway)" \
    "direct"

# Test production domains if reachable (light load only)
log "🌍 Testing Production Domains (if reachable)..."

# Check if production domains are reachable
if timeout 5s curl -s -f "https://demo.keystone-gateway.dev/" >/dev/null 2>&1; then
    log "Production domains are reachable - running light tests"
    
    run_perf_test \
        "Production Demo Service" \
        "https://demo.keystone-gateway.dev/" \
        "" \
        100 \
        5 \
        "Production demo service (light load)" \
        "production"
        
    run_perf_test \
        "Production API Service" \
        "https://api.keystone-gateway.dev/" \
        "" \
        100 \
        5 \
        "Production API service (light load)" \
        "production"
else
    log "Production domains not reachable - skipping production tests"
    jq --argjson test '{
        "name": "Production Tests",
        "type": "production",
        "status": "skipped",
        "reason": "domains_not_reachable"
    }' '.tests += [$test]' "$RESULTS_FILE" > "${RESULTS_FILE}.tmp" && mv "${RESULTS_FILE}.tmp" "$RESULTS_FILE"
fi

# Generate comprehensive report
log "📊 Generating Performance Report..."

# Calculate summary statistics
TOTAL_TESTS=$(jq '.tests | length' "$RESULTS_FILE")
SUCCESSFUL_TESTS=$(jq '[.tests[] | select(.status == "excellent" or .status == "good")] | length' "$RESULTS_FILE")
FAILED_TESTS=$(jq '[.tests[] | select(.status == "failed")] | length' "$RESULTS_FILE")
WARNING_TESTS=$(jq '[.tests[] | select(.status == "warning")] | length' "$RESULTS_FILE")

# Get gateway memory usage
MEMORY_KB=$(ps -o rss= -p "$GATEWAY_PID" 2>/dev/null || echo "0")
MEMORY_MB=$((MEMORY_KB / 1024))

# Add summary to results
jq --argjson summary '{
    "total_tests": '$TOTAL_TESTS',
    "successful_tests": '$SUCCESSFUL_TESTS',
    "failed_tests": '$FAILED_TESTS',
    "warning_tests": '$WARNING_TESTS',
    "gateway_memory_mb": '$MEMORY_MB'
}' '.summary = $summary' "$RESULTS_FILE" > "${RESULTS_FILE}.tmp" && mv "${RESULTS_FILE}.tmp" "$RESULTS_FILE"

# Display final report
echo ""
echo "📊 PERFORMANCE TEST RESULTS"
echo "============================"
echo -e "Test Run: ${CYAN}$TIMESTAMP${NC}"
echo -e "Total Tests: ${BLUE}$TOTAL_TESTS${NC}"
echo -e "Successful: ${GREEN}$SUCCESSFUL_TESTS${NC}"
echo -e "Warnings: ${YELLOW}$WARNING_TESTS${NC}"
echo -e "Failed: ${RED}$FAILED_TESTS${NC}"
echo -e "Gateway Memory: ${PURPLE}${MEMORY_MB}MB${NC}"
echo ""

# Show top performing tests
echo "🏆 Top Performing Tests:"
echo "------------------------"
jq -r '.tests[] | select(.metrics.requests_per_second) | "\(.name): \(.metrics.requests_per_second) req/sec"' "$RESULTS_FILE" | sort -k2 -nr | head -5

echo ""
echo "⚠️  Tests with Warnings or Failures:"
echo "------------------------------------"
jq -r '.tests[] | select(.status == "warning" or .status == "failed") | "\(.name): \(.status)"' "$RESULTS_FILE"

echo ""
echo "📁 Detailed Results:"
echo "   JSON Report: $RESULTS_FILE"
echo "   Gateway Log: $LOG_FILE"
echo ""

# Create HTML report
HTML_REPORT="$RESULTS_DIR/performance_report_$TIMESTAMP.html"
log "📄 Generating HTML report: $HTML_REPORT"

cat > "$HTML_REPORT" << 'EOF'
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Keystone Gateway Performance Report</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; background: #f8f9fa; }
        .container { max-width: 1200px; margin: 0 auto; }
        .header { background: white; padding: 30px; border-radius: 8px; margin-bottom: 20px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .summary { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 20px; margin: 20px 0; }
        .metric-card { background: white; padding: 20px; border-radius: 8px; text-align: center; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .metric-value { font-size: 2em; font-weight: bold; color: #007acc; }
        .metric-label { color: #666; margin-top: 5px; }
        .test-results { background: white; padding: 20px; border-radius: 8px; margin: 20px 0; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .test-item { border-bottom: 1px solid #eee; padding: 15px 0; }
        .test-name { font-weight: bold; color: #333; }
        .test-metrics { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 10px; margin-top: 10px; }
        .status-excellent { border-left: 4px solid #28a745; }
        .status-good { border-left: 4px solid #17a2b8; }
        .status-warning { border-left: 4px solid #ffc107; }
        .status-failed { border-left: 4px solid #dc3545; }
        .chart-container { background: white; padding: 20px; border-radius: 8px; margin: 20px 0; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🚀 Keystone Gateway Performance Report</h1>
            <p id="report-info">Loading...</p>
        </div>
        
        <div class="summary" id="summary">
            <!-- Summary cards will be populated by JavaScript -->
        </div>
        
        <div class="chart-container">
            <h2>📊 Performance Metrics</h2>
            <canvas id="performanceChart" width="400" height="200"></canvas>
        </div>
        
        <div class="test-results">
            <h2>📋 Detailed Test Results</h2>
            <div id="test-results">
                <!-- Test results will be populated by JavaScript -->
            </div>
        </div>
    </div>

    <script>
        // Performance data will be injected here
        const performanceData = PERFORMANCE_DATA_PLACEHOLDER;
        
        // Populate report info
        document.getElementById('report-info').innerHTML = `
            Generated: ${performanceData.test_run.timestamp}<br>
            Gateway Version: ${performanceData.test_run.gateway_version}<br>
            Config: ${performanceData.test_run.config_file}
        `;
        
        // Populate summary
        const summary = performanceData.summary;
        document.getElementById('summary').innerHTML = `
            <div class="metric-card">
                <div class="metric-value">${summary.total_tests}</div>
                <div class="metric-label">Total Tests</div>
            </div>
            <div class="metric-card">
                <div class="metric-value">${summary.successful_tests}</div>
                <div class="metric-label">Successful</div>
            </div>
            <div class="metric-card">
                <div class="metric-value">${summary.warning_tests}</div>
                <div class="metric-label">Warnings</div>
            </div>
            <div class="metric-card">
                <div class="metric-value">${summary.failed_tests}</div>
                <div class="metric-label">Failed</div>
            </div>
            <div class="metric-card">
                <div class="metric-value">${summary.gateway_memory_mb}</div>
                <div class="metric-label">Memory (MB)</div>
            </div>
        `;
        
        // Populate test results
        const testResultsHtml = performanceData.tests.map(test => `
						<div class="test-item ${test.status}">
								<div class="test-name">${test.name}</div>
								<div class="test-metrics">
										<div>Type: ${test.type}</div>
										<div>URL: ${test.url}</div>
										${test.host_header ? `<div>Host: ${test.host_header}</div>` : ''}
										<div>Status: ${test.status}</div>
										<div>Requests/sec: ${test.metrics.requests_per_second || 'N/A'}</div>
										<div>Time/req (ms): ${test.metrics.time_per_request_ms || 'N/A'}</div>
										<div>Failed: ${test.metrics.failed_requests || 0}</div>
										<div>Transfer rate (KB/s): ${test.metrics.transfer_rate_kb_sec || 'N/A'}</div>
								</div>
						</div>
				`).join('');
				document.getElementById('test-results').innerHTML = testResultsHtml;

				// Prepare data for chart
				const chartLabels = performanceData.tests.map(test => test.name);
				const chartData = performanceData.tests.map(test => test.metrics.requests_per_second || 0);

				// Create performance chart
				const ctx = document.getElementById('performanceChart').getContext('2d');
				new Chart(ctx, {
						type: 'bar',
						data: {
								labels: chartLabels,
								datasets: [{
										label: 'Requests per Second',
										data: chartData,
										backgroundColor: 'rgba(54, 162, 235, 0.2)',
										borderColor: 'rgba(54, 162, 235, 1)',
										borderWidth: 1
								}]
						},
						options: {
								scales: {
										y: {
												beginAtZero: true
										}
								}
						}
				});
