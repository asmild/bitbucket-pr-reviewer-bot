package ports

import (
	"context"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/models"
)

// ReviewQueue defines the interface for managing review requests
type ReviewQueue interface {
	// Enqueue adds a pull request to the review queue
	Enqueue(ctx context.Context, pr *models.PullRequest) (int, error)

	// Start starts the queue worker
	Start(ctx context.Context)

	// Stop gracefully stops the queue worker
	Stop(ctx context.Context) error

	// GetSize returns the current queue size
	GetSize() int

	// IsRunning returns true if the queue is running
	IsRunning() bool

	// GetActiveWorkers returns the number of workers currently processing PRs
	GetActiveWorkers() int
}
