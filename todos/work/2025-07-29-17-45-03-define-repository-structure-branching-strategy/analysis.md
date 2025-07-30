# Analysis: Define Repository Structure and Branching Strategy

## Current Repository State Analysis

### Git Branch Structure
- **Main branch:** `main` (production)
- **Current branch:** `todo-branch` (active development)
- **Feature branches:** 8 total including `add-global-lua-scripts`, `health_balance`, `landing`, `merger`, `mvp`
- **Status:** Well-organized core structure following Go conventions

### Directory Organization
```
keystone-gateway/
├── cmd/main.go                    # Single entry point
├── internal/                      # Private packages
│   ├── config/, routing/, lua/
├── configs/                       # Configuration files with examples
├── scripts/                       # Lua routing scripts
├── tests/                        # Comprehensive test suite (588KB)
├── docs/                         # Documentation (72KB)  
├── monitoring/                   # Basic Prometheus setup
└── todos/                        # Development tracking (140KB - needs cleanup)
```

### Current Workflow Analysis
- **Sophisticated CI/CD:** GitHub Actions with multi-stage pipeline, cross-platform builds
- **Production-ready:** Docker support, comprehensive Makefile, performance monitoring
- **Linear development:** Rebase workflow, no merge commits, single developer pattern
- **Mixed commit quality:** Some conventional commits, some informal messages

## Recommended Branching Strategy

### GitLab Flow with Environment Branches (Recommended)

**Branch Structure:**
```
main (production-ready)
├── staging (pre-production environment)
├── feature/* (short-lived feature branches)
├── hotfix/* (critical production fixes)
└── release/* (optional release preparation)
```

**Workflow:**
```
Feature Development: developer → feature/branch → staging → main
Hotfixes: main → hotfix/branch → staging → main
Regular Releases: feature/branch → staging (testing) → main (production)
```

### Why This Strategy Fits

1. **Production API Gateway:** Needs careful release management and staging
2. **Existing Docker Infrastructure:** Perfect fit for environment-based deployments
3. **Comprehensive Testing:** Performance testing suite fits staged approach
4. **CI/CD Integration:** Current GitHub Actions pipeline already supports this pattern

## Commit and Merge Strategy

### Conventional Commits (Enforced)
```
<type>[optional scope]: <description>

[optional body]
[optional footer(s)]
```

**Project-Specific Scopes:**
- `gateway`, `config`, `routing`, `middleware`, `lua`, `tests`, `deploy`

### Hybrid Merge Strategy
- **Small features (1-5 commits):** Rebase and merge (`--ff-only`)
- **Complex features:** Merge commits (`--no-ff`) for traceability
- **Experimental work:** Squash and merge for clean history

## Repository Structure Improvements

### Immediate Improvements Needed
1. **Clean development artifacts:** Remove `todos/`, `coverage.html`, temporary files
2. **Fix Go version consistency:** Align go.mod (1.19) with Dockerfile (1.22)
3. **Update placeholder URLs:** Replace `your-org/keystone-gateway` with actual repository
4. **Branch cleanup:** Remove stale feature branches (`merger`, `mvp`)

### Enhanced Structure Additions
1. **Environment-specific configs:**
   ```
   configs/
   ├── production.yaml (current high-performance settings)
   ├── staging.yaml (moderate settings for testing)
   └── development.yaml (local development)
   ```

2. **Deployment directory:**
   ```
   deploy/
   ├── docker-compose.staging.yml
   ├── k8s/ (if needed)
   └── scripts/
   ```

3. **CI/CD enhancements:**
   ```
   .github/workflows/
   ├── staging-deploy.yml
   ├── production-deploy.yml
   └── performance-tests.yml
   ```

## Todo System Integration

### Branch-Based Todo Management
- **Feature branches:** Development todos and task tracking
- **Staging:** Performance validation and testing todos
- **Main:** Production deployment and monitoring todos

### Workflow Integration
- Reference todo items in conventional commit messages
- Maintain clean commit history before merging to main
- Keep Claude-generated commit attribution format

## Implementation Roadmap

### Phase 1: Repository Cleanup (1-2 hours)
1. Remove development artifacts
2. Fix configuration inconsistencies  
3. Update documentation placeholders
4. Clean up stale branches

### Phase 2: Branching Strategy Setup (2-3 hours)
1. Create `staging` branch
2. Update GitHub branch protection rules
3. Modify CI/CD for environment-based deployments
4. Create environment-specific configurations

### Phase 3: Team Migration (1 week)
1. Update CONTRIBUTING.md with new guidelines
2. Train team on new workflow
3. Implement conventional commit requirements
4. Monitor and adjust based on feedback

This strategy balances production stability requirements with development agility, leveraging the existing sophisticated infrastructure while providing better organization and release management.