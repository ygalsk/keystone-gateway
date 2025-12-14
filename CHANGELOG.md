# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [3.0.0] - 2025-12-14

### Added
- gopher-luar library for automatic Go ↔ Lua type conversion
- Deep Lua modules architecture (request, response, http)
- Comprehensive DESIGN.md documentation
- Automatic request body caching
- Connection pooling optimization

### Changed
- **MAJOR**: Lua bindings reduced from 598 to ~50 lines (90% code reduction)
- Improved Lua API discoverability using properties instead of methods
- Request module with automatic caching and Chi URL parameter access
- Response module with proper content-type handling
- HTTP client module with HTTP/2 support

### Removed
- YAGNI classes and empty files
- Unused handlers package (71 lines)
- Unnecessary route group registration complexity

## [2.1.0] - 2025-11-03

### Added
- Optional redirect control to http_get() function for following HTTP redirects

## [2.0.1] - 2025-10-30

### Changed
- Improved server lifecycle management
- Enhanced configuration handling
- Increased timeouts for better reliability
- Streamlined CI/CD pipeline

## [2.0.0] - 2025-09-17

**BREAKING CHANGES**

### Added
- Unified compiler cache for all Lua scripts (50-70% memory reduction)
- Request limits with max_body_size enforcement
- HTTP/2 support with optimized timeouts
- http_get() and http_post() functions with context propagation
- Immutable context caching in Lua bindings

### Changed
- **BREAKING**: Merged script/global caches into single compiler
- **BREAKING**: Lua API changes:
  - `response:write()` → `response_write()`
  - `response:header()` → `response_header()`
  - `chi_middleware` signature changed
- Backend health checking with proper locking
- Simplified LuaRouteRegistry registration

### Performance
- Bytecode compilation for 50-70% memory reduction
- HTTP/2 enabled for all connections

### Migration Guide

**Updating from v1.x to v2.0.0:**

Old syntax:
```lua
response:write("Hello")
response:header("Content-Type", "text/plain")
```

New syntax:
```lua
response_write("Hello")
response_header("Content-Type", "text/plain")
```

## [1.5.0] - 2025-08-28

### Added
- Multiple Lua scripts per tenant support
- Path prefix detection for script routing
- http_get() function in Lua bindings
- get_env() binding for environment variable access

### Fixed
- Middleware caching complexity removed
- Lua route mounting issues
- Middleware ordering and registration
- Header persistence in response handling
- Chi router middleware ordering enforcement

## [1.4.0] - 2025-08-09

### Added
- Comprehensive test suite with fixture-based architecture
- Comprehensive Lua scripting examples (authentication, rate limiting)

### Changed
- Major KISS/DRY refactoring
- Lua-stone service eliminated (consolidated into single binary)
- Redesigned middleware system
- Improved domain validation

## [1.3.0] - 2025-07-31

### Added
- Go 2025 best practices project structure
- pkg/ directory for reusable packages
- Enhanced configs/ organization (defaults, environments)
- Improved scripts/lua/ structure (examples, utilities)
- Development tools consolidated in tools/

### Changed
- Reorganized project structure following Go conventions
- Enhanced separation of concerns across components
- Standardized directory naming conventions

### Removed
- Legacy files and directories cleanup

## [1.2.0] - 2025-07-18

### Added
- Host-based routing for multi-tenant gateway
- Hybrid routing (host + path combination)
- Comprehensive test suite for host-based routing
- Configuration examples and testing tools
- Performance benchmarking and analysis documentation

### Changed
- Domain validation and tenant validation logic
- Routing priority: hybrid > host-only > path-only
- Backward compatibility maintained for path-based routing

## [1.1.0] - 2025-07-18

### Added
- Multi-tenant routing with host and header detection

## [1.0.0] - 2025-07-18

### Added
- Initial release of Keystone Gateway
- Go-based reverse proxy with health-based load balancing
- Multi-tenant support with path-based routing
- Chi router framework for professional routing performance
- Lua scripting capabilities for routes and middleware
- Docker support with hardened Alpine image
- Makefile for Docker Swarm deployment automation
