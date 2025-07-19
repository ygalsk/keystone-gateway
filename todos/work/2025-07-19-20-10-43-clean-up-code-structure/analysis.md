# Code Structure Cleanup Analysis

## Current State Assessment

Based on the REPOSITORY_STRUCTURE_PLAN.md and codebase analysis, there are several critical structural inconsistencies that need to be addressed:

### **Key Issues Identified**

#### 1. **Naming Inconsistency: lua-engine vs lua-stone**
**Problem**: The directory `lua-engine/` conflicts with the intended `lua-stone` naming convention
- REPOSITORY_STRUCTURE_PLAN.md specifies `cmd/lua-stone/` for the Lua engine binary
- Architecture documents refer to "lua-stone" as the separate service
- Configuration files use "lua_engine" but should reference "lua-stone" service
- cmd/lua-stone/ directory exists but is empty
- All current Lua functionality is in `lua-engine/` directory

#### 2. **Missing cmd/lua-stone Implementation**
**Problem**: The lua-stone binary entry point is missing per Go project conventions
- REPOSITORY_STRUCTURE_PLAN.md shows `cmd/lua-stone/main.go` as the intended structure
- `cmd/lua-stone/` exists but has no main.go
- All lua functionality is currently in separate `lua-engine/` directory
- Should follow standard Go project layout with binaries in cmd/

#### 3. **internal/ Package Structure is Correct**
**Clarification**: After reviewing REPOSITORY_STRUCTURE_PLAN.md, internal/ packages should remain internal
- `internal/config` - Private configuration management (correct placement)
- `internal/routing` - Private routing engine (correct placement)  
- `internal/health` - Private health checking (correct placement)
- `internal/proxy` - Private reverse proxy logic (correct placement)
- Go's `internal/` packages prevent external imports (desired behavior)

#### 4. **pkg/client/ Needs Enhancement**
**Opportunity**: The public API client exists but could be improved
- `pkg/client/lua-client.go` - Public API client (correct placement)
- Should be the primary interface for external lua-stone interaction
- May need consolidation with lua-engine test client code

#### 5. **Duplicated Lua Client Code**
**Problem**: Lua client implementation exists in multiple places
- `pkg/client/lua-client.go` - Public API client (should be primary)
- `lua-engine/test-client.go` - Test client with similar types
- Different type definitions for same concepts

### **Required Structural Changes (Aligned with Plan)**

1. **Rename lua-engine → lua-stone**
   - Move `lua-engine/` → `lua-stone/`  
   - Update all references and documentation
   - Move main.go to `cmd/lua-stone/main.go`

2. **Maintain internal/ Structure**
   - Keep internal packages internal as per REPOSITORY_STRUCTURE_PLAN.md
   - internal/ prevents external imports (desired behavior)
   - Focuses on pkg/ for public APIs only

3. **Consolidate Lua Client Code**
   - Enhance `pkg/client/` as the primary public API
   - Remove duplication with test clients
   - Ensure consistent type definitions

4. **Update Build and Documentation**
   - Update Makefiles for cmd/lua-stone structure
   - Fix Docker configurations for lua-stone naming
   - Update documentation references from lua-engine to lua-stone

## Dependencies and Impact

- Chi-stone gateway imports from internal packages (correct, should remain)
- Test files reference internal packages (correct, should remain)
- Docker and build configs reference lua-engine paths (needs update to lua-stone)
- Documentation and configuration examples need lua-stone updates

## Alignment with REPOSITORY_STRUCTURE_PLAN.md

✅ **Correct Current Structure:**
- `cmd/chi-stone/` - Main gateway binary (implemented)
- `internal/config/` - Configuration management (implemented)
- `internal/routing/` - Routing engine (implemented)
- `pkg/client/` - Lua client SDK (implemented)

❌ **Missing/Incorrect Structure:**
- `cmd/lua-stone/` - Empty, needs main.go
- `lua-engine/` - Should be `lua-stone/`
- Configuration naming inconsistencies
