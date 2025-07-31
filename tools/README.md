# Development Tools

This directory contains development and maintenance tools for Keystone Gateway.

## Available Tools

### `validate-branching-strategy.sh`
Validates the repository branching strategy and ensures proper branch management.

**Usage:**
```bash
./tools/validate-branching-strategy.sh
```

## Tool Development Guidelines

When adding new tools:

1. **Place executable scripts directly in `/tools/`**
2. **Add documentation to this README**
3. **Ensure tools are cross-platform compatible when possible**
4. **Use clear, descriptive names**
5. **Include usage examples**

## Categories

Tools can be organized by purpose:
- **Validation**: Scripts that check code quality, branching, etc.
- **Build**: Build and compilation utilities
- **Deployment**: Deployment automation scripts
- **Development**: Local development helpers