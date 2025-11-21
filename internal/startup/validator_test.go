package startup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/config"
)

func TestValidationResult_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		errors   []*ValidationError
		expected bool
	}{
		{
			name:     "no errors",
			errors:   []*ValidationError{},
			expected: true,
		},
		{
			name: "has errors",
			errors: []*ValidationError{
				{Component: "Test", Issue: "test issue", Solution: "test solution"},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &ValidationResult{Errors: tt.errors}
			if got := result.IsValid(); got != tt.expected {
				t.Errorf("IsValid() = %v, expected %v", got, tt.expected)
			}
		})
	}
}

func TestValidationResult_AddError(t *testing.T) {
	result := &ValidationResult{}

	result.AddError("Component1", "Issue1", "Solution1")
	if len(result.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(result.Errors))
	}

	result.AddError("Component2", "Issue2", "Solution2")
	if len(result.Errors) != 2 {
		t.Errorf("Expected 2 errors, got %d", len(result.Errors))
	}

	if result.Errors[0].Component != "Component1" {
		t.Errorf("Expected first error component 'Component1', got '%s'", result.Errors[0].Component)
	}
}

func TestValidationResult_LogErrors(t *testing.T) {
	tests := []struct {
		name   string
		errors []*ValidationError
	}{
		{
			name:   "no errors",
			errors: []*ValidationError{},
		},
		{
			name: "single error",
			errors: []*ValidationError{
				{Component: "Test", Issue: "test issue", Solution: "test solution"},
			},
		},
		{
			name: "multiple errors",
			errors: []*ValidationError{
				{Component: "Test1", Issue: "issue1", Solution: "solution1"},
				{Component: "Test2", Issue: "issue2", Solution: "solution2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &ValidationResult{Errors: tt.errors}
			// LogErrors just logs, no return value to test
			// This test verifies it doesn't panic
			result.LogErrors()
		})
	}
}

func TestValidateConfiguration(t *testing.T) {
	tests := []struct {
		name          string
		cfg           *config.Config
		expectErrors  bool
		errorContains string
	}{
		{
			name: "valid configuration",
			cfg: &config.Config{
				Server: config.ServerConfig{Port: 8080},
				Bitbucket: config.BitbucketConfig{
					User:      "test-user",
					Token:     "test-token",
					EventType: "pr_opened",
				},
				Claude: config.ClaudeConfig{
					TimeoutMinutes: 10,
				},
			},
			expectErrors: false,
		},
		{
			name: "missing bitbucket user",
			cfg: &config.Config{
				Server: config.ServerConfig{Port: 8080},
				Bitbucket: config.BitbucketConfig{
					User:      "",
					Token:     "test-token",
					EventType: "pr_opened",
				},
				Claude: config.ClaudeConfig{TimeoutMinutes: 10},
			},
			expectErrors:  true,
			errorContains: "Bitbucket user",
		},
		{
			name: "missing bitbucket token",
			cfg: &config.Config{
				Server: config.ServerConfig{Port: 8080},
				Bitbucket: config.BitbucketConfig{
					User:      "test-user",
					Token:     "",
					EventType: "pr_opened",
				},
				Claude: config.ClaudeConfig{TimeoutMinutes: 10},
			},
			expectErrors:  true,
			errorContains: "Bitbucket token",
		},
		{
			name: "invalid event type",
			cfg: &config.Config{
				Server: config.ServerConfig{Port: 8080},
				Bitbucket: config.BitbucketConfig{
					User:      "test-user",
					Token:     "test-token",
					EventType: "invalid",
				},
				Claude: config.ClaudeConfig{TimeoutMinutes: 10},
			},
			expectErrors:  true,
			errorContains: "Invalid event_type",
		},
		{
			name: "invalid port - too low",
			cfg: &config.Config{
				Server: config.ServerConfig{Port: 0},
				Bitbucket: config.BitbucketConfig{
					User:      "test-user",
					Token:     "test-token",
					EventType: "pr_opened",
				},
				Claude: config.ClaudeConfig{TimeoutMinutes: 10},
			},
			expectErrors:  true,
			errorContains: "Invalid server port",
		},
		{
			name: "invalid port - too high",
			cfg: &config.Config{
				Server: config.ServerConfig{Port: 99999},
				Bitbucket: config.BitbucketConfig{
					User:      "test-user",
					Token:     "test-token",
					EventType: "pr_opened",
				},
				Claude: config.ClaudeConfig{TimeoutMinutes: 10},
			},
			expectErrors:  true,
			errorContains: "Invalid server port",
		},
		{
			name: "invalid claude timeout",
			cfg: &config.Config{
				Server: config.ServerConfig{Port: 8080},
				Bitbucket: config.BitbucketConfig{
					User:      "test-user",
					Token:     "test-token",
					EventType: "pr_opened",
				},
				Claude: config.ClaudeConfig{TimeoutMinutes: 0},
			},
			expectErrors:  true,
			errorContains: "Invalid Claude timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &ValidationResult{}
			validateConfiguration(tt.cfg, result)

			if tt.expectErrors && result.IsValid() {
				t.Error("Expected validation errors but got none")
			}

			if !tt.expectErrors && !result.IsValid() {
				t.Errorf("Expected no validation errors but got: %v", result.Errors)
			}

			if tt.errorContains != "" {
				found := false
				for _, err := range result.Errors {
					if containsString(err.Issue, tt.errorContains) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected error containing '%s', but didn't find it in: %v", tt.errorContains, result.Errors)
				}
			}
		})
	}
}

func TestValidateTemplates(t *testing.T) {
	// Create temporary test directory
	tmpDir := t.TempDir()

	tests := []struct {
		name          string
		setup         func() *config.Config
		expectErrors  bool
		errorContains string
	}{
		{
			name: "templates directory doesn't exist",
			setup: func() *config.Config {
				return &config.Config{
					Templates: config.TemplatesConfig{
						Directory: filepath.Join(tmpDir, "nonexistent"),
						Default:   "default",
					},
				}
			},
			expectErrors:  true,
			errorContains: "Templates directory does not exist",
		},
		{
			name: "default template doesn't exist",
			setup: func() *config.Config {
				templatesDir := filepath.Join(tmpDir, "templates1")
				os.MkdirAll(templatesDir, 0755)
				return &config.Config{
					Templates: config.TemplatesConfig{
						Directory: templatesDir,
						Default:   "nonexistent",
					},
				}
			},
			expectErrors:  true,
			errorContains: "Default template directory does not exist",
		},
		{
			name: "default template missing prompt.md",
			setup: func() *config.Config {
				templatesDir := filepath.Join(tmpDir, "templates2")
				defaultDir := filepath.Join(templatesDir, "default")
				os.MkdirAll(defaultDir, 0755)
				return &config.Config{
					Templates: config.TemplatesConfig{
						Directory: templatesDir,
						Default:   "default",
					},
				}
			},
			expectErrors:  true,
			errorContains: "Default template missing prompt.md",
		},
		{
			name: "default template prompt.md is empty",
			setup: func() *config.Config {
				templatesDir := filepath.Join(tmpDir, "templates3")
				defaultDir := filepath.Join(templatesDir, "default")
				os.MkdirAll(defaultDir, 0755)
				promptFile := filepath.Join(defaultDir, "prompt.md")
				os.WriteFile(promptFile, []byte(""), 0644)
				return &config.Config{
					Templates: config.TemplatesConfig{
						Directory: templatesDir,
						Default:   "default",
					},
				}
			},
			expectErrors:  true,
			errorContains: "Default template is empty",
		},
		{
			name: "valid template configuration",
			setup: func() *config.Config {
				templatesDir := filepath.Join(tmpDir, "templates4")
				defaultDir := filepath.Join(templatesDir, "default")
				os.MkdirAll(defaultDir, 0755)
				promptFile := filepath.Join(defaultDir, "prompt.md")
				os.WriteFile(promptFile, []byte("# Valid template content"), 0644)
				return &config.Config{
					Templates: config.TemplatesConfig{
						Directory: templatesDir,
						Default:   "default",
					},
				}
			},
			expectErrors: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.setup()
			result := &ValidationResult{}
			validateTemplates(cfg, result)

			if tt.expectErrors && result.IsValid() {
				t.Error("Expected validation errors but got none")
			}

			if !tt.expectErrors && !result.IsValid() {
				t.Errorf("Expected no validation errors but got: %v", result.Errors)
			}

			if tt.errorContains != "" {
				found := false
				for _, err := range result.Errors {
					if containsString(err.Issue, tt.errorContains) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected error containing '%s', but didn't find it in: %v", tt.errorContains, result.Errors)
				}
			}
		})
	}
}

func TestEnsureDirectoryWritable(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name      string
		dir       string
		expectErr bool
	}{
		{
			name:      "create new directory",
			dir:       filepath.Join(tmpDir, "newdir"),
			expectErr: false,
		},
		{
			name:      "existing writable directory",
			dir:       tmpDir,
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ensureDirectoryWritable(tt.dir)

			if tt.expectErr && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectErr && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			// Verify directory exists and is writable
			if !tt.expectErr {
				if _, err := os.Stat(tt.dir); os.IsNotExist(err) {
					t.Errorf("Directory was not created: %s", tt.dir)
				}
			}
		})
	}
}

func TestValidateDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name         string
		cfg          *config.Config
		expectErrors bool
	}{
		{
			name: "all directories writable",
			cfg: &config.Config{
				Logging: config.LoggingConfig{
					EnableFile: false, // Don't check logs directory
				},
				Metrics: config.MetricsConfig{
					Persistence: config.MetricsPersistenceConfig{
						Enabled: false, // Don't check metrics directory
					},
				},
			},
			expectErrors: false,
		},
		{
			name: "metrics persistence enabled",
			cfg: &config.Config{
				Logging: config.LoggingConfig{
					EnableFile: false,
				},
				Metrics: config.MetricsConfig{
					Persistence: config.MetricsPersistenceConfig{
						Enabled: true,
						Path:    filepath.Join(tmpDir, "metrics"),
					},
				},
			},
			expectErrors: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &ValidationResult{}
			validateDirectories(tt.cfg, result)

			if tt.expectErrors && result.IsValid() {
				t.Error("Expected validation errors but got none")
			}

			if !tt.expectErrors && !result.IsValid() {
				t.Errorf("Expected no validation errors but got: %v", result.Errors)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
