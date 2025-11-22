package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/errors"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/models"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/ports"
)

// Repository implements ports.GitRepository
type Repository struct {
	baseDir string
	logger  ports.Logger
}

// Config holds git repository configuration
type Config struct {
	BaseDir string // Base directory for storing repositories (default: ./projects)
}

// NewRepository creates a new Repository
func NewRepository(cfg Config, logger ports.Logger) *Repository {
	if cfg.BaseDir == "" {
		cfg.BaseDir = "./projects"
	}

	return &Repository{
		baseDir: cfg.BaseDir,
		logger:  logger,
	}
}

// Clone clones a repository to the local filesystem
func (r *Repository) Clone(ctx context.Context, pr *models.PullRequest, credentials ports.Credentials) (string, error) {
	localPath := r.GetLocalPath(pr.ProjectKey, pr.RepositoryID)

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return "", errors.Wrap(errors.ErrorCodeGitCloneFailed,
			"failed to create parent directory",
			err,
		).WithMetadata("path", localPath)
	}

	// Build authenticated clone URL
	authURL := r.buildAuthURL(pr.CloneURL, credentials)

	r.logger.Info("Cloning repository",
		"project", pr.ProjectKey,
		"repo", pr.RepositoryID,
		"path", localPath,
	)

	// Create command with context for cancellation support
	cmd := exec.CommandContext(ctx, "git", "clone", authURL, localPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Sanitize output to prevent credential leaks
		sanitizedOutput := r.sanitizeOutput(string(output), credentials)
		return "", errors.Wrap(errors.ErrorCodeGitCloneFailed,
			fmt.Sprintf("git clone failed: %s", sanitizedOutput),
			err,
		).WithMetadata("project", pr.ProjectKey).
			WithMetadata("repo", pr.RepositoryID)
	}

	r.logger.Info("Repository cloned successfully",
		"project", pr.ProjectKey,
		"repo", pr.RepositoryID,
		"path", localPath,
	)

	return localPath, nil
}

// Update updates an existing local repository
func (r *Repository) Update(ctx context.Context, localPath, branch string, credentials ports.Credentials) error {
	r.logger.Info("Updating repository",
		"path", localPath,
		"branch", branch,
	)

	// Fetch all branches
	r.logger.Debug("Fetching latest changes from remote")
	fetchCmd := exec.CommandContext(ctx, "git", "-C", localPath, "fetch", "--all")
	if output, err := fetchCmd.CombinedOutput(); err != nil {
		sanitizedOutput := r.sanitizeOutput(string(output), credentials)
		return errors.Wrap(errors.ErrorCodeGitUpdateFailed,
			fmt.Sprintf("git fetch failed: %s", sanitizedOutput),
			err,
		).WithMetadata("path", localPath)
	}

	// Reset to origin/branch
	r.logger.Debug("Resetting to origin branch",
		"branch", branch,
	)
	resetCmd := exec.CommandContext(ctx, "git", "-C", localPath, "reset", "--hard", fmt.Sprintf("origin/%s", branch))
	if output, err := resetCmd.CombinedOutput(); err != nil {
		sanitizedOutput := r.sanitizeOutput(string(output), credentials)
		return errors.Wrap(errors.ErrorCodeGitUpdateFailed,
			fmt.Sprintf("git reset failed: %s", sanitizedOutput),
			err,
		).WithMetadata("path", localPath).
			WithMetadata("branch", branch)
	}

	// Clean untracked files
	cleanCmd := exec.CommandContext(ctx, "git", "-C", localPath, "clean", "-fd")
	if _, err := cleanCmd.CombinedOutput(); err != nil {
		// Clean is not critical, just log warning
		r.logger.Warn("Failed to clean untracked files",
			"path", localPath,
			"error", err,
		)
	}

	r.logger.Info("Repository updated successfully",
		"path", localPath,
		"branch", branch,
	)

	return nil
}

// GetOrUpdate gets a repository (cloning if necessary, updating if exists)
func (r *Repository) GetOrUpdate(ctx context.Context, pr *models.PullRequest, credentials ports.Credentials) (string, error) {
	localPath := r.GetLocalPath(pr.ProjectKey, pr.RepositoryID)

	if r.Exists(pr.ProjectKey, pr.RepositoryID) {
		// Repository exists, update it
		r.logger.Debug("Repository exists locally, updating",
			"path", localPath,
		)

		err := r.Update(ctx, localPath, pr.SourceBranch, credentials)
		if err != nil {
			// Log warning but continue with existing version
			r.logger.Warn("Failed to update repository, using existing version",
				"path", localPath,
				"error", err,
			)
			// Note: Not returning error here - we'll use the existing cached version
			// This is a trade-off between availability and freshness
		}

		return localPath, nil
	}

	// Repository doesn't exist, clone it
	r.logger.Debug("Repository not found locally, cloning",
		"path", localPath,
	)

	return r.Clone(ctx, pr, credentials)
}

// GetLocalPath returns the local path for a repository
func (r *Repository) GetLocalPath(projectKey, repoSlug string) string {
	return filepath.Join(r.baseDir, projectKey, repoSlug)
}

// Exists checks if a repository exists locally
func (r *Repository) Exists(projectKey, repoSlug string) bool {
	localPath := r.GetLocalPath(projectKey, repoSlug)
	gitDir := filepath.Join(localPath, ".git")

	_, err := os.Stat(gitDir)
	return err == nil
}

// Clean removes a local repository
func (r *Repository) Clean(projectKey, repoSlug string) error {
	localPath := r.GetLocalPath(projectKey, repoSlug)

	r.logger.Info("Cleaning local repository",
		"project", projectKey,
		"repo", repoSlug,
		"path", localPath,
	)

	if err := os.RemoveAll(localPath); err != nil {
		return errors.Wrap(errors.ErrorCodeGitAccessDenied,
			"failed to remove repository directory",
			err,
		).WithMetadata("path", localPath)
	}

	r.logger.Info("Repository cleaned successfully",
		"project", projectKey,
		"repo", repoSlug,
	)

	return nil
}

// buildAuthURL builds an authenticated Git URL
func (r *Repository) buildAuthURL(cloneURL string, credentials ports.Credentials) string {
	// Replace https:// with https://username:token@
	if strings.HasPrefix(cloneURL, "https://") {
		return strings.Replace(cloneURL, "https://",
			fmt.Sprintf("https://%s:%s@", credentials.Username, credentials.Token), 1)
	}
	return cloneURL
}

// sanitizeOutput removes credentials from git command output to prevent leaks
func (r *Repository) sanitizeOutput(output string, credentials ports.Credentials) string {
	sanitized := output

	// Remove username and token
	if credentials.Username != "" {
		sanitized = strings.ReplaceAll(sanitized, credentials.Username, "***")
	}
	if credentials.Token != "" {
		sanitized = strings.ReplaceAll(sanitized, credentials.Token, "***")
	}

	// Remove any embedded credentials in URLs
	// Pattern: https://username:token@domain -> https://***:***@domain
	sanitized = strings.ReplaceAll(sanitized,
		fmt.Sprintf("%s:%s@", credentials.Username, credentials.Token),
		"***:***@",
	)

	return sanitized
}

// ValidateBranchName validates that a branch name contains only allowed characters
func ValidateBranchName(branch string) error {
	if branch == "" {
		return errors.New(errors.ErrorCodeValidationFailed, "branch name cannot be empty")
	}

	// Allow alphanumeric, underscore, dot, forward slash, and hyphen
	for _, char := range branch {
		if !((char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '_' || char == '.' || char == '/' || char == '-') {
			return errors.New(errors.ErrorCodeValidationFailed,
				fmt.Sprintf("invalid branch name '%s': contains invalid character '%c'", branch, char),
			)
		}
	}

	return nil
}
