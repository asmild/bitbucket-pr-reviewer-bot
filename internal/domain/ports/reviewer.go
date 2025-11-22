package ports

import (
	"context"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/models"
)

// AIReviewer defines the interface for AI-powered code review
type AIReviewer interface {
	// Review performs a code review on a pull request
	Review(ctx context.Context, request *ReviewRequest) (*models.ReviewResult, error)

	// GetModelName returns the name of the AI model being used
	GetModelName() string

	// IsAvailable checks if the AI reviewer is available
	IsAvailable(ctx context.Context) error
}

// ReviewRequest contains all information needed to perform a review
type ReviewRequest struct {
	PullRequest  *models.PullRequest
	RepositoryPath string
	Template     string
	Timeout      int // in seconds
}

// NewReviewRequest creates a new ReviewRequest
func NewReviewRequest(pr *models.PullRequest, repoPath, template string, timeout int) *ReviewRequest {
	return &ReviewRequest{
		PullRequest:    pr,
		RepositoryPath: repoPath,
		Template:       template,
		Timeout:        timeout,
	}
}
