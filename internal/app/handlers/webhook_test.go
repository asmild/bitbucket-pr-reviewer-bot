package handlers

import (
	"testing"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/models"
)

func TestShouldProcessComment(t *testing.T) {
	botUsername := "pr-reviewer-bot"
	triggerKeyword := "/review"

	handler := &WebhookHandler{
		config: WebhookConfig{
			BitbucketUsername: botUsername,
			TriggerKeyword:    triggerKeyword,
		},
		logger: &mockLogger{},
	}

	t.Run("processes valid comment with bot mention and trigger", func(t *testing.T) {
		comment := models.NewComment(
			123,
			"@pr-reviewer-bot /review please",
			"john.doe",
			"John Doe",
		)

		result := handler.shouldProcessComment(comment)
		if !result {
			t.Error("expected comment with bot mention and trigger to be processed")
		}
	})

	t.Run("ignores comment without trigger keyword", func(t *testing.T) {
		comment := models.NewComment(
			123,
			"@pr-reviewer-bot This looks good to me",
			"john.doe",
			"John Doe",
		)

		result := handler.shouldProcessComment(comment)
		if result {
			t.Error("expected comment without trigger keyword to be ignored")
		}
	})

	t.Run("ignores comment without bot mention", func(t *testing.T) {
		comment := models.NewComment(
			123,
			"/review please",
			"john.doe",
			"John Doe",
		)

		result := handler.shouldProcessComment(comment)
		if result {
			t.Error("expected comment without bot mention to be ignored")
		}
	})

	t.Run("ignores comment with trigger but no bot mention", func(t *testing.T) {
		comment := models.NewComment(
			123,
			"Can someone /review this?",
			"john.doe",
			"John Doe",
		)

		result := handler.shouldProcessComment(comment)
		if result {
			t.Error("expected comment with trigger but no bot mention to be ignored")
		}
	})

	t.Run("ignores comment from bot itself by username", func(t *testing.T) {
		comment := models.NewComment(
			123,
			"@pr-reviewer-bot /review please",
			botUsername, // Same as bot username
			"PR Reviewer Bot",
		)

		result := handler.shouldProcessComment(comment)
		if result {
			t.Error("expected comment from bot (by username) to be ignored")
		}
	})

	t.Run("ignores comment from bot itself by display name", func(t *testing.T) {
		comment := models.NewComment(
			123,
			"@pr-reviewer-bot /review please",
			"some.user",
			botUsername, // Same as bot username
		)

		result := handler.shouldProcessComment(comment)
		if result {
			t.Error("expected comment from bot (by display name) to be ignored")
		}
	})

	t.Run("ignores bot-generated review comments", func(t *testing.T) {
		comment := models.NewComment(
			123,
			"Here are my findings:\n\nSome review content\n\n---\nReviewed by Claude in 5.2s*",
			"john.doe",
			"John Doe",
		)

		result := handler.shouldProcessComment(comment)
		if result {
			t.Error("expected bot-generated review comment to be ignored")
		}
	})

	t.Run("processes comment with bot mention and trigger at start", func(t *testing.T) {
		comment := models.NewComment(
			123,
			"@pr-reviewer-bot /review",
			"alice.smith",
			"Alice Smith",
		)

		result := handler.shouldProcessComment(comment)
		if !result {
			t.Error("expected comment with bot mention and trigger at start to be processed")
		}
	})

	t.Run("processes comment with bot mention at start and trigger in middle", func(t *testing.T) {
		comment := models.NewComment(
			123,
			"@pr-reviewer-bot can you /review this PR?",
			"bob.jones",
			"Bob Jones",
		)

		result := handler.shouldProcessComment(comment)
		if !result {
			t.Error("expected comment with bot mention and trigger to be processed")
		}
	})

	t.Run("processes comment with trigger first then bot mention", func(t *testing.T) {
		comment := models.NewComment(
			123,
			"/review @pr-reviewer-bot please check this",
			"bob.jones",
			"Bob Jones",
		)

		result := handler.shouldProcessComment(comment)
		if !result {
			t.Error("expected comment with trigger and bot mention (any order) to be processed")
		}
	})

	t.Run("processes comment with bot mention at end", func(t *testing.T) {
		comment := models.NewComment(
			123,
			"Can you /review this PR? @pr-reviewer-bot",
			"bob.jones",
			"Bob Jones",
		)

		result := handler.shouldProcessComment(comment)
		if !result {
			t.Error("expected comment with bot mention at end to be processed")
		}
	})

	t.Run("ignores empty comment", func(t *testing.T) {
		comment := models.NewComment(
			123,
			"",
			"john.doe",
			"John Doe",
		)

		result := handler.shouldProcessComment(comment)
		if result {
			t.Error("expected empty comment to be ignored")
		}
	})

	t.Run("is case sensitive for trigger keyword", func(t *testing.T) {
		comment := models.NewComment(
			123,
			"@pr-reviewer-bot /REVIEW please", // uppercase
			"john.doe",
			"John Doe",
		)

		result := handler.shouldProcessComment(comment)
		if result {
			t.Error("expected case-sensitive trigger matching - /REVIEW should not match /review")
		}
	})

	t.Run("handles comment with special characters", func(t *testing.T) {
		comment := models.NewComment(
			123,
			"@pr-reviewer-bot please /review this <script>alert('xss')</script>",
			"john.doe",
			"John Doe",
		)

		result := handler.shouldProcessComment(comment)
		if !result {
			t.Error("expected comment with special characters and trigger to be processed")
		}
	})

	t.Run("processes comment with trigger as substring", func(t *testing.T) {
		comment := models.NewComment(
			123,
			"@pr-reviewer-bot this is a /reviewer comment", // /reviewer contains /review
			"john.doe",
			"John Doe",
		)

		result := handler.shouldProcessComment(comment)
		if !result {
			t.Error("expected comment containing trigger substring to be processed")
		}
	})

	t.Run("ignores comment with bot mention but without trigger substring", func(t *testing.T) {
		comment := models.NewComment(
			123,
			"@pr-reviewer-bot this needs a review", // "review" without "/"
			"john.doe",
			"John Doe",
		)

		result := handler.shouldProcessComment(comment)
		if result {
			t.Error("expected comment without trigger to be ignored")
		}
	})
}

func TestShouldProcessComment_BotGeneratedPatterns(t *testing.T) {
	handler := &WebhookHandler{
		config: WebhookConfig{
			BitbucketUsername: "pr-bot",
			TriggerKeyword:    "/review",
		},
		logger: &mockLogger{},
	}

	botGeneratedComments := []struct {
		name string
		text string
	}{
		{
			name: "standard bot review",
			text: "Review feedback here\n\nReviewed by Claude Sonnet in 3.5s*",
		},
		{
			name: "bot review with uppercase",
			text: "Feedback\n\nREVIEWED BY GPT-4 IN 2.1s*",
		},
		{
			name: "bot review mixed case",
			text: "Content\n\nReViEwEd By Claude in 5s*",
		},
	}

	for _, tc := range botGeneratedComments {
		t.Run(tc.name, func(t *testing.T) {
			comment := models.NewComment(
				123,
				tc.text,
				"john.doe",
				"John Doe",
			)

			result := handler.shouldProcessComment(comment)
			if result {
				t.Errorf("expected bot-generated comment to be ignored: %s", tc.name)
			}
		})
	}
}

func TestShouldProcessComment_EdgeCases(t *testing.T) {
	handler := &WebhookHandler{
		config: WebhookConfig{
			BitbucketUsername: "bot-user",
			TriggerKeyword:    "/review",
		},
		logger: &mockLogger{},
	}

	t.Run("handles very long comment with bot mention and trigger", func(t *testing.T) {
		longText := make([]byte, 10000)
		for i := range longText {
			longText[i] = 'a'
		}
		text := "@bot-user " + string(longText) + "/review"

		comment := models.NewComment(
			123,
			text,
			"john.doe",
			"John Doe",
		)

		result := handler.shouldProcessComment(comment)
		if !result {
			t.Error("expected long comment with bot mention and trigger to be processed")
		}
	})

	t.Run("handles unicode characters in comment", func(t *testing.T) {
		comment := models.NewComment(
			123,
			"@bot-user 请帮忙 /review 这个PR 🚀",
			"john.doe",
			"John Doe",
		)

		result := handler.shouldProcessComment(comment)
		if !result {
			t.Error("expected unicode comment with bot mention and trigger to be processed")
		}
	})

	t.Run("handles multiline comment with bot mention and trigger", func(t *testing.T) {
		comment := models.NewComment(
			123,
			"Line 1\n@bot-user Line 2\n/review\nLine 4",
			"john.doe",
			"John Doe",
		)

		result := handler.shouldProcessComment(comment)
		if !result {
			t.Error("expected multiline comment with bot mention and trigger to be processed")
		}
	})

	t.Run("handles nil comment gracefully", func(t *testing.T) {
		// This shouldn't happen in practice, but let's be defensive
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("shouldProcessComment panicked on nil comment: %v", r)
			}
		}()

		// If we get here without panic, the test passes
		// We can't actually pass nil due to Go's type system, but we can check empty fields
		comment := &models.Comment{}
		result := handler.shouldProcessComment(comment)
		if result {
			t.Error("expected empty comment to be ignored")
		}
	})
}

func TestTruncateString(t *testing.T) {
	t.Run("returns string unchanged if shorter than limit", func(t *testing.T) {
		input := "short"
		result := truncateString(input, 10)
		if result != "short" {
			t.Errorf("expected 'short', got '%s'", result)
		}
	})

	t.Run("returns string unchanged if equal to limit", func(t *testing.T) {
		input := "exact"
		result := truncateString(input, 5)
		if result != "exact" {
			t.Errorf("expected 'exact', got '%s'", result)
		}
	})

	t.Run("truncates string longer than limit", func(t *testing.T) {
		input := "this is a very long string"
		result := truncateString(input, 10)
		expected := "this is a ..."
		if result != expected {
			t.Errorf("expected '%s', got '%s'", expected, result)
		}
	})

	t.Run("handles empty string", func(t *testing.T) {
		input := ""
		result := truncateString(input, 10)
		if result != "" {
			t.Errorf("expected empty string, got '%s'", result)
		}
	})

	t.Run("handles zero max length", func(t *testing.T) {
		input := "test"
		result := truncateString(input, 0)
		expected := "..."
		if result != expected {
			t.Errorf("expected '%s', got '%s'", expected, result)
		}
	})
}

// Mock logger for testing
type mockLogger struct{}

func (m *mockLogger) Debug(msg string, args ...any) {}
func (m *mockLogger) Info(msg string, args ...any)  {}
func (m *mockLogger) Warn(msg string, args ...any)  {}
func (m *mockLogger) Error(msg string, args ...any) {}
func (m *mockLogger) Fatal(msg string, args ...any) {}
