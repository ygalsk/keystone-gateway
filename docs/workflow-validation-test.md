# Branching Strategy Workflow Validation Test

This document provides a step-by-step test to validate the complete workflow from feature development to production deployment using our new GitLab Flow with Environment Branches strategy.

## 🎯 Test Objective

Validate that the complete development workflow functions correctly:
1. Feature development
2. Staging deployment and testing
3. Production release
4. Hotfix procedures

## 📋 Prerequisites

- [ ] Repository cloned locally
- [ ] Git configured with user information
- [ ] Pre-commit hooks installed (optional)
- [ ] Docker and Docker Compose installed
- [ ] Access to staging and production environments

## 🧪 Test Scenario: Add New API Endpoint

We'll add a simple "version" endpoint to demonstrate the complete workflow.

### Step 1: Setup and Validation

```bash
# Validate branching strategy setup
make validate

# View available commands
make help

# Expected: All checks should pass and commands should be listed
```

### Step 2: Feature Development Workflow

#### 2.1 Create Feature Branch

```bash
# Use Makefile helper to start feature development
make feature-start FEATURE=add-version-endpoint

# This will:
# 1. Switch to staging branch
# 2. Pull latest changes
# 3. Create feature/add-version-endpoint branch
# 4. Start development environment

# Verify branch
git branch --show-current
# Expected: feature/add-version-endpoint
```

#### 2.2 Implement Feature

Add a simple version endpoint to the Lua routes:

```bash
# Create or update the API routes script
cat >> scripts/api-routes.lua << 'EOF'

-- Version endpoint for testing workflow
chi_route("GET", "/version", function(request, response)
    response:header("Content-Type", "application/json")
    local version_info = {
        version = "1.0.0",
        build_time = os.date("%Y-%m-%d %H:%M:%S"),
        environment = "development"
    }
    response:write(json.encode(version_info))
end)
EOF
```

#### 2.3 Commit Changes Using Conventional Commits

```bash
# Stage changes
git add scripts/api-routes.lua

# Commit with conventional commit format
git commit -m "feat(lua): add version endpoint for API information

This endpoint provides version and build information for the gateway.
Useful for deployment validation and debugging.

- Returns JSON with version, build time, and environment
- Available at GET /api/version"

# Verify commit message format
git log -1 --pretty=format:"%s"
```

#### 2.4 Push Feature Branch

```bash
# Push feature branch
git push origin feature/add-version-endpoint

# Expected: Branch pushed successfully
```

### Step 3: Pull Request and Code Review

#### 3.1 Create Pull Request to Staging

1. Go to GitHub repository
2. Create Pull Request from `feature/add-version-endpoint` to `staging`
3. Fill out PR template:
   - **Description**: Add version endpoint for API information
   - **Type of Change**: ✅ New feature
   - **Testing**: ✅ Manual testing completed

#### 3.2 Automated Checks

Verify that automated checks pass:
- [ ] CI pipeline runs successfully
- [ ] Code quality checks pass
- [ ] Security scans pass
- [ ] Build verification succeeds

### Step 4: Staging Deployment and Testing

#### 4.1 Merge to Staging

After code review approval:
1. Merge PR to staging branch
2. Verify automatic staging deployment triggers

#### 4.2 Staging Environment Testing

```bash
# Deploy to staging using Makefile
make staging-up

# Check staging health
make staging-health

# Test new version endpoint
curl -f http://localhost:8081/api/version

# Expected JSON response:
# {
#   "version": "1.0.0", 
#   "build_time": "2025-07-30 12:00:00",
#   "environment": "staging"
# }

# View staging environment info
make staging-info
```

#### 4.3 Integration Testing

```bash
# Run comprehensive test suite
make test

# Run load tests against staging
make test-load

# Manual validation using Makefile:
make staging-health  # Health checks
make staging-logs    # View logs
make stats           # Resource usage
```

### Step 5: Production Release

#### 5.1 Create Production Release PR

```bash
# Create PR from staging to main
git checkout staging
git pull origin staging
git checkout main
git pull origin main

# Verify staging is ahead of main
git log --oneline main..staging
```

Create PR from `staging` to `main`:
- **Title**: Release: Add version endpoint
- **Description**: Deploy version endpoint to production after successful staging validation

#### 5.2 Production Deployment

After PR approval and merge:
1. Automatic production deployment triggers
2. Monitor deployment logs
3. Verify health checks

#### 5.3 Production Validation

```bash
# Deploy to production using Makefile (with confirmation)
make prod-up

# Check production health
make prod-health

# Test version endpoint in production  
curl -f http://localhost:8080/api/version

# View production environment info
make prod-info

# Monitor production deployment
make stats    # Resource usage
make health   # Overall system health
```

### Step 6: Hotfix Workflow Test

#### 6.1 Simulate Production Issue

Let's simulate finding a bug in production that needs immediate fixing:

```bash
# Use Makefile helper to start hotfix
make hotfix-start HOTFIX=fix-version-endpoint-timezone

# This will:
# 1. Switch to main branch
# 2. Pull latest changes  
# 3. Create hotfix/fix-version-endpoint-timezone branch

# Make a critical fix (example: fix timezone issue)
sed -i 's/os.date("%Y-%m-%d %H:%M:%S")/os.date("!%Y-%m-%d %H:%M:%S UTC")/g' scripts/api-routes.lua

# Commit with hotfix convention
git add scripts/api-routes.lua
git commit -m "fix(lua): correct timezone display in version endpoint

Version endpoint was showing local time instead of UTC.
This could cause confusion in production monitoring.

- Changed to UTC time format
- Added explicit UTC suffix for clarity"

# Test the hotfix locally
make dev-up
make dev-health
```

#### 6.2 Hotfix Deployment

```bash
# Push hotfix branch
git push origin hotfix/fix-version-endpoint-timezone

# Create PR to main (fast-track review)
# After merge, automatic production deployment
# Cherry-pick to staging to maintain consistency
git checkout staging
git cherry-pick <hotfix-commit-hash>
git push origin staging
```

## ✅ Validation Checklist

### Feature Development Process
- [ ] Feature branch created from staging
- [ ] Conventional commit messages used
- [ ] Code follows project standards
- [ ] Pre-commit hooks worked (if installed)

### CI/CD Pipeline
- [ ] Pipeline triggered on feature branch push
- [ ] All tests and checks passed
- [ ] Docker images built successfully
- [ ] Deployment scripts accessible

### Staging Environment
- [ ] Automatic deployment to staging worked
- [ ] New feature accessible in staging
- [ ] Integration tests passed
- [ ] Environment-specific configuration used

### Production Release
- [ ] Staging to main PR created successfully
- [ ] Production deployment triggered automatically
- [ ] Zero-downtime deployment achieved
- [ ] Production validation passed

### Hotfix Process
- [ ] Hotfix branch created from main
- [ ] Fast-track review and merge
- [ ] Emergency production deployment
- [ ] Changes propagated to staging

### Documentation and Monitoring
- [ ] Deployment logs available
- [ ] Metrics and monitoring functional
- [ ] Health checks working
- [ ] Performance acceptable

## 🚨 Troubleshooting Common Issues

### Build Failures
```bash
# Check CI logs
# Review commit message format
# Verify configuration syntax
```

### Deployment Issues
```bash
# Check deployment logs
./deployments/scripts/deploy-staging.sh

# Verify configuration
kubectl get pods -n keystone-gateway-staging
docker-compose -f deployments/docker/docker-compose.staging.yml ps
```

### Environment Configuration
```bash
# Validate configs
./scripts/validate-branching-strategy.sh

# Check environment-specific settings
cat configs/environments/staging.yaml
cat configs/environments/production-high-load.yaml
```

## 📊 Success Criteria

### Complete Success ✅
- All validation checklist items pass
- Feature deployed successfully to production
- Hotfix process validated
- No manual intervention required
- Documentation accurate and helpful

### Partial Success ⚠️
- Most checklist items pass
- Minor issues that don't affect core functionality
- Some manual intervention required
- Documentation mostly accurate

### Failure ❌
- Multiple critical checklist items fail
- Core functionality broken
- Significant manual intervention required
- Major gaps in implementation

## 🔄 Cleanup

After completing the validation test:

```bash
# Clean up feature branch
git checkout staging
git branch -d feature/add-version-endpoint
git push origin --delete feature/add-version-endpoint

# Clean up hotfix branch
git checkout main
git branch -d hotfix/fix-version-endpoint-timezone
git push origin --delete hotfix/fix-version-endpoint-timezone

# Optional: Reset test changes if needed
git checkout staging
git reset --hard origin/staging
```

## 📝 Test Results Documentation

After completing the test, document:

1. **Date and Time**: When the test was performed
2. **Environment**: Local, staging, production details
3. **Results**: Which items passed/failed
4. **Issues Found**: Any problems encountered
5. **Recommendations**: Improvements for the workflow
6. **Performance**: Timing of various steps

This validation test ensures that our branching strategy and development workflow function correctly in practice, not just in theory.