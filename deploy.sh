#!/bin/bash
# deploy.sh - Production deployment script for Keystone Gateway
# Usage: ./deploy.sh [environment] [action]
# Environment: production, staging (default: production)
# Action: deploy, stop, restart, logs, status (default: deploy)

set -e

# Configuration
DEFAULT_ENV="production"
DEFAULT_ACTION="deploy"
ENV=${1:-$DEFAULT_ENV}
ACTION=${2:-$DEFAULT_ACTION}

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Logging function
log() {
    echo -e "${CYAN}[$(date +'%H:%M:%S')]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# Check if running as root
if [ "$EUID" -eq 0 ]; then
    warning "Running as root - consider using a non-root user with docker group membership"
fi

# Validate environment
if [ "$ENV" != "production" ] && [ "$ENV" != "staging" ]; then
    error "Invalid environment: $ENV. Use 'production' or 'staging'"
    exit 1
fi

# Set compose file based on environment
if [ "$ENV" = "production" ]; then
    COMPOSE_FILE="docker-compose.production.yml"
    CONFIG_FILE="configs/production.yaml"
else
    COMPOSE_FILE="docker-compose.staging.yml"
    CONFIG_FILE="configs/staging.yaml"
fi

# Check prerequisites
check_prerequisites() {
    log "🔍 Checking prerequisites..."
    
    local missing_deps=false
    
    if ! command -v docker >/dev/null 2>&1; then
        error "Docker is not installed"
        missing_deps=true
    fi
    
    if ! command -v docker-compose >/dev/null 2>&1; then
        error "Docker Compose is not installed"
        missing_deps=true
    fi
    
    if [ ! -f "$COMPOSE_FILE" ]; then
        error "Compose file not found: $COMPOSE_FILE"
        missing_deps=true
    fi
    
    if [ ! -f "$CONFIG_FILE" ]; then
        error "Config file not found: $CONFIG_FILE"
        missing_deps=true
    fi
    
    if [ "$missing_deps" = true ]; then
        error "Please install missing dependencies before continuing"
        exit 1
    fi
    
    success "All prerequisites satisfied"
}

# Pre-deployment security checks
security_check() {
    log "🔒 Running security checks..."
    
    local security_issues=false
    
    # Check for default passwords in production config
    if [ "$ENV" = "production" ]; then
        if grep -q "change-this.*password" "$CONFIG_FILE" 2>/dev/null; then
            error "Default passwords found in production config. Please update all passwords!"
            security_issues=true
        fi
        
        if grep -q "change-this.*secret" "$CONFIG_FILE" 2>/dev/null; then
            error "Default secrets found in production config. Please update all secrets!"
            security_issues=true
        fi
    fi
    
    # Check file permissions
    if [ -f "$CONFIG_FILE" ]; then
        local perms=$(stat -c "%a" "$CONFIG_FILE" 2>/dev/null || stat -f "%A" "$CONFIG_FILE" 2>/dev/null)
        if [ "$perms" -gt 644 ]; then
            warning "Config file has overly permissive permissions: $perms"
        fi
    fi
    
    if [ "$security_issues" = true ]; then
        error "Security issues found. Please fix them before deploying to production."
        exit 1
    fi
    
    success "Security checks passed"
}

# Build and deploy
deploy() {
    log "🚀 Starting deployment to $ENV environment..."
    
    # Build the gateway binary first
    log "🔨 Building Keystone Gateway..."
    if [ -f "go.mod" ]; then
        go mod tidy
        go build -o keystone-gateway -ldflags "-X main.version=1.2.1" .
        success "Gateway built successfully"
    else
        error "go.mod not found. Make sure you're in the project root directory."
        exit 1
    fi
    
    # Create necessary directories
    log "📁 Creating directories..."
    mkdir -p monitoring
    mkdir -p logs
    
    # Stop existing services
    log "🛑 Stopping existing services..."
    docker-compose -f "$COMPOSE_FILE" down --remove-orphans 2>/dev/null || true
    
    # Pull latest images
    log "📥 Pulling latest images..."
    docker-compose -f "$COMPOSE_FILE" pull
    
    # Build and start services
    log "🏗️  Building and starting services..."
    docker-compose -f "$COMPOSE_FILE" up -d --build --force-recreate
    
    # Wait for services to be healthy
    log "⏳ Waiting for services to be healthy..."
    local max_wait=120
    local wait_time=0
    
    while [ $wait_time -lt $max_wait ]; do
        local healthy_services=$(docker-compose -f "$COMPOSE_FILE" ps --filter "health=healthy" -q | wc -l)
        local total_services=$(docker-compose -f "$COMPOSE_FILE" ps -q | wc -l)
        
        if [ "$healthy_services" -eq "$total_services" ] && [ "$total_services" -gt 0 ]; then
            success "All services are healthy"
            break
        fi
        
        if [ $wait_time -eq $max_wait ]; then
            error "Services failed to become healthy within $max_wait seconds"
            show_status
            exit 1
        fi
        
        echo -n "."
        sleep 5
        wait_time=$((wait_time + 5))
    done
    
    # Run post-deployment tests
    log "🧪 Running post-deployment tests..."
    sleep 10  # Give services time to fully initialize
    
    # Test gateway health
    if curl -sf "http://localhost:8080/admin/health" >/dev/null; then
        success "Gateway health check passed"
    else
        error "Gateway health check failed"
        exit 1
    fi
    
    # Test SSL (if in production)
    if [ "$ENV" = "production" ]; then
        log "🔐 Testing SSL endpoints..."
        # Note: This requires the domains to be properly configured
        # Uncomment when domains are live:
        # if curl -sf "https://demo.keystone-gateway.dev" >/dev/null; then
        #     success "SSL endpoints accessible"
        # else
        #     warning "SSL endpoints not yet accessible (DNS/certificates may still be propagating)"
        # fi
    fi
    
    success "Deployment completed successfully!"
    show_status
}

# Stop services
stop() {
    log "🛑 Stopping $ENV services..."
    docker-compose -f "$COMPOSE_FILE" down --remove-orphans
    success "Services stopped"
}

# Restart services
restart() {
    log "🔄 Restarting $ENV services..."
    docker-compose -f "$COMPOSE_FILE" restart
    success "Services restarted"
}

# Show logs
show_logs() {
    log "📋 Showing $ENV logs..."
    docker-compose -f "$COMPOSE_FILE" logs --tail=100 -f
}

# Show status
show_status() {
    log "📊 Service Status for $ENV:"
    echo ""
    docker-compose -f "$COMPOSE_FILE" ps
    echo ""
    
    log "🏥 Health Status:"
    docker-compose -f "$COMPOSE_FILE" ps --filter "health=healthy" -q | wc -l | xargs -I {} echo "Healthy services: {}"
    docker-compose -f "$COMPOSE_FILE" ps --filter "health=unhealthy" -q | wc -l | xargs -I {} echo "Unhealthy services: {}"
    
    echo ""
    log "📊 Resource Usage:"
    docker stats --no-stream --format "table {{.Container}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}" $(docker-compose -f "$COMPOSE_FILE" ps -q)
}

# Cleanup old images and volumes
cleanup() {
    log "🧹 Cleaning up old images and volumes..."
    docker system prune -f
    docker volume prune -f
    success "Cleanup completed"
}

# Backup data
backup() {
    log "💾 Creating backup..."
    local backup_dir="backups/$(date +%Y%m%d_%H%M%S)"
    mkdir -p "$backup_dir"
    
    # Backup volumes
    docker run --rm -v keystone-postgres-data:/source -v "$(pwd)/$backup_dir":/backup alpine tar czf /backup/postgres-data.tar.gz -C /source .
    docker run --rm -v keystone-redis-data:/source -v "$(pwd)/$backup_dir":/backup alpine tar czf /backup/redis-data.tar.gz -C /source .
    docker run --rm -v keystone-grafana-data:/source -v "$(pwd)/$backup_dir":/backup alpine tar czf /backup/grafana-data.tar.gz -C /source .
    
    # Backup configuration
    cp -r configs "$backup_dir/"
    
    success "Backup created at $backup_dir"
}

# Main execution
main() {
    echo "🔷 Keystone Gateway Production Deployment"
    echo "Environment: $ENV"
    echo "Action: $ACTION"
    echo "=========================================="
    echo ""
    
    case "$ACTION" in
        "deploy")
            check_prerequisites
            security_check
            deploy
            ;;
        "stop")
            stop
            ;;
        "restart")
            restart
            ;;
        "logs")
            show_logs
            ;;
        "status")
            show_status
            ;;
        "cleanup")
            cleanup
            ;;
        "backup")
            backup
            ;;
        *)
            error "Invalid action: $ACTION"
            echo "Valid actions: deploy, stop, restart, logs, status, cleanup, backup"
            exit 1
            ;;
    esac
}

# Run main function
main