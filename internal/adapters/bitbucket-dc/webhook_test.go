package bitbucket

import (
	"encoding/json"
	"testing"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/errors"
)

func TestNewWebhookParser(t *testing.T) {
	parser := NewWebhookParser()
	if parser == nil {
		t.Fatal("NewWebhookParser should return non-nil parser")
	}
}

func TestParse(t *testing.T) {
	parser := NewWebhookParser()

	t.Run("parses valid PR opened event", func(t *testing.T) {
		payload := createValidPRPayload()
		event, err := parser.Parse(payload)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if event == nil {
			t.Fatal("expected event, got nil")
		}

		eventType := event.GetEventType()
		if eventType != "pr:opened" {
			t.Errorf("expected 'pr:opened', got '%s'", eventType)
		}
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		payload := []byte(`{invalid json}`)
		_, err := parser.Parse(payload)

		if err == nil {
			t.Error("expected error for invalid JSON")
		}
		if errors.GetCode(err) != errors.ErrorCodeVCSInvalidPayload {
			t.Errorf("expected ErrorCodeVCSInvalidPayload, got %s", errors.GetCode(err))
		}
	})

	t.Run("returns error for missing required fields", func(t *testing.T) {
		payload := []byte(`{"eventKey": "pr:opened"}`)
		_, err := parser.Parse(payload)

		if err == nil {
			t.Error("expected error for missing required fields")
		}
		if errors.GetCode(err) != errors.ErrorCodeValidationFailed {
			t.Errorf("expected ErrorCodeValidationFailed, got %s", errors.GetCode(err))
		}
	})
}

func TestWebhookEvent_GetEventType(t *testing.T) {
	parser := NewWebhookParser()

	t.Run("extracts PR opened event type", func(t *testing.T) {
		payload := createValidPRPayload()
		event, err := parser.Parse(payload)
		if err != nil {
			t.Fatalf("expected no error parsing, got %v", err)
		}

		eventType := event.GetEventType()
		if eventType != "pr:opened" {
			t.Errorf("expected 'pr:opened', got '%s'", eventType)
		}
	})

	t.Run("extracts comment added event type", func(t *testing.T) {
		payload := createValidCommentPayload()
		event, err := parser.Parse(payload)
		if err != nil {
			t.Fatalf("expected no error parsing, got %v", err)
		}

		eventType := event.GetEventType()
		if eventType != "pr:comment:added" {
			t.Errorf("expected 'pr:comment:added', got '%s'", eventType)
		}
	})
}

func TestWebhookEvent_GetPullRequest(t *testing.T) {
	parser := NewWebhookParser()

	t.Run("extracts valid PR data", func(t *testing.T) {
		payload := createValidPRPayload()
		event, err := parser.Parse(payload)
		if err != nil {
			t.Fatalf("expected no error parsing, got %v", err)
		}

		pr, err := event.GetPullRequest()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if pr.ID != 123 {
			t.Errorf("expected ID 123, got %d", pr.ID)
		}
		if pr.Title != "Test PR" {
			t.Errorf("expected title 'Test PR', got '%s'", pr.Title)
		}
		if pr.Author != "John Doe" {
			t.Errorf("expected author 'John Doe', got '%s'", pr.Author)
		}
		if pr.SourceBranch != "feature/test" {
			t.Errorf("expected source branch 'feature/test', got '%s'", pr.SourceBranch)
		}
		if pr.DestinationBranch != "main" {
			t.Errorf("expected destination branch 'main', got '%s'", pr.DestinationBranch)
		}
		if pr.ProjectKey != "PROJ" {
			t.Errorf("expected project key 'PROJ', got '%s'", pr.ProjectKey)
		}
		if pr.RepositorySlug != "myrepo" {
			t.Errorf("expected repository slug 'myrepo', got '%s'", pr.RepositorySlug)
		}
	})

	t.Run("uses repository slug from fromRef when root is empty", func(t *testing.T) {
		payload := createPayloadWithRepoSlugInFromRef()
		event, err := parser.Parse(payload)
		if err != nil {
			t.Fatalf("expected no error parsing, got %v", err)
		}

		pr, err := event.GetPullRequest()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if pr.RepositorySlug != "myrepo" {
			t.Errorf("expected repository slug 'myrepo', got '%s'", pr.RepositorySlug)
		}
	})

	t.Run("extracts project key from fromRef when root is empty", func(t *testing.T) {
		payload := createPayloadWithProjectKeyInFromRef()
		event, err := parser.Parse(payload)
		if err != nil {
			t.Fatalf("expected no error parsing, got %v", err)
		}

		pr, err := event.GetPullRequest()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if pr.ProjectKey != "PROJ" {
			t.Errorf("expected project key 'PROJ', got '%s'", pr.ProjectKey)
		}
	})
}

func TestWebhookEvent_GetComment(t *testing.T) {
	parser := NewWebhookParser()

	t.Run("returns nil for PR opened event", func(t *testing.T) {
		payload := createValidPRPayload()
		event, err := parser.Parse(payload)
		if err != nil {
			t.Fatalf("expected no error parsing, got %v", err)
		}

		comment := event.GetComment()
		if comment != nil {
			t.Errorf("expected nil comment for PR event, got %+v", comment)
		}
	})

	t.Run("returns comment data for comment event", func(t *testing.T) {
		payload := createValidCommentPayload()
		event, err := parser.Parse(payload)
		if err != nil {
			t.Fatalf("expected no error parsing, got %v", err)
		}

		comment := event.GetComment()
		if comment == nil {
			t.Fatal("expected comment, got nil")
		}

		if comment.ID != 456 {
			t.Errorf("expected comment ID 456, got %d", comment.ID)
		}
		if comment.Text != "/review" {
			t.Errorf("expected comment text '/review', got '%s'", comment.Text)
		}
		if comment.AuthorName != "john.doe" {
			t.Errorf("expected author name 'john.doe', got '%s'", comment.AuthorName)
		}
		if comment.AuthorDisplayName != "John Doe" {
			t.Errorf("expected author display name 'John Doe', got '%s'", comment.AuthorDisplayName)
		}
	})
}

// Helper functions to create test payloads

func createValidPRPayload() []byte {
	payload := map[string]interface{}{
		"eventKey": "pr:opened",
		"pullRequest": map[string]interface{}{
			"id":          123,
			"title":       "Test PR",
			"description": "Test description",
			"author": map[string]interface{}{
				"user": map[string]interface{}{
					"name":        "john.doe",
					"displayName": "John Doe",
				},
			},
			"fromRef": map[string]interface{}{
				"displayId": "feature/test",
			},
			"toRef": map[string]interface{}{
				"displayId": "main",
			},
			"links": map[string]interface{}{
				"self": []map[string]interface{}{
					{"href": "https://bitbucket.example.com/projects/PROJ/repos/myrepo/pull-requests/123"},
				},
			},
		},
		"repository": map[string]interface{}{
			"id":   100,
			"slug": "myrepo",
			"project": map[string]interface{}{
				"id":  10,
				"key": "PROJ",
			},
			"links": map[string]interface{}{
				"clone": []map[string]interface{}{
					{"name": "http", "href": "https://bitbucket.example.com/scm/proj/myrepo.git"},
				},
			},
		},
	}
	data, _ := json.Marshal(payload)
	return data
}

func createValidCommentPayload() []byte {
	payload := map[string]interface{}{
		"eventKey": "pr:comment:added",
		"pullRequest": map[string]interface{}{
			"id":          123,
			"title":       "Test PR",
			"description": "Test description",
			"author": map[string]interface{}{
				"user": map[string]interface{}{
					"name":        "alice.smith",
					"displayName": "Alice Smith",
				},
			},
			"fromRef": map[string]interface{}{
				"displayId": "feature/test",
			},
			"toRef": map[string]interface{}{
				"displayId": "main",
			},
		},
		"repository": map[string]interface{}{
			"id":   100,
			"slug": "myrepo",
			"project": map[string]interface{}{
				"id":  10,
				"key": "PROJ",
			},
			"links": map[string]interface{}{
				"clone": []map[string]interface{}{
					{"name": "http", "href": "https://bitbucket.example.com/scm/proj/myrepo.git"},
				},
			},
		},
		"comment": map[string]interface{}{
			"id":   456,
			"text": "/review",
			"author": map[string]interface{}{
				"user": map[string]interface{}{
					"name":        "john.doe",
					"displayName": "John Doe",
				},
			},
		},
	}
	data, _ := json.Marshal(payload)
	return data
}

func createPayloadWithRepoSlugInFromRef() []byte {
	payload := map[string]interface{}{
		"eventKey": "pr:opened",
		"pullRequest": map[string]interface{}{
			"id":          123,
			"title":       "Test PR",
			"description": "Test description",
			"author": map[string]interface{}{
				"user": map[string]interface{}{
					"name":        "john.doe",
					"displayName": "John Doe",
				},
			},
			"fromRef": map[string]interface{}{
				"displayId": "feature/test",
				"repository": map[string]interface{}{
					"id":   100,
					"slug": "myrepo",
					"project": map[string]interface{}{
						"id":  10,
						"key": "PROJ",
					},
					"links": map[string]interface{}{
						"clone": []map[string]interface{}{
							{"name": "http", "href": "https://bitbucket.example.com/scm/proj/myrepo.git"},
						},
					},
				},
			},
			"toRef": map[string]interface{}{
				"displayId": "main",
			},
		},
		"repository": map[string]interface{}{
			"id":   100,
			"slug": "", // Empty root slug
		},
	}

	data, _ := json.Marshal(payload)
	return data
}

func createPayloadWithProjectKeyInFromRef() []byte {
	payload := map[string]interface{}{
		"eventKey": "pr:opened",
		"pullRequest": map[string]interface{}{
			"id":          123,
			"title":       "Test PR",
			"description": "Test description",
			"author": map[string]interface{}{
				"user": map[string]interface{}{
					"name":        "john.doe",
					"displayName": "John Doe",
				},
			},
			"fromRef": map[string]interface{}{
				"displayId": "feature/test",
				"repository": map[string]interface{}{
					"id":   100,
					"slug": "myrepo",
					"project": map[string]interface{}{
						"id":  10,
						"key": "PROJ",
					},
					"links": map[string]interface{}{
						"clone": []map[string]interface{}{
							{"name": "http", "href": "https://bitbucket.example.com/scm/proj/myrepo.git"},
						},
					},
				},
			},
			"toRef": map[string]interface{}{
				"displayId": "main",
			},
		},
		"repository": map[string]interface{}{
			"id": 100,
			"slug": "myrepo",
			"links": map[string]interface{}{
				"clone": []map[string]interface{}{
					{"name": "http", "href": "https://bitbucket.example.com/scm/proj/myrepo.git"},
				},
			},
		},
	}

	data, _ := json.Marshal(payload)
	return data
}
