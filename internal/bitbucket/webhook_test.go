package bitbucket

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"testing"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/config"
)

func TestValidateSignature_ValidSignature(t *testing.T) {
	secret := "test-secret"
	payload := []byte(`{"test":"data"}`)

	// Generate valid signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	err := ValidateSignature(secret, payload, signature)
	if err != nil {
		t.Fatalf("expected no error for valid signature, got %v", err)
	}
}

func TestValidateSignature_InvalidSignature(t *testing.T) {
	secret := "test-secret"
	payload := []byte(`{"test":"data"}`)
	invalidSignature := "sha256=invalidsignature"

	err := ValidateSignature(secret, payload, invalidSignature)
	if err == nil {
		t.Fatalf("expected error for invalid signature")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("invalid")) {
		t.Fatalf("expected error to contain 'invalid'")
	}
}

func TestValidateSignature_EmptySecret(t *testing.T) {
	payload := []byte(`{"test":"data"}`)

	// Empty secret should skip validation
	err := ValidateSignature("", payload, "any-signature")
	if err != nil {
		t.Fatalf("expected no error for empty secret, got %v", err)
	}
}

func TestValidateSignature_MissingSignature(t *testing.T) {
	secret := "test-secret"
	payload := []byte(`{"test":"data"}`)

	err := ValidateSignature(secret, payload, "")
	if err == nil {
		t.Fatalf("expected error for missing signature")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("missing")) {
		t.Fatalf("expected error to contain 'missing'")
	}
}

func TestValidateSignature_DifferentPayload(t *testing.T) {
	secret := "test-secret"
	payload1 := []byte(`{"test":"data1"}`)
	payload2 := []byte(`{"test":"data2"}`)

	// Generate signature for payload1
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload1)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	// Try to validate signature for payload2
	err := ValidateSignature(secret, payload2, signature)
	if err == nil {
		t.Fatalf("expected error for mismatched payload")
	}
}

func TestValidateCommentTrigger_Valid(t *testing.T) {
	cfg := &config.Config{
		Bitbucket: config.BitbucketConfig{
			User:           "reviewbot",
			TriggerKeyword: "review",
		},
	}

	payload := &WebhookPayload{
		Comment: &Comment{
			Text: "@reviewbot review this PR",
		},
	}

	err := ValidateCommentTrigger(cfg, payload)
	if err != nil {
		t.Fatalf("expected no error for valid trigger, got %v", err)
	}
}

func TestValidateCommentTrigger_MissingComment(t *testing.T) {
	cfg := &config.Config{
		Bitbucket: config.BitbucketConfig{
			User:           "reviewbot",
			TriggerKeyword: "review",
		},
	}

	payload := &WebhookPayload{
		Comment: nil,
	}

	err := ValidateCommentTrigger(cfg, payload)
	if err == nil {
		t.Fatalf("expected error for missing comment")
	}
}

func TestValidateCommentTrigger_MissingUserMention(t *testing.T) {
	cfg := &config.Config{
		Bitbucket: config.BitbucketConfig{
			User:           "reviewbot",
			TriggerKeyword: "review",
		},
	}

	payload := &WebhookPayload{
		Comment: &Comment{
			Text: "review this PR please",
		},
	}

	err := ValidateCommentTrigger(cfg, payload)
	if err == nil {
		t.Fatalf("expected error for missing user mention")
	}
}

func TestValidateCommentTrigger_MissingKeyword(t *testing.T) {
	cfg := &config.Config{
		Bitbucket: config.BitbucketConfig{
			User:           "botuser",
			TriggerKeyword: "review",
		},
	}

	payload := &WebhookPayload{
		Comment: &Comment{
			Text: "@botuser please check this",
		},
	}

	err := ValidateCommentTrigger(cfg, payload)
	if err == nil {
		t.Fatalf("expected error for missing trigger keyword")
	}
}

func TestValidateCommentTrigger_WithWhitespace(t *testing.T) {
	cfg := &config.Config{
		Bitbucket: config.BitbucketConfig{
			User:           "reviewbot",
			TriggerKeyword: "review",
		},
	}

	payload := &WebhookPayload{
		Comment: &Comment{
			Text: "   @reviewbot review this   ",
		},
	}

	err := ValidateCommentTrigger(cfg, payload)
	if err != nil {
		t.Fatalf("expected no error with whitespace, got %v", err)
	}
}

func TestValidateProject_NoRestrictions(t *testing.T) {
	cfg := &config.Config{
		Bitbucket: config.BitbucketConfig{
			AllowedProjectKeys: []string{},
		},
	}

	payload := &WebhookPayload{
		Repository: Repository{
			Project: &Project{
				Key: "ANY",
			},
		},
	}

	err := ValidateProject(cfg, payload)
	if err != nil {
		t.Fatalf("expected no error with no project restrictions, got %v", err)
	}
}

func TestValidateProject_AllowedProject(t *testing.T) {
	cfg := &config.Config{
		Bitbucket: config.BitbucketConfig{
			AllowedProjectKeys: []string{"PROJ1", "PROJ2"},
		},
	}

	payload := &WebhookPayload{
		Repository: Repository{
			Project: &Project{
				Key: "PROJ1",
			},
		},
	}

	err := ValidateProject(cfg, payload)
	if err != nil {
		t.Fatalf("expected no error for allowed project, got %v", err)
	}
}

func TestValidateProject_DisallowedProject(t *testing.T) {
	cfg := &config.Config{
		Bitbucket: config.BitbucketConfig{
			AllowedProjectKeys: []string{"PROJ1", "PROJ2"},
		},
	}

	payload := &WebhookPayload{
		Repository: Repository{
			Project: &Project{
				Key: "PROJ3",
			},
		},
	}

	err := ValidateProject(cfg, payload)
	if err == nil {
		t.Fatalf("expected error for disallowed project")
	}
}

func TestValidateProject_MissingProject(t *testing.T) {
	cfg := &config.Config{
		Bitbucket: config.BitbucketConfig{
			AllowedProjectKeys: []string{"PROJ1"},
		},
	}

	payload := &WebhookPayload{
		Repository: Repository{
			Project: nil,
		},
	}

	err := ValidateProject(cfg, payload)
	if err == nil {
		t.Fatalf("expected error for missing project")
	}
}

func TestParsePayload_Valid(t *testing.T) {
	payloadJSON := `{
		"pullRequest": {
			"id": 1,
			"title": "Test PR",
			"description": "Test description",
			"author": {"user": {"displayName": "User"}},
			"fromRef": {"displayId": "feature/test", "repository": {"slug": "repo", "links": {"clone": []}}},
			"toRef": {"displayId": "main"},
			"links": {"self": [{"href": "http://example.com/pr/1"}]}
		},
		"repository": {"slug": "repo", "links": {"clone": [{"name": "http", "href": "http://example.com/repo.git"}]}},
		"comment": null
	}`

	payload, bytes, err := ParsePayload(io.NopCloser(bytes.NewBufferString(payloadJSON)))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if payload == nil {
		t.Fatalf("expected payload to be non-nil")
	}

	if len(bytes) == 0 {
		t.Fatalf("expected raw bytes to be returned")
	}
}

func TestParsePayload_InvalidJSON(t *testing.T) {
	payloadJSON := `{invalid json`

	_, _, err := ParsePayload(io.NopCloser(bytes.NewBufferString(payloadJSON)))
	if err == nil {
		t.Fatalf("expected error for invalid JSON")
	}
}

func TestParsePayload_MissingTitle(t *testing.T) {
	payloadJSON := `{
		"pullRequest": {
			"id": 1,
			"title": "",
			"author": {"user": {"displayName": "User"}},
			"fromRef": {"displayId": "feature/test", "repository": {"slug": "repo"}},
			"toRef": {"displayId": "main"}
		},
		"repository": {"slug": "repo"}
	}`

	_, _, err := ParsePayload(io.NopCloser(bytes.NewBufferString(payloadJSON)))
	if err == nil {
		t.Fatalf("expected error for missing title")
	}
}

func TestParsePayload_InvalidBranchName(t *testing.T) {
	payloadJSON := `{
		"pullRequest": {
			"id": 1,
			"title": "Test",
			"author": {"user": {"displayName": "User"}},
			"fromRef": {"displayId": "feature/test@invalid", "repository": {"slug": "repo"}},
			"toRef": {"displayId": "main"}
		},
		"repository": {"slug": "repo"}
	}`

	_, _, err := ParsePayload(io.NopCloser(bytes.NewBufferString(payloadJSON)))
	if err == nil {
		t.Fatalf("expected error for invalid branch name")
	}
}

func TestExtractPRData_Success(t *testing.T) {
	payload := &WebhookPayload{
		PullRequest: PullRequest{
			ID:          42,
			Title:       "Test PR",
			Description: "Test description",
			Author: Author{
				User: User{
					DisplayName: "TestUser",
				},
			},
			FromRef: Ref{
				DisplayID: "feature/test",
				Repository: Repository{
					Slug: "test-repo",
				},
			},
			ToRef: Ref{
				DisplayID: "main",
			},
			Links: PRLinks{
				Self: []Link{
					{Href: "http://example.com/pr/42"},
				},
			},
		},
		Repository: Repository{
			Slug: "test-repo",
			Project: &Project{
				Key: "PROJ",
			},
			Links: RepoLinks{
				Clone: []CloneLink{
					{Name: "http", Href: "http://example.com/repo.git"},
				},
			},
		},
	}

	prData, err := ExtractPRData(payload)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if prData.PRID != 42 {
		t.Errorf("expected PR ID 42, got %d", prData.PRID)
	}
	if prData.Title != "Test PR" {
		t.Errorf("expected title 'Test PR', got %s", prData.Title)
	}
	if prData.Repository != "test-repo" {
		t.Errorf("expected repo 'test-repo', got %s", prData.Repository)
	}
	if prData.ProjectKey != "PROJ" {
		t.Errorf("expected project 'PROJ', got %s", prData.ProjectKey)
	}
	if prData.CommentID != 0 {
		t.Errorf("expected comment ID 0, got %d", prData.CommentID)
	}
}

func TestExtractPRData_WithComment(t *testing.T) {
	payload := &WebhookPayload{
		PullRequest: PullRequest{
			ID:    42,
			Title: "Test PR",
			Author: Author{
				User: User{
					DisplayName: "TestUser",
				},
			},
			FromRef: Ref{
				DisplayID: "feature/test",
				Repository: Repository{
					Slug: "test-repo",
				},
			},
			ToRef: Ref{
				DisplayID: "main",
			},
			Links: PRLinks{
				Self: []Link{
					{Href: "http://example.com/pr/42"},
				},
			},
		},
		Repository: Repository{
			Slug: "test-repo",
			Links: RepoLinks{
				Clone: []CloneLink{
					{Name: "http", Href: "http://example.com/repo.git"},
				},
			},
		},
		Comment: &Comment{
			ID: 123,
		},
	}

	prData, err := ExtractPRData(payload)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if prData.CommentID != 123 {
		t.Errorf("expected comment ID 123, got %d", prData.CommentID)
	}
}

func TestExtractPRData_NoHTTPCloneURL(t *testing.T) {
	payload := &WebhookPayload{
		PullRequest: PullRequest{
			ID:    42,
			Title: "Test PR",
			Author: Author{
				User: User{
					DisplayName: "TestUser",
				},
			},
			FromRef: Ref{
				DisplayID: "feature/test",
				Repository: Repository{
					Slug: "test-repo",
				},
			},
			ToRef: Ref{
				DisplayID: "main",
			},
		},
		Repository: Repository{
			Slug: "test-repo",
			Links: RepoLinks{
				Clone: []CloneLink{
					{Name: "ssh", Href: "git@example.com:repo.git"},
				},
			},
		},
	}

	_, err := ExtractPRData(payload)
	if err == nil {
		t.Fatalf("expected error for no HTTP clone URL")
	}
}

func TestExtractProjectKey_Valid(t *testing.T) {
	tests := []struct {
		cloneURL  string
		expected  string
	}{
		{"https://bitbucket.example.com/scm/ci/repo.git", "CI"},
		{"https://bitbucket.example.com/scm/proj/repo.git", "PROJ"},
		{"http://git.example.com/scm/test/my-repo.git", "TEST"},
	}

	for _, tt := range tests {
		t.Run(tt.cloneURL, func(t *testing.T) {
			key := ExtractProjectKey(tt.cloneURL)
			if key != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, key)
			}
		})
	}
}

func TestExtractProjectKey_Invalid(t *testing.T) {
	tests := []struct {
		name     string
		cloneURL string
	}{
		{"No scm path", "https://git.example.com/repo.git"},
		{"Empty URL", ""},
		{"Invalid format", "invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := ExtractProjectKey(tt.cloneURL)
			if key != "" {
				t.Errorf("expected empty string, got %s", key)
			}
		})
	}
}

func TestBranchNameRegex(t *testing.T) {
	validBranches := []string{
		"main",
		"feature/test",
		"bugfix/issue-123",
		"release_v1.0",
		"dev.branch",
		"feature/PROJ-123",
		"release/v1.0-alpha",
	}

	invalidBranches := []string{
		"feature@test",
		"branch name",
		"feature:test",
		"feature|test",
		"feature\\test",
	}

	for _, branch := range validBranches {
		if !branchNameRegex.MatchString(branch) {
			t.Errorf("expected %s to be valid", branch)
		}
	}

	for _, branch := range invalidBranches {
		if branchNameRegex.MatchString(branch) {
			t.Errorf("expected %s to be invalid", branch)
		}
	}
}

func TestValidatePayloadStructure_MissingDisplayName(t *testing.T) {
	payload := &WebhookPayload{
		PullRequest: PullRequest{
			Title: "Test",
			Author: Author{
				User: User{
					DisplayName: "",
				},
			},
			FromRef: Ref{
				DisplayID: "feature/test",
				Repository: Repository{
					Slug: "repo",
				},
			},
			ToRef: Ref{
				DisplayID: "main",
			},
		},
		Repository: Repository{
			Slug: "repo",
		},
	}

	err := validatePayloadStructure(payload)
	if err == nil {
		t.Fatalf("expected error for missing display name")
	}
}

func TestExtractPRData_RepositorySlugFallback(t *testing.T) {
	// Test when repo slug is missing at root but present in fromRef
	payload := &WebhookPayload{
		PullRequest: PullRequest{
			ID:    42,
			Title: "Test PR",
			Author: Author{
				User: User{
					DisplayName: "TestUser",
				},
			},
			FromRef: Ref{
				DisplayID: "feature/test",
				Repository: Repository{
					Slug: "fallback-repo",
					Links: RepoLinks{
						Clone: []CloneLink{
							{Name: "http", Href: "http://example.com/repo.git"},
						},
					},
				},
			},
			ToRef: Ref{
				DisplayID: "main",
			},
			Links: PRLinks{
				Self: []Link{
					{Href: "http://example.com/pr/42"},
				},
			},
		},
		Repository: Repository{
			Slug: "",
			Links: RepoLinks{
				Clone: []CloneLink{},
			},
		},
	}

	prData, err := ExtractPRData(payload)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if prData.Repository != "fallback-repo" {
		t.Errorf("expected repo 'fallback-repo', got %s", prData.Repository)
	}
}
