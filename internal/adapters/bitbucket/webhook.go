package bitbucket

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/errors"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/models"
)

var branchNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_./-]+$`)

// WebhookParser implements ports.VCSWebhookParser for Bitbucket
type WebhookParser struct{}

// NewWebhookParser creates a new WebhookParser
func NewWebhookParser() *WebhookParser {
	return &WebhookParser{}
}

// ParsePullRequestEvent parses a pull request webhook event
func (p *WebhookParser) ParsePullRequestEvent(payload []byte) (*models.PullRequest, error) {
	var webhookPayload WebhookPayload
	if err := json.Unmarshal(payload, &webhookPayload); err != nil {
		return nil, errors.Wrap(errors.ErrorCodeVCSInvalidPayload,
			"invalid JSON payload",
			err,
		)
	}

	if err := p.validatePayload(&webhookPayload); err != nil {
		return nil, err
	}

	return p.extractPullRequest(&webhookPayload)
}

// ParseCommentEvent parses a comment webhook event
func (p *WebhookParser) ParseCommentEvent(payload []byte) (*models.PullRequest, error) {
	var webhookPayload WebhookPayload
	if err := json.Unmarshal(payload, &webhookPayload); err != nil {
		return nil, errors.Wrap(errors.ErrorCodeVCSInvalidPayload,
			"invalid JSON payload",
			err,
		)
	}

	if err := p.validatePayload(&webhookPayload); err != nil {
		return nil, err
	}

	if webhookPayload.Comment == nil {
		return nil, errors.New(errors.ErrorCodeVCSInvalidPayload,
			"comment is missing from payload",
		)
	}

	pr, err := p.extractPullRequest(&webhookPayload)
	if err != nil {
		return nil, err
	}

	// Add comment ID
	return pr.WithCommentID(webhookPayload.Comment.ID), nil
}

// GetEventType extracts the event type from the payload
func (p *WebhookParser) GetEventType(payload []byte) (string, error) {
	var event struct {
		EventKey string `json:"eventKey"`
	}

	if err := json.Unmarshal(payload, &event); err != nil {
		return "", errors.Wrap(errors.ErrorCodeVCSInvalidPayload,
			"failed to parse event type",
			err,
		)
	}

	return event.EventKey, nil
}

// validatePayload validates the webhook payload structure
func (p *WebhookParser) validatePayload(payload *WebhookPayload) error {
	if payload.PullRequest.Title == "" {
		return errors.New(errors.ErrorCodeValidationFailed,
			"pullRequest.title is required",
		)
	}

	if payload.PullRequest.Author.User.DisplayName == "" {
		return errors.New(errors.ErrorCodeValidationFailed,
			"pullRequest.author.user.displayName is required",
		)
	}

	if payload.PullRequest.FromRef.DisplayID == "" {
		return errors.New(errors.ErrorCodeValidationFailed,
			"pullRequest.fromRef.displayId is required",
		)
	}

	if payload.PullRequest.ToRef.DisplayID == "" {
		return errors.New(errors.ErrorCodeValidationFailed,
			"pullRequest.toRef.displayId is required",
		)
	}

	// Repository slug can be at root level or in fromRef
	if payload.Repository.Slug == "" && payload.PullRequest.FromRef.Repository.Slug == "" {
		return errors.New(errors.ErrorCodeValidationFailed,
			"repository.slug is required",
		)
	}

	// Validate branch names
	if !branchNameRegex.MatchString(payload.PullRequest.FromRef.DisplayID) {
		return errors.New(errors.ErrorCodeValidationFailed,
			fmt.Sprintf("invalid source branch name: %s", payload.PullRequest.FromRef.DisplayID),
		)
	}

	if !branchNameRegex.MatchString(payload.PullRequest.ToRef.DisplayID) {
		return errors.New(errors.ErrorCodeValidationFailed,
			fmt.Sprintf("invalid destination branch name: %s", payload.PullRequest.ToRef.DisplayID),
		)
	}

	return nil
}

// extractPullRequest extracts PR data from the webhook payload
func (p *WebhookParser) extractPullRequest(payload *WebhookPayload) (*models.PullRequest, error) {
	// Determine repository slug
	repoSlug := payload.Repository.Slug
	if repoSlug == "" {
		repoSlug = payload.PullRequest.FromRef.Repository.Slug
	}

	// Find HTTP clone URL
	cloneURL := p.findCloneURL(payload)
	if cloneURL == "" {
		return nil, errors.New(errors.ErrorCodeVCSInvalidPayload,
			"no HTTP clone URL found in payload",
		)
	}

	// Get PR URL
	prURL := ""
	if len(payload.PullRequest.Links.Self) > 0 {
		prURL = payload.PullRequest.Links.Self[0].Href
	}

	// Extract project key
	projectKey := p.extractProjectKey(payload)
	if projectKey == "" {
		return nil, errors.New(errors.ErrorCodeVCSInvalidPayload,
			"could not determine project key from payload",
		)
	}

	return models.NewPullRequest(
		payload.PullRequest.ID,
		projectKey,
		repoSlug,
		payload.PullRequest.Title,
		payload.PullRequest.Description,
		payload.PullRequest.Author.User.DisplayName,
		payload.PullRequest.FromRef.DisplayID,
		payload.PullRequest.ToRef.DisplayID,
		cloneURL,
		prURL,
	)
}

// findCloneURL finds the HTTP clone URL from the payload
func (p *WebhookParser) findCloneURL(payload *WebhookPayload) string {
	// Try root repository first
	if len(payload.Repository.Links.Clone) > 0 {
		for _, link := range payload.Repository.Links.Clone {
			if link.Name == "http" {
				return link.Href
			}
		}
	}

	// Try fromRef.repository
	if len(payload.PullRequest.FromRef.Repository.Links.Clone) > 0 {
		for _, link := range payload.PullRequest.FromRef.Repository.Links.Clone {
			if link.Name == "http" {
				return link.Href
			}
		}
	}

	return ""
}

// extractProjectKey extracts the project key from the payload
func (p *WebhookParser) extractProjectKey(payload *WebhookPayload) string {
	if payload.Repository.Project != nil {
		return payload.Repository.Project.Key
	}

	if payload.PullRequest.FromRef.Repository.Project != nil {
		return payload.PullRequest.FromRef.Repository.Project.Key
	}

	if payload.PullRequest.ToRef.Repository.Project != nil {
		return payload.PullRequest.ToRef.Repository.Project.Key
	}

	return ""
}

// ExtractBaseURL extracts the base URL from a clone URL
// Example: https://bitbucket.example.com/scm/proj/repo.git -> https://bitbucket.example.com
func ExtractBaseURL(cloneURL string) string {
	parts := strings.Split(cloneURL, "/scm/")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// WebhookPayload represents the Bitbucket Data Center webhook payload
type WebhookPayload struct {
	PullRequest PullRequest `json:"pullRequest"`
	Repository  Repository  `json:"repository"`
	Comment     *Comment    `json:"comment"`
}

type PullRequest struct {
	ID          int     `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Author      Author  `json:"author"`
	FromRef     Ref     `json:"fromRef"`
	ToRef       Ref     `json:"toRef"`
	Links       PRLinks `json:"links"`
}

type Author struct {
	User User `json:"user"`
}

type User struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
}

type Ref struct {
	ID           string     `json:"id"`
	DisplayID    string     `json:"displayId"`
	LatestCommit string     `json:"latestCommit"`
	Repository   Repository `json:"repository"`
}

type PRLinks struct {
	Self []Link `json:"self"`
}

type Repository struct {
	Slug    string    `json:"slug"`
	Name    string    `json:"name"`
	Project *Project  `json:"project"`
	Links   RepoLinks `json:"links"`
}

type Project struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

type RepoLinks struct {
	Clone []CloneLink `json:"clone"`
	Self  []Link      `json:"self"`
}

type Link struct {
	Href string `json:"href"`
}

type CloneLink struct {
	Name string `json:"name"`
	Href string `json:"href"`
}

type Comment struct {
	ID      int    `json:"id"`
	Text    string `json:"text"`
	Author  Author `json:"author"`
	Version int    `json:"version"`
}
