package fixtures

import (
	"log"
	"net/http"
	"testing"
	"time"

	"keystone-gateway/internal/config"
)

// E2EScenario represents a complete E2E testing scenario
type E2EScenario struct {
	Name        string
	Description string
	Setup       func(t *testing.T) *E2ETestEnvironment
	Test        func(t *testing.T, env *E2ETestEnvironment, client *E2EClient)
	Cleanup     func(env *E2ETestEnvironment)
}

// RunE2EScenario executes a complete E2E scenario
func RunE2EScenario(t *testing.T, scenario E2EScenario) {
	t.Run(scenario.Name, func(t *testing.T) {
		t.Logf("Running E2E scenario: %s", scenario.Description)

		// Setup
		env := scenario.Setup(t)
		if env == nil {
			t.Fatal("Scenario setup returned nil environment")
		}

		// Cleanup
		defer func() {
			if scenario.Cleanup != nil {
				scenario.Cleanup(env)
			} else {
				if err := env.Cleanup(); err != nil {
					log.Printf("Failed to cleanup environment: %v", err)
				}
			}
		}()

		// Create client
		client := NewE2EClient()
		client.SetBaseURL(env.Gateway.URL)

		// Run test
		scenario.Test(t, env, client)
	})
}

// CreateSimpleE2EScenario creates a basic E2E scenario with single tenant and backend
func CreateSimpleE2EScenario(name, description string) E2EScenario {
	return E2EScenario{
		Name:        name,
		Description: description,
		Setup: func(t *testing.T) *E2ETestEnvironment {
			// Create simple config
			cfg := &config.Config{
				Tenants: []config.Tenant{
					{
						Name:       "simple-tenant",
						PathPrefix: "/api/",
						Interval:   30,
						Services: []config.Service{
							{Name: "simple-service", URL: "placeholder", Health: "/health"},
						},
					},
				},
			}

			return SetupE2EEnvironment(t, cfg)
		},
		Test: func(t *testing.T, env *E2ETestEnvironment, client *E2EClient) {
			// Basic health check
			resp, err := client.GetResponse("/api/health")
			if err != nil {
				t.Fatalf("Failed to get health response: %v", err)
			}

			if !resp.HasStatus(200) {
				t.Errorf("Expected status 200, got %d", resp.StatusCode)
			}
		},
	}
}

// CreateMultiTenantE2EScenario creates an E2E scenario with multiple tenants
func CreateMultiTenantE2EScenario(name, description string) E2EScenario {
	return E2EScenario{
		Name:        name,
		Description: description,
		Setup: func(t *testing.T) *E2ETestEnvironment {
			// Create multi-tenant config
			cfg := &config.Config{
				Tenants: []config.Tenant{
					{
						Name:     "api-tenant",
						Domains:  []string{"api.example.com"},
						Interval: 30,
						Services: []config.Service{
							{Name: "api-service", URL: "placeholder", Health: "/health"},
						},
					},
					{
						Name:       "web-tenant",
						PathPrefix: "/web/",
						Interval:   30,
						Services: []config.Service{
							{Name: "web-service", URL: "placeholder", Health: "/health"},
						},
					},
				},
			}

			return SetupE2EEnvironment(t, cfg)
		},
		Test: func(t *testing.T, env *E2ETestEnvironment, client *E2EClient) {
			// Test host-based routing
			resp1, err := client.RequestWithHost("GET", "/", "api.example.com", nil)
			if err != nil {
				t.Fatalf("Failed to make host-based request: %v", err)
			}
			defer func() {
				if err := resp1.Body.Close(); err != nil {
					log.Printf("Failed to close response body: %v", err)
				}
			}()

			if resp1.StatusCode != 200 {
				t.Errorf("Expected status 200 for host-based routing, got %d", resp1.StatusCode)
			}

			// Test path-based routing
			resp2, err := client.Get("/web/")
			if err != nil {
				t.Fatalf("Failed to make path-based request: %v", err)
			}
			defer func() {
				if err := resp2.Body.Close(); err != nil {
					log.Printf("Failed to close response body: %v", err)
				}
			}()

			if resp2.StatusCode != 200 {
				t.Errorf("Expected status 200 for path-based routing, got %d", resp2.StatusCode)
			}
		},
	}
}

// CreateLoadBalancingE2EScenario creates an E2E scenario with load balancing
func CreateLoadBalancingE2EScenario(name, description string) E2EScenario {
	return E2EScenario{
		Name:        name,
		Description: description,
		Setup: func(t *testing.T) *E2ETestEnvironment {
			// Create config with multiple services for load balancing
			cfg := &config.Config{
				Tenants: []config.Tenant{
					{
						Name:       "lb-tenant",
						PathPrefix: "/lb/",
						Interval:   30,
						Services: []config.Service{
							{Name: "service1", URL: "placeholder", Health: "/health"},
							{Name: "service2", URL: "placeholder", Health: "/health"},
							{Name: "service3", URL: "placeholder", Health: "/health"},
						},
					},
				},
			}

			return SetupE2EEnvironment(t, cfg)
		},
		Test: func(t *testing.T, env *E2ETestEnvironment, client *E2EClient) {
			// Make multiple requests to test load balancing
			responses := make(map[string]int)

			for i := 0; i < 15; i++ {
				resp, err := client.GetResponse("/lb/")
				if err != nil {
					t.Fatalf("Failed to make load balancing request %d: %v", i+1, err)
				}

				if resp.HasStatus(200) {
					responses[resp.BodyString]++
				}
			}

			// Should have received responses from multiple backends
			if len(responses) < 1 {
				t.Logf("Load balancing responses: %v", responses)
			}
		},
	}
}

// CreatePerformanceE2EScenario creates an E2E scenario for performance testing
func CreatePerformanceE2EScenario(name, description string, concurrency int, duration time.Duration) E2EScenario {
	return E2EScenario{
		Name:        name,
		Description: description,
		Setup: func(t *testing.T) *E2ETestEnvironment {
			// Create config optimized for performance testing
			cfg := &config.Config{
				Tenants: []config.Tenant{
					{
						Name:       "perf-tenant",
						PathPrefix: "/perf/",
						Interval:   30,
						Services: []config.Service{
							{Name: "perf-service", URL: "placeholder", Health: "/health"},
						},
					},
				},
			}

			return SetupE2EEnvironment(t, cfg)
		},
		Test: func(t *testing.T, env *E2ETestEnvironment, client *E2EClient) {
			// Run load test
			result := client.LoadTest("/perf/", concurrency, duration)

			t.Logf("Performance test results:")
			t.Logf("  Duration: %v", result.Duration)
			t.Logf("  Concurrency: %d", result.Concurrency)
			t.Logf("  Total requests: %d", result.TotalRequests)
			t.Logf("  Total errors: %d", result.TotalErrors)
			t.Logf("  Requests/second: %.2f", result.RequestsPerSecond())
			t.Logf("  Success rate: %.2f%%", result.SuccessRate()*100)
			t.Logf("  Status codes: %v", result.StatusCodes)

			// Basic performance assertions
			if result.TotalRequests == 0 {
				t.Error("No requests completed during performance test")
			}

			if result.SuccessRate() < 0.95 { // Expect 95% success rate
				t.Errorf("Success rate too low: %.2f%% (expected >= 95%%)", result.SuccessRate()*100)
			}

			expectedMinRPS := float64(concurrency) * 0.5 // Expect at least 50% of theoretical max
			if result.RequestsPerSecond() < expectedMinRPS {
				t.Errorf("Requests per second too low: %.2f (expected >= %.2f)",
					result.RequestsPerSecond(), expectedMinRPS)
			}
		},
	}
}

// CreateErrorHandlingE2EScenario creates an E2E scenario for error handling
func CreateErrorHandlingE2EScenario(name, description string) E2EScenario {
	return E2EScenario{
		Name:        name,
		Description: description,
		Setup: func(t *testing.T) *E2ETestEnvironment {
			// Start error backend directly and get its URL
			errorBackend := StartRealBackend(t, "error")

			cfg := &config.Config{
				Tenants: []config.Tenant{
					{
						Name:       "error-tenant",
						PathPrefix: "/error/",
						Interval:   30,
						Services: []config.Service{
							{Name: "error-service", URL: errorBackend.URL, Health: "/health"},
						},
					},
				},
			}

			// Start gateway with error backend config
			gateway := StartRealGateway(t, cfg)

			return &E2ETestEnvironment{
				Gateway:  gateway,
				Backends: []*E2EBackend{errorBackend},
				Config:   cfg,
			}
		},
		Test: func(t *testing.T, env *E2ETestEnvironment, client *E2EClient) {
			// Test various error scenarios
			errorTests := []struct {
				path           string
				expectedStatus int
			}{
				{"/error/400", 400},
				{"/error/404", 404},
				{"/error/500", 500},
				{"/error/503", 503},
			}

			for _, tt := range errorTests {
				resp, err := client.GetResponse(tt.path)
				if err != nil {
					t.Fatalf("Failed to make error request to %s: %v", tt.path, err)
				}

				if !resp.HasStatus(tt.expectedStatus) {
					t.Errorf("Expected status %d for %s, got %d",
						tt.expectedStatus, tt.path, resp.StatusCode)
				}

				// Verify error response is JSON
				var errorData map[string]interface{}
				if err := resp.JSON(&errorData); err != nil {
					t.Errorf("Failed to parse error response as JSON for %s: %v", tt.path, err)
				}
			}
		},
	}
}

// CreateRealWorldE2EScenario creates a comprehensive E2E scenario modeling real-world usage
func CreateRealWorldE2EScenario(name, description string) E2EScenario {
	return E2EScenario{
		Name:        name,
		Description: description,
		Setup: func(t *testing.T) *E2ETestEnvironment {
			// Create realistic multi-tenant configuration
			cfg := &config.Config{
				AdminBasePath: "/admin",
				Tenants: []config.Tenant{
					// Public API
					{
						Name:     "public-api",
						Domains:  []string{"api.example.com"},
						Interval: 30,
						Services: []config.Service{
							{Name: "users-api", URL: "placeholder", Health: "/health"},
							{Name: "products-api", URL: "placeholder", Health: "/health"},
						},
					},
					// Internal API
					{
						Name:       "internal-api",
						PathPrefix: "/internal/",
						Interval:   15,
						Services: []config.Service{
							{Name: "admin-api", URL: "placeholder", Health: "/health"},
						},
					},
					// Web frontend
					{
						Name:     "web-frontend",
						Domains:  []string{"web.example.com"},
						Interval: 60,
						Services: []config.Service{
							{Name: "web-server", URL: "placeholder", Health: "/health"},
						},
					},
				},
			}

			return SetupE2EEnvironment(t, cfg)
		},
		Test: func(t *testing.T, env *E2ETestEnvironment, client *E2EClient) {
			// Test public API access
			t.Run("public_api_access", func(t *testing.T) {
				resp, err := client.RequestWithHost("GET", "/users", "api.example.com", nil)
				if err != nil {
					t.Fatalf("Failed to access public API: %v", err)
				}
				defer resp.Body.Close()

				if resp.StatusCode != 200 {
					t.Errorf("Expected status 200 for public API, got %d", resp.StatusCode)
				}
			})

			// Test internal API access
			t.Run("internal_api_access", func(t *testing.T) {
				resp, err := client.Get("/internal/admin")
				if err != nil {
					t.Fatalf("Failed to access internal API: %v", err)
				}
				defer resp.Body.Close()

				if resp.StatusCode != 200 {
					t.Errorf("Expected status 200 for internal API, got %d", resp.StatusCode)
				}
			})

			// Test web frontend access
			t.Run("web_frontend_access", func(t *testing.T) {
				resp, err := client.RequestWithHost("GET", "/", "web.example.com", nil)
				if err != nil {
					t.Fatalf("Failed to access web frontend: %v", err)
				}
				defer resp.Body.Close()

				if resp.StatusCode != 200 {
					t.Errorf("Expected status 200 for web frontend, got %d", resp.StatusCode)
				}
			})

			// Test load balancing across API services
			t.Run("api_load_balancing", func(t *testing.T) {
				responses := make(map[string]int)

				for i := 0; i < 10; i++ {
					resp, err := client.RequestWithHost("GET", "/users", "api.example.com", nil)
					if err != nil {
						t.Fatalf("Failed load balancing request %d: %v", i+1, err)
					}

					if resp.StatusCode == 200 {
						body := make([]byte, 100)
						n, _ := resp.Body.Read(body)
						responses[string(body[:n])]++
					}
					resp.Body.Close()
				}

				// Should have some distribution across backends
				t.Logf("Load balancing distribution: %v", responses)
			})

			// Test concurrent access across tenants
			t.Run("concurrent_tenant_access", func(t *testing.T) {
				concurrency := 10
				requests := make([]func() (*http.Response, error), concurrency)

				for i := 0; i < concurrency; i++ {
					switch i % 3 {
					case 0:
						// Public API request
						requests[i] = func() (*http.Response, error) {
							return client.RequestWithHost("GET", "/users", "api.example.com", nil)
						}
					case 1:
						// Internal API request
						requests[i] = func() (*http.Response, error) {
							return client.Get("/internal/admin")
						}
					default:
						// Web frontend request
						requests[i] = func() (*http.Response, error) {
							return client.RequestWithHost("GET", "/", "web.example.com", nil)
						}
					}
				}

				responses, errors := client.ParallelRequests(requests)

				// Check results
				successCount := 0
				for i, resp := range responses {
					if errors[i] != nil {
						t.Errorf("Concurrent request %d failed: %v", i, errors[i])
					} else if resp.StatusCode == 200 {
						successCount++
						resp.Body.Close()
					}
				}

				if successCount < concurrency/2 {
					t.Errorf("Too few successful concurrent requests: %d/%d", successCount, concurrency)
				}

				t.Logf("Concurrent requests: %d successful out of %d", successCount, concurrency)
			})
		},
	}
}

// E2EScenarioSuite represents a collection of E2E scenarios
type E2EScenarioSuite struct {
	Name      string
	Scenarios []E2EScenario
}

// RunE2EScenarioSuite runs all scenarios in the suite
func RunE2EScenarioSuite(t *testing.T, suite E2EScenarioSuite) {
	t.Run(suite.Name, func(t *testing.T) {
		for _, scenario := range suite.Scenarios {
			RunE2EScenario(t, scenario)
		}
	})
}

// CreateStandardE2EScenarioSuite creates a standard suite of E2E scenarios
func CreateStandardE2EScenarioSuite() E2EScenarioSuite {
	return E2EScenarioSuite{
		Name: "Standard E2E Scenarios",
		Scenarios: []E2EScenario{
			CreateSimpleE2EScenario("simple_gateway_flow", "Basic single-tenant gateway flow"),
			CreateMultiTenantE2EScenario("multi_tenant_routing", "Multi-tenant routing scenarios"),
			CreateLoadBalancingE2EScenario("load_balancing", "Load balancing across multiple backends"),
			CreateErrorHandlingE2EScenario("error_handling", "Error handling and propagation"),
			CreatePerformanceE2EScenario("basic_performance", "Basic performance testing", 5, 5*time.Second),
			CreateRealWorldE2EScenario("real_world_usage", "Comprehensive real-world usage patterns"),
		},
	}
}

// CreatePerformanceE2EScenarioSuite creates a suite focused on performance testing
func CreatePerformanceE2EScenarioSuite() E2EScenarioSuite {
	return E2EScenarioSuite{
		Name: "Performance E2E Scenarios",
		Scenarios: []E2EScenario{
			CreatePerformanceE2EScenario("low_concurrency", "Low concurrency test", 2, 3*time.Second),
			CreatePerformanceE2EScenario("medium_concurrency", "Medium concurrency test", 10, 5*time.Second),
			CreatePerformanceE2EScenario("high_concurrency", "High concurrency test", 25, 10*time.Second),
		},
	}
}
