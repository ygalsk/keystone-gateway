package types

import (
	"testing"
	"time"
)

// TestHealthTracker_TwoStrikeRule tests the 2-strike rule with table-driven approach
func TestHealthTracker_TwoStrikeRule(t *testing.T) {
	tests := []struct {
		name     string
		actions  []string // "fail", "success", "degrade"
		expected HealthStatus
	}{
		{
			name:     "single_failure_stays_unknown",
			actions:  []string{"fail"},
			expected: HealthStatusUnknown, // First failure doesn't change status from unknown
		},
		{
			name:     "two_failures_become_unhealthy",
			actions:  []string{"fail", "fail"},
			expected: HealthStatusUnhealthy,
		},
		{
			name:     "recovery_after_failure",
			actions:  []string{"fail", "fail", "success"},
			expected: HealthStatusHealthy,
		},
		{
			name:     "failure_resets_count_after_success",
			actions:  []string{"fail", "success", "fail"},
			expected: HealthStatusHealthy,
		},
		{
			name:     "multiple_failures_stay_unhealthy",
			actions:  []string{"fail", "fail", "fail"},
			expected: HealthStatusUnhealthy,
		},
		{
			name:     "degraded_status_maintained",
			actions:  []string{"degrade"},
			expected: HealthStatusDegraded,
		},
		{
			name:     "degraded_then_success_becomes_healthy",
			actions:  []string{"degrade", "success"},
			expected: HealthStatusHealthy,
		},
		{
			name:     "degraded_then_failure_becomes_unhealthy",
			actions:  []string{"degrade", "fail"},
			expected: HealthStatusUnhealthy,
		},
		{
			name:     "success_only_stays_healthy",
			actions:  []string{"success"},
			expected: HealthStatusHealthy,
		},
		{
			name:     "no_actions_stays_unknown",
			actions:  []string{},
			expected: HealthStatusUnknown,
		},
		{
			name:     "complex_pattern_with_recovery",
			actions:  []string{"fail", "fail", "success", "fail", "success"},
			expected: HealthStatusHealthy,
		},
		{
			name:     "alternate_fail_success_pattern",
			actions:  []string{"fail", "success", "fail", "success", "fail", "success"},
			expected: HealthStatusHealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ht := NewHealthTracker()

			for _, action := range tt.actions {
				switch action {
				case "fail":
					ht.MarkUnhealthy()
				case "success":
					ht.MarkHealthy()
				case "degrade":
					ht.MarkDegraded()
				}
			}

			if got := ht.Status(); got != tt.expected {
				t.Errorf("Status() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestHealthTracker_IsHealthy tests the IsHealthy convenience method
func TestHealthTracker_IsHealthy(t *testing.T) {
	tests := []struct {
		name     string
		status   HealthStatus
		expected bool
	}{
		{
			name:     "healthy_status_returns_true",
			status:   HealthStatusHealthy,
			expected: true,
		},
		{
			name:     "unhealthy_status_returns_false",
			status:   HealthStatusUnhealthy,
			expected: false,
		},
		{
			name:     "degraded_status_returns_false",
			status:   HealthStatusDegraded,
			expected: false,
		},
		{
			name:     "unknown_status_returns_false",
			status:   HealthStatusUnknown,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ht := NewHealthTracker()
			
			// Set the desired status
			switch tt.status {
			case HealthStatusHealthy:
				ht.MarkHealthy()
			case HealthStatusUnhealthy:
				ht.MarkUnhealthy()
				ht.MarkUnhealthy() // 2-strike rule
			case HealthStatusDegraded:
				ht.MarkDegraded()
			case HealthStatusUnknown:
				// Keep default status
			}

			if got := ht.IsHealthy(); got != tt.expected {
				t.Errorf("IsHealthy() = %v, want %v for status %v", got, tt.expected, tt.status)
			}
		})
	}
}

// TestHealthTracker_FailureCount tests failure count tracking
func TestHealthTracker_FailureCount(t *testing.T) {
	tests := []struct {
		name            string
		actions         []string
		expectedCount   int32
		expectedStatus  HealthStatus
	}{
		{
			name:           "no_failures",
			actions:        []string{"success"},
			expectedCount:  0,
			expectedStatus: HealthStatusHealthy,
		},
		{
			name:           "single_failure",
			actions:        []string{"fail"},
			expectedCount:  1,
			expectedStatus: HealthStatusUnknown, // Still unknown after 1 failure
		},
		{
			name:           "two_failures",
			actions:        []string{"fail", "fail"},
			expectedCount:  2,
			expectedStatus: HealthStatusUnhealthy,
		},
		{
			name:           "failure_reset_by_success",
			actions:        []string{"fail", "fail", "success"},
			expectedCount:  0,
			expectedStatus: HealthStatusHealthy,
		},
		{
			name:           "degraded_counts_as_failure",
			actions:        []string{"degrade"},
			expectedCount:  1,
			expectedStatus: HealthStatusDegraded,
		},
		{
			name:           "degraded_then_fail_reaches_threshold",
			actions:        []string{"degrade", "fail"},
			expectedCount:  2,
			expectedStatus: HealthStatusUnhealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ht := NewHealthTracker()

			for _, action := range tt.actions {
				switch action {
				case "fail":
					ht.MarkUnhealthy()
				case "success":
					ht.MarkHealthy()
				case "degrade":
					ht.MarkDegraded()
				}
			}

			if got := ht.FailureCount(); got != tt.expectedCount {
				t.Errorf("FailureCount() = %d, want %d", got, tt.expectedCount)
			}

			if got := ht.Status(); got != tt.expectedStatus {
				t.Errorf("Status() = %v, want %v", got, tt.expectedStatus)
			}
		})
	}
}

// TestHealthTracker_LastCheck tests last check timestamp tracking
func TestHealthTracker_LastCheck(t *testing.T) {
	ht := NewHealthTracker()

	// Initially should be zero time
	if !ht.LastCheck().IsZero() {
		t.Error("Expected LastCheck() to return zero time initially")
	}

	beforeMark := time.Now()
	ht.MarkHealthy()
	afterMark := time.Now()

	lastCheck := ht.LastCheck()
	if lastCheck.Before(beforeMark) || lastCheck.After(afterMark) {
		t.Error("LastCheck() timestamp not within expected range after MarkHealthy")
	}

	// Mark unhealthy and verify timestamp updates
	time.Sleep(1 * time.Millisecond) // Ensure different timestamp
	beforeMark2 := time.Now()
	ht.MarkUnhealthy()
	afterMark2 := time.Now()

	lastCheck2 := ht.LastCheck()
	if lastCheck2.Before(beforeMark2) || lastCheck2.After(afterMark2) {
		t.Error("LastCheck() timestamp not updated after MarkUnhealthy")
	}

	if !lastCheck2.After(lastCheck) {
		t.Error("LastCheck() should be later after second mark")
	}
}

// TestHealthTracker_Stats tests comprehensive statistics gathering
func TestHealthTracker_Stats(t *testing.T) {
	tests := []struct {
		name                    string
		actions                 []string
		expectedStatus          HealthStatus
		expectedTotalChecks     int64
		expectedSuccessfulChecks int64
		expectedFailureCount    int32
	}{
		{
			name:                    "only_successes",
			actions:                 []string{"success", "success", "success"},
			expectedStatus:          HealthStatusHealthy,
			expectedTotalChecks:     3,
			expectedSuccessfulChecks: 3,
			expectedFailureCount:    0,
		},
		{
			name:                    "mixed_with_final_success",
			actions:                 []string{"fail", "success", "fail", "success"},
			expectedStatus:          HealthStatusHealthy,
			expectedTotalChecks:     4,
			expectedSuccessfulChecks: 2,
			expectedFailureCount:    0, // Reset by last success
		},
		{
			name:                    "mixed_with_final_failure",
			actions:                 []string{"success", "fail", "fail"},
			expectedStatus:          HealthStatusUnhealthy,
			expectedTotalChecks:     3,
			expectedSuccessfulChecks: 1,
			expectedFailureCount:    2,
		},
		{
			name:                    "degraded_state",
			actions:                 []string{"degrade", "degrade"},
			expectedStatus:          HealthStatusDegraded,
			expectedTotalChecks:     2,
			expectedSuccessfulChecks: 0,
			expectedFailureCount:    1, // Degraded keeps count at 1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ht := NewHealthTracker()

			for _, action := range tt.actions {
				switch action {
				case "fail":
					ht.MarkUnhealthy()
				case "success":
					ht.MarkHealthy()
				case "degrade":
					ht.MarkDegraded()
				}
			}

			stats := ht.Stats()

			if stats.Status != tt.expectedStatus {
				t.Errorf("Stats().Status = %v, want %v", stats.Status, tt.expectedStatus)
			}

			if stats.TotalChecks != tt.expectedTotalChecks {
				t.Errorf("Stats().TotalChecks = %d, want %d", stats.TotalChecks, tt.expectedTotalChecks)
			}

			if stats.SuccessfulChecks != tt.expectedSuccessfulChecks {
				t.Errorf("Stats().SuccessfulChecks = %d, want %d", stats.SuccessfulChecks, tt.expectedSuccessfulChecks)
			}

			if stats.FailureCount != tt.expectedFailureCount {
				t.Errorf("Stats().FailureCount = %d, want %d", stats.FailureCount, tt.expectedFailureCount)
			}

			// Test success rate calculation
			expectedSuccessRate := float64(0)
			if tt.expectedTotalChecks > 0 {
				expectedSuccessRate = float64(tt.expectedSuccessfulChecks) / float64(tt.expectedTotalChecks)
			}

			if stats.SuccessRate != expectedSuccessRate {
				t.Errorf("Stats().SuccessRate = %f, want %f", stats.SuccessRate, expectedSuccessRate)
			}

			// LastCheck should be recent if any actions were performed
			if len(tt.actions) > 0 && stats.LastCheck.IsZero() {
				t.Error("Stats().LastCheck should not be zero after performing actions")
			}
		})
	}
}

// TestHealthStatus_String tests string representation of health statuses
func TestHealthStatus_String(t *testing.T) {
	tests := []struct {
		status   HealthStatus
		expected string
	}{
		{HealthStatusHealthy, "healthy"},
		{HealthStatusUnhealthy, "unhealthy"},
		{HealthStatusDegraded, "degraded"},
		{HealthStatusUnknown, "unknown"},
		{HealthStatus(999), "unknown"}, // Invalid status
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.status.String(); got != tt.expected {
				t.Errorf("String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// TestHealthConfig_Validation tests health configuration validation
func TestHealthConfig_Validation(t *testing.T) {
	tests := []struct {
		name      string
		config    HealthConfig
		wantError bool
		errorField string // Expected field in error
	}{
		{
			name: "valid_config",
			config: HealthConfig{
				Enabled:             true,
				Path:                "/health",
				Interval:            30 * time.Second,
				Timeout:             5 * time.Second,
				FailureThreshold:    2,
				SuccessThreshold:    1,
				ExpectedStatusCodes: []int{200, 204},
			},
			wantError: false,
		},
		{
			name: "zero_interval",
			config: HealthConfig{
				Interval:            0,
				Timeout:             5 * time.Second,
				FailureThreshold:    2,
				SuccessThreshold:    1,
				ExpectedStatusCodes: []int{200},
			},
			wantError:  true,
			errorField: "interval",
		},
		{
			name: "negative_interval",
			config: HealthConfig{
				Interval:            -1 * time.Second,
				Timeout:             5 * time.Second,
				FailureThreshold:    2,
				SuccessThreshold:    1,
				ExpectedStatusCodes: []int{200},
			},
			wantError:  true,
			errorField: "interval",
		},
		{
			name: "zero_timeout",
			config: HealthConfig{
				Interval:            30 * time.Second,
				Timeout:             0,
				FailureThreshold:    2,
				SuccessThreshold:    1,
				ExpectedStatusCodes: []int{200},
			},
			wantError:  true,
			errorField: "timeout",
		},
		{
			name: "timeout_greater_than_interval",
			config: HealthConfig{
				Interval:            5 * time.Second,
				Timeout:             10 * time.Second,
				FailureThreshold:    2,
				SuccessThreshold:    1,
				ExpectedStatusCodes: []int{200},
			},
			wantError:  true,
			errorField: "timeout",
		},
		{
			name: "zero_failure_threshold",
			config: HealthConfig{
				Interval:            30 * time.Second,
				Timeout:             5 * time.Second,
				FailureThreshold:    0,
				SuccessThreshold:    1,
				ExpectedStatusCodes: []int{200},
			},
			wantError:  true,
			errorField: "failure_threshold",
		},
		{
			name: "zero_success_threshold",
			config: HealthConfig{
				Interval:            30 * time.Second,
				Timeout:             5 * time.Second,
				FailureThreshold:    2,
				SuccessThreshold:    0,
				ExpectedStatusCodes: []int{200},
			},
			wantError:  true,
			errorField: "success_threshold",
		},
		{
			name: "empty_expected_status_codes",
			config: HealthConfig{
				Interval:            30 * time.Second,
				Timeout:             5 * time.Second,
				FailureThreshold:    2,
				SuccessThreshold:    1,
				ExpectedStatusCodes: []int{},
			},
			wantError:  true,
			errorField: "expected_status_codes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.wantError {
				if err == nil {
					t.Error("Expected validation error, got nil")
					return
				}

				// Check if error contains expected field
				configErr, ok := err.(*ConfigError)
				if !ok {
					t.Errorf("Expected ConfigError, got %T", err)
					return
				}

				if configErr.Field != tt.errorField {
					t.Errorf("Expected error field %q, got %q", tt.errorField, configErr.Field)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected validation error: %v", err)
				}
			}
		})
	}
}

// TestDefaultHealthConfig tests the default configuration factory
func TestDefaultHealthConfig(t *testing.T) {
	config := DefaultHealthConfig()

	// Test all expected default values
	if !config.Enabled {
		t.Error("Expected default config to be enabled")
	}

	if config.Path != "/health" {
		t.Errorf("Expected default path '/health', got %q", config.Path)
	}

	if config.Interval != 30*time.Second {
		t.Errorf("Expected default interval 30s, got %v", config.Interval)
	}

	if config.Timeout != 5*time.Second {
		t.Errorf("Expected default timeout 5s, got %v", config.Timeout)
	}

	if config.FailureThreshold != 2 {
		t.Errorf("Expected default failure threshold 2, got %d", config.FailureThreshold)
	}

	if config.SuccessThreshold != 1 {
		t.Errorf("Expected default success threshold 1, got %d", config.SuccessThreshold)
	}

	expectedStatusCodes := []int{200, 204}
	if len(config.ExpectedStatusCodes) != len(expectedStatusCodes) {
		t.Errorf("Expected %d status codes, got %d", len(expectedStatusCodes), len(config.ExpectedStatusCodes))
	}

	for i, expected := range expectedStatusCodes {
		if i >= len(config.ExpectedStatusCodes) || config.ExpectedStatusCodes[i] != expected {
			t.Errorf("Expected status code %d at index %d, got %d", expected, i, config.ExpectedStatusCodes[i])
		}
	}

	// Verify default config is valid
	if err := config.Validate(); err != nil {
		t.Errorf("Default config should be valid, got error: %v", err)
	}
}

// TestConfigError tests the ConfigError type
func TestConfigError(t *testing.T) {
	err := &ConfigError{
		Field:   "test_field",
		Message: "test message",
	}

	expected := "config error in field 'test_field': test message"
	if got := err.Error(); got != expected {
		t.Errorf("Error() = %q, want %q", got, expected)
	}
}

// TestHealthTracker_Concurrent tests concurrent access to health tracker
func TestHealthTracker_Concurrent(t *testing.T) {
	ht := NewHealthTracker()
	done := make(chan bool)
	const numGoroutines = 100

	// Start multiple goroutines performing health operations
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()

			// Perform various operations
			if id%3 == 0 {
				ht.MarkHealthy()
			} else if id%3 == 1 {
				ht.MarkUnhealthy()
			} else {
				ht.MarkDegraded()
			}

			// Read operations
			_ = ht.Status()
			_ = ht.IsHealthy()
			_ = ht.FailureCount()
			_ = ht.LastCheck()
			_ = ht.Stats()
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify final state is consistent
	stats := ht.Stats()
	if stats.TotalChecks != int64(numGoroutines) {
		t.Errorf("Expected %d total checks, got %d", numGoroutines, stats.TotalChecks)
	}

	// Verify that the last operation determines the final state
	status := ht.Status()
	if status != HealthStatusHealthy && status != HealthStatusUnhealthy && status != HealthStatusDegraded {
		t.Errorf("Unexpected final status: %v", status)
	}
}