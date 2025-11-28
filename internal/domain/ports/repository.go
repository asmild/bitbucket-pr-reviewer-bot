package ports

import (
	"context"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/models"
)

// GitRepository defines the interface for git repository operations
type GitRepository interface {
	// Clone clones a repository to the local filesystem
	Clone(ctx context.Context, pr *models.PullRequest, credentials Credentials) (string, error)

	// Update updates an existing local repository
	Update(ctx context.Context, localPath, branch string, credentials Credentials) error

	// GetOrUpdate gets a repository (cloning if necessary, updating if exists)
	GetOrUpdate(ctx context.Context, pr *models.PullRequest, credentials Credentials) (string, error)

	// GetLocalPath returns the local path for a repository PR
	GetLocalPath(projectKey, repoSlug string, prID int) string

	// Exists checks if a repository PR exists locally
	Exists(projectKey, repoSlug string, prID int) bool

	// Clean removes a local repository PR
	Clean(projectKey, repoSlug string, prID int) error
}

// Credentials represents git credentials
type Credentials struct {
	Username string
	Token    string
}

// NewCredentials creates new Credentials
func NewCredentials(username, token string) Credentials {
	return Credentials{
		Username: username,
		Token:    token,
	}
}
