package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all application configuration
type Config struct {
	// Server configuration
	Server ServerConfig `yaml:"server"`

	// Claude configuration
	Claude ClaudeConfig `yaml:"claude"`

	// Bitbucket configuration
	Bitbucket BitbucketConfig `yaml:"bitbucket"`

	// Templates configuration
	Templates TemplatesConfig `yaml:"templates"`

	// Circuit breaker configuration
	CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker"`

	// Metrics persistence configuration
	Metrics MetricsConfig `yaml:"metrics"`

	// Logging configuration
	Logging LoggingConfig `yaml:"logging"`
}

type ServerConfig struct {
	Port int `yaml:"port"`
}

type ClaudeConfig struct {
	Model          string `yaml:"model"`
	TimeoutMinutes int    `yaml:"timeout_minutes"`
}

type BitbucketConfig struct {
	User               string   `yaml:"user"`
	Token              string   `yaml:"token"`
	WebhookSecret      string   `yaml:"webhook_secret"`
	AllowedProjectKeys []string `yaml:"allowed_project_keys"`
	EventType          string   `yaml:"event_type"`
	TriggerKeyword     string   `yaml:"trigger_keyword"`
}

type TemplatesConfig struct {
	Directory string                    `yaml:"directory"`
	Default   string                    `yaml:"default"`
	Projects  map[string]ProjectTemplate `yaml:"projects"`
}

type ProjectTemplate struct {
	Template     string            `yaml:"template"`      // Applied to all repos in this project
	Repositories map[string]string `yaml:"repositories"` // Per-repo overrides
}

type CircuitBreakerConfig struct {
	FailureThreshold int `yaml:"failure_threshold"`
	ResetTimeoutMS   int `yaml:"reset_timeout_ms"`
}

type MetricsConfig struct {
	Persistence MetricsPersistenceConfig `yaml:"persistence"`
}

type MetricsPersistenceConfig struct {
	Enabled        bool          `yaml:"enabled"`
	Type           string        `yaml:"type"`
	Path           string        `yaml:"path"`
	SaveIntervalMS int           `yaml:"save_interval_ms"`
	SaveInterval   time.Duration `yaml:"-"`
}

type LoggingConfig struct {
	Level             string `yaml:"level"`
	FileRetentionDays int    `yaml:"file_retention_days"`
	MaxFileSize       string `yaml:"max_file_size"`
	EnableConsole     bool   `yaml:"enable_console"`
	EnableFile        bool   `yaml:"enable_file"`
}

// Load loads configuration from YAML file and environment variables
// Environment variables have higher priority and will override YAML values
// Configuration file is searched in the following order:
// 1. CONFIG_PATH environment variable
// 2. ./config.yaml (current directory)
// 3. ~/.bb-pr-reviewer/config.yaml (user home)
// 4. /etc/bb-pr-reviewer/config.yaml (system-wide)
func Load() *Config {
	// Find config file
	configPath := FindConfigFile()

	if configPath == "" {
		log.Printf("Info: No config file found. Using defaults with environment variable overrides.")
		log.Printf("Searched locations: %v", ConfigSearchPaths())
	}

	return LoadWithPath(configPath)
}

// LoadWithPath loads configuration from a specific YAML file path
func LoadWithPath(configPath string) *Config {
	cfg := getDefaultConfig()

	// Try to load from YAML file if it exists and path is not empty
	if configPath != "" {
		if _, err := os.Stat(configPath); err == nil {
			if err := loadYAML(configPath, cfg); err != nil {
				log.Printf("Warning: Failed to load config from %s: %v. Using defaults with env overrides.", configPath, err)
			} else {
				log.Printf("Info: Loaded configuration from %s", configPath)
			}
		}
	}

	// Override with environment variables (higher priority)
	applyEnvOverrides(cfg)

	// Post-process configuration
	cfg.Metrics.Persistence.SaveInterval = time.Duration(cfg.Metrics.Persistence.SaveIntervalMS) * time.Millisecond

	if err := cfg.Validate(); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}

	return cfg
}

// getDefaultTemplatesDirectory finds templates directory or returns default
func getDefaultTemplatesDirectory() string {
	templatesDir := FindTemplatesDirectory()
	if templatesDir != "" {
		return templatesDir
	}
	// Fallback to current directory
	return "./templates"
}

// getDefaultConfig returns a Config with default values
func getDefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port: 8080,
		},
		Claude: ClaudeConfig{
			Model:          "sonnet",
			TimeoutMinutes: 10,
		},
		Bitbucket: BitbucketConfig{
			AllowedProjectKeys: []string{},
			EventType:          "comment_added",
			TriggerKeyword:     "/review",
		},
		Templates: TemplatesConfig{
			Directory: getDefaultTemplatesDirectory(),
			Default:   "default",
			Projects:  make(map[string]ProjectTemplate),
		},
		CircuitBreaker: CircuitBreakerConfig{
			FailureThreshold: 3,
			ResetTimeoutMS:   30000,
		},
		Metrics: MetricsConfig{
			Persistence: MetricsPersistenceConfig{
				Enabled:        false,
				Type:           "filesystem",
				Path:           "./metrics-storage",
				SaveIntervalMS: 30000,
			},
		},
		Logging: LoggingConfig{
			Level:             "info",
			FileRetentionDays: 30,
			MaxFileSize:       "20m",
			EnableConsole:     true,
			EnableFile:        true,
		},
	}
}

// loadYAML loads configuration from a YAML file
func loadYAML(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	return nil
}

// applyEnvOverrides applies environment variable overrides to the config
// Environment variables have higher priority than YAML values
func applyEnvOverrides(cfg *Config) {
	// Server overrides
	if port := getEnvAsInt("PORT", 0); port != 0 {
		cfg.Server.Port = port
	}

	// Claude overrides
	if model := os.Getenv("CLAUDE_MODEL"); model != "" {
		cfg.Claude.Model = model
	}
	if timeout := getEnvAsInt("CLAUDE_TIMEOUT_CONFIG", 0); timeout != 0 {
		cfg.Claude.TimeoutMinutes = timeout
	}

	// Bitbucket overrides
	if user := os.Getenv("BITBUCKET_USER"); user != "" {
		cfg.Bitbucket.User = user
	}
	if token := os.Getenv("BITBUCKET_TOKEN"); token != "" {
		cfg.Bitbucket.Token = token
	}
	if secret := os.Getenv("BITBUCKET_WEBHOOK_SECRET"); secret != "" {
		cfg.Bitbucket.WebhookSecret = secret
	}
	if projectKeys := os.Getenv("BITBUCKET_ALLOWED_PROJECT_KEYS"); projectKeys != "" {
		cfg.Bitbucket.AllowedProjectKeys = parseCommaSeparated(projectKeys)
	}
	if eventType := os.Getenv("BITBUCKET_EVENT_TYPE"); eventType != "" {
		cfg.Bitbucket.EventType = eventType
	}
	if keyword := os.Getenv("TRIGGER_KEYWORD"); keyword != "" {
		cfg.Bitbucket.TriggerKeyword = keyword
	}

	// Circuit breaker overrides
	if threshold := getEnvAsInt("CB_FAILURE_THRESHOLD", 0); threshold != 0 {
		cfg.CircuitBreaker.FailureThreshold = threshold
	}
	if timeout := getEnvAsInt("CB_RESET_TIMEOUT_MS", 0); timeout != 0 {
		cfg.CircuitBreaker.ResetTimeoutMS = timeout
	}

	// Metrics overrides
	if enabled := os.Getenv("METRICS_PERSISTENCE_ENABLED"); enabled != "" {
		cfg.Metrics.Persistence.Enabled = getEnvAsBool("METRICS_PERSISTENCE_ENABLED", cfg.Metrics.Persistence.Enabled)
	}
	if persistType := os.Getenv("METRICS_PERSISTENCE_TYPE"); persistType != "" {
		cfg.Metrics.Persistence.Type = persistType
	}
	if path := os.Getenv("METRICS_PERSISTENCE_PATH"); path != "" {
		cfg.Metrics.Persistence.Path = path
	}
	if interval := getEnvAsInt("METRICS_PERSISTENCE_SAVE_INTERVAL_MS", 0); interval != 0 {
		cfg.Metrics.Persistence.SaveIntervalMS = interval
	}

	// Templates overrides
	// TEMPLATES_DIRECTORY takes precedence over auto-discovery
	if directory := os.Getenv("TEMPLATES_DIRECTORY"); directory != "" {
		cfg.Templates.Directory = directory
	} else if cfg.Templates.Directory == "./templates" {
		// If still using default, try to find templates directory
		if found := FindTemplatesDirectory(); found != "" {
			cfg.Templates.Directory = found
		}
	}
	if defaultTemplate := os.Getenv("TEMPLATES_DEFAULT"); defaultTemplate != "" {
		cfg.Templates.Default = defaultTemplate
	}

	// Logging overrides
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		cfg.Logging.Level = level
	}
	if retention := getEnvAsInt("LOG_FILE_RETENTION_DAYS", 0); retention != 0 {
		cfg.Logging.FileRetentionDays = retention
	}
	if size := os.Getenv("LOG_MAX_FILE_SIZE"); size != "" {
		cfg.Logging.MaxFileSize = size
	}
	if console := os.Getenv("LOG_ENABLE_CONSOLE"); console != "" {
		cfg.Logging.EnableConsole = getEnvAsBool("LOG_ENABLE_CONSOLE", cfg.Logging.EnableConsole)
	}
	if file := os.Getenv("LOG_ENABLE_FILE"); file != "" {
		cfg.Logging.EnableFile = getEnvAsBool("LOG_ENABLE_FILE", cfg.Logging.EnableFile)
	}
}

// Validate validates required configuration
func (c *Config) Validate() error {
	if c.Bitbucket.User == "" {
		return &ValidationError{Field: "bitbucket.user", Message: "is required"}
	}
	if c.Bitbucket.Token == "" {
		return &ValidationError{Field: "bitbucket.token", Message: "is required"}
	}
	if c.Bitbucket.EventType != "pr_opened" && c.Bitbucket.EventType != "comment_added" {
		return &ValidationError{Field: "bitbucket.event_type", Message: "must be either 'pr_opened' or 'comment_added'"}
	}
	return nil
}

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("Config validation error: %s %s", e.Field, e.Message)
}

// Helper functions for environment variable parsing
func getEnv(key, defaultVal string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultVal
}

func getEnvAsInt(key string, defaultVal int) int {
	valueStr := os.Getenv(key)
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return defaultVal
}

func getEnvAsBool(key string, defaultVal bool) bool {
	valueStr := os.Getenv(key)
	if value, err := strconv.ParseBool(valueStr); err == nil {
		return value
	}
	return defaultVal
}

func parseCommaSeparated(value string) []string {
	if value == "" {
		return []string{}
	}

	var result []string
	parts := strings.Split(value, ",")
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

