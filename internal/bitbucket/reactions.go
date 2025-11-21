package bitbucket

import (
	"fmt"
	"strings"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/logger"
	"github.com/asmild/bitbucket-pr-reviewer-bot/pkg/models"
)

// Emoji mapping from common names to Bitbucket reaction names
var emojiMap = map[string]string{
	"eyes":          "eyes",
	"thinking_face": "thinking_face",
	"+1":            "thumbsup",
	"x":             "x",
	"-1":            "thumbsdown",
	"tada":          "tada",
}

// AddCommentReaction adds an emoji reaction to a PR comment
func (c *Client) AddCommentReaction(projectKey, repoSlug string, prID, commentID int, emoji string) error {
	// Get Bitbucket reaction name
	reactionName := emojiMap[emoji]
	if reactionName == "" {
		reactionName = emoji
	}

	logger.Debugf("Adding reaction '%s' to comment %d", emoji, commentID)

	// Bitbucket Data Center comment-likes plugin API
	url := fmt.Sprintf("%s/rest/comment-likes/latest/projects/%s/repos/%s/pull-requests/%d/comments/%d/reactions/%s",
		c.baseURL, projectKey, repoSlug, prID, commentID, reactionName)

	_, err := c.put(url, nil)
	if err != nil {
		return err
	}

	logger.Debugf("Successfully added reaction '%s' to comment %d", emoji, commentID)
	return nil
}

// RemoveCommentReaction removes an emoji reaction from a PR comment
func (c *Client) RemoveCommentReaction(projectKey, repoSlug string, prID, commentID int, emoji string) error {
	// Get Bitbucket reaction name
	reactionName := emojiMap[emoji]
	if reactionName == "" {
		reactionName = emoji
	}

	logger.Debugf("Removing reaction '%s' from comment %d", emoji, commentID)

	// Bitbucket Data Center comment-likes plugin API
	url := fmt.Sprintf("%s/rest/comment-likes/latest/projects/%s/repos/%s/pull-requests/%d/comments/%d/reactions/%s",
		c.baseURL, projectKey, repoSlug, prID, commentID, reactionName)

	// Delete will return error for non-2xx status, but 404 is acceptable for deletes
	err := c.delete(url)
	if err != nil && !strings.Contains(err.Error(), "404") {
		return err
	}

	if err != nil {
		logger.Debugf("Reaction '%s' not found on comment %d (already removed or never added)", emoji, commentID)
	} else {
		logger.Debugf("Successfully removed reaction '%s' from comment %d", emoji, commentID)
	}

	return nil
}

// ReplaceReaction removes the previous emoji and adds a new one
func (c *Client) ReplaceReaction(prData *models.PRData, newEmoji string) {
	if prData.CommentID == 0 {
		return
	}

	c.emojiTracker.mu.Lock()
	previousEmoji, exists := c.emojiTracker.current[prData.CommentID]
	c.emojiTracker.current[prData.CommentID] = newEmoji
	c.emojiTracker.mu.Unlock()

	// Remove previous emoji if it exists
	if exists && previousEmoji != "" {
		if err := c.RemoveCommentReaction(prData.ProjectKey, prData.Repository, prData.PRID, prData.CommentID, previousEmoji); err != nil {
			logger.Warnf("Failed to remove previous '%s' reaction: %v", previousEmoji, err)
		}
	}

	// Add new emoji
	if err := c.AddCommentReaction(prData.ProjectKey, prData.Repository, prData.PRID, prData.CommentID, newEmoji); err != nil {
		logger.Warnf("Failed to add '%s' reaction: %v", newEmoji, err)
	}
}
