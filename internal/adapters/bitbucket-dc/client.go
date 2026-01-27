package bitbucket

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/errors"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/ports"
)

// Client implements ports.VCSClient for Bitbucket Data Center
type Client struct {
	baseURL      string
	username     string
	token        string
	httpClient   *http.Client
	logger       ports.Logger
	emojiTracker *emojiTracker
}

// Config holds Bitbucket client configuration
type Config struct {
	BaseURL  string
	Username string
	Token    string
	Timeout  time.Duration
}

// NewClient creates a new Bitbucket client
func NewClient(cfg Config, logger ports.Logger) *Client {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	return &Client{
		baseURL:  cfg.BaseURL,
		username: cfg.Username,
		token:    cfg.Token,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		logger: logger,
		emojiTracker: &emojiTracker{
			current: make(map[int]string),
		},
	}
}

// commentResponse represents the response from posting a comment
type commentResponse struct {
	ID int `json:"id"`
}

// PostComment posts a comment on a pull request. If parentCommentID > 0, posts as a reply to that comment.
// This is a convenience wrapper around PostCommentWithID that discards the returned comment ID.
func (c *Client) PostComment(ctx context.Context, projectKey, repoSlug string, prID int, comment string, parentCommentID int) error {
	_, err := c.PostCommentWithID(ctx, projectKey, repoSlug, prID, comment, parentCommentID)
	return err
}

// PostCommentWithID posts a comment and returns the created comment ID.
func (c *Client) PostCommentWithID(ctx context.Context, projectKey, repoSlug string, prID int, comment string, parentCommentID int) (int, error) {
	url := fmt.Sprintf("%s/rest/api/1.0/projects/%s/repos/%s/pull-requests/%d/comments",
		c.baseURL, projectKey, repoSlug, prID)

	payload := map[string]interface{}{
		"text": comment,
	}

	// If parentCommentID is provided, add it to payload to create a reply
	if parentCommentID > 0 {
		payload["parent"] = map[string]interface{}{
			"id": parentCommentID,
		}
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return 0, errors.Wrap(errors.ErrorCodeVCSAPIError,
			"failed to marshal comment payload",
			err,
		)
	}

	if parentCommentID > 0 {
		c.logger.Debug("Posting reply to comment",
			"project", projectKey,
			"repo", repoSlug,
			"pr_id", prID,
			"parent_comment_id", parentCommentID,
		)
	} else {
		c.logger.Debug("Posting comment to PR",
			"project", projectKey,
			"repo", repoSlug,
			"pr_id", prID,
		)
	}

	respBody, err := c.post(ctx, url, payloadBytes)
	if err != nil {
		domainErr := errors.Wrap(errors.ErrorCodeVCSAPIError,
			"failed to post comment",
			err,
		).WithMetadata("project", projectKey).
			WithMetadata("repo", repoSlug).
			WithMetadata("pr_id", prID)

		if parentCommentID > 0 {
			domainErr.WithMetadata("parent_comment_id", parentCommentID)
		}

		return 0, domainErr
	}

	// Parse response to get comment ID
	var resp commentResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		c.logger.Warn("Failed to parse comment response, comment was posted but ID unknown",
			"error", err,
		)
		return 0, nil
	}

	if parentCommentID > 0 {
		c.logger.Debug("Successfully posted reply to comment",
			"project", projectKey,
			"pr_id", prID,
			"parent_comment_id", parentCommentID,
			"comment_id", resp.ID,
		)
	} else {
		c.logger.Debug("Successfully posted comment to PR",
			"project", projectKey,
			"pr_id", prID,
			"comment_id", resp.ID,
		)
	}

	return resp.ID, nil
}

// UpdateComment updates an existing comment on a pull request.
func (c *Client) UpdateComment(ctx context.Context, projectKey, repoSlug string, prID, commentID, version int, text string) error {
	url := fmt.Sprintf("%s/rest/api/1.0/projects/%s/repos/%s/pull-requests/%d/comments/%d",
		c.baseURL, projectKey, repoSlug, prID, commentID)

	payload := map[string]interface{}{
		"text":    text,
		"version": version,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return errors.Wrap(errors.ErrorCodeVCSAPIError,
			"failed to marshal comment update payload",
			err,
		)
	}

	c.logger.Debug("Updating comment",
		"project", projectKey,
		"repo", repoSlug,
		"pr_id", prID,
		"comment_id", commentID,
	)

	_, err = c.put(ctx, url, payloadBytes)
	if err != nil {
		return errors.Wrap(errors.ErrorCodeVCSAPIError,
			"failed to update comment",
			err,
		).WithMetadata("project", projectKey).
			WithMetadata("repo", repoSlug).
			WithMetadata("pr_id", prID).
			WithMetadata("comment_id", commentID)
	}

	c.logger.Debug("Successfully updated comment",
		"project", projectKey,
		"pr_id", prID,
		"comment_id", commentID,
	)

	return nil
}

// DeleteComment deletes a comment from a pull request.
func (c *Client) DeleteComment(ctx context.Context, projectKey, repoSlug string, prID, commentID, version int) error {
	url := fmt.Sprintf("%s/rest/api/1.0/projects/%s/repos/%s/pull-requests/%d/comments/%d?version=%d",
		c.baseURL, projectKey, repoSlug, prID, commentID, version)

	c.logger.Debug("Deleting comment",
		"project", projectKey,
		"repo", repoSlug,
		"pr_id", prID,
		"comment_id", commentID,
	)

	err := c.delete(ctx, url)
	if err != nil {
		return errors.Wrap(errors.ErrorCodeVCSAPIError,
			"failed to delete comment",
			err,
		).WithMetadata("project", projectKey).
			WithMetadata("repo", repoSlug).
			WithMetadata("pr_id", prID).
			WithMetadata("comment_id", commentID)
	}

	c.logger.Debug("Successfully deleted comment",
		"project", projectKey,
		"pr_id", prID,
		"comment_id", commentID,
	)

	return nil
}

// AddCommentReaction adds an emoji reaction to a comment
func (c *Client) AddCommentReaction(ctx context.Context, projectKey, repoSlug string, prID, commentID int, emoji string) error {
	// Get Bitbucket reaction name
	reactionName := emojiToBitbucket(emoji)

	c.logger.Debug("Adding reaction to comment",
		"emoji", emoji,
		"comment_id", commentID,
	)

	// Bitbucket Data Center comment-likes plugin API
	url := fmt.Sprintf("%s/rest/comment-likes/latest/projects/%s/repos/%s/pull-requests/%d/comments/%d/reactions/%s",
		c.baseURL, projectKey, repoSlug, prID, commentID, reactionName)

	_, err := c.put(ctx, url, nil)
	if err != nil {
		return errors.Wrap(errors.ErrorCodeVCSAPIError,
			"failed to add reaction",
			err,
		).WithMetadata("emoji", emoji).
			WithMetadata("comment_id", commentID)
	}

	// Track the emoji
	c.emojiTracker.set(commentID, emoji)

	c.logger.Debug("Successfully added reaction",
		"emoji", emoji,
		"comment_id", commentID,
	)

	return nil
}

// RemoveCommentReaction removes an emoji reaction from a comment
func (c *Client) RemoveCommentReaction(ctx context.Context, projectKey, repoSlug string, prID, commentID int, emoji string) error {
	// Get Bitbucket reaction name
	reactionName := emojiToBitbucket(emoji)

	c.logger.Debug("Removing reaction from comment",
		"emoji", emoji,
		"comment_id", commentID,
	)

	// Bitbucket Data Center comment-likes plugin API
	url := fmt.Sprintf("%s/rest/comment-likes/latest/projects/%s/repos/%s/pull-requests/%d/comments/%d/reactions/%s",
		c.baseURL, projectKey, repoSlug, prID, commentID, reactionName)

	err := c.delete(ctx, url)
	// 404 is acceptable for deletes (reaction already removed or never added)
	if err != nil && !strings.Contains(err.Error(), "404") {
		return errors.Wrap(errors.ErrorCodeVCSAPIError,
			"failed to remove reaction",
			err,
		).WithMetadata("emoji", emoji).
			WithMetadata("comment_id", commentID)
	}

	// Clear from tracker
	c.emojiTracker.remove(commentID)

	if err != nil {
		c.logger.Debug("Reaction not found (already removed or never added)",
			"emoji", emoji,
			"comment_id", commentID,
		)
	} else {
		c.logger.Debug("Successfully removed reaction",
			"emoji", emoji,
			"comment_id", commentID,
		)
	}

	return nil
}

// ValidateWebhookSignature validates the webhook signature
func (c *Client) ValidateWebhookSignature(payload []byte, signature, secret string) bool {
	// Implementation for Bitbucket webhook signature validation
	// For now, we'll implement a basic HMAC validation
	// Note: Bitbucket Data Center uses X-Hub-Signature header with HMAC-SHA256
	return validateHMACSignature(payload, signature, secret)
}

// SetBaseURL sets the base URL for the VCS API
func (c *Client) SetBaseURL(baseURL string) {
	c.baseURL = baseURL
}

// TestConnection tests if the API is accessible with current credentials
func (c *Client) TestConnection(ctx context.Context) error {
	if c.baseURL == "" {
		return errors.New(errors.ErrorCodeConfigMissing, "base URL is not configured")
	}

	// Try to list projects with limit=1 (minimal data transfer)
	url := fmt.Sprintf("%s/rest/api/latest/projects?limit=1", c.baseURL)

	_, err := c.get(ctx, url)
	if err != nil {
		// Error already wrapped with proper domain error codes from handleResponse
		return err
	}

	return nil
}

// HTTP request helpers
func (c *Client) doRequest(ctx context.Context, method, url string, body []byte) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewBuffer(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, errors.Wrap(errors.ErrorCodeNetworkFailure,
			"failed to create request",
			err,
		)
	}

	// Set authentication
	req.SetBasicAuth(c.username, c.token)

	// Set standard headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(errors.ErrorCodeNetworkFailure,
			"failed to send request",
			err,
		)
	}

	return resp, nil
}

func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	resp, err := c.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return c.handleResponse(resp)
}

func (c *Client) post(ctx context.Context, url string, body []byte) ([]byte, error) {
	resp, err := c.doRequest(ctx, "POST", url, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return c.handleResponse(resp)
}

func (c *Client) put(ctx context.Context, url string, body []byte) ([]byte, error) {
	resp, err := c.doRequest(ctx, "PUT", url, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return c.handleResponse(resp)
}

func (c *Client) delete(ctx context.Context, url string) error {
	resp, err := c.doRequest(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// For DELETE, we don't care about response body
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func (c *Client) handleResponse(resp *http.Response) ([]byte, error) {
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(errors.ErrorCodeNetworkFailure,
			"failed to read response body",
			err,
		)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, errors.ErrUnauthorized
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, errors.ErrNotFound
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, errors.New(errors.ErrorCodeRateLimitExceeded, "rate limit exceeded")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, errors.New(errors.ErrorCodeVCSAPIError,
			fmt.Sprintf("API request failed with status %d: %s", resp.StatusCode, string(bodyBytes)),
		)
	}

	return bodyBytes, nil
}

// emojiTracker manages emoji reactions per comment
type emojiTracker struct {
	current map[int]string
	mu      sync.Mutex
}

func (et *emojiTracker) set(commentID int, emoji string) {
	et.mu.Lock()
	defer et.mu.Unlock()
	et.current[commentID] = emoji
}

func (et *emojiTracker) get(commentID int) (string, bool) {
	et.mu.Lock()
	defer et.mu.Unlock()
	emoji, ok := et.current[commentID]
	return emoji, ok
}

func (et *emojiTracker) remove(commentID int) {
	et.mu.Lock()
	defer et.mu.Unlock()
	delete(et.current, commentID)
}

// emojiToBitbucket converts common emoji names to Bitbucket reaction names
func emojiToBitbucket(emoji string) string {
	emojiMap := map[string]string{
		"PROCESSING":       "eyes",
		"X":                "x",
		"WHITE_CHECK_MARK": "white_check_mark",
	}

	if mapped, ok := emojiMap[emoji]; ok {
		return mapped
	}
	return strings.ToLower(emoji)
}
