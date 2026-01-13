package validator

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/config"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/errors"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/ports"
)

// ValidationError represents a startup validation error
type ValidationError struct {
	Component string
	Issue     string
	Solution  string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s\n  Solution: %s", e.Component, e.Issue, e.Solution)
}

// ValidationResult holds all validation errors and info messages
type ValidationResult struct {
	Errors   []*ValidationError
	InfoMsgs []string
}

// IsValid returns true if there are no validation errors
func (r *ValidationResult) IsValid() bool {
	return len(r.Errors) == 0
}

// AddError adds a validation error
func (r *ValidationResult) AddError(component, issue, solution string) {
	r.Errors = append(r.Errors, &ValidationError{
		Component: component,
		Issue:     issue,
		Solution:  solution,
	})
}

// AddInfo adds an informational message
func (r *ValidationResult) AddInfo(msg string) {
	r.InfoMsgs = append(r.InfoMsgs, msg)
}

// LogResults logs all validation results using the provided logger
func (r *ValidationResult) LogResults(logger ports.Logger) {
	// Log info messages
	for _, msg := range r.InfoMsgs {
		logger.Info(msg)
	}

	// Log errors if any
	if !r.IsValid() {
		logger.Error("Startup Dependencies Check Failed")
		logger.Error("")

		for i, err := range r.Errors {
			logger.Error(fmt.Sprintf("%d. %s", i+1, err.Error()))
			logger.Error("")
		}

		logger.Error("Please fix the above issues and restart the application.")
	}
}

// ValidateStartup checks all startup dependencies and requirements
func ValidateStartup(cfg *config.Config, logger ports.Logger, vcsClient ports.VCSClient) *ValidationResult {
	result := &ValidationResult{}

	logger.Info("Running startup checks...")

	// Check configuration
	validateConfiguration(cfg, result)
	if !result.IsValid() {
		return result
	}

	// Validate VCS credentials
	validateVCSCredentials(cfg, vcsClient, result)
	if !result.IsValid() {
		return result
	}

	// Check external dependencies
	validateClaudeCLI(result)
	validateGit(result)

	// Check profiles (templates)
	validateProfiles(cfg, result, logger)

	// Check directory permissions
	validateDirectories(cfg, result)

	// Check persistent storage (reachable, readable, writable) if enabled
	if cfg.Metrics.Persistence.Enabled {
		validatePersistentStorage(cfg, result)
	}

	return result
}

// validateConfiguration checks required configuration values
func validateConfiguration(cfg *config.Config, result *ValidationResult) {
	// Log Bitbucket platform type
	if cfg.Bitbucket.SelfHosted {
		result.AddInfo(fmt.Sprintf("[OK] Bitbucket: Data Center / Server (base URL: %s)", cfg.Bitbucket.BaseURL))
	} else {
		result.AddInfo("[OK] Bitbucket: Cloud")
	}

	if cfg.Bitbucket.User == "" {
		result.AddError(
			"Configuration",
			"Bitbucket username is not configured",
			"Set BITBUCKET_USER environment variable or bitbucket.user in config.yaml",
		)
	}

	if cfg.Bitbucket.Token == "" {
		result.AddError(
			"Configuration",
			"Bitbucket token is not configured",
			"Set BITBUCKET_TOKEN environment variable or bitbucket.token in config.yaml",
		)
	}

	// Validate self-hosted configuration
	if cfg.Bitbucket.SelfHosted && cfg.Bitbucket.BaseURL == "" {
		result.AddError(
			"Configuration",
			"Bitbucket base_url is required when self-hosted is true",
			"Set BITBUCKET_BASE_URL environment variable or bitbucket.base_url in config.yaml (e.g., 'https://bitbucket.example.com')",
		)
	}

	// Event validation is now handled by config.Validate() which validates each TriggeringEvent

	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		result.AddError(
			"Configuration",
			fmt.Sprintf("Invalid server port: %d", cfg.Server.Port),
			"Set PORT environment variable or server.port in config.yaml to a valid port number (1-65535)",
		)
	}

	if cfg.Claude.TimeoutMinutes < 1 {
		result.AddError(
			"Configuration",
			fmt.Sprintf("Invalid Claude timeout: %d minutes", cfg.Claude.TimeoutMinutes),
			"Set claude.timeout_minutes to a positive value (recommended: 5-30 minutes)",
		)
	}
}

// validateVCSCredentials tests VCS API credentials using the VCS client interface
func validateVCSCredentials(cfg *config.Config, vcsClient ports.VCSClient, result *ValidationResult) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test connection using the VCS client
	err := vcsClient.TestConnection(ctx)
	if err != nil {
		// Parse error to provide helpful messages
		errMsg := err.Error()
		if err == errors.ErrUnauthorized {
			result.AddError(
				"VCS Credentials",
				"Authentication failed - invalid credentials",
				"Check your credentials configuration",
			)
		} else if strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "deadline exceeded") {
			result.AddError(
				"VCS Connection",
				"Connection timed out",
				"Check your network connection and VCS configuration",
			)
		} else {
			result.AddError(
				"VCS Connection",
				fmt.Sprintf("Failed to connect: %v", err),
				"Check your VCS configuration and network connection",
			)
		}
		return
	}

	// Successfully authenticated
	result.AddInfo(fmt.Sprintf("[OK] VCS credentials: Valid (authenticated as %s)", cfg.Bitbucket.User))
}

// validateClaudeCLI checks if Claude CLI is installed and accessible
func validateClaudeCLI(result *ValidationResult) {
	// Check if claude command exists
	_, err := exec.LookPath("claude")
	if err != nil {
		result.AddError(
			"Claude CLI",
			"Claude CLI is not installed or not in PATH",
			"Install Claude CLI from https://docs.anthropic.com/en/docs/claude-code",
		)
		return
	}

	// Try to get version to verify it's working
	cmd := exec.Command("claude", "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		result.AddError(
			"Claude CLI",
			fmt.Sprintf("Claude CLI is installed but not working properly: %v", err),
			"Reinstall Claude CLI or check your installation",
		)
		return
	}

	// Success - log version info
	version := strings.TrimSpace(string(output))
	result.AddInfo(fmt.Sprintf("[OK] Claude CLI: %s", version))

	// Check Claude authentication
	validateClaudeAuth(result)

	// Check if Bitbucket MCP is configured
	validateBitbucketMCP(result)
}

// validateGit checks if Git is installed and accessible
func validateGit(result *ValidationResult) {
	// Check if git command exists
	_, err := exec.LookPath("git")
	if err != nil {
		result.AddError(
			"Git",
			"Git is not installed or not in PATH",
			"Install Git: https://git-scm.com/downloads",
		)
		return
	}

	// Try to get version
	cmd := exec.Command("git", "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		result.AddError(
			"Git",
			fmt.Sprintf("Git is installed but not working properly: %v", err),
			"Reinstall Git or check your installation",
		)
		return
	}

	// Success - log version info
	version := strings.TrimSpace(string(output))
	result.AddInfo(fmt.Sprintf("[OK] %s", version))
}

// validateProfiles checks profiles directory and default profile
func validateProfiles(cfg *config.Config, result *ValidationResult, logger ports.Logger) {
	// Check if profiles directory exists
	if _, err := os.Stat(cfg.Profiles.Directory); os.IsNotExist(err) {
		result.AddError(
			"Profiles",
			fmt.Sprintf("Profiles directory does not exist: %s", cfg.Profiles.Directory),
			fmt.Sprintf("Create directory: mkdir -p %s", cfg.Profiles.Directory),
		)
		return
	}

	// Check if default profile .md file exists
	defaultProfilePath := filepath.Join(cfg.Profiles.Directory, cfg.Profiles.Default+".md")
	if _, err := os.Stat(defaultProfilePath); os.IsNotExist(err) {
		result.AddError(
			"Profiles",
			fmt.Sprintf("Default profile file does not exist: %s", defaultProfilePath),
			fmt.Sprintf("Create default profile: touch %s", defaultProfilePath),
		)
		return
	}

	// Check if default profile is not empty
	fileInfo, err := os.Stat(defaultProfilePath)
	if err != nil {
		result.AddError(
			"Profiles",
			fmt.Sprintf("Cannot read default profile: %v", err),
			"Check file permissions for profiles directory",
		)
		return
	}

	if fileInfo.Size() == 0 {
		result.AddError(
			"Profiles",
			fmt.Sprintf("Default profile is empty: %s", defaultProfilePath),
			"Add content to the profile file",
		)
		return
	}

	// Success
	result.AddInfo(fmt.Sprintf("[OK] Profiles: Found default profile at %s", defaultProfilePath))

	// Check configured project profiles (non-fatal warnings)
	validateProjectProfiles(cfg, logger)
}

// validateProjectProfiles checks if configured project profiles exist
func validateProjectProfiles(cfg *config.Config, logger ports.Logger) {
	for projectKey, projectProfile := range cfg.Profiles.Projects {
		// Check project-level profile
		if projectProfile.Profile != "" {
			profilePath := filepath.Join(cfg.Profiles.Directory, projectProfile.Profile+".md")
			if _, err := os.Stat(profilePath); os.IsNotExist(err) {
				logger.Warn(fmt.Sprintf("Profile for project %s not found: %s", projectKey, profilePath))
			}
		}

		// Check repository-level profiles
		for repoName, profileName := range projectProfile.Repositories {
			profilePath := filepath.Join(cfg.Profiles.Directory, profileName+".md")
			if _, err := os.Stat(profilePath); os.IsNotExist(err) {
				logger.Warn(fmt.Sprintf("Profile for %s/%s not found: %s", projectKey, repoName, profilePath))
			}
		}
	}
}

// validateDirectories checks directory permissions for writing
func validateDirectories(cfg *config.Config, result *ValidationResult) {
	// Check git base directory - using hardcoded "./projects" for now
	gitBaseDir := "./projects"
	if err := checkDirWritable(gitBaseDir); err != nil {
		result.AddError(
			"Directories",
			fmt.Sprintf("Cannot write to git directory: %s", gitBaseDir),
			fmt.Sprintf("Check permissions: chmod 755 %s or create directory: mkdir -p %s", gitBaseDir, gitBaseDir),
		)
	} else {
		result.AddInfo(fmt.Sprintf("[OK] Git base directory: %s (writable)", gitBaseDir))
	}

	// Check logs directory (if file logging is enabled)
	if cfg.Logging.EnableFile {
		logsDir := "./logs"
		if err := checkDirWritable(logsDir); err != nil {
			result.AddError(
				"Directories",
				fmt.Sprintf("Cannot write to logs directory: %s", logsDir),
				fmt.Sprintf("Create directory: mkdir -p %s && chmod 755 %s", logsDir, logsDir),
			)
		} else {
			result.AddInfo(fmt.Sprintf("[OK] Logs directory: %s (writable)", logsDir))
		}
	}
}

// checkDirWritable validates that directory exists and is writable
func checkDirWritable(dir string) error {
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return fmt.Errorf("does not exist")
	}
	if err != nil {
		return fmt.Errorf("cannot access: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory")
	}

	testFile := filepath.Join(dir, ".write-test")
	f, err := os.Create(testFile)
	if err != nil {
		return fmt.Errorf("not writable: %w", err)
	}
	_ = f.Close()
	_ = os.Remove(testFile)

	return nil
}

// validatePersistentStorage checks if persistent storage is reachable, readable, and writable
func validatePersistentStorage(cfg *config.Config, result *ValidationResult) {
	storagePath := cfg.Metrics.Persistence.Path
	storageType := cfg.Metrics.Persistence.Type

	switch storageType {
	case "filesystem":
		validateFilesystemStorage(storagePath, result)
	case "sqlite":
		validateSQLiteStorage(storagePath, result)
	default:
		result.AddError(
			"Persistent Storage",
			fmt.Sprintf("Unknown persistence type: %s", storageType),
			"Set metrics.persistence.type to 'filesystem' or 'sqlite' in config.yaml",
		)
	}
}

// validateFilesystemStorage checks filesystem-based storage is reachable, readable, and writable
func validateFilesystemStorage(storagePath string, result *ValidationResult) {
	if err := checkDirWritable(storagePath); err != nil {
		result.AddError(
			"Persistent Storage",
			fmt.Sprintf("Storage directory not accessible: %s - %v", storagePath, err),
			fmt.Sprintf("Create directory manually: mkdir -p %s && chmod 755 %s", storagePath, storagePath),
		)
		return
	}
	result.AddInfo(fmt.Sprintf("✓ Persistent storage: %s (filesystem) - reachable, readable, writable", storagePath))
}

// validateSQLiteStorage checks SQLite database is reachable, readable, and writable
func validateSQLiteStorage(storagePath string, result *ValidationResult) {
	// Determine directory to check - if path has extension, it's a file path
	dir := storagePath
	if filepath.Ext(storagePath) != "" {
		dir = filepath.Dir(storagePath)
	}

	if err := checkDirWritable(dir); err != nil {
		result.AddError(
			"Persistent Storage",
			fmt.Sprintf("Storage directory not accessible: %s - %v", dir, err),
			fmt.Sprintf("Create directory manually: mkdir -p %s && chmod 755 %s", dir, dir),
		)
		return
	}

	// Test database read/write using the metrics table (already initialized by metrics collector)
	dbPath := storagePath
	if filepath.Ext(storagePath) == "" {
		dbPath = filepath.Join(storagePath, "metrics.db")
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		result.AddError(
			"Persistent Storage",
			fmt.Sprintf("Cannot open SQLite database: %s - %v", dbPath, err),
			"Check SQLite installation and file permissions",
		)
		return
	}
	defer db.Close()

	testName := "_storage_validation_test"
	if _, err := db.Exec(`INSERT OR REPLACE INTO metrics (name, labels_json, value) VALUES (?, '{}', 1)`, testName); err != nil {
		result.AddError(
			"Persistent Storage",
			fmt.Sprintf("SQLite write test failed: %s - %v", dbPath, err),
			"Check file permissions and disk space",
		)
		return
	}

	var value float64
	if err := db.QueryRow(`SELECT value FROM metrics WHERE name = ?`, testName).Scan(&value); err != nil {
		result.AddError(
			"Persistent Storage",
			fmt.Sprintf("SQLite read test failed: %s - %v", dbPath, err),
			"Check database integrity",
		)
		return
	}

	_, _ = db.Exec(`DELETE FROM metrics WHERE name = ?`, testName)

	result.AddInfo(fmt.Sprintf("✓ Persistent storage: %s (sqlite) - reachable, readable, writable", storagePath))
}

// validateClaudeAuth checks if Claude CLI is authenticated
func validateClaudeAuth(result *ValidationResult) {
	// Try a minimal prompt that requires authentication
	// Using a simple prompt with -p flag and --model haiku for speed
	// With a timeout to detect invalid credentials (they cause hangs)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude", "-p", "hello", "--model", "haiku")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String() + stderr.String()

	// Check for timeout (invalid credentials cause command to hang)
	if ctx.Err() == context.DeadlineExceeded {
		result.AddError(
			"Claude Authentication",
			"Claude CLI authentication check timed out - likely invalid credentials",
			"Authenticate Claude CLI:\n"+
				"  Run: claude setup-token\n"+
				"  Or set ANTHROPIC_API_KEY environment variable\n"+
				"  Make sure your API key is valid (not expired or invalid)",
		)
		return
	}

	// Check for authentication errors
	// Claude shows "Invalid API key · Please run /login" for invalid/missing credentials
	outputLower := strings.ToLower(output)
	if err != nil ||
		strings.Contains(outputLower, "invalid api key") ||
		strings.Contains(outputLower, "please run /login") {
		result.AddError(
			"Claude Authentication",
			"Claude CLI is not authenticated or credentials are invalid",
			"Authenticate Claude CLI:\n"+
				"  Run: claude setup-token\n"+
				"  Or set ANTHROPIC_API_KEY environment variable\n"+
				"  Make sure your API key is valid",
		)
		return
	}

	result.AddInfo("[OK] Claude CLI is authenticated")
}

// validateBitbucketMCP checks if Bitbucket MCP is configured in Claude Code
func validateBitbucketMCP(result *ValidationResult) {
	// Use claude mcp list to check configured MCP servers
	cmd := exec.Command("claude", "mcp", "list")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String() + stderr.String()

	// Check if command succeeded
	if err != nil {
		result.AddError(
			"Bitbucket MCP",
			"Failed to check MCP servers configuration",
			"Ensure Claude CLI is properly installed and try: claude mcp list",
		)
		return
	}

	// Check if no MCP servers configured
	if strings.Contains(output, "No MCP servers configured") {
		result.AddError(
			"Bitbucket MCP",
			"No MCP servers configured in Claude Code",
			"Install and configure Bitbucket MCP:\n"+
				"  1. Run: claude mcp add\n"+
				"  2. Choose '@atlassian-dc-mcp/bitbucket'\n"+
				"  3. Configure your Bitbucket Data Center URL and credentials\n"+
				"  Or install manually: npx -y @atlassian-dc-mcp/bitbucket install",
		)
		return
	}

	// Check if output contains Bitbucket MCP indicators
	hasBitbucketMCP := strings.Contains(strings.ToLower(output), "bitbucket") ||
		strings.Contains(output, "@atlassian-dc-mcp/bitbucket")

	if !hasBitbucketMCP {
		result.AddError(
			"Bitbucket MCP",
			"Bitbucket MCP server is not configured (found other MCP servers but not Bitbucket)",
			"Add Bitbucket MCP server:\n"+
				"  1. Run: claude mcp add\n"+
				"  2. Choose '@atlassian-dc-mcp/bitbucket'\n"+
				"  3. Configure your Bitbucket Data Center URL and credentials",
		)
		return
	}

	result.AddInfo("[OK] Bitbucket MCP server is configured")
}
