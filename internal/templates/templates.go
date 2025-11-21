package templates

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/config"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/logger"
	"github.com/asmild/bitbucket-pr-reviewer-bot/pkg/models"
)

const (
	promptFileName = "prompt.md"
)

// GetTemplate gets the template name for a repository
// Resolution priority:
// 1. cfg.Templates.Projects[projectKey].Repositories[repository] - repo-specific override
// 2. cfg.Templates.Projects[projectKey].Template - project-level template
// 3. cfg.Templates.Default - global default
func GetTemplate(cfg *config.Config, projectKey, repository string) string {
	// Check if project configuration exists
	if projectConfig, ok := cfg.Templates.Projects[projectKey]; ok {
		// Try repository-specific override first
		if template, ok := projectConfig.Repositories[repository]; ok {
			return template
		}

		// Try project-level template
		if projectConfig.Template != "" {
			return projectConfig.Template
		}
	}

	// Return default template
	return cfg.Templates.Default
}

// LoadAndProcessTemplate loads and processes a template with PR data
func LoadAndProcessTemplate(cfg *config.Config, projectKey, repository string, prData *models.PRData) (string, error) {
	templateName := GetTemplate(cfg, projectKey, repository)
	templatePath := filepath.Join(cfg.Templates.Directory, templateName, promptFileName)

	logger.Debugf("Loading template %s for %s/%s from %s", templateName, projectKey, repository, templatePath)

	content, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to load template: %w", err)
	}

	// Validate template structure
	if err := validateTemplate(string(content)); err != nil {
		return "", err
	}

	// Substitute variables
	processed := substituteVariables(string(content), prData)

	return processed, nil
}

// validateTemplate validates that the template contains required sections
func validateTemplate(content string) error {
	requiredSections := []string{"Role:", "Goal:", "PR:"}

	for _, section := range requiredSections {
		if !strings.Contains(content, section) {
			return fmt.Errorf("template missing required section: %s", section)
		}
	}

	return nil
}

// substituteVariables replaces template variables with actual values
func substituteVariables(content string, prData *models.PRData) string {
	replacements := map[string]string{
		"{{prUrl}}":             prData.PRUrl,
		"{{title}}":             prData.Title,
		"{{description}}":       prData.Description,
		"{{author}}":            prData.Author,
		"{{repository}}":        prData.Repository,
		"{{sourceBranch}}":      prData.SourceBranch,
		"{{destinationBranch}}": prData.DestinationBranch,
		"{{repoCloneUrl}}":      prData.RepoCloneURL,
	}

	result := content
	for placeholder, value := range replacements {
		result = strings.ReplaceAll(result, placeholder, value)
	}

	return result
}

// ListTemplates returns a list of available templates
func ListTemplates(cfg *config.Config) ([]string, error) {
	entries, err := os.ReadDir(cfg.Templates.Directory)
	if err != nil {
		return nil, fmt.Errorf("failed to list templates in %s: %w", cfg.Templates.Directory, err)
	}

	var templates []string
	for _, entry := range entries {
		if entry.IsDir() {
			// Check if prompt.md exists
			promptPath := filepath.Join(cfg.Templates.Directory, entry.Name(), promptFileName)
			if _, err := os.Stat(promptPath); err == nil {
				templates = append(templates, entry.Name())
			}
		}
	}

	return templates, nil
}
