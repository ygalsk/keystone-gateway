# Clean up the code structure and implement the implied code and repository structure
**Status:** InProgress
**Agent PID:** 346669

## Original Todo
clean up the code structure and implement the implied code and repositry structure

## Description
Address critical structural inconsistencies to align with REPOSITORY_STRUCTURE_PLAN.md. The current state has naming conflicts (lua-engine vs lua-stone), missing cmd/lua-stone implementation, and needs proper adherence to the planned structure including internal/ packages staying internal and proper lua-stone separation.

## Implementation Plan
- [x] **Rename lua-engine to lua-stone** - Move lua-engine/ directory to lua-stone/ for naming consistency with plan
- [x] **Implement cmd/lua-stone/main.go** - Move lua-stone main.go from lua-stone/ to cmd/lua-stone/ following Go conventions
- [x] **Keep internal/ packages internal** - Maintain internal/ for private packages as per REPOSITORY_STRUCTURE_PLAN.md
- [x] **Enhance pkg/client/** - Improve public API client in pkg/client/ for external consumption  
- [x] **Consolidate Lua client code** - Unify type definitions and remove duplication between clients
- [x] **Update build system** - Fix Makefiles, Docker configs for new lua-stone structure
- [ ] **Update documentation** - Fix all references from lua-engine to lua-stone
- [ ] **Update configuration** - Ensure config examples use consistent lua-stone naming
- [ ] **Validate all functionality** - Run tests to ensure everything works after restructuring

## Notes
The REPOSITORY_STRUCTURE_PLAN.md clearly shows internal/ packages should remain internal (not moved to pkg/), and cmd/lua-stone/ should contain the lua-stone binary entry point. Current lua-engine/ directory conflicts with intended lua-stone naming throughout the architecture.
