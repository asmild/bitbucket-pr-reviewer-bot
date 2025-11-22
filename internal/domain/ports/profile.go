package ports

import (
	"context"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/models"
)

// ProfileProvider defines the interface for review profile management
// A profile determines which template to use for reviewing a PR
type ProfileProvider interface {
	// GetProfile retrieves the appropriate review profile (template content) for a pull request
	GetProfile(ctx context.Context, pr *models.PullRequest) (string, error)

	// ValidateProfile validates that a profile is well-formed
	ValidateProfile(profile string) error

	// ReloadProfiles reloads profiles from the filesystem
	ReloadProfiles() error
}
