# Documentation

Welcome to the Keystone Gateway documentation! This directory contains comprehensive guides and references for using and developing with Keystone Gateway.

## Quick Navigation

### Getting Started
- **[Getting Started Guide](getting-started.md)** - Step-by-step tutorial from installation to your first working gateway

### User Documentation
- **[Configuration Reference](configuration.md)** - Complete reference for YAML configuration options
- **[Lua Scripting Guide](lua-scripting.md)** - Comprehensive guide to writing Lua routing scripts
- **[Quick Reference](quick-reference.md)** - Essential commands and code snippets

### Development
- **[Contributing Guidelines](../CONTRIBUTING.md)** - How to contribute to the project
- **[Project README](../README.md)** - Project overview and quick start

## Documentation Structure

```
docs/
├── README.md              # This file - documentation index
├── getting-started.md     # Tutorial for new users
├── configuration.md       # Complete configuration reference
├── lua-scripting.md       # Lua scripting API and examples
└── quick-reference.md     # Essential commands and code snippets
```

## Additional Resources

### Configuration Examples
- **[Configuration Examples](../configs/examples/)** - Ready-to-use YAML configurations
  - `simple.yaml` - Basic single-tenant setup
  - `multi-tenant.yaml` - Multi-tenant configuration
  - `production.yaml` - Production-ready configuration
  - `development.yaml` - Local development setup

### Lua Script Examples
- **[Lua Script Examples](../scripts/examples/)** - Production-ready Lua routing scripts
  - `auth-routes.lua` - Authentication and authorization patterns
  - `ab-testing-routes.lua` - A/B testing implementation
  - `canary-routes.lua` - Canary deployment strategies
  - `test-routes.lua` - Basic testing patterns

## Documentation Guidelines

When contributing to documentation:

1. **Keep it simple** - Follow KISS principles
2. **Use examples** - Show practical usage
3. **Stay current** - Update docs with code changes
4. **Test examples** - Ensure all code examples work

## Getting Help

- **Search existing documentation** first
- **Check configuration examples** for common patterns
- **Review Lua script examples** for routing patterns
- **Create GitHub issues** for documentation improvements

## Feedback

Documentation feedback is welcome! Please:
- Open issues for unclear or missing documentation
- Suggest improvements via pull requests
- Share your use cases to help improve examples