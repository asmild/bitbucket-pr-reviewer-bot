package bitbucket

import (
	"encoding/json"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/errors"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/models"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/ports"
)

// WebhookParser implements ports.VCSWebhookParser for Bitbucket Data Center
type WebhookParser struct{}

// WebhookEvent implements ports.VCSWebhookEvent for Bitbucket
type WebhookEvent struct {
	payload *WebhookPayload
}

// NewWebhookParser creates a new WebhookParser
func NewWebhookParser() *WebhookParser {
	return &WebhookParser{}
}

// GetEventType returns the type of the webhook event
func (e *WebhookEvent) GetEventType() string {
	return e.payload.EventKey
}

// Parse parses the raw webhook payload and returns a parsed webhook event
func (p *WebhookParser) Parse(payload []byte) (ports.VCSWebhookEvent, error) {
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

	return &WebhookEvent{payload: &webhookPayload}, nil
}

// GetPullRequest extracts PR data from the webhook payload
func (e *WebhookEvent) GetPullRequest() (*models.PullRequest, error) {
	payload := e.payload

	// Determine repository slug
	repoId := payload.Repository.ID
	if repoId == 0 {
		repoId = payload.PullRequest.FromRef.Repository.ID
	}

	// Determine repository slug
	repoSlug := payload.Repository.Slug
	if repoSlug == "" {
		repoSlug = payload.PullRequest.FromRef.Repository.Slug
	}

	// Find HTTP clone URL
	cloneURL := e.findCloneURL()
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
	projectKey := e.findProjectKey()
	if projectKey == "" {
		return nil, errors.New(errors.ErrorCodeVCSInvalidPayload,
			"could not determine project key from payload",
		)
	}

	return models.NewPullRequest(
		payload.PullRequest.ID,
		repoId,
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

// GetComment returns comment information if this is a comment event
func (e *WebhookEvent) GetComment() *models.Comment {
	if e.payload.Comment == nil {
		return nil
	}

	return models.NewComment(
		e.payload.Comment.ID,
		e.payload.Comment.Text,
		e.payload.Comment.Author.User.Name,
		e.payload.Comment.Author.User.DisplayName,
	)
}

// validatePayload validates the webhook payload structure
func (p *WebhookParser) validatePayload(payload *WebhookPayload) error {
	if payload.PullRequest.ID == 0 {
		return errors.New(errors.ErrorCodeValidationFailed,
			"pullRequest.id is required",
		)
	}

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

	return nil
}

// GetCloneURL finds and return the HTTP clone URL from the payload
func (e *WebhookEvent) findCloneURL() string {
	payload := e.payload

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

	// Try toRef.repository as last resort
	if len(payload.PullRequest.ToRef.Repository.Links.Clone) > 0 {
		for _, link := range payload.PullRequest.ToRef.Repository.Links.Clone {
			if link.Name == "http" {
				return link.Href
			}
		}
	}

	return ""
}

// findProjectKey extracts the project key from the payload
func (e *WebhookEvent) findProjectKey() string {
	payload := e.payload

	// Try root repository first
	if payload.Repository.Project != nil && payload.Repository.Project.Key != "" {
		return payload.Repository.Project.Key
	}

	// Try fromRef.repository
	if payload.PullRequest.FromRef.Repository.Project != nil && payload.PullRequest.FromRef.Repository.Project.Key != "" {
		return payload.PullRequest.FromRef.Repository.Project.Key
	}

	// Try toRef.repository as last resort
	if payload.PullRequest.ToRef.Repository.Project != nil && payload.PullRequest.ToRef.Repository.Project.Key != "" {
		return payload.PullRequest.ToRef.Repository.Project.Key
	}

	return ""
}

// WebhookPayload represents the Bitbucket Data Center webhook payload
type WebhookPayload struct {
	EventKey    string      `json:"eventKey"`
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
	ID      int       `json:"id"`
	Slug    string    `json:"slug"`
	Name    string    `json:"name"`
	Project *Project  `json:"project"`
	Links   RepoLinks `json:"links"`
}

type Project struct {
	ID   int    `json:"id"`
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
