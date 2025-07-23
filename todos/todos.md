-       🔥 The Problem You're Facing
        You’re probably seeing:

        Repeated setup logic for every test case.

        Lots of clutter: e.g. manually creating requests, Lua engines, config, mocks, etc.

        Harder-to-maintain test tables because setup details are inline with logic.

        ✅ The Solution: Use Fixtures to Clean Up
        🔧 Step 1: Identify Repeated Patterns
        Look at what you're repeating in your test cases. Examples might include:

        Creating http.Request objects

        Initializing your Lua engine

        Loading a Lua script

        Simulating API gateway context

        Asserting similar header/body/response fields

        🔧 Step 2: Wrap Repetitive Setup in Fixture Functions
        Let’s say you often set up a request + Lua engine + script:

        go
        Copy
        Edit
        func setupGatewayTest(t *testing.T, script string) (*Gateway, *http.Request) {
            engine := NewLuaEngine()
            if err := engine.LoadScript(script); err != nil {
                t.Fatalf("failed to load script: %v", err)
            }

            gw := NewGateway(engine)

            req := httptest.NewRequest("GET", "/test", nil)
            req.Header.Set("X-Test-Header", "true")

            return gw, req
        }
        This reduces clutter in your tests massively.

        🔧 Step 3: Simplify Table-Driven Test Logic
        Your table tests can now look like:

        go
        Copy
        Edit
        tests := []struct {
            name     string
            script   string
            expected string
        }{
            {
                name: "adds header",
                script: `function rewrite(req) req.headers["X-Test"] = "added" end`,
                expected: "added",
            },
            {
                name: "no change",
                script: `function rewrite(req) end`,
                expected: "",
            },
        }

        for _, tt := range tests {
            t.Run(tt.name, func(t *testing.T) {
                gw, req := setupGatewayTest(t, tt.script)
                gw.HandleRequest(req)

                if got := req.Header.Get("X-Test"); got != tt.expected {
                    t.Errorf("expected header to be '%s', got '%s'", tt.expected, got)
                }
            })
        }
        Now each test case is focused, compact, and readable, and any complexity is pushed into your fixture.

        🧠 Tip: You Can Compose Fixtures
        If you’re testing different parts (e.g., rewrite vs access phases), you can write phase-specific fixtures like:

        go
        Copy
        Edit
        func setupRewriteTest(t *testing.T, script string) *LuaEngine { ... }
        func setupAccessTest(t *testing.T, user string) *LuaEngine { ... }
        And call those depending on the test context.

        ✅ Result
        Your tests become much shorter and cleaner.

        New tests are easy to add — just a new table row.

        If your gateway evolves, only the fixture changes, not hundreds of tests.
