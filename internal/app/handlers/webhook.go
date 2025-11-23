package handlers

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/adapters/bitbucket"
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

// WebhookConfig holds webhook handler configuration
type WebhookConfig struct {
	WebhookSecret      string
	AllowedProjectKeys []string
	TriggerKeyword     string
	EventType          string // "pr_opened" or "comment_added"
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

	// Validate signature if secret is configured
	if h.config.WebhookSecret != "" {
		signature := r.Header.Get("X-Hub-Signature")
		if !h.vcsClient.ValidateWebhookSignature(body, signature, h.config.WebhookSecret) {
			h.logger.Warn("Invalid webhook signature")
			http.Error(w, "Invalid signature", http.StatusUnauthorized)
			return
		}
	}

	// Parse event type
	eventType, err := h.webhookParser.GetEventType(body)
	if err != nil {
		h.logger.Error("Failed to parse event type", "error", err)
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	h.metrics.IncrementWebhookReceived(eventType)

	// Check if we should process this event type
	if !h.shouldProcessEvent(eventType) {
		h.logger.Debug("Ignoring event type",
			"event_type", eventType,
			"configured_type", h.config.EventType,
		)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Event type not configured for processing"))
		return
	}

	// Parse webhook based on event type
	pr, err := h.parseWebhook(eventType, body)
	if err != nil {
		h.logger.Error("Failed to parse webhook", "error", err)
		http.Error(w, "Failed to parse webhook", http.StatusBadRequest)
		return
	}

	// Validate project
	if !h.isProjectAllowed(pr.ProjectKey) {
		h.logger.Warn("Webhook from unauthorized project",
			"project", pr.ProjectKey,
		)
		http.Error(w, "Project not authorized", http.StatusForbidden)
		return
	}

	// For comment events, validate trigger
	if eventType == EventTypeCommentAdded && !pr.IsManualTrigger() {
		h.logger.Debug("Comment event without manual trigger, ignoring")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Comment does not contain trigger"))
		return
	}

	// Extract base URL from clone URL and set it on the VCS client
	baseURL := bitbucket.ExtractBaseURL(pr.CloneURL)
	if baseURL != "" {
		h.vcsClient.SetBaseURL(baseURL)
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
	// If no specific event type configured, accept all
	if h.config.EventType == "" {
		return true
	}

	// Map config format ("pr_opened") to Bitbucket format ("pr:opened")
	switch h.config.EventType {
	case config.EventTypePROpened:
		return eventType == EventTypePROpened
	case config.EventTypeCommentAdded:
		return eventType == EventTypeCommentAdded
	default:
		return false
	}
}

// parseWebhook parses the webhook payload based on event type
func (h *WebhookHandler) parseWebhook(eventType string, body []byte) (*models.PullRequest, error) {
	switch eventType {
	case EventTypePROpened:
		return h.webhookParser.ParsePullRequestEvent(body)
	case EventTypeCommentAdded:
		pr, err := h.webhookParser.ParseCommentEvent(body)
		if err != nil {
			return nil, err
		}

		// For comment events, we need to validate the trigger
		// This is a simplified check - in reality, you'd parse the comment text
		// from the payload and check for the trigger keyword
		return pr, nil

	default:
		return nil, errors.New(errors.ErrorCodeVCSInvalidPayload,
			"unsupported event type: "+eventType,
		)
	}
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
