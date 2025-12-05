package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGetDefaultConfig(t *testing.T) {
	cfg := getDefaultConfig()

	// Test critical defaults only
	if cfg.Server.Port != 8080 {
		t.Errorf("Expected default port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Claude.Model != "sonnet" {
		t.Errorf("Expected default Claude model 'sonnet', got '%s'", cfg.Claude.Model)
	}
	if cfg.Bitbucket.SelfHosted != false {
		t.Errorf("Expected default SelfHosted false, got %t", cfg.Bitbucket.SelfHosted)
	}
	if cfg.Bitbucket.BaseURL != "" {
		t.Errorf("Expected default BaseURL empty, got '%s'", cfg.Bitbucket.BaseURL)
	}

	// Test default triggering events
	if len(cfg.Bitbucket.Events) != 1 {
		t.Errorf("Expected 1 default event, got %d", len(cfg.Bitbucket.Events))
	}
	if cfg.Bitbucket.Events[0].GetType() != EventTypeCommentAdded {
		t.Errorf("Expected default event type 'comment_added', got '%s'", cfg.Bitbucket.Events[0].GetType())
	}

	// Test that RawEvents is empty by default
	if len(cfg.Bitbucket.RawEvents) != 0 {
		t.Errorf("Expected empty RawEvents, got %d", len(cfg.Bitbucket.RawEvents))
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
  self-hosted: true
  base_url: "http://bitbucket.test.com"
  user: testuser
  token: testtoken
  webhook_secret: secret123
  allowed_project_keys:
    - TESTPROJ1
    - TESTPROJ2
  triggering_events:
    - type: pr_opened
    - type: comment_added
      keyword: "/lgtm"

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

	cfg := Load(configPath)

	// Verify YAML values are loaded correctly (sample key fields)
	if cfg.Server.Port != 9999 {
		t.Errorf("Expected port 9999, got %d", cfg.Server.Port)
	}
	if cfg.Claude.Model != "opus" {
		t.Errorf("Expected Claude model 'opus', got '%s'", cfg.Claude.Model)
	}
	if cfg.Bitbucket.SelfHosted != true {
		t.Errorf("Expected SelfHosted true, got %t", cfg.Bitbucket.SelfHosted)
	}
	if cfg.Bitbucket.BaseURL != "http://bitbucket.test.com" {
		t.Errorf("Expected BaseURL 'http://bitbucket.test.com', got '%s'", cfg.Bitbucket.BaseURL)
	}
	if cfg.Bitbucket.User != "testuser" {
		t.Errorf("Expected user 'testuser', got '%s'", cfg.Bitbucket.User)
	}
	if len(cfg.Bitbucket.AllowedProjectKeys) != 2 || cfg.Bitbucket.AllowedProjectKeys[0] != "TESTPROJ1" {
		t.Errorf("Expected project keys [TESTPROJ1, TESTPROJ2], got %v", cfg.Bitbucket.AllowedProjectKeys)
	}

	// Test triggering events
	if len(cfg.Bitbucket.Events) != 2 {
		t.Errorf("Expected 2 events, got %d", len(cfg.Bitbucket.Events))
	}

	// Test computed duration
	expectedDuration := 60000 * time.Millisecond
	if cfg.Metrics.Persistence.SaveInterval != expectedDuration {
		t.Errorf("Expected SaveInterval %v, got %v", expectedDuration, cfg.Metrics.Persistence.SaveInterval)
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
  self-hosted: false
  base_url: "http://yaml.bitbucket.com"
  user: yamluser
  token: yamltoken
  webhook_secret: yamlsecret
  allowed_project_keys:
    - YAMLPROJ
  triggering_events:
    - type: comment_added
      keyword: "/approve"
`

	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	// Set environment variables (these should override YAML)
	oldEnvVars := map[string]string{
		"PORT":                             os.Getenv("PORT"),
		"CLAUDE_MODEL":                     os.Getenv("CLAUDE_MODEL"),
		"CLAUDE_TIMEOUT_CONFIG":            os.Getenv("CLAUDE_TIMEOUT_CONFIG"),
		"BITBUCKET_SELF_HOSTED":            os.Getenv("BITBUCKET_SELF_HOSTED"),
		"BITBUCKET_BASE_URL":               os.Getenv("BITBUCKET_BASE_URL"),
		"BITBUCKET_USER":                   os.Getenv("BITBUCKET_USER"),
		"BITBUCKET_TOKEN":                  os.Getenv("BITBUCKET_TOKEN"),
		"BITBUCKET_WEBHOOK_SECRET":         os.Getenv("BITBUCKET_WEBHOOK_SECRET"),
		"BITBUCKET_ALLOWED_PROJECT_KEYS":   os.Getenv("BITBUCKET_ALLOWED_PROJECT_KEYS"),
		"BITBUCKET_PR_EVENT_PR_OPENED":     os.Getenv("BITBUCKET_PR_EVENT_PR_OPENED"),
		"BITBUCKET_PR_EVENT_COMMENT_ADDED": os.Getenv("BITBUCKET_PR_EVENT_COMMENT_ADDED"),
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
	os.Setenv("BITBUCKET_SELF_HOSTED", "true")
	os.Setenv("BITBUCKET_BASE_URL", "http://env.bitbucket.com")
	os.Setenv("BITBUCKET_USER", "envuser")
	os.Setenv("BITBUCKET_TOKEN", "envtoken")
	os.Setenv("BITBUCKET_WEBHOOK_SECRET", "envsecret")
	os.Setenv("BITBUCKET_ALLOWED_PROJECT_KEYS", "ENVPROJ1, ENVPROJ2, ENVPROJ3")
	os.Setenv("BITBUCKET_PR_EVENT_PR_OPENED", "true")
	os.Setenv("BITBUCKET_PR_EVENT_COMMENT_ADDED", "/env-review")

	cfg := Load(configPath)

	// Verify env vars override YAML values (test key override mechanism)
	if cfg.Server.Port != 9000 {
		t.Errorf("Expected port 9000 from env, got %d", cfg.Server.Port)
	}
	if cfg.Claude.Model != "haiku" {
		t.Errorf("Expected Claude model 'haiku' from env, got '%s'", cfg.Claude.Model)
	}
	if cfg.Bitbucket.SelfHosted != true {
		t.Errorf("Expected SelfHosted true from env (overriding YAML false), got %t", cfg.Bitbucket.SelfHosted)
	}
	if cfg.Bitbucket.BaseURL != "http://env.bitbucket.com" {
		t.Errorf("Expected BaseURL 'http://env.bitbucket.com' from env, got '%s'", cfg.Bitbucket.BaseURL)
	}
	if cfg.Bitbucket.User != "envuser" {
		t.Errorf("Expected user 'envuser' from env, got '%s'", cfg.Bitbucket.User)
	}
	if len(cfg.Bitbucket.AllowedProjectKeys) != 3 || cfg.Bitbucket.AllowedProjectKeys[0] != "ENVPROJ1" {
		t.Errorf("Expected 3 project keys from env, got %v", cfg.Bitbucket.AllowedProjectKeys)
	}

	// Verify events are overridden/merged
	if len(cfg.Bitbucket.Events) != 2 {
		t.Errorf("Expected 2 events (merged), got %d", len(cfg.Bitbucket.Events))
	}
}

func TestConfigWithoutYAMLFile(t *testing.T) {
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

	cfg := Load("/nonexistent/config.yaml")

	// Verify defaults are used when no YAML file exists
	if cfg.Server.Port != 8080 || cfg.Claude.Model != "sonnet" {
		t.Errorf("Expected defaults: port=8080, model=sonnet, got port=%d, model=%s", cfg.Server.Port, cfg.Claude.Model)
	}
	// But env vars should still apply
	if cfg.Bitbucket.User != "testuser" || cfg.Bitbucket.Token != "testtoken" {
		t.Errorf("Expected env vars to apply even without YAML")
	}

	// Should have default events
	if len(cfg.Bitbucket.Events) != 1 {
		t.Errorf("Expected 1 default event, got %d", len(cfg.Bitbucket.Events))
	}
}

func TestValidation(t *testing.T) {
	tests := []struct {
		name        string
		user        string
		token       string
		events      []TriggeringEvent
		expectError bool
	}{
		{
			name:  "Valid config with pr_opened",
			user:  "testuser",
			token: "testtoken",
			events: []TriggeringEvent{
				&PROpenedEvent{},
			},
			expectError: false,
		},
		{
			name:  "Valid config with comment_added",
			user:  "testuser",
			token: "testtoken",
			events: []TriggeringEvent{
				&CommentAddedEvent{Keyword: "/review"},
			},
			expectError: false,
		},
		{
			name:  "Valid config with both events",
			user:  "testuser",
			token: "testtoken",
			events: []TriggeringEvent{
				&PROpenedEvent{},
				&CommentAddedEvent{Keyword: "/review"},
			},
			expectError: false,
		},
		{
			name:  "Missing user",
			user:  "",
			token: "testtoken",
			events: []TriggeringEvent{
				&PROpenedEvent{},
			},
			expectError: true,
		},
		{
			name:  "Missing token",
			user:  "testuser",
			token: "",
			events: []TriggeringEvent{
				&PROpenedEvent{},
			},
			expectError: true,
		},
		{
			name:  "Invalid event - comment_added without keyword",
			user:  "testuser",
			token: "testtoken",
			events: []TriggeringEvent{
				&CommentAddedEvent{Keyword: ""},
			},
			expectError: true,
		},
		{
			name:        "Missing both user and token",
			user:        "",
			token:       "",
			events:      []TriggeringEvent{&PROpenedEvent{}},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := getDefaultConfig()
			cfg.Bitbucket.User = tt.user
			cfg.Bitbucket.Token = tt.token
			cfg.Bitbucket.Events = tt.events

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
	// Test getEnvAsInt
	os.Setenv("TEST_INT", "123")
	defer os.Unsetenv("TEST_INT")

	if val := getEnvAsInt("TEST_INT", 0); val != 123 {
		t.Errorf("Expected 123, got %d", val)
	}
	if val := getEnvAsInt("NONEXISTENT", 456); val != 456 {
		t.Errorf("Expected 456, got %d", val)
	}
	os.Setenv("TEST_STRING", "notanumber")
	defer os.Unsetenv("TEST_STRING")
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

	cfg := Load(configPath)

	// Verify specified values
	if cfg.Server.Port != 4000 || cfg.Bitbucket.User != "testuser" {
		t.Errorf("Expected YAML values: port=4000, user=testuser")
	}

	// Verify unspecified fields use defaults
	if cfg.Claude.Model != "sonnet" || cfg.Logging.Level != "info" {
		t.Errorf("Expected defaults for unspecified fields")
	}
	if cfg.Bitbucket.SelfHosted != false || cfg.Bitbucket.BaseURL != "" {
		t.Errorf("Expected Bitbucket defaults: SelfHosted=false, BaseURL=empty")
	}

	// Should have default events since not specified in YAML
	if len(cfg.Bitbucket.Events) != 1 {
		t.Errorf("Expected 1 default event, got %d", len(cfg.Bitbucket.Events))
	}
}

func TestTriggeringEventsFromYAML(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "events-config.yaml")

	yamlContent := `
bitbucket:
  user: testuser
  token: testtoken
  triggering_events:
    - type: pr_opened
    - type: comment_added
      keyword: "/test"
`

	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	cfg := Load(configPath)

	// Verify events loaded from YAML
	if len(cfg.Bitbucket.Events) != 2 {
		t.Fatalf("Expected 2 events, got %d", len(cfg.Bitbucket.Events))
	}

	// Check event types
	foundPROpened := false
	foundCommentAdded := false
	for _, event := range cfg.Bitbucket.Events {
		if event.GetType() == EventTypePROpened {
			foundPROpened = true
		}
		if event.GetType() == EventTypeCommentAdded {
			foundCommentAdded = true
			commentEvent, ok := event.(*CommentAddedEvent)
			if !ok {
				t.Error("Expected CommentAddedEvent type")
			} else if commentEvent.Keyword != "/test" {
				t.Errorf("Expected keyword '/test', got '%s'", commentEvent.Keyword)
			}
		}
	}

	if !foundPROpened {
		t.Error("Expected to find pr_opened event")
	}
	if !foundCommentAdded {
		t.Error("Expected to find comment_added event")
	}
}

func TestTriggeringEventsFromEnv(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "empty-config.yaml")

	yamlContent := `
bitbucket:
  user: testuser
  token: testtoken
`

	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test config file: %v", err)
	}

	oldEnvVars := map[string]string{
		"BITBUCKET_PR_EVENT_PR_OPENED":     os.Getenv("BITBUCKET_PR_EVENT_PR_OPENED"),
		"BITBUCKET_PR_EVENT_COMMENT_ADDED": os.Getenv("BITBUCKET_PR_EVENT_COMMENT_ADDED"),
	}

	defer func() {
		for key, val := range oldEnvVars {
			if val == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, val)
			}
		}
	}()

	os.Setenv("BITBUCKET_PR_EVENT_PR_OPENED", "true")
	os.Setenv("BITBUCKET_PR_EVENT_COMMENT_ADDED", "/analyze")

	cfg := Load(configPath)

	// Verify events loaded from env
	if len(cfg.Bitbucket.Events) != 2 {
		t.Fatalf("Expected 2 events, got %d", len(cfg.Bitbucket.Events))
	}
}

func TestParseCommaSeparated(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "single value",
			input:    "TEST1",
			expected: []string{"TEST1"},
		},
		{
			name:     "multiple values",
			input:    "TEST1,TEST2,TEST3",
			expected: []string{"TEST1", "TEST2", "TEST3"},
		},
		{
			name:     "values with spaces",
			input:    "TEST1, TEST2 , TEST3",
			expected: []string{"TEST1", "TEST2", "TEST3"},
		},
		{
			name:     "trailing comma",
			input:    "TEST1,TEST2,",
			expected: []string{"TEST1", "TEST2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseCommaSeparated(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d items, got %d", len(tt.expected), len(result))
			}
			for i, expected := range tt.expected {
				if i >= len(result) || result[i] != expected {
					t.Errorf("Expected '%s' at index %d, got '%s'", expected, i, result[i])
				}
			}
		})
	}
}
