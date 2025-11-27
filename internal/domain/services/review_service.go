package services

import (
	"context"
	"sync"
	"time"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/errors"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/models"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/ports"
)

// ReviewService orchestrates the pull request review process
type ReviewService struct {
	vcsClient       ports.VCSClient
	gitRepo         ports.GitRepository
	aiReviewer      ports.AIReviewer
	profileProvider ports.ProfileProvider
	metrics         ports.MetricsCollector
	logger          ports.Logger
	credentials     ports.Credentials
	reviewTimeout   int
	wg              sync.WaitGroup
}

// NewReviewService creates a new ReviewService
func NewReviewService(
	vcsClient ports.VCSClient,
	gitRepo ports.GitRepository,
	aiReviewer ports.AIReviewer,
	profileProvider ports.ProfileProvider,
	metrics ports.MetricsCollector,
	logger ports.Logger,
	credentials ports.Credentials,
	reviewTimeout int,
) *ReviewService {
	return &ReviewService{
		vcsClient:       vcsClient,
		gitRepo:         gitRepo,
		aiReviewer:      aiReviewer,
		profileProvider: profileProvider,
		metrics:         metrics,
		logger:          logger,
		credentials:     credentials,
		reviewTimeout:   reviewTimeout,
	}
}

// ReviewPullRequest performs a complete review of a pull request
func (s *ReviewService) ReviewPullRequest(ctx context.Context, pr *models.PullRequest) error {
	startTime := time.Now()

	s.logger.Info("Starting PR review",
		"project", pr.ProjectKey,
		"repo", pr.RepositorySlug,
		"pr_id", pr.ID,
		"author", pr.Author,
	)

	s.metrics.IncrementReviewStarted(pr.ProjectKey)
	s.metrics.IncrementUniquePRReviewed(pr.ProjectKey, pr.RepositorySlug, pr.ID)

	// Ensure repository cleanup after review (success or failure)
	defer func() {
		if err := s.gitRepo.Clean(pr.ProjectKey, pr.RepositorySlug); err != nil {
			s.logger.Warn("Failed to clean repository after review",
				"project", pr.ProjectKey,
				"repo", pr.RepositorySlug,
				"error", err,
			)
		}
	}()

	// Perform the review (Claude will post comments via MCP)
	_, err := s.performReview(ctx, pr)
	if err != nil {
		s.handleReviewError(ctx, pr, err)
		return err
	}

	// Success - update metrics
	duration := time.Since(startTime)
	s.metrics.IncrementReviewCompleted(pr.ProjectKey, "success")
	s.metrics.ObserveReviewDuration(pr.ProjectKey, duration)

	s.logger.Info("PR review completed successfully",
		"project", pr.ProjectKey,
		"pr_id", pr.ID,
		"duration", duration,
	)

	return nil
}

// performReview executes the core review logic
func (s *ReviewService) performReview(ctx context.Context, pr *models.PullRequest) (*models.ReviewResult, error) {
	// Step 1: Get or update repository
	repoPath, err := s.getRepository(ctx, pr)
	if err != nil {
		return nil, err
	}

	// Step 2: Get the appropriate profile (template content)
	profile, err := s.profileProvider.GetProfile(ctx, pr)
	if err != nil {
		return nil, errors.Wrap(errors.ErrorCodeProfileNotFound,
			"failed to get review profile",
			err,
		)
	}

	// Step 3: Perform AI review
	reviewReq := ports.NewReviewRequest(pr, repoPath, profile, s.reviewTimeout)
	result, err := s.aiReviewer.Review(ctx, reviewReq)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// getRepository retrieves or updates the git repository
func (s *ReviewService) getRepository(ctx context.Context, pr *models.PullRequest) (string, error) {
	startTime := time.Now()

	repoPath, err := s.gitRepo.GetOrUpdate(ctx, pr, s.credentials)
	if err != nil {
		return "", errors.Wrap(errors.ErrorCodeGitCloneFailed,
			"failed to get repository",
			err,
		).WithMetadata("project", pr.ProjectKey).
			WithMetadata("repo", pr.RepositoryID)
	}

	s.metrics.ObserveGitCloneDuration(time.Since(startTime))
	return repoPath, nil
}

// postReviewComment posts the review result as a comment
// handleReviewError handles errors during the review process
func (s *ReviewService) handleReviewError(ctx context.Context, pr *models.PullRequest, err error) {
	errorType := string(errors.GetCode(err))
	s.metrics.IncrementReviewFailed(pr.ProjectKey, errorType)
}
