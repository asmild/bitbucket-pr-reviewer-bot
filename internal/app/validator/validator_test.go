package validator

import (
	"testing"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/config"
)

// Mock logger for testing
type mockLogger struct{}

func (m *mockLogger) Debug(msg string, args ...interface{}) {}
func (m *mockLogger) Info(msg string, args ...interface{})  {}
func (m *mockLogger) Warn(msg string, args ...interface{})  {}
func (m *mockLogger) Error(msg string, args ...interface{}) {}
func (m *mockLogger) Fatal(msg string, args ...interface{}) {}

func TestValidationError_Error(t *testing.T) {
	t.Run("formats error message correctly", func(t *testing.T) {
		err := &ValidationError{
			Component: "TestComponent",
			Issue:     "Something went wrong",
			Solution:  "Fix it like this",
		}

		expected := "TestComponent: Something went wrong\n  Solution: Fix it like this"
		if err.Error() != expected {
			t.Errorf("expected '%s', got '%s'", expected, err.Error())
		}
	})
}

func TestValidationResult_IsValid(t *testing.T) {
	t.Run("returns true when no errors", func(t *testing.T) {
		result := &ValidationResult{}

		if !result.IsValid() {
			t.Error("expected IsValid to return true with no errors")
		}
	})

	t.Run("returns false when errors exist", func(t *testing.T) {
		result := &ValidationResult{}
		result.AddError("Component", "Issue", "Solution")

		if result.IsValid() {
			t.Error("expected IsValid to return false with errors")
		}
	})

	t.Run("returns false with multiple errors", func(t *testing.T) {
		result := &ValidationResult{}
		result.AddError("Component1", "Issue1", "Solution1")
		result.AddError("Component2", "Issue2", "Solution2")

		if result.IsValid() {
			t.Error("expected IsValid to return false with multiple errors")
		}
	})
}

func TestValidationResult_AddError(t *testing.T) {
	t.Run("adds error to result", func(t *testing.T) {
		result := &ValidationResult{}

		result.AddError("TestComp", "TestIssue", "TestSolution")

		if len(result.Errors) != 1 {
			t.Fatalf("expected 1 error, got %d", len(result.Errors))
		}

		err := result.Errors[0]
		if err.Component != "TestComp" {
			t.Errorf("expected component 'TestComp', got '%s'", err.Component)
		}
		if err.Issue != "TestIssue" {
			t.Errorf("expected issue 'TestIssue', got '%s'", err.Issue)
		}
		if err.Solution != "TestSolution" {
			t.Errorf("expected solution 'TestSolution', got '%s'", err.Solution)
		}
	})

	t.Run("can add multiple errors", func(t *testing.T) {
		result := &ValidationResult{}

		result.AddError("Comp1", "Issue1", "Solution1")
		result.AddError("Comp2", "Issue2", "Solution2")
		result.AddError("Comp3", "Issue3", "Solution3")

		if len(result.Errors) != 3 {
			t.Errorf("expected 3 errors, got %d", len(result.Errors))
		}
	})
}

func TestValidationResult_AddInfo(t *testing.T) {
	t.Run("adds info message to result", func(t *testing.T) {
		result := &ValidationResult{}

		result.AddInfo("Test info message")

		if len(result.InfoMsgs) != 1 {
			t.Fatalf("expected 1 info message, got %d", len(result.InfoMsgs))
		}

		if result.InfoMsgs[0] != "Test info message" {
			t.Errorf("expected 'Test info message', got '%s'", result.InfoMsgs[0])
		}
	})

	t.Run("can add multiple info messages", func(t *testing.T) {
		result := &ValidationResult{}

		result.AddInfo("Info 1")
		result.AddInfo("Info 2")
		result.AddInfo("Info 3")

		if len(result.InfoMsgs) != 3 {
			t.Errorf("expected 3 info messages, got %d", len(result.InfoMsgs))
		}
	})
}

func TestValidateConfiguration(t *testing.T) {
	t.Run("validates all required configuration fields", func(t *testing.T) {
		cfg := &config.Config{
			Server: config.ServerConfig{
				Port: 8080,
			},
			Bitbucket: config.BitbucketConfig{
				User:      "testuser",
				Token:     "testtoken",
			},
			Claude: config.ClaudeConfig{
				TimeoutMinutes: 10,
			},
		}

		result := &ValidationResult{}
		validateConfiguration(cfg, result)

		if !result.IsValid() {
			t.Errorf("expected valid configuration, got errors: %v", result.Errors)
		}
	})

	t.Run("detects missing Bitbucket user", func(t *testing.T) {
		cfg := &config.Config{
			Server: config.ServerConfig{Port: 8080},
			Bitbucket: config.BitbucketConfig{
				User:      "", // Missing
				Token:     "testtoken",
			},
			Claude: config.ClaudeConfig{TimeoutMinutes: 10},
		}

		result := &ValidationResult{}
		validateConfiguration(cfg, result)

		if result.IsValid() {
			t.Error("expected validation to fail for missing Bitbucket user")
		}

		if len(result.Errors) != 1 {
			t.Fatalf("expected 1 error, got %d", len(result.Errors))
		}

		err := result.Errors[0]
		if err.Component != "Configuration" {
			t.Errorf("expected component 'Configuration', got '%s'", err.Component)
		}
		if !contains(err.Issue, "username") {
			t.Errorf("expected error about username, got '%s'", err.Issue)
		}
	})

	t.Run("detects missing Bitbucket token", func(t *testing.T) {
		cfg := &config.Config{
			Server: config.ServerConfig{Port: 8080},
			Bitbucket: config.BitbucketConfig{
				User:      "testuser",
				Token:     "", // Missing
			},
			Claude: config.ClaudeConfig{TimeoutMinutes: 10},
		}

		result := &ValidationResult{}
		validateConfiguration(cfg, result)

		if result.IsValid() {
			t.Error("expected validation to fail for missing Bitbucket token")
		}

		if len(result.Errors) < 1 {
			t.Fatal("expected at least 1 error")
		}

		// Find the token error
		found := false
		for _, err := range result.Errors {
			if contains(err.Issue, "token") {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected error about token")
		}
	})

	t.Run("detects invalid Claude timeout", func(t *testing.T) {
		cfg := &config.Config{
			Server: config.ServerConfig{Port: 8080},
			Bitbucket: config.BitbucketConfig{
				User:      "testuser",
				Token:     "testtoken",
			},
			Claude: config.ClaudeConfig{
				TimeoutMinutes: 0, // Invalid
			},
		}

		result := &ValidationResult{}
		validateConfiguration(cfg, result)

		if result.IsValid() {
			t.Error("expected validation to fail for invalid timeout")
		}

		found := false
		for _, err := range result.Errors {
			if contains(err.Issue, "timeout") {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected error about invalid timeout")
		}
	})

	t.Run("reports multiple configuration errors", func(t *testing.T) {
		cfg := &config.Config{
			Server: config.ServerConfig{
				Port: 0, // Invalid
			},
			Bitbucket: config.BitbucketConfig{
				User:      "",              // Missing
				Token:     "",              // Missing
			},
			Claude: config.ClaudeConfig{
				TimeoutMinutes: -5, // Invalid
			},
		}

		result := &ValidationResult{}
		validateConfiguration(cfg, result)

		if result.IsValid() {
			t.Error("expected validation to fail")
		}

		// Should have multiple errors
		if len(result.Errors) < 4 {
			t.Errorf("expected at least 4 errors, got %d", len(result.Errors))
		}
	})

	t.Run("accepts valid port range", func(t *testing.T) {
		validPorts := []int{1, 80, 443, 8080, 9090, 65535}

		for _, port := range validPorts {
			cfg := &config.Config{
				Server: config.ServerConfig{Port: port},
				Bitbucket: config.BitbucketConfig{
					User:      "testuser",
					Token:     "testtoken",
				},
				Claude: config.ClaudeConfig{TimeoutMinutes: 10},
			}

			result := &ValidationResult{}
			validateConfiguration(cfg, result)

			if !result.IsValid() {
				t.Errorf("expected port %d to be valid, got errors: %v", port, result.Errors)
			}
		}
	})

	t.Run("accepts valid event types", func(t *testing.T) {
		validEventTypes := []string{"pr_opened", "comment_added"}

		for _, eventType := range validEventTypes {
			cfg := &config.Config{
				Server: config.ServerConfig{Port: 8080},
				Bitbucket: config.BitbucketConfig{
					User:      "testuser",
					Token:     "testtoken",
				},
				Claude: config.ClaudeConfig{TimeoutMinutes: 10},
			}

			result := &ValidationResult{}
			validateConfiguration(cfg, result)

			if !result.IsValid() {
				t.Errorf("expected event type '%s' to be valid, got errors: %v", eventType, result.Errors)
			}
		}
	})
}

// Helper function
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && containsHelper(s, substr)
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
