# New Feature Workflow

**Purpose:** Step-by-step process for adding new features to Keystone Gateway  
**Agents Involved:** ARCHITECT, BACKEND, LUA, DOCS, TESTING, REVIEWER  
**Human Control Points:** Design approval, Final merge

---

## Workflow Overview

```
1. HUMAN requests feature
   ↓
2. ARCHITECT reviews design
   ↓
3. HUMAN approves design
   ↓
4. BACKEND implements (parallel: LUA, DOCS, TESTING)
   ↓
5. REVIEWER reviews all work
   ↓
6. HUMAN final review & merge
```

---

## Step-by-Step Process

### Step 1: Feature Request (HUMAN)

**Human provides:**
```markdown
## Feature Request: [Name]

**What:** [Description of feature]

**Why:** [Problem this solves]

**Use Case:** [Example scenario]

**Scope:** [What's included, what's not]

**Priority:** [High/Medium/Low]

**Questions:**
- Is this a primitive or business logic?
- Should this be in gateway core or tenant code?
```

**Tag:** `@architect` for design review

---

### Step 2: Architectural Review (ARCHITECT)

**ARCHITECT analyzes:**
1. Is this a primitive? (generic capability)
2. Is this business logic? (specific implementation)
3. Does it fit our philosophy? (general-purpose)
4. Can tenants do this themselves? (with existing primitives)

**ARCHITECT provides:**
```markdown
## ARCHITECT Review: [Feature]

**Classification:** [Primitive / Business Logic / Hybrid]

**Recommendation:** [Add to core / Tenant code / Reject]

**If adding to core:**
- Module design: [Interface + hidden complexity]
- Deep module test: [Pass/Fail]
- Information hiding: [What's hidden]
- General-purpose: [Use cases]

**If tenant code:**
- Existing primitives that support this: [List]
- Example implementation: [Lua code]

**Design Compliance:**
- [x] Deep module pattern
- [x] Information hiding
- [x] General-purpose
- [x] No business logic in core

**Human Decision Required:**
- [ ] Approve design?
- [ ] Approve adding to core?
```

**Tag:** `@human` for approval

---

### Step 3: Design Approval (HUMAN)

**Human decides:**
```markdown
## HUMAN Decision: [Feature]

**Decision:** [APPROVED / REVISE / REJECT]

**If APPROVED:**
- Proceed with implementation
- Follow ARCHITECT design

**If REVISE:**
- What needs changing: [Details]
- Re-submit to ARCHITECT

**If REJECT:**
- Reason: [Why]
- Alternative: [If any]

**Next Step:** 
[Tag agents to begin implementation]
```

**If approved, tag:** `@backend @lua @docs @testing`

---

### Step 4a: Backend Implementation (BACKEND)

**BACKEND implements:**
```markdown
## BACKEND Implementation: [Feature]

**Module:** `internal/lua/modules/[name].go`

**Interface (Public):**
```go
type [Name] struct {
    // Public API only
}

func (x *[Name]) Method1() Result
func (x *[Name]) Method2() Result
```

**Implementation (Private):**
```go
// Hidden complexity
func (x *[Name]) internalMethod()
// Connection management
// Caching
// Error recovery
```

**Thread Safety:**
- [ ] Goroutine-safe
- [ ] No shared mutable state or properly synchronized

**Testing:**
- [ ] Unit tests for public methods
- [ ] Edge cases covered

**Files Created/Modified:**
- `internal/lua/modules/[name].go` (new)
- `internal/lua/modules/[name]_test.go` (new)

**Ready for:** @lua @reviewer
```

---

### Step 4b: Lua Integration (LUA)

**LUA creates bindings:**
```markdown
## LUA Integration: [Feature]

**Binding Method:** gopher-luar (automatic)

**Exposed to Lua:**
```go
// In chi_bindings.go
L.SetGlobal("[Name]", luar.New(L, modules.New[Name]()))
```

**Lua API:**
```lua
-- Natural Lua usage
local result = [Name]:Method1()
local result2 = [Name]:Method2()
```

**Example Script:** `scripts/lua/examples/[name]_demo.lua`
```lua
chi_route("GET", "/demo", function(req, res)
    local result = [Name]:Method1()
    res:Write(result)
end)
```

**Files Created/Modified:**
- `internal/lua/chi_bindings.go` (modified - add binding)
- `scripts/lua/examples/[name]_demo.lua` (new)

**Ready for:** @docs @reviewer
```

---

### Step 4c: Documentation (DOCS)

**DOCS writes:**
```markdown
## Documentation: [Feature]

**Updated Files:**
- `docs/lua.md` (add [Name] section)
- `README.md` (add to features if user-facing)

**docs/lua.md Addition:**
```markdown
### [Name]

**Description:** [What it does]

**Methods:**
- `Method1()` - [Description]
- `Method2()` - [Description]

**Example:**
```lua
local result = [Name]:Method1()
```

**Common Use Cases:**
1. [Use case 1]
2. [Use case 2]

**Error Handling:**
[How errors work]
```

**Examples:**
- Working example in `examples/[name]_demo.lua`
- Inline code examples in docs
- Common pitfalls documented

**Ready for:** @reviewer
```

---

### Step 4d: Testing (TESTING)

**TESTING provides:**
```markdown
## Test Coverage: [Feature]

**Unit Tests:**
- `internal/lua/modules/[name]_test.go`
- Tests: [List test functions]
- Coverage: [X]%

**Integration Tests:**
- `tests/integration/[name]_test.go`
- Tests Lua integration end-to-end

**Edge Cases Tested:**
- [ ] Nil/empty inputs
- [ ] Concurrent access
- [ ] Error conditions
- [ ] Large inputs
- [ ] Boundary conditions

**Test Results:**
```
=== RUN   TestName
--- PASS: TestName (0.01s)
PASS
```

**Ready for:** @reviewer
```

---

### Step 5: Comprehensive Review (REVIEWER)

**REVIEWER checks ALL work:**
```markdown
## REVIEWER Comprehensive Review: [Feature]

**Architecture (ARCHITECT work):**
- [ ] Deep module pattern followed
- [ ] Information properly hidden
- [ ] General-purpose approach
- [ ] DESIGN.md compliant

**Backend (BACKEND work):**
- [ ] Go code quality
- [ ] Thread-safe implementation
- [ ] Follows conventions
- [ ] Tests present

**Lua Integration (LUA work):**
- [ ] gopher-luar used correctly
- [ ] Simple, discoverable API
- [ ] Example script works
- [ ] Minimal glue code

**Documentation (DOCS work):**
- [ ] docs/lua.md updated
- [ ] Examples are clear
- [ ] Accurate and complete

**Testing (TESTING work):**
- [ ] Good coverage
- [ ] Tests meaningful
- [ ] Edge cases covered

**Integration:**
- [ ] All pieces work together
- [ ] Consistent with codebase
- [ ] No breaking changes

**Verdict:**
- [ ] ✅ APPROVE - Ready for human
- [ ] 🔄 REVISE - Issues found
- [ ] ❌ REJECT - Major violations

**Issues:** [List any issues]

**Ready for:** @human (final approval)
```

---

### Step 6: Final Approval (HUMAN)

**Human reviews:**
```markdown
## HUMAN Final Review: [Feature]

**Reviewed:**
- [ ] ARCHITECT design
- [ ] BACKEND implementation
- [ ] LUA integration
- [ ] DOCS documentation
- [ ] TESTING coverage
- [ ] REVIEWER comprehensive review

**Decision:**
- [ ] ✅ MERGE - Approved
- [ ] 🔄 REQUEST CHANGES - [What needs changing]
- [ ] ❌ REJECT - [Why]

**If merging:**
```bash
git checkout main
git merge feature/[name]
git push origin main
```

**Post-Merge:**
- [ ] Update CHANGELOG.md
- [ ] Tag version if needed
- [ ] Deploy if applicable
```

---

## Parallel Execution

Steps 4a-4d can run in parallel:

```
BACKEND ─────┐
             │
LUA    ──────┼───► REVIEWER ───► HUMAN
             │
DOCS   ──────┤
             │
TESTING ─────┘
```

**Coordination:**
- BACKEND finishes first (others depend on it)
- LUA needs BACKEND module to bind
- DOCS can start writing as BACKEND progresses
- TESTING can start planning as design is approved

---

## Example: Adding WebSocket Support

### Step 1: HUMAN Request
```markdown
## Feature Request: WebSocket Support

**What:** Add WebSocket client primitive to Lua

**Why:** Tenants need real-time bidirectional communication

**Use Case:** 
```lua
local ws = WebSocket.Upgrade(req, res)
ws:Send("hello")
local msg = ws:Receive()
```

**Scope:** 
- In: WebSocket client, upgrade, send, receive, close
- Out: WebSocket server, broadcasting, rooms

@architect review design
```

### Step 2: ARCHITECT Review
```markdown
## ARCHITECT Review: WebSocket

**Classification:** Primitive ✓

**Recommendation:** Add to core

**Design:**
- Module: `internal/lua/modules/websocket.go`
- Interface: Upgrade, Send, Receive, Close
- Hidden: Connection management, frame handling, pings

**Deep Module Test:**
- Interface: 4 methods ✓
- Implementation: Substantial ✓
- Hidden complexity: Connection handling, error recovery ✓

**Human Decision Required:**
- [ ] Approve WebSocket as primitive?
- [ ] Approve 4-method interface?

@human for approval
```

### Step 3: HUMAN Approval
```markdown
## HUMAN Decision: WebSocket

**Decision:** APPROVED ✓

**Rationale:** WebSocket is a standard protocol, general-purpose

**Next:** Proceed with implementation

@backend @lua @docs @testing begin work
```

### Step 4a: BACKEND Implementation
```go
// internal/lua/modules/websocket.go
package modules

type WebSocket struct {
    conn *websocket.Conn
}

func (ws *WebSocket) Send(message string) error
func (ws *WebSocket) Receive() (string, error)
func (ws *WebSocket) Close() error

// + unit tests
```

### Step 4b: LUA Integration
```go
// internal/lua/chi_bindings.go
L.SetGlobal("WebSocket", luar.New(L, modules.WebSocket{}))
```

```lua
-- scripts/lua/examples/websocket_demo.lua
chi_route("GET", "/ws", function(req, res)
    local ws = WebSocket.Upgrade(req, res)
    while true do
        local msg = ws:Receive()
        ws:Send("Echo: " .. msg)
    end
end)
```

### Step 4c: DOCS
```markdown
### WebSocket

WebSocket client for bidirectional real-time communication.

**Methods:**
- `Upgrade(req, res)` - Upgrade HTTP to WebSocket
- `Send(message)` - Send message
- `Receive()` - Receive message (blocking)
- `Close()` - Close connection

**Example:** See `examples/websocket_demo.lua`
```

### Step 4d: TESTING
```go
func TestWebSocket(t *testing.T) {
    // Test upgrade, send, receive, close
}

func TestWebSocketConcurrent(t *testing.T) {
    // Test concurrent connections
}
```

### Step 5: REVIEWER
```markdown
## REVIEWER: ✅ APPROVED

All agents completed work, design compliant, ready for human.
```

### Step 6: HUMAN MERGE
```markdown
## HUMAN: ✅ MERGED

WebSocket support added, v1.3.0 tagged
```

---

## Quick Reference

**Agent Tags:**
- `@architect` - Design review
- `@backend` - Go implementation
- `@lua` - Lua bindings
- `@docs` - Documentation
- `@testing` - Test coverage
- `@reviewer` - Final review
- `@human` - Human decision

**Workflow States:**
1. 📝 REQUEST - Human requested
2. 🔍 REVIEW - ARCHITECT reviewing
3. ⏳ PENDING - Awaiting human approval
4. 🔨 IMPLEMENT - Agents working
5. 👀 REVIEW - REVIEWER checking
6. ✅ APPROVED - Ready for merge
7. 🎉 MERGED - Complete

---

## Success Criteria

**Feature is complete when:**
- ✅ ARCHITECT approved design
- ✅ BACKEND implemented deep module
- ✅ LUA created bindings with gopher-luar
- ✅ DOCS updated and examples work
- ✅ TESTING has good coverage
- ✅ REVIEWER approved everything
- ✅ HUMAN merged to main

**Feature follows philosophy when:**
- ✅ Deep module (simple interface, complex implementation)
- ✅ General-purpose (not business-specific)
- ✅ Information hidden (no leakage)
- ✅ Complexity pulled down (in Go, not Lua)
- ✅ No anti-patterns present