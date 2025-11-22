package ports

import (
	"context"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/models"
)

// VCSClient defines the interface for version control system operations (e.g., Bitbucket)
type VCSClient interface {
	// PostComment posts a comment on a pull request
	PostComment(ctx context.Context, projectKey, repoSlug string, prID int, comment string) error

	// AddCommentReaction adds an emoji reaction to a comment
	AddCommentReaction(ctx context.Context, projectKey, repoSlug string, prID, commentID int, emoji string) error

	// RemoveCommentReaction removes an emoji reaction from a comment
	RemoveCommentReaction(ctx context.Context, projectKey, repoSlug string, prID, commentID int, emoji string) error

	// GetPullRequest retrieves pull request information
	GetPullRequest(ctx context.Context, projectKey, repoSlug string, prID int) (*models.PullRequest, error)

	// ValidateWebhookSignature validates the webhook signature
	ValidateWebhookSignature(payload []byte, signature, secret string) bool

	// SetBaseURL sets the base URL for the VCS API
	SetBaseURL(baseURL string)
}

// VCSWebhookParser defines the interface for parsing VCS webhook payloads
type VCSWebhookParser interface {
	// ParsePullRequestEvent parses a pull request webhook event
	ParsePullRequestEvent(payload []byte) (*models.PullRequest, error)

	// ParseCommentEvent parses a comment webhook event
	ParseCommentEvent(payload []byte) (*models.PullRequest, error)

	// GetEventType extracts the event type from the payload
	GetEventType(payload []byte) (string, error)
}
