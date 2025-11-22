package profiles

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/config"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/errors"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/models"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/ports"
)


// Provider implements ports.ProfileProvider
type Provider struct {
	directory       string
	defaultProfile  string
	projectProfiles map[string]config.ProjectProfile
	logger          ports.Logger
}

// Config holds profile provider configuration
type Config struct {
	Directory       string
	DefaultProfile  string
	ProjectProfiles map[string]config.ProjectProfile
}

// NewProvider creates a new profile provider
func NewProvider(cfg Config, logger ports.Logger) *Provider {
	if cfg.Directory == "" {
		cfg.Directory = "./profiles"
	}
	if cfg.DefaultProfile == "" {
		cfg.DefaultProfile = "default"
	}
	if cfg.ProjectProfiles == nil {
		cfg.ProjectProfiles = make(map[string]config.ProjectProfile)
	}

	return &Provider{
		directory:       cfg.Directory,
		defaultProfile:  cfg.DefaultProfile,
		projectProfiles: cfg.ProjectProfiles,
		logger:          logger,
	}
}

// GetProfile retrieves the appropriate profile (template content) for a pull request
func (p *Provider) GetProfile(ctx context.Context, pr *models.PullRequest) (string, error) {
	// Determine profile name based on hierarchy
	profileName := p.resolveProfileName(pr.ProjectKey, pr.RepositoryID)

	// Load the profile file: <profile-name>.md
	profilePath := filepath.Join(p.directory, profileName+".md")

	p.logger.Debug("Loading review profile",
		"profile", profileName,
		"path", profilePath,
		"project", pr.ProjectKey,
		"repo", pr.RepositoryID,
	)

	content, err := os.ReadFile(profilePath)
	if err != nil {
		return "", errors.Wrap(errors.ErrorCodeTemplateNotFound,
			fmt.Sprintf("failed to load profile '%s'", profileName),
			err,
		).WithMetadata("profile", profileName).
			WithMetadata("path", profilePath)
	}

	profileContent := string(content)

	// Validate profile structure
	if err := p.ValidateProfile(profileContent); err != nil {
		return "", err
	}

	// Substitute variables
	processed := p.substituteVariables(profileContent, pr)

	p.logger.Debug("Profile loaded and processed successfully",
		"profile", profileName,
		"content_length", len(processed),
	)

	return processed, nil
}

// ValidateProfile validates that a profile is well-formed
func (p *Provider) ValidateProfile(profile string) error {
	// Check for required sections
	requiredSections := []string{"Role:", "Goal:", "PR:"}

	missing := []string{}
	for _, section := range requiredSections {
		if !strings.Contains(profile, section) {
			missing = append(missing, section)
		}
	}

	if len(missing) > 0 {
		return errors.New(errors.ErrorCodeTemplateInvalid,
			fmt.Sprintf("profile missing required sections: %v", missing),
		)
	}

	// Check minimum length
	if len(profile) < 100 {
		return errors.New(errors.ErrorCodeTemplateInvalid,
			"profile is too short (minimum 100 characters)",
		)
	}

	return nil
}

// ReloadProfiles reloads profiles from the filesystem
func (p *Provider) ReloadProfiles() error {
	// Validate that profile directory exists
	if _, err := os.Stat(p.directory); os.IsNotExist(err) {
		return errors.Wrap(errors.ErrorCodeTemplateNotFound,
			"profile directory does not exist",
			err,
		).WithMetadata("directory", p.directory)
	}

	// List and validate all profiles
	profiles, err := p.listProfiles()
	if err != nil {
		return errors.Wrap(errors.ErrorCodeTemplateNotFound,
			"failed to list profiles",
			err,
		)
	}

	p.logger.Info("Profiles reloaded",
		"count", len(profiles),
		"directory", p.directory,
	)

	return nil
}

// resolveProfileName determines which profile to use based on hierarchy
// Resolution priority:
// 1. Repository-specific profile
// 2. Project-level profile
// 3. Default profile
func (p *Provider) resolveProfileName(projectKey, repoSlug string) string {
	// Check if project configuration exists
	if projectConfig, ok := p.projectProfiles[projectKey]; ok {
		// Try repository-specific override first
		if profile, ok := projectConfig.Repositories[repoSlug]; ok {
			return profile
		}

		// Try project-level profile
		if projectConfig.Profile != "" {
			return projectConfig.Profile
		}
	}

	// Return default profile
	return p.defaultProfile
}

// substituteVariables replaces profile variables with actual PR data
func (p *Provider) substituteVariables(content string, pr *models.PullRequest) string {
	replacements := map[string]string{
		"{{prUrl}}":             pr.URL,
		"{{title}}":             pr.Title,
		"{{description}}":       pr.Description,
		"{{author}}":            pr.Author,
		"{{repository}}":        pr.RepositoryID,
		"{{sourceBranch}}":      pr.SourceBranch,
		"{{destinationBranch}}": pr.DestinationBranch,
		"{{repoCloneUrl}}":      pr.CloneURL,
		"{{projectKey}}":        pr.ProjectKey,
		"{{prId}}":              fmt.Sprintf("%d", pr.ID),
	}

	result := content
	for placeholder, value := range replacements {
		result = strings.ReplaceAll(result, placeholder, value)
	}

	return result
}

// listProfiles returns a list of available profiles
func (p *Provider) listProfiles() ([]string, error) {
	entries, err := os.ReadDir(p.directory)
	if err != nil {
		return nil, err
	}

	var profiles []string
	for _, entry := range entries {
		// Look for .md files
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".md" {
			// Profile name is filename without .md extension
			profileName := strings.TrimSuffix(entry.Name(), ".md")
			profiles = append(profiles, profileName)
		}
	}

	return profiles, nil
}
