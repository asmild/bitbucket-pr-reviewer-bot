package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/logger"
)

// CloneRepository clones a repository to the specified path
func CloneRepository(cloneURL, projectPath, username, token string) error {
	// Build authenticated clone URL
	authURL := buildAuthURL(cloneURL, username, token)

	logger.Infof("Cloning repository from %s into local cache at %s", cloneURL, projectPath)

	cmd := exec.Command("git", "clone", authURL, projectPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to clone repository: %w, output: %s", err, string(output))
	}

	logger.Infof("Repository successfully cloned to local cache: %s", projectPath)
	return nil
}

// UpdateRepository fetches and resets the repository to the specified branch
func UpdateRepository(projectPath, sourceBranch, username, token string) error {
	logger.Infof("Updating local cache from remote: %s (branch: %s)", projectPath, sourceBranch)

	// Fetch all branches
	logger.Debugf("Fetching latest changes from remote...")
	fetchCmd := exec.Command("git", "-C", projectPath, "fetch", "--all")
	if output, err := fetchCmd.CombinedOutput(); err != nil {
		logger.Warnf("Failed to fetch from remote: %v, output: %s", err, string(output))
		return fmt.Errorf("failed to fetch repository: %w", err)
	}

	// Reset to origin/sourceBranch
	logger.Debugf("Resetting local cache to origin/%s...", sourceBranch)
	resetCmd := exec.Command("git", "-C", projectPath, "reset", "--hard", fmt.Sprintf("origin/%s", sourceBranch))
	if output, err := resetCmd.CombinedOutput(); err != nil {
		logger.Warnf("Failed to reset to origin/%s: %v, output: %s", sourceBranch, err, string(output))
		return fmt.Errorf("failed to reset repository: %w", err)
	}

	logger.Infof("Local cache updated successfully and synchronized with origin/%s", sourceBranch)
	return nil
}

// EnsureRepository ensures the repository exists and is up to date
func EnsureRepository(cloneURL, projectPath, sourceBranch, username, token string) error {
	if _, err := os.Stat(filepath.Join(projectPath, ".git")); os.IsNotExist(err) {
		// Repository doesn't exist, clone it
		logger.Infof("Local cache not found at %s, creating initial clone...", projectPath)
		if err := CloneRepository(cloneURL, projectPath, username, token); err != nil {
			return err
		}
	} else {
		// Repository exists, update it
		logger.Infof("Local cache found at %s, synchronizing with remote...", projectPath)
		if err := UpdateRepository(projectPath, sourceBranch, username, token); err != nil {
			logger.Warnf("Failed to sync local cache with remote, using existing version: %v", err)
			// Continue even if update fails
		}
	}

	return nil
}

// ValidateBranchName validates that a branch name contains only allowed characters
func ValidateBranchName(branch string) error {
	// Allow alphanumeric, underscore, dot, forward slash, and hyphen
	for _, char := range branch {
		if !((char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '_' || char == '.' || char == '/' || char == '-') {
			return fmt.Errorf("invalid branch name: %s", branch)
		}
	}
	return nil
}

// buildAuthURL builds an authenticated Git URL
func buildAuthURL(cloneURL, username, token string) string {
	// Replace https:// with https://username:token@
	if strings.HasPrefix(cloneURL, "https://") {
		return strings.Replace(cloneURL, "https://", fmt.Sprintf("https://%s:%s@", username, token), 1)
	}
	return cloneURL
}

// GetProjectPath returns the path for a project with structure: ./projects/{projectKey}/{repository}
func GetProjectPath(projectKey, repository string) string {
	return filepath.Join("./projects", projectKey, repository)
}
