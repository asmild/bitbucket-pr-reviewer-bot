package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGetDefaultConfig(t *testing.T) {
	cfg := getDefaultConfig()

	// Test server defaults
	if cfg.Server.Port != 8080 {
		t.Errorf("Expected default port 8080, got %d", cfg.Server.Port)
	}

	// Test Claude defaults
	if cfg.Claude.Model != "sonnet" {
		t.Errorf("Expected default Claude model 'sonnet', got '%s'", cfg.Claude.Model)
	}
	if cfg.Claude.TimeoutMinutes != 10 {
		t.Errorf("Expected default Claude timeout 10, got %d", cfg.Claude.TimeoutMinutes)
	}

	// Test Bitbucket defaults
	if len(cfg.Bitbucket.AllowedProjectKeys) != 0 {
		t.Errorf("Expected default project keys to be empty, got %v", cfg.Bitbucket.AllowedProjectKeys)
	}
	if cfg.Bitbucket.EventType != "comment_added" {
		t.Errorf("Expected default EventType 'comment_added', got '%s'", cfg.Bitbucket.EventType)
	}
	if cfg.Bitbucket.TriggerKeyword != "/review" {
		t.Errorf("Expected default TriggerKeyword '/review', got '%s'", cfg.Bitbucket.TriggerKeyword)
	}

	// Test circuit breaker defaults
	if cfg.CircuitBreaker.FailureThreshold != 3 {
		t.Errorf("Expected default failure threshold 3, got %d", cfg.CircuitBreaker.FailureThreshold)
	}
	if cfg.CircuitBreaker.ResetTimeoutMS != 30000 {
		t.Errorf("Expected default reset timeout 30000, got %d", cfg.CircuitBreaker.ResetTimeoutMS)
	}

	// Test metrics defaults
	if cfg.Metrics.Persistence.Enabled != false {
		t.Errorf("Expected default persistence enabled false, got %t", cfg.Metrics.Persistence.Enabled)
	}
	if cfg.Metrics.Persistence.Type != "filesystem" {
		t.Errorf("Expected default persistence type 'filesystem', got '%s'", cfg.Metrics.Persistence.Type)
	}
	if cfg.Metrics.Persistence.Path != "./metrics-storage" {
		t.Errorf("Expected default persistence path './metrics-storage', got '%s'", cfg.Metrics.Persistence.Path)
	}
	if cfg.Metrics.Persistence.SaveIntervalMS != 30000 {
		t.Errorf("Expected default save interval 30000, got %d", cfg.Metrics.Persistence.SaveIntervalMS)
	}

	// Test logging defaults
	if cfg.Logging.Level != "info" {
		t.Errorf("Expected default log level 'info', got '%s'", cfg.Logging.Level)
	}
	if cfg.Logging.FileRetentionDays != 30 {
		t.Errorf("Expected default retention days 30, got %d", cfg.Logging.FileRetentionDays)
	}
	if cfg.Logging.MaxFileSize != "20m" {
		t.Errorf("Expected default max file size '20m', got '%s'", cfg.Logging.MaxFileSize)
	}
	if cfg.Logging.EnableConsole != true {
		t.Errorf("Expected default console enabled true, got %t", cfg.Logging.EnableConsole)
	}
	if cfg.Logging.EnableFile != true {
		t.Errorf("Expected default file enabled true, got %t", cfg.Logging.EnableFile)
	}
}

func TestLoadYAMLConfig(t *testing.T) {
	// Create temporary YAML file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.yaml")

	yamlContent := `
server:
  port: 9999

claude:
  model: opus
  timeout_minutes: 20

bitbucket:
  user: testuser
  token: testtoken
  webhook_secret: secret123
  allowed_project_keys:
    - TESTPROJ1
    - TESTPROJ2
  event_type: "comment_added"
  trigger_keyword: "/lgtm"

circuit_breaker:
  failure_threshold: 5
  reset_timeout_ms: 60000

metrics:
  persistence:
    enabled: true
    type: sqlite
    path: /var/metrics
    save_interval_ms: 60000

logging:
  level: debug
  file_retention_days: 60
  max_file_size: 50m
  enable_console: false
  enable_file: true
`

	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	cfg := LoadWithPath(configPath)

	// Test server config
	if cfg.Server.Port != 9999 {
		t.Errorf("Expected port 9999, got %d", cfg.Server.Port)
	}

	// Test Claude config
	if cfg.Claude.Model != "opus" {
		t.Errorf("Expected Claude model 'opus', got '%s'", cfg.Claude.Model)
	}
	if cfg.Claude.TimeoutMinutes != 20 {
		t.Errorf("Expected Claude timeout 20, got %d", cfg.Claude.TimeoutMinutes)
	}

	// Test Bitbucket config
	if cfg.Bitbucket.User != "testuser" {
		t.Errorf("Expected user 'testuser', got '%s'", cfg.Bitbucket.User)
	}
	if cfg.Bitbucket.Token != "testtoken" {
		t.Errorf("Expected token 'testtoken', got '%s'", cfg.Bitbucket.Token)
	}
	if cfg.Bitbucket.WebhookSecret != "secret123" {
		t.Errorf("Expected webhook secret 'secret123', got '%s'", cfg.Bitbucket.WebhookSecret)
	}
	if len(cfg.Bitbucket.AllowedProjectKeys) != 2 {
		t.Errorf("Expected 2 project keys, got %d", len(cfg.Bitbucket.AllowedProjectKeys))
	}
	if len(cfg.Bitbucket.AllowedProjectKeys) >= 2 {
		if cfg.Bitbucket.AllowedProjectKeys[0] != "TESTPROJ1" {
			t.Errorf("Expected first project key 'TESTPROJ1', got '%s'", cfg.Bitbucket.AllowedProjectKeys[0])
		}
		if cfg.Bitbucket.AllowedProjectKeys[1] != "TESTPROJ2" {
			t.Errorf("Expected second project key 'TESTPROJ2', got '%s'", cfg.Bitbucket.AllowedProjectKeys[1])
		}
	}
	if cfg.Bitbucket.EventType != "comment_added" {
		t.Errorf("Expected EventType 'comment_added', got '%s'", cfg.Bitbucket.EventType)
	}
	if cfg.Bitbucket.TriggerKeyword != "/lgtm" {
		t.Errorf("Expected TriggerKeyword '/lgtm', got '%s'", cfg.Bitbucket.TriggerKeyword)
	}

	// Test circuit breaker config
	if cfg.CircuitBreaker.FailureThreshold != 5 {
		t.Errorf("Expected failure threshold 5, got %d", cfg.CircuitBreaker.FailureThreshold)
	}
	if cfg.CircuitBreaker.ResetTimeoutMS != 60000 {
		t.Errorf("Expected reset timeout 60000, got %d", cfg.CircuitBreaker.ResetTimeoutMS)
	}

	// Test metrics config
	if cfg.Metrics.Persistence.Enabled != true {
		t.Errorf("Expected persistence enabled true, got %t", cfg.Metrics.Persistence.Enabled)
	}
	if cfg.Metrics.Persistence.Type != "sqlite" {
		t.Errorf("Expected persistence type 'sqlite', got '%s'", cfg.Metrics.Persistence.Type)
	}
	if cfg.Metrics.Persistence.Path != "/var/metrics" {
		t.Errorf("Expected persistence path '/var/metrics', got '%s'", cfg.Metrics.Persistence.Path)
	}
	if cfg.Metrics.Persistence.SaveIntervalMS != 60000 {
		t.Errorf("Expected save interval 60000, got %d", cfg.Metrics.Persistence.SaveIntervalMS)
	}

	// Test that SaveInterval is computed correctly
	expectedDuration := 60000 * time.Millisecond
	if cfg.Metrics.Persistence.SaveInterval != expectedDuration {
		t.Errorf("Expected SaveInterval %v, got %v", expectedDuration, cfg.Metrics.Persistence.SaveInterval)
	}

	// Test logging config
	if cfg.Logging.Level != "debug" {
		t.Errorf("Expected log level 'debug', got '%s'", cfg.Logging.Level)
	}
	if cfg.Logging.FileRetentionDays != 60 {
		t.Errorf("Expected retention days 60, got %d", cfg.Logging.FileRetentionDays)
	}
	if cfg.Logging.MaxFileSize != "50m" {
		t.Errorf("Expected max file size '50m', got '%s'", cfg.Logging.MaxFileSize)
	}
	if cfg.Logging.EnableConsole != false {
		t.Errorf("Expected console enabled false, got %t", cfg.Logging.EnableConsole)
	}
	if cfg.Logging.EnableFile != true {
		t.Errorf("Expected file enabled true, got %t", cfg.Logging.EnableFile)
	}
}

func TestEnvOverridesYAML(t *testing.T) {
	// Create temporary YAML file with values
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.yaml")

	yamlContent := `
server:
  port: 8080

claude:
  model: opus
  timeout_minutes: 20

bitbucket:
  user: yamluser
  token: yamltoken
  webhook_secret: yamlsecret
  allowed_project_keys:
    - YAMLPROJ
  event_type: "pr_opened"
  trigger_keyword: "/approve"
`

	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	// Set environment variables (these should override YAML)
	oldEnvVars := map[string]string{
		"PORT":                     os.Getenv("PORT"),
		"CLAUDE_MODEL":             os.Getenv("CLAUDE_MODEL"),
		"CLAUDE_TIMEOUT_CONFIG":    os.Getenv("CLAUDE_TIMEOUT_CONFIG"),
		"BITBUCKET_USER":                 os.Getenv("BITBUCKET_USER"),
		"BITBUCKET_TOKEN":                os.Getenv("BITBUCKET_TOKEN"),
		"BITBUCKET_WEBHOOK_SECRET":       os.Getenv("BITBUCKET_WEBHOOK_SECRET"),
		"BITBUCKET_ALLOWED_PROJECT_KEYS": os.Getenv("BITBUCKET_ALLOWED_PROJECT_KEYS"),
		"BITBUCKET_EVENT_TYPE":           os.Getenv("BITBUCKET_EVENT_TYPE"),
		"TRIGGER_KEYWORD":                os.Getenv("TRIGGER_KEYWORD"),
	}

	// Cleanup function to restore old env vars
	defer func() {
		for key, val := range oldEnvVars {
			if val == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, val)
			}
		}
	}()

	// Set new env vars
	os.Setenv("PORT", "9000")
	os.Setenv("CLAUDE_MODEL", "haiku")
	os.Setenv("CLAUDE_TIMEOUT_CONFIG", "30")
	os.Setenv("BITBUCKET_USER", "envuser")
	os.Setenv("BITBUCKET_TOKEN", "envtoken")
	os.Setenv("BITBUCKET_WEBHOOK_SECRET", "envsecret")
	os.Setenv("BITBUCKET_ALLOWED_PROJECT_KEYS", "ENVPROJ1, ENVPROJ2, ENVPROJ3")
	os.Setenv("BITBUCKET_EVENT_TYPE", "comment_added")
	os.Setenv("TRIGGER_KEYWORD", "/env-review")

	cfg := LoadWithPath(configPath)

	// Verify that env vars override YAML values
	if cfg.Server.Port != 9000 {
		t.Errorf("Expected port 9000 from env, got %d", cfg.Server.Port)
	}
	if cfg.Claude.Model != "haiku" {
		t.Errorf("Expected Claude model 'haiku' from env, got '%s'", cfg.Claude.Model)
	}
	if cfg.Claude.TimeoutMinutes != 30 {
		t.Errorf("Expected Claude timeout 30 from env, got %d", cfg.Claude.TimeoutMinutes)
	}
	if cfg.Bitbucket.User != "envuser" {
		t.Errorf("Expected user 'envuser' from env, got '%s'", cfg.Bitbucket.User)
	}
	if cfg.Bitbucket.Token != "envtoken" {
		t.Errorf("Expected token 'envtoken' from env, got '%s'", cfg.Bitbucket.Token)
	}
	if cfg.Bitbucket.WebhookSecret != "envsecret" {
		t.Errorf("Expected webhook secret 'envsecret' from env, got '%s'", cfg.Bitbucket.WebhookSecret)
	}
	if len(cfg.Bitbucket.AllowedProjectKeys) != 3 {
		t.Errorf("Expected 3 project keys from env, got %d: %v", len(cfg.Bitbucket.AllowedProjectKeys), cfg.Bitbucket.AllowedProjectKeys)
	}
	if len(cfg.Bitbucket.AllowedProjectKeys) >= 3 {
		if cfg.Bitbucket.AllowedProjectKeys[0] != "ENVPROJ1" {
			t.Errorf("Expected first project key 'ENVPROJ1' from env, got '%s'", cfg.Bitbucket.AllowedProjectKeys[0])
		}
		if cfg.Bitbucket.AllowedProjectKeys[1] != "ENVPROJ2" {
			t.Errorf("Expected second project key 'ENVPROJ2' from env, got '%s'", cfg.Bitbucket.AllowedProjectKeys[1])
		}
		if cfg.Bitbucket.AllowedProjectKeys[2] != "ENVPROJ3" {
			t.Errorf("Expected third project key 'ENVPROJ3' from env, got '%s'", cfg.Bitbucket.AllowedProjectKeys[2])
		}
	}
	if cfg.Bitbucket.EventType != "comment_added" {
		t.Errorf("Expected EventType 'comment_added' from env, got '%s'", cfg.Bitbucket.EventType)
	}
	if cfg.Bitbucket.TriggerKeyword != "/env-review" {
		t.Errorf("Expected TriggerKeyword '/env-review' from env, got '%s'", cfg.Bitbucket.TriggerKeyword)
	}
}

func TestConfigWithoutYAMLFile(t *testing.T) {
	// Set required env vars
	oldUser := os.Getenv("BITBUCKET_USER")
	oldToken := os.Getenv("BITBUCKET_TOKEN")

	defer func() {
		if oldUser == "" {
			os.Unsetenv("BITBUCKET_USER")
		} else {
			os.Setenv("BITBUCKET_USER", oldUser)
		}
		if oldToken == "" {
			os.Unsetenv("BITBUCKET_TOKEN")
		} else {
			os.Setenv("BITBUCKET_TOKEN", oldToken)
		}
	}()

	os.Setenv("BITBUCKET_USER", "testuser")
	os.Setenv("BITBUCKET_TOKEN", "testtoken")

	// Load config with non-existent file path
	cfg := LoadWithPath("/nonexistent/config.yaml")

	// Should use defaults
	if cfg.Server.Port != 8080 {
		t.Errorf("Expected default port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Claude.Model != "sonnet" {
		t.Errorf("Expected default Claude model 'sonnet', got '%s'", cfg.Claude.Model)
	}

	// But env vars should still be applied
	if cfg.Bitbucket.User != "testuser" {
		t.Errorf("Expected user 'testuser' from env, got '%s'", cfg.Bitbucket.User)
	}
	if cfg.Bitbucket.Token != "testtoken" {
		t.Errorf("Expected token 'testtoken' from env, got '%s'", cfg.Bitbucket.Token)
	}
}

func TestValidation(t *testing.T) {
	tests := []struct {
		name        string
		user        string
		token       string
		eventType   string
		expectError bool
	}{
		{
			name:        "Valid config with pr_opened",
			user:        "testuser",
			token:       "testtoken",
			eventType:   "pr_opened",
			expectError: false,
		},
		{
			name:        "Valid config with comment_added",
			user:        "testuser",
			token:       "testtoken",
			eventType:   "comment_added",
			expectError: false,
		},
		{
			name:        "Missing user",
			user:        "",
			token:       "testtoken",
			eventType:   "pr_opened",
			expectError: true,
		},
		{
			name:        "Missing token",
			user:        "testuser",
			token:       "",
			eventType:   "pr_opened",
			expectError: true,
		},
		{
			name:        "Invalid event type",
			user:        "testuser",
			token:       "testtoken",
			eventType:   "invalid_event",
			expectError: true,
		},
		{
			name:        "Missing both user and token",
			user:        "",
			token:       "",
			eventType:   "pr_opened",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := getDefaultConfig()
			cfg.Bitbucket.User = tt.user
			cfg.Bitbucket.Token = tt.token
			cfg.Bitbucket.EventType = tt.eventType

			err := cfg.Validate()
			if tt.expectError && err == nil {
				t.Error("Expected validation error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
		})
	}
}

func TestValidationError(t *testing.T) {
	err := &ValidationError{
		Field:   "test.field",
		Message: "is invalid",
	}

	expected := "Config validation error: test.field is invalid"
	if err.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, err.Error())
	}
}

func TestGetEnvHelpers(t *testing.T) {
	// Test getEnv
	os.Setenv("TEST_STRING", "testvalue")
	defer os.Unsetenv("TEST_STRING")

	if val := getEnv("TEST_STRING", "default"); val != "testvalue" {
		t.Errorf("Expected 'testvalue', got '%s'", val)
	}
	if val := getEnv("NONEXISTENT", "default"); val != "default" {
		t.Errorf("Expected 'default', got '%s'", val)
	}

	// Test getEnvAsInt
	os.Setenv("TEST_INT", "123")
	defer os.Unsetenv("TEST_INT")

	if val := getEnvAsInt("TEST_INT", 0); val != 123 {
		t.Errorf("Expected 123, got %d", val)
	}
	if val := getEnvAsInt("NONEXISTENT", 456); val != 456 {
		t.Errorf("Expected 456, got %d", val)
	}
	if val := getEnvAsInt("TEST_STRING", 789); val != 789 {
		t.Errorf("Expected 789 for invalid int, got %d", val)
	}

	// Test getEnvAsBool
	os.Setenv("TEST_BOOL_TRUE", "true")
	os.Setenv("TEST_BOOL_FALSE", "false")
	os.Setenv("TEST_BOOL_INVALID", "notabool")
	defer os.Unsetenv("TEST_BOOL_TRUE")
	defer os.Unsetenv("TEST_BOOL_FALSE")
	defer os.Unsetenv("TEST_BOOL_INVALID")

	if val := getEnvAsBool("TEST_BOOL_TRUE", false); val != true {
		t.Errorf("Expected true, got %t", val)
	}
	if val := getEnvAsBool("TEST_BOOL_FALSE", true); val != false {
		t.Errorf("Expected false, got %t", val)
	}
	if val := getEnvAsBool("NONEXISTENT", true); val != true {
		t.Errorf("Expected true (default), got %t", val)
	}
	if val := getEnvAsBool("TEST_BOOL_INVALID", false); val != false {
		t.Errorf("Expected false (default for invalid), got %t", val)
	}
}

func TestPartialYAMLConfig(t *testing.T) {
	// Test that missing fields in YAML use defaults
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "partial-config.yaml")

	yamlContent := `
server:
  port: 4000

bitbucket:
  user: testuser
  token: testtoken
`

	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	cfg := LoadWithPath(configPath)

	// Specified in YAML
	if cfg.Server.Port != 4000 {
		t.Errorf("Expected port 4000, got %d", cfg.Server.Port)
	}
	if cfg.Bitbucket.User != "testuser" {
		t.Errorf("Expected user 'testuser', got '%s'", cfg.Bitbucket.User)
	}

	// Not specified, should use defaults
	if cfg.Claude.Model != "sonnet" {
		t.Errorf("Expected default Claude model 'sonnet', got '%s'", cfg.Claude.Model)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("Expected default log level 'info', got '%s'", cfg.Logging.Level)
	}
}
