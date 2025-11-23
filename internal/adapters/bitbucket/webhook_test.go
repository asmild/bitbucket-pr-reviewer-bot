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

func TestGetEventType(t *testing.T) {
	parser := NewWebhookParser()

	t.Run("extracts event type from payload", func(t *testing.T) {
		payload := []byte(`{"eventKey": "pr:opened"}`)
		eventType, err := parser.GetEventType(payload)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if eventType != "pr:opened" {
			t.Errorf("expected 'pr:opened', got '%s'", eventType)
		}
	})

	t.Run("handles comment event", func(t *testing.T) {
		payload := []byte(`{"eventKey": "pr:comment:added"}`)
		eventType, err := parser.GetEventType(payload)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if eventType != "pr:comment:added" {
			t.Errorf("expected 'pr:comment:added', got '%s'", eventType)
		}
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		payload := []byte(`{invalid json}`)
		_, err := parser.GetEventType(payload)

		if err == nil {
			t.Error("expected error for invalid JSON")
		}
		if errors.GetCode(err) != errors.ErrorCodeVCSInvalidPayload {
			t.Errorf("expected ErrorCodeVCSInvalidPayload, got %s", errors.GetCode(err))
		}
	})

	t.Run("handles missing eventKey", func(t *testing.T) {
		payload := []byte(`{}`)
		eventType, err := parser.GetEventType(payload)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if eventType != "" {
			t.Errorf("expected empty string, got '%s'", eventType)
		}
	})
}

func TestParsePullRequestEvent(t *testing.T) {
	parser := NewWebhookParser()

	t.Run("parses valid PR event", func(t *testing.T) {
		payload := createValidPRPayload()
		pr, err := parser.ParsePullRequestEvent(payload)

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
		if pr.RepositoryID != "myrepo" {
			t.Errorf("expected repository 'myrepo', got '%s'", pr.RepositoryID)
		}
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		payload := []byte(`{invalid json}`)
		_, err := parser.ParsePullRequestEvent(payload)

		if err == nil {
			t.Error("expected error for invalid JSON")
		}
		if errors.GetCode(err) != errors.ErrorCodeVCSInvalidPayload {
			t.Errorf("expected ErrorCodeVCSInvalidPayload, got %s", errors.GetCode(err))
		}
	})

	t.Run("returns error for missing title", func(t *testing.T) {
		payload := createPayloadWithMissingField("title")
		_, err := parser.ParsePullRequestEvent(payload)

		if err == nil {
			t.Error("expected error for missing title")
		}
		if errors.GetCode(err) != errors.ErrorCodeValidationFailed {
			t.Errorf("expected ErrorCodeValidationFailed, got %s", errors.GetCode(err))
		}
	})

	t.Run("returns error for missing author", func(t *testing.T) {
		payload := createPayloadWithMissingField("author")
		_, err := parser.ParsePullRequestEvent(payload)

		if err == nil {
			t.Error("expected error for missing author")
		}
	})

	t.Run("returns error for missing fromRef", func(t *testing.T) {
		payload := createPayloadWithMissingField("fromRef")
		_, err := parser.ParsePullRequestEvent(payload)

		if err == nil {
			t.Error("expected error for missing fromRef")
		}
	})

	t.Run("returns error for missing toRef", func(t *testing.T) {
		payload := createPayloadWithMissingField("toRef")
		_, err := parser.ParsePullRequestEvent(payload)

		if err == nil {
			t.Error("expected error for missing toRef")
		}
	})

	t.Run("returns error for missing repository slug", func(t *testing.T) {
		payload := createPayloadWithMissingField("repositorySlug")
		_, err := parser.ParsePullRequestEvent(payload)

		if err == nil {
			t.Error("expected error for missing repository slug")
		}
	})

	t.Run("returns error for invalid source branch name", func(t *testing.T) {
		payload := createPayloadWithInvalidBranch("source")
		_, err := parser.ParsePullRequestEvent(payload)

		if err == nil {
			t.Error("expected error for invalid source branch name")
		}
		if errors.GetCode(err) != errors.ErrorCodeValidationFailed {
			t.Errorf("expected ErrorCodeValidationFailed, got %s", errors.GetCode(err))
		}
	})

	t.Run("returns error for invalid destination branch name", func(t *testing.T) {
		payload := createPayloadWithInvalidBranch("destination")
		_, err := parser.ParsePullRequestEvent(payload)

		if err == nil {
			t.Error("expected error for invalid destination branch name")
		}
	})

	t.Run("returns error for missing clone URL", func(t *testing.T) {
		payload := createPayloadWithMissingField("cloneURL")
		_, err := parser.ParsePullRequestEvent(payload)

		if err == nil {
			t.Error("expected error for missing clone URL")
		}
		if errors.GetCode(err) != errors.ErrorCodeVCSInvalidPayload {
			t.Errorf("expected ErrorCodeVCSInvalidPayload, got %s", errors.GetCode(err))
		}
	})

	t.Run("uses repository slug from fromRef when root is empty", func(t *testing.T) {
		payload := createPayloadWithRepoSlugInFromRef()
		pr, err := parser.ParsePullRequestEvent(payload)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if pr.RepositoryID != "myrepo" {
			t.Errorf("expected repository 'myrepo', got '%s'", pr.RepositoryID)
		}
	})
}

func TestParseCommentEvent(t *testing.T) {
	parser := NewWebhookParser()

	t.Run("parses valid comment event", func(t *testing.T) {
		payload := createValidCommentPayload()
		pr, err := parser.ParseCommentEvent(payload)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if pr.ID != 123 {
			t.Errorf("expected ID 123, got %d", pr.ID)
		}
		if pr.CommentID != 456 {
			t.Errorf("expected comment ID 456, got %d", pr.CommentID)
		}
	})

	t.Run("returns error for missing comment", func(t *testing.T) {
		payload := createValidPRPayload() // PR payload without comment
		_, err := parser.ParseCommentEvent(payload)

		if err == nil {
			t.Error("expected error for missing comment")
		}
		if errors.GetCode(err) != errors.ErrorCodeVCSInvalidPayload {
			t.Errorf("expected ErrorCodeVCSInvalidPayload, got %s", errors.GetCode(err))
		}
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		payload := []byte(`{invalid json}`)
		_, err := parser.ParseCommentEvent(payload)

		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})
}

func TestExtractBaseURL(t *testing.T) {
	t.Run("extracts base URL from clone URL", func(t *testing.T) {
		cloneURL := "https://bitbucket.example.com/scm/proj/repo.git"
		baseURL := ExtractBaseURL(cloneURL)

		expected := "https://bitbucket.example.com"
		if baseURL != expected {
			t.Errorf("expected '%s', got '%s'", expected, baseURL)
		}
	})

	t.Run("handles URL without /scm/", func(t *testing.T) {
		cloneURL := "https://bitbucket.example.com/other/path"
		baseURL := ExtractBaseURL(cloneURL)

		// When /scm/ is not present, Split returns the whole string as first element
		expected := "https://bitbucket.example.com/other/path"
		if baseURL != expected {
			t.Errorf("expected '%s', got '%s'", expected, baseURL)
		}
	})

	t.Run("handles empty URL", func(t *testing.T) {
		baseURL := ExtractBaseURL("")

		if baseURL != "" {
			t.Errorf("expected empty string, got '%s'", baseURL)
		}
	})
}

func TestBranchNameValidation(t *testing.T) {
	parser := NewWebhookParser()

	validBranches := []string{
		"main",
		"feature/test",
		"bugfix/issue-123",
		"release/1.0.0",
		"hotfix_urgent",
		"feature.new",
	}

	for _, branch := range validBranches {
		t.Run("accepts valid branch "+branch, func(t *testing.T) {
			payload := createPayloadWithBranches(branch, "main")
			_, err := parser.ParsePullRequestEvent(payload)

			if err != nil {
				t.Errorf("branch '%s' should be valid, got error: %v", branch, err)
			}
		})
	}

	invalidBranches := []string{
		"branch with spaces",
		"branch<script>",
		"branch&cmd",
		"branch;rm",
		"branch|pipe",
		"branch`backtick",
	}

	for _, branch := range invalidBranches {
		t.Run("rejects invalid branch "+branch, func(t *testing.T) {
			payload := createPayloadWithBranches(branch, "main")
			_, err := parser.ParsePullRequestEvent(payload)

			if err == nil {
				t.Errorf("branch '%s' should be invalid", branch)
			}
			if errors.GetCode(err) != errors.ErrorCodeValidationFailed {
				t.Errorf("expected validation error, got %s", errors.GetCode(err))
			}
		})
	}
}

func TestProjectKeyExtraction(t *testing.T) {
	parser := NewWebhookParser()

	t.Run("extracts project key from root repository", func(t *testing.T) {
		payload := createValidPRPayload()
		pr, err := parser.ParsePullRequestEvent(payload)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if pr.ProjectKey != "PROJ" {
			t.Errorf("expected project key 'PROJ', got '%s'", pr.ProjectKey)
		}
	})

	t.Run("extracts project key from fromRef when root is empty", func(t *testing.T) {
		payload := createPayloadWithProjectKeyInFromRef()
		pr, err := parser.ParsePullRequestEvent(payload)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if pr.ProjectKey != "PROJ" {
			t.Errorf("expected project key 'PROJ', got '%s'", pr.ProjectKey)
		}
	})

	t.Run("returns error when project key not found", func(t *testing.T) {
		payload := createPayloadWithMissingField("projectKey")
		_, err := parser.ParsePullRequestEvent(payload)

		if err == nil {
			t.Error("expected error for missing project key")
		}
	})
}

// Helper functions to create test payloads

func createValidPRPayload() []byte {
	payload := map[string]interface{}{
		"pullRequest": map[string]interface{}{
			"id":          123,
			"title":       "Test PR",
			"description": "Test description",
			"author": map[string]interface{}{
				"user": map[string]interface{}{
					"displayName": "John Doe",
				},
			},
			"fromRef": map[string]interface{}{
				"displayId": "feature/test",
				"repository": map[string]interface{}{
					"slug": "myrepo",
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
			"links": map[string]interface{}{
				"self": []map[string]interface{}{
					{"href": "https://bitbucket.example.com/projects/PROJ/repos/myrepo/pull-requests/123"},
				},
			},
		},
		"repository": map[string]interface{}{
			"slug": "myrepo",
			"project": map[string]interface{}{
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
		"pullRequest": map[string]interface{}{
			"id":          123,
			"title":       "Test PR",
			"description": "Test description",
			"author": map[string]interface{}{
				"user": map[string]interface{}{
					"displayName": "John Doe",
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
			"slug": "myrepo",
			"project": map[string]interface{}{
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
		},
	}
	data, _ := json.Marshal(payload)
	return data
}

func createPayloadWithMissingField(field string) []byte {
	payload := map[string]interface{}{
		"pullRequest": map[string]interface{}{
			"id":          123,
			"title":       "Test PR",
			"description": "Test description",
			"author": map[string]interface{}{
				"user": map[string]interface{}{
					"displayName": "John Doe",
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
			"slug": "myrepo",
			"project": map[string]interface{}{
				"key": "PROJ",
			},
			"links": map[string]interface{}{
				"clone": []map[string]interface{}{
					{"name": "http", "href": "https://bitbucket.example.com/scm/proj/myrepo.git"},
				},
			},
		},
	}

	switch field {
	case "title":
		payload["pullRequest"].(map[string]interface{})["title"] = ""
	case "author":
		payload["pullRequest"].(map[string]interface{})["author"] = map[string]interface{}{
			"user": map[string]interface{}{
				"displayName": "",
			},
		}
	case "fromRef":
		payload["pullRequest"].(map[string]interface{})["fromRef"] = map[string]interface{}{
			"displayId": "",
		}
	case "toRef":
		payload["pullRequest"].(map[string]interface{})["toRef"] = map[string]interface{}{
			"displayId": "",
		}
	case "repositorySlug":
		payload["repository"].(map[string]interface{})["slug"] = ""
		payload["pullRequest"].(map[string]interface{})["fromRef"] = map[string]interface{}{
			"displayId": "feature/test",
			"repository": map[string]interface{}{
				"slug": "",
			},
		}
	case "cloneURL":
		payload["repository"].(map[string]interface{})["links"] = map[string]interface{}{
			"clone": []map[string]interface{}{},
		}
		payload["pullRequest"].(map[string]interface{})["fromRef"] = map[string]interface{}{
			"displayId": "feature/test",
			"repository": map[string]interface{}{
				"slug": "myrepo",
				"links": map[string]interface{}{
					"clone": []map[string]interface{}{},
				},
			},
		}
	case "projectKey":
		delete(payload["repository"].(map[string]interface{}), "project")
		payload["pullRequest"].(map[string]interface{})["fromRef"] = map[string]interface{}{
			"displayId": "feature/test",
			"repository": map[string]interface{}{
				"slug": "myrepo",
				"links": map[string]interface{}{
					"clone": []map[string]interface{}{
						{"name": "http", "href": "https://bitbucket.example.com/scm/proj/myrepo.git"},
					},
				},
			},
		}
		payload["pullRequest"].(map[string]interface{})["toRef"] = map[string]interface{}{
			"displayId": "main",
			"repository": map[string]interface{}{},
		}
	}

	data, _ := json.Marshal(payload)
	return data
}

func createPayloadWithInvalidBranch(branchType string) []byte {
	payload := map[string]interface{}{
		"pullRequest": map[string]interface{}{
			"id":          123,
			"title":       "Test PR",
			"description": "Test description",
			"author": map[string]interface{}{
				"user": map[string]interface{}{
					"displayName": "John Doe",
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
			"slug": "myrepo",
			"project": map[string]interface{}{
				"key": "PROJ",
			},
			"links": map[string]interface{}{
				"clone": []map[string]interface{}{
					{"name": "http", "href": "https://bitbucket.example.com/scm/proj/myrepo.git"},
				},
			},
		},
	}

	if branchType == "source" {
		payload["pullRequest"].(map[string]interface{})["fromRef"] = map[string]interface{}{
			"displayId": "invalid branch name",
		}
	} else {
		payload["pullRequest"].(map[string]interface{})["toRef"] = map[string]interface{}{
			"displayId": "invalid branch name",
		}
	}

	data, _ := json.Marshal(payload)
	return data
}

func createPayloadWithBranches(sourceBranch, destBranch string) []byte {
	payload := map[string]interface{}{
		"pullRequest": map[string]interface{}{
			"id":          123,
			"title":       "Test PR",
			"description": "Test description",
			"author": map[string]interface{}{
				"user": map[string]interface{}{
					"displayName": "John Doe",
				},
			},
			"fromRef": map[string]interface{}{
				"displayId": sourceBranch,
			},
			"toRef": map[string]interface{}{
				"displayId": destBranch,
			},
		},
		"repository": map[string]interface{}{
			"slug": "myrepo",
			"project": map[string]interface{}{
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

func createPayloadWithRepoSlugInFromRef() []byte {
	payload := map[string]interface{}{
		"pullRequest": map[string]interface{}{
			"id":          123,
			"title":       "Test PR",
			"description": "Test description",
			"author": map[string]interface{}{
				"user": map[string]interface{}{
					"displayName": "John Doe",
				},
			},
			"fromRef": map[string]interface{}{
				"displayId": "feature/test",
				"repository": map[string]interface{}{
					"slug": "myrepo",
					"project": map[string]interface{}{
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
			"slug": "", // Empty root slug
		},
	}

	data, _ := json.Marshal(payload)
	return data
}

func createPayloadWithProjectKeyInFromRef() []byte {
	payload := map[string]interface{}{
		"pullRequest": map[string]interface{}{
			"id":          123,
			"title":       "Test PR",
			"description": "Test description",
			"author": map[string]interface{}{
				"user": map[string]interface{}{
					"displayName": "John Doe",
				},
			},
			"fromRef": map[string]interface{}{
				"displayId": "feature/test",
				"repository": map[string]interface{}{
					"slug": "myrepo",
					"project": map[string]interface{}{
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
