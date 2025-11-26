package ports

import (
	"context"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/models"
)

// VCSClient defines the interface for version control system operations (e.g., Bitbucket)
type VCSClient interface {
	//PostComment posts a comment on a pull request
	//PostComment(ctx context.Context, projectKey, repoSlug string, prID int, comment string) error

	// AddCommentReaction adds an emoji reaction to a comment
	AddCommentReaction(ctx context.Context, projectKey, repoSlug string, prID, commentID int, emoji string) error

	// RemoveCommentReaction removes an emoji reaction from a comment
	RemoveCommentReaction(ctx context.Context, projectKey, repoSlug string, prID, commentID int, emoji string) error

	// ValidateWebhookSignature validates the webhook signature
	ValidateWebhookSignature(payload []byte, signature, secret string) bool

	// SetBaseURL sets the base URL for the VCS API
	SetBaseURL(baseURL string)
}

// VCSWebhookParser defines the interface for parsing VCS webhook payloads
type VCSWebhookParser interface {
	// Parse parses the raw webhook payload and returns a parsed webhook event
	Parse(payload []byte) (VCSWebhookEvent, error)
}

// VCSWebhookEvent represents a parsed webhook event
type VCSWebhookEvent interface {
	// GetEventType returns the type of the webhook event (e.g., "pr:opened", "pr:comment:added")
	GetEventType() string

	// GetPullRequest extracts pull request information from the event
	GetPullRequest() (*models.PullRequest, error)

	// GetComment returns comment information if this is a comment event, nil otherwise
	GetComment() *models.Comment
}
