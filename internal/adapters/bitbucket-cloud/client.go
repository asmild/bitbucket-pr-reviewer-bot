package bitbucketcloud

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/errors"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/ports"
)

// Client implements ports.VCSClient for Bitbucket Cloud
type Client struct {
	baseURL    string
	username   string
	token      string
	httpClient *http.Client
	logger     ports.Logger
}

// Config holds Bitbucket Cloud client configuration
type Config struct {
	Username string
	Token    string
	Timeout  time.Duration
}

// NewClient creates a new Bitbucket Cloud client
func NewClient(cfg Config, logger ports.Logger) *Client {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	return &Client{
		baseURL:  "https://api.bitbucket.org",
		username: cfg.Username,
		token:    cfg.Token,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		logger: logger,
	}
}

// PostComment posts a comment on a pull request. If parentCommentID > 0, posts as a reply to that comment.
func (c *Client) PostComment(ctx context.Context, projectKey, repoSlug string, prID int, comment string, parentCommentID int) error {
	return fmt.Errorf("bitbucket cloud adapter: PostComment not implemented")
}

// AddCommentReaction adds an emoji reaction to a comment
// Note: Bitbucket Cloud 2.0 API does not support comment reactions
func (c *Client) AddCommentReaction(ctx context.Context, projectKey, repoSlug string, prID, commentID int, emoji string) error {
	c.logger.Warn("Bitbucket Cloud does not support comment reactions - feature unavailable")
	return nil // Don't error out, just skip the feature
}

// RemoveCommentReaction removes an emoji reaction from a comment
// Note: Bitbucket Cloud 2.0 API does not support comment reactions
func (c *Client) RemoveCommentReaction(ctx context.Context, projectKey, repoSlug string, prID, commentID int, emoji string) error {
	c.logger.Warn("Bitbucket Cloud does not support comment reactions - feature unavailable")
	return nil // Don't error out, just skip the feature
}

// ValidateWebhookSignature validates the webhook signature
// Note: Bitbucket Cloud uses UUID-based verification, not HMAC
func (c *Client) ValidateWebhookSignature(payload []byte, signature, secret string) bool {
	// TODO: Implement UUID-based verification for Bitbucket Cloud
	c.logger.Warn("Bitbucket Cloud webhook signature validation not implemented")
	return true // Temporarily allow all webhooks
}

// SetBaseURL sets the base URL for the VCS API
// Note: Bitbucket Cloud always uses https://api.bitbucket.org
func (c *Client) SetBaseURL(baseURL string) {
	c.logger.Warn("SetBaseURL called on Bitbucket Cloud client - Cloud uses fixed API URL")
	// Ignore - Cloud has fixed base URL
}

// TestConnection tests if the API is accessible with current credentials
func (c *Client) TestConnection(ctx context.Context) error {
	// Try to list workspaces with pagelen=1 (minimal data transfer)
	url := fmt.Sprintf("%s/2.0/workspaces?pagelen=1", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set authentication - Cloud uses Basic Auth with username and App Password
	req.SetBasicAuth(c.username, c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to Bitbucket Cloud API: %w", err)
	}
	defer resp.Body.Close()

	// Read response body for error details
	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusUnauthorized {
		return errors.ErrUnauthorized
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}
