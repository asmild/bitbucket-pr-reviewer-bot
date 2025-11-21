package bitbucket

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/config"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/logger"
	"github.com/asmild/bitbucket-pr-reviewer-bot/pkg/models"
)

// Event type constants
const (
	EventTypePROpened     = "pr:opened"
	EventTypeCommentAdded = "pr:comment:added"
)

var branchNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_./-]+$`)

// ValidateSignature validates the webhook signature
func ValidateSignature(secret string, payload []byte, signature string) error {
	if secret == "" {
		logger.Warn("Webhook secret not configured, skipping signature validation")
		return nil
	}

	if signature == "" {
		return fmt.Errorf("missing webhook signature")
	}

	// Compute expected signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expectedMAC := mac.Sum(nil)
	expectedSignature := "sha256=" + hex.EncodeToString(expectedMAC)

	// Compare using constant-time comparison
	if subtle.ConstantTimeCompare([]byte(signature), []byte(expectedSignature)) != 1 {
		return fmt.Errorf("invalid webhook signature")
	}

	return nil
}

// ValidateCommentTrigger checks if the comment contains the trigger keyword and mentions the configured user
func ValidateCommentTrigger(cfg *config.Config, payload *WebhookPayload) error {
	if payload.Comment == nil {
		return fmt.Errorf("comment is missing from payload")
	}

	commentText := strings.TrimSpace(payload.Comment.Text)
	triggerKeyword := cfg.Bitbucket.TriggerKeyword
	userMention := "@" + cfg.Bitbucket.User

	// Check for user mention
	if !strings.Contains(commentText, userMention) {
		return fmt.Errorf("comment does not mention user '%s'", userMention)
	}

	// Check for trigger keyword
	if !strings.Contains(commentText, triggerKeyword) {
		return fmt.Errorf("comment does not contain trigger keyword '%s'", triggerKeyword)
	}

	return nil
}

// ValidateProject validates that the webhook is from an allowed project
func ValidateProject(cfg *config.Config, payload *WebhookPayload) error {
	// If no project keys are configured, accept all projects
	if len(cfg.Bitbucket.AllowedProjectKeys) == 0 {
		return nil
	}

	if payload.Repository.Project == nil {
		return fmt.Errorf("could not determine project from payload")
	}

	projectKey := payload.Repository.Project.Key

	// Check if project key is in the allowed list
	for _, allowedKey := range cfg.Bitbucket.AllowedProjectKeys {
		if projectKey == allowedKey {
			return nil
		}
	}

	return fmt.Errorf("webhooks only accepted from projects: %v, got: %s", cfg.Bitbucket.AllowedProjectKeys, projectKey)
}

// ParsePayload parses and validates the webhook payload
func ParsePayload(body io.Reader) (*WebhookPayload, []byte, error) {
	payloadBytes, err := io.ReadAll(body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read request body: %w", err)
	}

	var payload WebhookPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, payloadBytes, fmt.Errorf("invalid JSON payload: %w", err)
	}

	if err := validatePayloadStructure(&payload); err != nil {
		return nil, payloadBytes, err
	}

	return &payload, payloadBytes, nil
}

// validatePayloadStructure validates the payload structure
func validatePayloadStructure(payload *WebhookPayload) error {
	if payload.PullRequest.Title == "" {
		return fmt.Errorf("pullRequest.title is required")
	}

	if payload.PullRequest.Author.User.DisplayName == "" {
		return fmt.Errorf("pullRequest.author.user.displayName is required")
	}

	if payload.PullRequest.FromRef.DisplayID == "" {
		return fmt.Errorf("pullRequest.fromRef.displayId is required")
	}

	if payload.PullRequest.ToRef.DisplayID == "" {
		return fmt.Errorf("pullRequest.toRef.displayId is required")
	}

	// Repository slug can be at root level or in fromRef for different event types
	if payload.Repository.Slug == "" && payload.PullRequest.FromRef.Repository.Slug == "" {
		return fmt.Errorf("repository.slug is required")
	}

	// Validate branch names
	if !branchNameRegex.MatchString(payload.PullRequest.FromRef.DisplayID) {
		return fmt.Errorf("invalid source branch name")
	}

	if !branchNameRegex.MatchString(payload.PullRequest.ToRef.DisplayID) {
		return fmt.Errorf("invalid destination branch name")
	}

	return nil
}

// ExtractPRData extracts PR data from the webhook payload
func ExtractPRData(payload *WebhookPayload) (*models.PRData, error) {
	// Determine repository slug - can be at root or in fromRef
	repoSlug := payload.Repository.Slug
	if repoSlug == "" {
		repoSlug = payload.PullRequest.FromRef.Repository.Slug
	}

	// Find HTTP clone URL - try root repository first, then fromRef
	cloneURL := ""
	if len(payload.Repository.Links.Clone) > 0 {
		for _, link := range payload.Repository.Links.Clone {
			if link.Name == "http" {
				cloneURL = link.Href
				break
			}
		}
	}

	// If not found at root, try fromRef.repository
	if cloneURL == "" && len(payload.PullRequest.FromRef.Repository.Links.Clone) > 0 {
		for _, link := range payload.PullRequest.FromRef.Repository.Links.Clone {
			if link.Name == "http" {
				cloneURL = link.Href
				break
			}
		}
	}

	if cloneURL == "" {
		return nil, fmt.Errorf("no HTTP clone URL found")
	}

	// Get PR URL from self links
	prURL := ""
	if len(payload.PullRequest.Links.Self) > 0 {
		prURL = payload.PullRequest.Links.Self[0].Href
	}

	// Get comment ID if present
	commentID := 0
	if payload.Comment != nil {
		commentID = payload.Comment.ID
	}

	// Extract project key from repository
	projectKey := ""
	if payload.Repository.Project != nil {
		projectKey = payload.Repository.Project.Key
	} else if payload.PullRequest.FromRef.Repository.Project != nil {
		projectKey = payload.PullRequest.FromRef.Repository.Project.Key
	}

	return &models.PRData{
		Title:             payload.PullRequest.Title,
		Description:       payload.PullRequest.Description,
		Author:            payload.PullRequest.Author.User.DisplayName,
		SourceBranch:      payload.PullRequest.FromRef.DisplayID,
		DestinationBranch: payload.PullRequest.ToRef.DisplayID,
		PRUrl:             prURL,
		Repository:        repoSlug,
		RepoCloneURL:      cloneURL,
		PRID:              payload.PullRequest.ID,
		CommentID:         commentID,
		ProjectKey:        projectKey,
	}, nil
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

// ExtractProjectKey extracts project key from repository URL or clone URL
func ExtractProjectKey(cloneURL string) string {
	// Example: https://bitbucket.example.com/scm/proj/repo.git -> PROJ
	parts := strings.Split(cloneURL, "/scm/")
	if len(parts) < 2 {
		return ""
	}

	pathParts := strings.Split(parts[1], "/")
	if len(pathParts) < 1 {
		return ""
	}

	return strings.ToUpper(pathParts[0])
}
