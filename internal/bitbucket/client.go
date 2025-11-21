package bitbucket

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/config"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/logger"
)

// NewClient creates a new Bitbucket API client
// Note: baseURL will be set from the webhook payload's clone URL when processing events
func NewClient(cfg *config.Config) *Client {
	return &Client{
		baseURL: "", // Will be set from webhook payload
		user:    cfg.Bitbucket.User,
		token:   cfg.Bitbucket.Token,
		client:  &http.Client{},
		emojiTracker: &EmojiTracker{
			current: make(map[int]string),
		},
	}
}

// SetBaseURL sets the Bitbucket server base URL (extracted from webhook clone URL)
func (c *Client) SetBaseURL(baseURL string) {
	c.baseURL = baseURL
}

// ReplyToComment replies to an existing comment with text (used as fallback for reactions)
func (c *Client) ReplyToComment(projectKey, repoSlug string, prID, commentID int, text string) error {
	url := fmt.Sprintf("%s/rest/api/1.0/projects/%s/repos/%s/pull-requests/%d/comments",
		c.baseURL, projectKey, repoSlug, prID)

	payload := map[string]interface{}{
		"text":   text,
		"parent": map[string]int{"id": commentID},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal reply payload: %w", err)
	}

	logger.Debugf("Replying to comment %d with: %s", commentID, text)

	_, err = c.post(url, payloadBytes)
	if err != nil {
		return err
	}

	logger.Debugf("Successfully replied to comment %d", commentID)
	return nil
}

// PostComment posts a comment to a PR
func (c *Client) PostComment(projectKey, repoSlug string, prID int, text string) error {
	url := fmt.Sprintf("%s/rest/api/1.0/projects/%s/repos/%s/pull-requests/%d/comments",
		c.baseURL, projectKey, repoSlug, prID)

	payload := map[string]interface{}{
		"text": text,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal comment payload: %w", err)
	}

	logger.Debugf("Posting comment to PR %d", prID)

	_, err = c.post(url, payloadBytes)
	if err != nil {
		return err
	}

	logger.Debugf("Successfully posted comment to PR %d", prID)
	return nil
}
