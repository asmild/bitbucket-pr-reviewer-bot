package services

import (
	"context"
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

	// Add processing emoji reaction if manual trigger
	if pr.IsManualTrigger() {
		if err := s.addReaction(ctx, pr, "PROCESSING"); err != nil {
			s.logger.Warn("Failed to add processing reaction", "error", err)
		}
	}

	// Perform the review (Claude will post comments via MCP)
	_, err := s.performReview(ctx, pr)
	if err != nil {
		s.handleReviewError(ctx, pr, err)
		return err
	}

	// Success - update metrics and reactions
	duration := time.Since(startTime)
	s.metrics.IncrementReviewCompleted(pr.ProjectKey, "success")
	s.metrics.ObserveReviewDuration(pr.ProjectKey, duration)

	if pr.IsManualTrigger() {
		if err := s.removeReaction(ctx, pr, "PROCESSING"); err != nil {
			s.logger.Warn("Failed to remove processing reaction", "error", err)
		}
		// Add success reaction
		if err := s.addReaction(ctx, pr, "WHITE_CHECK_MARK"); err != nil {
			s.logger.Warn("Failed to add success reaction", "error", err)
		}
	}

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

	// Add failure emoji reaction if manual trigger
	if pr.IsManualTrigger() {
		if remErr := s.removeReaction(ctx, pr, "PROCESSING"); remErr != nil {
			s.logger.Warn("Failed to remove processing reaction", "error", remErr)
		}

		emoji := s.getErrorEmoji(err)
		if addErr := s.addReaction(ctx, pr, emoji); addErr != nil {
			s.logger.Warn("Failed to add error reaction", "error", addErr)
		}
	}
}

// getErrorEmoji returns the appropriate emoji for an error type
func (s *ReviewService) getErrorEmoji(err error) string {
	// Use X emoji for all error types - more visible than confused
	return "X"
}

// addReaction adds an emoji reaction to a comment
func (s *ReviewService) addReaction(ctx context.Context, pr *models.PullRequest, emoji string) error {
	if pr.CommentID == 0 {
		return nil
	}
	return s.vcsClient.AddCommentReaction(ctx, pr.ProjectKey, pr.RepositorySlug, pr.ID, pr.CommentID, emoji)
}

// removeReaction removes an emoji reaction from a comment
func (s *ReviewService) removeReaction(ctx context.Context, pr *models.PullRequest, emoji string) error {
	if pr.CommentID == 0 {
		return nil
	}
	return s.vcsClient.RemoveCommentReaction(ctx, pr.ProjectKey, pr.RepositorySlug, pr.ID, pr.CommentID, emoji)
}
