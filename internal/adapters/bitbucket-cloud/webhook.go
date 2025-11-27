package bitbucketcloud

import (
	"encoding/json"
	"fmt"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/models"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/ports"
)

// WebhookParser implements ports.VCSWebhookParser for Bitbucket Cloud
type WebhookParser struct{}

// NewWebhookParser creates a new Bitbucket Cloud webhook parser
func NewWebhookParser() *WebhookParser {
	return &WebhookParser{}
}

// Parse parses a Bitbucket Cloud webhook payload
func (p *WebhookParser) Parse(payload []byte) (ports.VCSWebhookEvent, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse webhook payload: %w", err)
	}

	// TODO: Implement actual Bitbucket Cloud webhook parsing
	// Cloud uses different structure:
	// - Event types: "pullrequest:created", "pullrequest:updated", etc.
	// - Field names: "pullrequest" (lowercase), "source", "destination"
	// - Organization: "workspace" instead of "project"

	return nil, fmt.Errorf("bitbucket cloud webhook parser: not implemented")
}

// WebhookEvent represents a parsed Bitbucket Cloud webhook event
type WebhookEvent struct {
	raw map[string]interface{}
}

// GetEventType returns the event type
func (e *WebhookEvent) GetEventType() string {
	// TODO: Extract event type from Cloud webhook
	// Cloud format: "pullrequest:created", "pullrequest:updated", etc.
	return ""
}

// GetPullRequest extracts pull request information
func (e *WebhookEvent) GetPullRequest() (*models.PullRequest, error) {
	// TODO: Parse Cloud PR structure
	// Different field names and nesting compared to Data Center
	return nil, fmt.Errorf("bitbucket cloud: GetPullRequest not implemented")
}

// GetComment returns comment information if available
func (e *WebhookEvent) GetComment() *models.Comment {
	// TODO: Parse Cloud comment structure
	return nil
}
