package profiles

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

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

	// In-memory cache for profile contents
	cache   map[string]string
	cacheMu sync.RWMutex
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
		cache:           make(map[string]string),
	}
}

// GetProfile retrieves the appropriate profile (template content) for a pull request
func (p *Provider) GetProfile(ctx context.Context, pr *models.PullRequest) (string, error) {
	// Determine profile name based on hierarchy
	profileName := p.resolveProfileName(pr.ProjectKey, pr.RepositorySlug)

	// Try to get from cache first (read lock)
	p.cacheMu.RLock()
	cached, found := p.cache[profileName]
	p.cacheMu.RUnlock()

	var profileContent string
	if found {
		p.logger.Debug("Profile loaded from cache",
			"profile", profileName,
			"project", pr.ProjectKey,
			"repo", pr.RepositorySlug,
		)
		profileContent = cached
	} else {
		// Cache miss - load from disk
		profilePath := filepath.Join(p.directory, profileName+".md")

		p.logger.Debug("Loading review profile from disk",
			"profile", profileName,
			"path", profilePath,
			"project", pr.ProjectKey,
			"repo", pr.RepositorySlug,
		)

		content, err := os.ReadFile(profilePath)
		if err != nil {
			return "", errors.Wrap(errors.ErrorCodeProfileNotFound,
				fmt.Sprintf("failed to load profile '%s'", profileName),
				err,
			).WithMetadata("profile", profileName).
				WithMetadata("path", profilePath)
		}

		profileContent = string(content)

		// Validate profile structure
		if err := p.ValidateProfile(profileContent); err != nil {
			return "", err
		}

		// Store in cache (write lock)
		p.cacheMu.Lock()
		p.cache[profileName] = profileContent
		p.cacheMu.Unlock()

		p.logger.Debug("Profile loaded from disk and cached",
			"profile", profileName,
		)
	}

	// Substitute variables (always do this as PR data changes)
	processed := p.substituteVariables(profileContent, pr)

	p.logger.Debug("Profile processed successfully",
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
		return errors.New(errors.ErrorCodeProfileInvalid,
			fmt.Sprintf("profile missing required sections: %v", missing),
		)
	}

	// Check minimum length
	if len(profile) < 100 {
		return errors.New(errors.ErrorCodeProfileInvalid,
			"profile is too short (minimum 100 characters)",
		)
	}

	return nil
}

// ReloadProfiles reloads profiles from the filesystem and clears the cache
func (p *Provider) ReloadProfiles() error {
	// Validate that profile directory exists
	if _, err := os.Stat(p.directory); os.IsNotExist(err) {
		return errors.Wrap(errors.ErrorCodeProfileNotFound,
			"profile directory does not exist",
			err,
		).WithMetadata("directory", p.directory)
	}

	// List and validate all profiles
	profiles, err := p.listProfiles()
	if err != nil {
		return errors.Wrap(errors.ErrorCodeProfileNotFound,
			"failed to list profiles",
			err,
		)
	}

	// Clear cache to force reload on next request
	p.cacheMu.Lock()
	p.cache = make(map[string]string)
	p.cacheMu.Unlock()

	p.logger.Info("Profiles reloaded and cache cleared",
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
		"{{repository}}":        pr.RepositorySlug,
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
