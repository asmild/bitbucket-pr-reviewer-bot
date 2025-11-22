package models

import (
	"fmt"
	"strings"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/errors"
)

// PullRequest represents a pull request with all necessary information for review
type PullRequest struct {
	// Identifiers
	ID         int
	CommentID  int // For manual trigger via comment

	// Metadata
	Title       string
	Description string
	Author      string
	URL         string

	// Branch information
	SourceBranch      string
	DestinationBranch string

	// Repository information
	ProjectKey   string
	RepositoryID string
	CloneURL     string
}

// NewPullRequest creates a new PullRequest with validation
func NewPullRequest(
	id int,
	projectKey, repoID string,
	title, description, author string,
	sourceBranch, destBranch string,
	cloneURL, url string,
) (*PullRequest, error) {
	pr := &PullRequest{
		ID:                id,
		ProjectKey:        projectKey,
		RepositoryID:      repoID,
		Title:             title,
		Description:       description,
		Author:            author,
		SourceBranch:      sourceBranch,
		DestinationBranch: destBranch,
		CloneURL:          cloneURL,
		URL:               url,
	}

	if err := pr.Validate(); err != nil {
		return nil, err
	}

	return pr, nil
}

// Validate performs validation on the PullRequest
func (pr *PullRequest) Validate() error {
	var validationErrors []string

	if pr.ID <= 0 {
		validationErrors = append(validationErrors, "ID must be positive")
	}

	if strings.TrimSpace(pr.ProjectKey) == "" {
		validationErrors = append(validationErrors, "ProjectKey is required")
	}

	if strings.TrimSpace(pr.RepositoryID) == "" {
		validationErrors = append(validationErrors, "RepositoryID is required")
	}

	if strings.TrimSpace(pr.Title) == "" {
		validationErrors = append(validationErrors, "Title is required")
	}

	if strings.TrimSpace(pr.Author) == "" {
		validationErrors = append(validationErrors, "Author is required")
	}

	if strings.TrimSpace(pr.SourceBranch) == "" {
		validationErrors = append(validationErrors, "SourceBranch is required")
	}

	if strings.TrimSpace(pr.DestinationBranch) == "" {
		validationErrors = append(validationErrors, "DestinationBranch is required")
	}

	if strings.TrimSpace(pr.CloneURL) == "" {
		validationErrors = append(validationErrors, "CloneURL is required")
	}

	if len(validationErrors) > 0 {
		return errors.Wrap(
			errors.ErrorCodeValidationFailed,
			"pull request validation failed",
			fmt.Errorf("%s", strings.Join(validationErrors, "; ")),
		)
	}

	return nil
}

// WithCommentID sets the comment ID for manual trigger
func (pr *PullRequest) WithCommentID(commentID int) *PullRequest {
	pr.CommentID = commentID
	return pr
}

// IsManualTrigger returns true if this PR review was manually triggered via comment
func (pr *PullRequest) IsManualTrigger() bool {
	return pr.CommentID > 0
}

// GetRepositoryPath returns a unique path for the repository
func (pr *PullRequest) GetRepositoryPath() string {
	return fmt.Sprintf("%s/%s", pr.ProjectKey, pr.RepositoryID)
}

// String returns a human-readable representation of the PR
func (pr *PullRequest) String() string {
	return fmt.Sprintf("PR#%d: %s (%s -> %s) by %s",
		pr.ID, pr.Title, pr.SourceBranch, pr.DestinationBranch, pr.Author)
}

// ReviewContext contains contextual information for the review process
type ReviewContext struct {
	PullRequest *PullRequest
	Template    string
	ProjectKey  string
	RepoSlug    string
}

// NewReviewContext creates a new ReviewContext
func NewReviewContext(pr *PullRequest, template string) *ReviewContext {
	return &ReviewContext{
		PullRequest: pr,
		Template:    template,
		ProjectKey:  pr.ProjectKey,
		RepoSlug:    pr.RepositoryID,
	}
}
