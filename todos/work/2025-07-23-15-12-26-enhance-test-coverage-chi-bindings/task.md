# Enhance test coverage for chi-bindings and fix broken tests
**Status:** InProgress
**Agent PID:** 62644

## Original Todo
-enhance test coverage for chi-bindings and fix broken tests

## Description
We need to enhance test coverage for the Chi router bindings by fixing broken tests and adding missing test cases. The analysis revealed that while basic route registration works well, critical features like middleware registration and route groups are completely broken, with 100% failure rates in their respective test suites.

## Implementation Plan
- [x] Fix middleware registration in chi_bindings.go:88-174 - middleware not applying headers correctly
- [x] Fix route groups implementation in chi_bindings.go:176-201 - groups returning 404 instead of proper responses
- [ ] Fix route registry integration for groups in lua_routes.go:116-153
- [ ] Add test coverage for edge cases in middleware chaining
- [ ] Add test coverage for nested route groups
- [ ] Add test coverage for middleware inheritance in route groups
- [ ] Fix E2E binary build issues in gateway_test.go
- [ ] Add performance tests for concurrent route registration
- [ ] Automated test: Run `make test` to verify all fixes
- [ ] User test: Test middleware functionality with sample Lua scripts

## Notes
Implementation notes