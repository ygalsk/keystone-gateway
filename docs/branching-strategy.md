# Branching Strategy and Workflow

This document describes the comprehensive branching strategy and development workflow for Keystone Gateway. We use **GitLab Flow with Environment Branches** designed specifically for our multi-tenant API gateway architecture.

## 🎯 Strategy Overview

Our branching strategy balances development agility with production stability, providing clear paths for feature development, testing, and deployment while maintaining the sophisticated infrastructure of Keystone Gateway.

### Key Principles

1. **Environment-Based Branches**: Each branch represents a deployment environment
2. **Linear History**: Prefer fast-forward merges where possible
3. **Conventional Commits**: Standardized commit messages for automation
4. **Automated Deployments**: CI/CD pipeline handles deployments
5. **Review Requirements**: All changes require code review

## 🌳 Branch Structure

### Long-lived Branches

#### `main` (Production)
- **Purpose**: Production-ready code deployed to live environment
- **Protection**: Protected branch with required reviews
- **Deployment**: Automatic deployment to production
- **Configuration**: `configs/environments/production-high-load.yaml`
- **Merge Policy**: Only from `staging` after successful testing

#### `staging` (Staging Environment)
- **Purpose**: Pre-production testing and validation
- **Protection**: Protected branch with required reviews
- **Deployment**: Automatic deployment to staging environment
- **Configuration**: `configs/environments/staging.yaml`
- **Merge Policy**: From feature branches after code review

#### `develop` (Integration)
- **Purpose**: Integration branch for ongoing development (optional)
- **Usage**: For projects with continuous integration needs
- **Deployment**: Development/testing deployments only

### Short-lived Branches

#### `feature/*` (Feature Development)
- **Naming**: `feature/add-rate-limiting`, `feature/jwt-authentication`
- **Base**: Created from `staging` branch
- **Purpose**: New feature development
- **Lifespan**: Deleted after merge to staging
- **Testing**: Unit and integration tests required

#### `bugfix/*` (Bug Fixes)
- **Naming**: `bugfix/fix-memory-leak`, `bugfix/health-check-timeout`
- **Base**: Created from `staging` branch
- **Purpose**: Non-critical bug fixes
- **Lifespan**: Deleted after merge to staging
- **Testing**: Regression tests required

#### `hotfix/*` (Critical Production Fixes)
- **Naming**: `hotfix/security-patch`, `hotfix/critical-crash`
- **Base**: Created from `main` branch
- **Purpose**: Critical production issues requiring immediate fix
- **Merge**: Direct to `main` with fast-track review
- **Cherry-pick**: Applied to `staging` to maintain consistency

## 🔄 Development Workflow

### 1. Feature Development

```bash
# Start new feature
git checkout staging
git pull origin staging
git checkout -b feature/add-lua-middleware

# Develop and commit (using conventional commits)
git add .
git commit -m "feat(lua): add middleware support for request validation"

# Push and create PR
git push origin feature/add-lua-middleware
# Create Pull Request to staging branch
```

### 2. Code Review Process

#### Review Requirements
- [ ] Code follows project standards
- [ ] Tests added/updated
- [ ] Documentation updated
- [ ] Security considerations addressed
- [ ] Performance impact evaluated
- [ ] Configuration examples provided (if applicable)

#### Automated Checks
- [ ] CI pipeline passes
- [ ] Code quality checks pass
- [ ] Security scans pass
- [ ] Performance benchmarks meet requirements

### 3. Staging Deployment

```bash
# After PR approval and merge to staging
# Automatic deployment to staging environment
# Configuration: configs/environments/staging.yaml
# URL: https://staging.keystone-gateway.dev
```

#### Staging Testing Checklist
- [ ] Feature works as expected
- [ ] Integration with existing features
- [ ] Performance impact assessment
- [ ] Security validation
- [ ] Load balancing behavior
- [ ] Health check functionality
- [ ] Admin API responses

### 4. Production Release

```bash
# After successful staging testing
git checkout main
git pull origin main
git merge staging --no-ff
git push origin main

# Automatic production deployment
# Configuration: configs/environments/production-high-load.yaml
```

### 5. Hotfix Process

```bash
# Critical production issue
git checkout main
git pull origin main
git checkout -b hotfix/critical-security-fix

# Make minimal, focused changes
git commit -m "fix(security): patch SQL injection vulnerability"

# Create PR to main (expedited review)
git push origin hotfix/critical-security-fix

# After merge to main, cherry-pick to staging
git checkout staging
git cherry-pick <hotfix-commit-hash>
git push origin staging
```

## 🚀 CI/CD Integration

### Automated Triggers

#### On Push to `staging`
- Full test suite execution
- Security scanning
- Build Docker images
- Deploy to staging environment
- Run integration tests
- Performance benchmarks

#### On Push to `main`
- Full test suite execution
- Security scanning
- Build production Docker images
- Deploy to production environment
- Run smoke tests
- Monitor deployment health

#### On Pull Request
- Unit and integration tests
- Code quality checks
- Security scanning
- Build verification
- Performance impact analysis

### Environment-Specific Configurations

#### Staging Environment
```yaml
# configs/environments/staging.yaml
tenants:
  - name: "staging-api"
    domains: ["staging.keystone-gateway.dev", "api-staging.keystone-gateway.dev"]
    # Staging-specific settings
```

#### Production Environment
```yaml
# configs/environments/production-high-load.yaml
tenants:
  - name: "production-api"
    domains: ["keystone-gateway.dev", "api.keystone-gateway.dev"]
    # Production-optimized settings
```

## 📋 Commit Message Standards

### Conventional Commits Format

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Types and Scopes

#### Commit Types
- `feat`: New features
- `fix`: Bug fixes
- `docs`: Documentation
- `style`: Code formatting
- `refactor`: Code restructuring
- `perf`: Performance improvements
- `test`: Testing
- `chore`: Build/tools
- `ci`: CI/CD changes

#### Scopes (Keystone Gateway Specific)
- `routing`: Gateway routing logic
- `config`: Configuration management
- `health`: Health checking
- `proxy`: Reverse proxy functionality
- `lua`: Lua scripting integration
- `admin`: Admin API
- `docker`: Containerization
- `security`: Security improvements

### Examples

```bash
feat(routing): implement weighted load balancing algorithm
fix(health): resolve goroutine leak in health checker
docs(lua): add examples for middleware implementation  
perf(proxy): optimize connection pooling for high throughput
ci: add staging environment deployment pipeline
```

## 🔒 Branch Protection Rules

### `main` Branch Protection
- Require pull request reviews (2 reviewers)
- Require status checks to pass
- Require branches to be up to date
- Restrict pushes to administrators
- Require linear history

### `staging` Branch Protection  
- Require pull request reviews (1 reviewer)
- Require status checks to pass
- Require branches to be up to date
- Allow administrators and maintainers to push

### Status Checks Required
- CI pipeline success
- Security scan pass
- Code quality checks
- Performance benchmarks
- Integration tests

## 🗂️ Directory Structure for Deployments

### Environment Configurations
```
configs/
├── environments/
│   ├── staging.yaml              # Staging environment
│   ├── production-high-load.yaml # Production environment
│   └── development.yaml          # Local development (optional)
```

### Deployment Scripts
```
deployments/
├── docker/
│   ├── Dockerfile                # Production dockerfile
│   ├── docker-compose.staging.yml
│   └── docker-compose.production.yml
├── k8s/
│   ├── staging/                  # Kubernetes staging manifests
│   └── production/               # Kubernetes production manifests
└── scripts/
    ├── deploy-staging.sh
    └── deploy-production.sh
```

## 📈 Monitoring and Rollback

### Deployment Monitoring
- Health check endpoints verification
- Performance metrics monitoring
- Error rate tracking
- Resource utilization monitoring

### Rollback Procedures

#### Quick Rollback (Docker)
```bash
# Rollback to previous image version
docker-compose -f docker-compose.production.yml down
docker-compose -f docker-compose.production.yml up -d --scale keystone-gateway=3
```

#### Git-based Rollback
```bash
# Revert problematic merge
git checkout main
git revert <merge-commit-hash>
git push origin main
```

## 🧪 Testing Strategy by Branch

### Feature Branches
- Unit tests for new functionality
- Integration tests for component interaction
- Local development testing

### Staging Branch
- Full integration test suite
- Load testing with realistic scenarios
- Security penetration testing
- Cross-browser/environment testing

### Main Branch
- Production smoke tests
- Monitoring and alerting verification
- Performance baseline validation
- Rollback procedure testing

## 🔄 Workflow Automation

### GitHub Actions Integration

#### Workflow Triggers
```yaml
on:
  push:
    branches: [ main, staging, develop ]
  pull_request:
    branches: [ main, staging, develop ]
```

#### Environment-specific Deployments
- Staging: Automatic on `staging` branch push
- Production: Automatic on `main` branch push
- Feature: Build verification only

### Pre-commit Hooks
```yaml
# .pre-commit-config.yaml
repos:
  - repo: https://github.com/compilerla/conventional-pre-commit
    hooks:
      - id: conventional-pre-commit
```

## 📚 Additional Resources

### Documentation References
- [Contributing Guidelines](../CONTRIBUTING.md)
- [Configuration Reference](configuration.md)
- [Lua Scripting Guide](lua-scripting.md)
- [Getting Started](getting-started.md)

### Tools and Setup
```bash
# Install commit message template
git config commit.template .gitmessage

# Install pre-commit hooks
pip install pre-commit
pre-commit install
pre-commit install --hook-type commit-msg
```

### Best Practices Checklist

#### Before Creating a Feature Branch
- [ ] Latest `staging` branch pulled
- [ ] Clear understanding of requirements
- [ ] Relevant documentation reviewed

#### Before Submitting PR
- [ ] All tests pass locally
- [ ] Code follows style guidelines
- [ ] Documentation updated
- [ ] Security considerations addressed
- [ ] Performance impact evaluated

#### Before Staging Deployment
- [ ] Code review completed
- [ ] All CI checks pass
- [ ] Integration test plan ready
- [ ] Rollback plan prepared

#### Before Production Release
- [ ] Staging testing completed successfully
- [ ] Performance benchmarks met
- [ ] Security validation completed
- [ ] Monitoring and alerting configured
- [ ] Team notified of deployment

This branching strategy ensures smooth development cycles while maintaining the high performance and reliability standards required for Keystone Gateway's multi-tenant API gateway architecture.