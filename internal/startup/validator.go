package startup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/config"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/logger"
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

// ValidationResult holds all validation errors
type ValidationResult struct {
	Errors []*ValidationError
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

// LogErrors logs all validation errors using the logger
func (r *ValidationResult) LogErrors() {
	if r.IsValid() {
		return
	}

	logger.Error("Startup Dependencies Check Failed")
	logger.Error("")

	for i, err := range r.Errors {
		logger.Error(fmt.Sprintf("%d. %s", i+1, err.Error()))
		logger.Error("")
	}

	logger.Error("Please fix the above issues and restart the application.")
}

// ValidateDependencies checks all startup dependencies and requirements
func ValidateDependencies(cfg *config.Config) *ValidationResult {
	result := &ValidationResult{}

	// Check configuration
	validateConfiguration(cfg, result)

	// Check external dependencies
	validateClaudeCLI(result)
	validateGit(result)

	// Check templates
	validateTemplates(cfg, result)

	// Check directory permissions
	validateDirectories(cfg, result)

	return result
}

// validateConfiguration checks required configuration values
func validateConfiguration(cfg *config.Config, result *ValidationResult) {
	if cfg.Bitbucket.User == "" {
		result.AddError(
			"Configuration",
			"Bitbucket user is not configured",
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

	if cfg.Bitbucket.EventType != "pr_opened" && cfg.Bitbucket.EventType != "comment_added" {
		result.AddError(
			"Configuration",
			fmt.Sprintf("Invalid event_type: %s", cfg.Bitbucket.EventType),
			"Set bitbucket.event_type to either 'pr_opened' or 'comment_added'",
		)
	}

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
	logger.Info(fmt.Sprintf("Claude CLI: %s", version))

	// Check if authenticated with a minimal test command
	cmd = exec.Command("claude", "-p", "ping", "--tools", "\"\"")
	output, err = cmd.CombinedOutput()
	outputStr := string(output)

	// Check for authentication errors
	if err != nil || strings.Contains(outputStr, "authentication") || strings.Contains(outputStr, "Authentication required") {
		result.AddError(
			"Claude CLI",
			"Claude CLI is not authenticated",
			"Run 'claude setup-token' to authenticate with your API key",
		)
		return
	}

	logger.Info("Claude CLI authentication verified")
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
	logger.Info(version)
}

// validateTemplates checks templates directory and default template
func validateTemplates(cfg *config.Config, result *ValidationResult) {
	// Check if templates directory exists
	if _, err := os.Stat(cfg.Templates.Directory); os.IsNotExist(err) {
		result.AddError(
			"Templates",
			fmt.Sprintf("Templates directory does not exist: %s", cfg.Templates.Directory),
			fmt.Sprintf("Create directory: mkdir -p %s", cfg.Templates.Directory),
		)
		return
	}

	// Check if default template exists
	defaultTemplatePath := filepath.Join(cfg.Templates.Directory, cfg.Templates.Default)
	if _, err := os.Stat(defaultTemplatePath); os.IsNotExist(err) {
		result.AddError(
			"Templates",
			fmt.Sprintf("Default template directory does not exist: %s", defaultTemplatePath),
			fmt.Sprintf("Create default template: mkdir -p %s && touch %s/prompt.md", defaultTemplatePath, defaultTemplatePath),
		)
		return
	}

	// Check if default template has prompt.md
	promptPath := filepath.Join(defaultTemplatePath, "prompt.md")
	if _, err := os.Stat(promptPath); os.IsNotExist(err) {
		result.AddError(
			"Templates",
			fmt.Sprintf("Default template missing prompt.md: %s", promptPath),
			fmt.Sprintf("Create prompt file: touch %s", promptPath),
		)
		return
	}

	// Check if prompt.md is not empty
	fileInfo, err := os.Stat(promptPath)
	if err != nil {
		result.AddError(
			"Templates",
			fmt.Sprintf("Cannot read default template: %v", err),
			"Check file permissions for templates directory",
		)
		return
	}

	if fileInfo.Size() == 0 {
		result.AddError(
			"Templates",
			fmt.Sprintf("Default template is empty: %s", promptPath),
			"Add content to the template file or copy from config.example.yaml",
		)
		return
	}

	// Success
	logger.Info(fmt.Sprintf("Templates: Found default template at %s", defaultTemplatePath))

	// Check configured project templates (non-fatal warnings)
	validateProjectTemplates(cfg, result)
}

// validateProjectTemplates checks if configured project templates exist
func validateProjectTemplates(cfg *config.Config, result *ValidationResult) {
	for projectKey, projectTemplate := range cfg.Templates.Projects {
		// Check project-level template
		if projectTemplate.Template != "" {
			templatePath := filepath.Join(cfg.Templates.Directory, projectTemplate.Template)
			if _, err := os.Stat(templatePath); os.IsNotExist(err) {
				logger.Warn(fmt.Sprintf("Template for project %s not found: %s", projectKey, templatePath))
			} else {
				promptPath := filepath.Join(templatePath, "prompt.md")
				if _, err := os.Stat(promptPath); os.IsNotExist(err) {
					logger.Warn(fmt.Sprintf("Template %s missing prompt.md", projectTemplate.Template))
				}
			}
		}

		// Check repository-level templates
		for repoName, templateName := range projectTemplate.Repositories {
			templatePath := filepath.Join(cfg.Templates.Directory, templateName)
			if _, err := os.Stat(templatePath); os.IsNotExist(err) {
				logger.Warn(fmt.Sprintf("Template for %s/%s not found: %s", projectKey, repoName, templatePath))
			} else {
				promptPath := filepath.Join(templatePath, "prompt.md")
				if _, err := os.Stat(promptPath); os.IsNotExist(err) {
					logger.Warn(fmt.Sprintf("Template %s missing prompt.md", templateName))
				}
			}
		}
	}
}

// validateDirectories checks directory permissions for writing
func validateDirectories(cfg *config.Config, result *ValidationResult) {
	// Check logs directory (will be created by logger)
	logsDir := "./logs"
	if cfg.Logging.EnableFile {
		if err := ensureDirectoryWritable(logsDir); err != nil {
			result.AddError(
				"Directories",
				fmt.Sprintf("Cannot write to logs directory: %v", err),
				fmt.Sprintf("Create directory or fix permissions: mkdir -p %s && chmod 755 %s", logsDir, logsDir),
			)
		} else {
			logger.Info(fmt.Sprintf("Logs directory is writable: %s", logsDir))
		}
	}

	// Check metrics directory if persistence is enabled
	if cfg.Metrics.Persistence.Enabled {
		metricsDir := cfg.Metrics.Persistence.Path
		if err := ensureDirectoryWritable(metricsDir); err != nil {
			result.AddError(
				"Directories",
				fmt.Sprintf("Cannot write to metrics directory: %v", err),
				fmt.Sprintf("Create directory or fix permissions: mkdir -p %s && chmod 755 %s", metricsDir, metricsDir),
			)
		} else {
			logger.Info(fmt.Sprintf("Metrics directory is writable: %s", metricsDir))
		}
	}

	// Check projects directory (for git repositories)
	projectsDir := "./projects"
	if err := ensureDirectoryWritable(projectsDir); err != nil {
		result.AddError(
			"Directories",
			fmt.Sprintf("Cannot write to projects directory: %v", err),
			fmt.Sprintf("Create directory or fix permissions: mkdir -p %s && chmod 755 %s", projectsDir, projectsDir),
		)
	} else {
		logger.Info(fmt.Sprintf("Projects directory is writable: %s", projectsDir))
	}
}

// ensureDirectoryWritable checks if a directory exists and is writable
func ensureDirectoryWritable(dir string) error {
	// Create if doesn't exist
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("cannot create directory: %w", err)
		}
	}

	// Check if writable by creating a temp file
	testFile := filepath.Join(dir, ".write-test")
	f, err := os.Create(testFile)
	if err != nil {
		return fmt.Errorf("directory is not writable: %w", err)
	}
	f.Close()
	os.Remove(testFile)

	return nil
}
