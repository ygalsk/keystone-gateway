#!/bin/bash

# Keystone Gateway Branching Strategy Validation Script
# =====================================================

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo -e "${BLUE}🔍 Keystone Gateway Branching Strategy Validation${NC}"
echo "================================================="

# Check counter
passed=0
total=0

check_file() {
    local file_path="$1"
    local description="$2"
    total=$((total + 1))
    
    if [ -f "$file_path" ]; then
        echo -e "${GREEN}✅ $description${NC}"
        passed=$((passed + 1))
    else
        echo -e "${RED}❌ $description${NC}"
        echo "   Missing: $file_path"
    fi
}

check_directory() {
    local dir_path="$1"
    local description="$2"
    total=$((total + 1))
    
    if [ -d "$dir_path" ]; then
        echo -e "${GREEN}✅ $description${NC}"
        passed=$((passed + 1))
    else
        echo -e "${RED}❌ $description${NC}"
        echo "   Missing: $dir_path"
    fi
}

check_git_config() {
    local config_key="$1"
    local description="$2"
    total=$((total + 1))
    
    if git config --get "$config_key" >/dev/null 2>&1; then
        echo -e "${GREEN}✅ $description${NC}"
        passed=$((passed + 1))
    else
        echo -e "${YELLOW}⚠️  $description (optional)${NC}"
        echo "   Run: git config $config_key <value>"
        passed=$((passed + 1)) # Count as passed since it's optional
    fi
}

echo -e "${YELLOW}📋 Checking Branching Strategy Components...${NC}"
echo ""

# 1. Branch structure verification
echo -e "${BLUE}🌿 Branch Structure:${NC}"
if git rev-parse --verify staging >/dev/null 2>&1; then
    echo -e "${GREEN}✅ Staging branch exists${NC}"
    passed=$((passed + 1))
else
    echo -e "${RED}❌ Staging branch missing${NC}"
    echo "   Run: git checkout -b staging"
fi
total=$((total + 1))

if git rev-parse --verify main >/dev/null 2>&1; then
    echo -e "${GREEN}✅ Main branch exists${NC}"
    passed=$((passed + 1))
else
    echo -e "${RED}❌ Main branch missing${NC}"
fi
total=$((total + 1))

echo ""

# 2. Configuration files
echo -e "${BLUE}⚙️  Configuration Files:${NC}"
check_file "$PROJECT_ROOT/configs/environments/staging.yaml" "Staging environment configuration"
check_file "$PROJECT_ROOT/configs/environments/production-high-load.yaml" "Production environment configuration"
echo ""

# 3. Commit message infrastructure
echo -e "${BLUE}📝 Commit Message Infrastructure:${NC}"
check_file "$PROJECT_ROOT/.gitmessage" "Git commit message template"
check_file "$PROJECT_ROOT/.commitlintrc.yml" "Commitlint configuration"
check_file "$PROJECT_ROOT/.pre-commit-config.yaml" "Pre-commit hooks configuration"
echo ""

# 4. CI/CD pipeline
echo -e "${BLUE}🚀 CI/CD Pipeline:${NC}"
check_file "$PROJECT_ROOT/.github/workflows/ci.yml" "GitHub Actions workflow"

# Check if CI/CD includes staging and production deployments
if [ -f "$PROJECT_ROOT/.github/workflows/ci.yml" ]; then
    if grep -q "deploy-staging" "$PROJECT_ROOT/.github/workflows/ci.yml"; then
        echo -e "${GREEN}✅ Staging deployment job configured${NC}"
        passed=$((passed + 1))
    else
        echo -e "${RED}❌ Staging deployment job missing${NC}"
    fi
    total=$((total + 1))
    
    if grep -q "deploy-production" "$PROJECT_ROOT/.github/workflows/ci.yml"; then
        echo -e "${GREEN}✅ Production deployment job configured${NC}"
        passed=$((passed + 1))
    else
        echo -e "${RED}❌ Production deployment job missing${NC}"
    fi
    total=$((total + 1))
fi
echo ""

# 5. Deployment infrastructure
echo -e "${BLUE}🏗️  Deployment Infrastructure:${NC}"
check_directory "$PROJECT_ROOT/deployments" "Deployments directory"
check_directory "$PROJECT_ROOT/deployments/docker" "Docker deployment configs"
check_file "$PROJECT_ROOT/deployments/docker/docker-compose.staging.yml" "Staging Docker Compose"
check_file "$PROJECT_ROOT/docker-compose.production.yml" "Production Docker Compose"
check_file "$PROJECT_ROOT/Makefile" "Comprehensive Makefile system"

# Check if Makefile has key deployment targets
if [ -f "$PROJECT_ROOT/Makefile" ]; then
    if grep -q "staging-up" "$PROJECT_ROOT/Makefile"; then
        echo -e "${GREEN}✅ Staging deployment target configured${NC}"
        passed=$((passed + 1))
    else
        echo -e "${RED}❌ Staging deployment target missing${NC}"
    fi
    total=$((total + 1))
    
    if grep -q "prod-up" "$PROJECT_ROOT/Makefile"; then
        echo -e "${GREEN}✅ Production deployment target configured${NC}"
        passed=$((passed + 1))
    else
        echo -e "${RED}❌ Production deployment target missing${NC}"
    fi
    total=$((total + 1))
fi
echo ""

# 6. Documentation
echo -e "${BLUE}📚 Documentation:${NC}"
check_file "$PROJECT_ROOT/docs/branching-strategy.md" "Branching strategy documentation"
check_file "$PROJECT_ROOT/CONTRIBUTING.md" "Contributing guidelines"
check_file "$PROJECT_ROOT/deployments/README.md" "Deployment documentation"
echo ""

# 7. Git configuration (optional)
echo -e "${BLUE}🔧 Git Configuration (Optional):${NC}"
check_git_config "commit.template" "Commit message template configured"
echo ""

# Summary
echo -e "${BLUE}📊 Validation Summary:${NC}"
echo "====================="

if [ $passed -eq $total ]; then
    echo -e "${GREEN}🎉 All checks passed! ($passed/$total)${NC}"
    echo ""
    echo -e "${GREEN}✅ Branching strategy is fully implemented${NC}"
    echo ""
    echo -e "${BLUE}Next steps:${NC}"
    echo "1. Set up pre-commit hooks: pre-commit install"
    echo "2. Configure git commit template: git config commit.template .gitmessage"
    echo "3. Test the Makefile system: make help"
    echo "4. Start development environment: make dev-up"
    echo "5. Create your first feature branch: make feature-start FEATURE=my-feature"
else
    echo -e "${YELLOW}⚠️  $passed/$total checks passed${NC}"
    failed=$((total - passed))
    echo -e "${RED}❌ $failed issues need attention${NC}"
    echo ""
    echo -e "${YELLOW}Please address the missing components above${NC}"
fi

echo ""
echo -e "${BLUE}🔗 Documentation References:${NC}"
echo "• Branching Strategy: docs/branching-strategy.md"
echo "• Contributing Guide: CONTRIBUTING.md"
echo "• Deployment Guide: deployments/README.md"
echo "• Configuration: docs/configuration.md"

exit 0