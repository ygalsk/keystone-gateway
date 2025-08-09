# Development Guide

Simple development workflow for Keystone Gateway.

## Quick Start

```bash
git clone <repo>
cd keystone-gateway
make dev
```

## Development Workflow

```bash
# 1. Make changes
vim internal/routing/gateway.go

# 2. Test changes
make test

# 3. Commit (hooks run automatically)
git add .
git commit -m "fix: backend health check timeout"

# 4. Push
git push origin feature-branch
```

## Pre-commit Hooks

Automatically run on every commit:
- `go fmt` - Format code
- `golangci-lint` - Lint and fix issues
- `go test` - Run all tests
- Basic file checks (YAML, whitespace, etc.)

### Install Pre-commit
```bash
pip install pre-commit
pre-commit install
```

## CI/CD Pipeline

GitHub Actions runs on every push:

**Pull Request:**
- Tests (Go 1.22, 1.23)
- Linting (golangci-lint)
- Security scan (gosec, trivy)
- Multi-arch builds

**Main Branch:**
- All PR checks
- Docker image build
- Auto-deployment to staging

**Release:**
- Create binaries (linux, macOS, windows)
- Docker images
- GitHub release

## Testing

```bash
# Run all tests
make test

# Run specific test types
make test-unit
make test-integration

# Run with race detection
go test -race ./...
```

## Building

```bash
# Local build
go build -o keystone-gateway ./cmd

# Docker build
make build

# Multi-platform
make docker
```

## Code Style

- Use `slog` for logging (structured JSON)
- Follow Go standard conventions
- Keep functions small and focused
- Add component field to all logs

## Commit Messages

```bash
git commit -m "fix: circuit breaker timeout issue"
git commit -m "feat: add health check interval config"
git commit -m "docs: update logging examples"
```

Prefixes: `fix:`, `feat:`, `docs:`, `test:`, `refactor:`

## Release Process

```bash
# Create release
git tag v1.3.0
git push origin v1.3.0

# CI automatically:
# - Builds binaries
# - Creates Docker images
# - Publishes GitHub release
```

## Security

```bash
# Run security scan
gosec ./...

# Common issues:
# - File inclusion (G304) - Expected for config/script loading
# - Integer overflow (G115) - Check math operations
# - HTTP timeouts (G112) - Add ReadHeaderTimeout
```

## Troubleshooting

**Pre-commit fails:**
```bash
pre-commit run --all-files
```

**Tests fail:**
```bash
go test -v ./tests/unit
```

**Linting errors:**
```bash
golangci-lint run --fix
```

**Security issues:**
```bash
gosec ./...
```

**CI fails:**
Check GitHub Actions tab for details.

Keep it simple. Write code. Ship it.
