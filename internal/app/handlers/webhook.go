package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/config"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/errors"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/models"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/ports"
)

// WebhookHandler handles webhook requests
type WebhookHandler struct {
	vcsClient     ports.VCSClient
	webhookParser ports.VCSWebhookParser
	queue         ports.ReviewQueue
	rateLimiter   ports.RateLimiter
	metrics       ports.MetricsCollector
	logger        ports.Logger
	config        WebhookConfig
}

// TestPayload represents a test webhook payload
type TestPayload struct {
	Test bool `json:"test"`
}

// WebhookConfig holds webhook handler configuration
type WebhookConfig struct {
	WebhookSecret      string
	AllowedProjectKeys []string
	TriggeringEvents   []config.TriggeringEvent
	BitbucketUsername  string
}

// Event type constants
const (
	EventTypePROpened     = "pr:opened"
	EventTypeCommentAdded = "pr:comment:added"
)

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(
	vcsClient ports.VCSClient,
	webhookParser ports.VCSWebhookParser,
	queue ports.ReviewQueue,
	rateLimiter ports.RateLimiter,
	metrics ports.MetricsCollector,
	logger ports.Logger,
	config WebhookConfig,
) *WebhookHandler {
	return &WebhookHandler{
		vcsClient:     vcsClient,
		webhookParser: webhookParser,
		queue:         queue,
		rateLimiter:   rateLimiter,
		metrics:       metrics,
		logger:        logger,
		config:        config,
	}
}

// HandleWebhook handles incoming webhook requests
func (h *WebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests
	if r.Method != http.MethodPost {
		h.logger.Warn("Invalid webhook method",
			"method", r.Method,
		)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Apply rate limiting if configured
	if h.rateLimiter != nil && !h.rateLimiter.Allow() {
		h.logger.Warn("Rate limit exceeded for webhook")
		h.metrics.IncrementWebhookReceived("rate_limited")
		http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
		return
	}

	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("Failed to read webhook body", "error", err)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Check for test payload
	if h.isTestPayload(body) {
		h.logger.Debug("Received test webhook payload")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Test webhook received"))
		return
	}

	// Validate signature if secret is configured
	if h.config.WebhookSecret != "" {
		signature := r.Header.Get("X-Hub-Signature")
		if !h.vcsClient.ValidateWebhookSignature(body, signature, h.config.WebhookSecret) {
			h.logger.Warn("Invalid webhook signature")
			http.Error(w, "Invalid signature", http.StatusUnauthorized)
			return
		}
	}

	// Parse webhook payload
	webhookEvent, err := h.webhookParser.Parse(body)
	if err != nil {
		h.logger.Error("Failed to parse webhook payload", "error", err)
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	// Get event type
	eventType := webhookEvent.GetEventType()
	if eventType == "" {
		h.logger.Error("Event type is missing from payload")
		http.Error(w, "Invalid payload: missing eventKey", http.StatusBadRequest)
		return
	}

	// Check if we should process this event type (early return)
	if !h.shouldProcessEvent(eventType) {
		h.logger.Debug("Ignoring event type",
			"event_type", eventType,
		)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Event type not configured for processing"))
		return
	}

	// Extract pull request
	pr, err := webhookEvent.GetPullRequest()
	if err != nil {
		h.logger.Error("Failed to extract pull request", "error", err)
		http.Error(w, "Failed to extract pull request", http.StatusBadRequest)
		return
	}

	// For comment events, handle trigger logic
	if eventType == EventTypeCommentAdded {
		comment := webhookEvent.GetComment()
		if comment == nil {
			h.logger.Error("Comment event without comment data")
			http.Error(w, "Invalid comment event", http.StatusBadRequest)
			return
		}

		// Validate comment trigger
		if !h.shouldProcessComment(comment) {
			h.logger.Debug("Comment event ignored", "reason", "bot comment or invalid trigger")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Comment ignored"))
			return
		}

		// Set comment ID for manual trigger
		pr = pr.WithCommentID(comment.ID)
	}

	// Increment webhook metric only for valid, processed events
	h.metrics.IncrementWebhookReceived(eventType)

	// Log webhook details for debugging
	h.logger.Debug("Webhook received",
		"event_type", eventType,
		"pr_id", pr.ID,
		"pr_title", pr.Title,
		"project_key", pr.ProjectKey,
		"repo_slug", pr.RepositorySlug,
		"manual_trigger", pr.IsManualTrigger(),
	)

	// Validate project
	if !h.isProjectAllowed(pr.ProjectKey) {
		h.logger.Warn("Webhook from unauthorized project",
			"project", pr.ProjectKey,
		)
		http.Error(w, "Project not authorized", http.StatusForbidden)
		return
	}

	// Enqueue the PR for review
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	_, err = h.queue.Enqueue(ctx, pr)
	if err != nil {
		if err == errors.ErrQueueFull {
			h.logger.Warn("Queue is full, rejecting webhook")
			http.Error(w, "Queue is full, please try again later", http.StatusServiceUnavailable)
			return
		}

		h.logger.Error("Failed to enqueue PR", "error", err)
		http.Error(w, "Failed to enqueue review", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte("Webhook received and queued for processing"))
}

// shouldProcessEvent checks if the event type should be processed
func (h *WebhookHandler) shouldProcessEvent(eventType string) bool {
	// Map Bitbucket event format ("pr:opened") to config format ("pr_opened")
	var configEventType string
	switch eventType {
	case EventTypePROpened:
		configEventType = config.EventTypePROpened
	case EventTypeCommentAdded:
		configEventType = config.EventTypeCommentAdded
	default:
		return false
	}

	// Check if this event type is in the configured triggering events
	for _, event := range h.config.TriggeringEvents {
		if event.GetType() == configEventType {
			return true
		}
	}

	return false
}

// shouldProcessComment checks if a comment should trigger a review
func (h *WebhookHandler) shouldProcessComment(comment *models.Comment) bool {
	// Get the trigger keyword from configured comment_added event
	var triggerKeyword string
	for _, event := range h.config.TriggeringEvents {
		if event.GetType() == config.EventTypeCommentAdded {
			if commentEvent, ok := event.(*config.CommentAddedEvent); ok {
				triggerKeyword = commentEvent.Keyword
			}
			break
		}
	}

	// Debug log the comment details before validation
	h.logger.Debug("Processing comment",
		"comment_id", comment.ID,
		"author_name", comment.AuthorName,
		"author_display_name", comment.AuthorDisplayName,
		"text_preview", truncateString(comment.Text, 100),
		"bot_username", h.config.BitbucketUsername,
		"trigger_keyword", triggerKeyword,
	)

	// Check 1: Ignore comments from the bot itself to prevent infinite loops
	if comment.AuthorName == h.config.BitbucketUsername || comment.AuthorDisplayName == h.config.BitbucketUsername {
		h.logger.Debug("Ignoring comment from bot itself",
			"author_name", comment.AuthorName,
			"author_display_name", comment.AuthorDisplayName,
		)
		return false
	}

	// Check 2: Ignore comments that look like bot-generated reviews
	// Bot reviews always contain "Reviewed by" pattern
	commentTextLower := strings.ToLower(comment.Text)
	if strings.Contains(commentTextLower, "reviewed by") &&
		strings.Contains(commentTextLower, "in ") &&
		strings.Contains(commentTextLower, "s*") {
		h.logger.Debug("Ignoring bot-generated review comment")
		return false
	}

	// Check 3: Verify comment mentions the bot (e.g., @bot-username)
	botMention := "@" + h.config.BitbucketUsername
	hasMention := strings.Contains(comment.Text, botMention)
	if !hasMention {
		h.logger.Debug("Comment does not mention bot",
			"bot_mention", botMention,
		)
		return false
	}

	// Check 4: Verify comment contains trigger keyword
	hasTrigger := strings.Contains(comment.Text, triggerKeyword)
	if !hasTrigger {
		h.logger.Debug("Comment does not contain trigger keyword",
			"trigger_keyword", triggerKeyword,
		)
	}

	return hasTrigger
}

// truncateString truncates a string to a maximum length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// isProjectAllowed checks if a project is in the allowed list
func (h *WebhookHandler) isProjectAllowed(projectKey string) bool {
	// If no project keys are configured, allow all
	if len(h.config.AllowedProjectKeys) == 0 {
		return true
	}

	for _, allowed := range h.config.AllowedProjectKeys {
		if projectKey == allowed {
			return true
		}
	}

	return false
}

// isTestPayload checks if the payload is a test payload {"test": true}
func (h *WebhookHandler) isTestPayload(body []byte) bool {
	// Simple check for test payload
	if err := json.Unmarshal(body, &TestPayload{}); err == nil {
		var testPayload TestPayload
		if err := json.Unmarshal(body, &testPayload); err == nil {
			return testPayload.Test
		}
	}
	return false
}
