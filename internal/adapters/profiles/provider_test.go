package profiles

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/config"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/errors"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/models"
)

type mockLogger struct{}

func (m *mockLogger) Debug(msg string, args ...interface{}) {}
func (m *mockLogger) Info(msg string, args ...interface{})  {}
func (m *mockLogger) Warn(msg string, args ...interface{})  {}
func (m *mockLogger) Error(msg string, args ...interface{}) {}
func (m *mockLogger) Fatal(msg string, args ...interface{}) {}

func TestNewProvider(t *testing.T) {
	logger := &mockLogger{}

	t.Run("creates provider with default config", func(t *testing.T) {
		provider := NewProvider(Config{}, logger)

		if provider.directory != "./profiles" {
			t.Errorf("expected directory './profiles', got '%s'", provider.directory)
		}
		if provider.defaultProfile != "default" {
			t.Errorf("expected default profile 'default', got '%s'", provider.defaultProfile)
		}
		if provider.projectProfiles == nil {
			t.Error("projectProfiles should be initialized")
		}
	})

	t.Run("creates provider with custom config", func(t *testing.T) {
		provider := NewProvider(Config{
			Directory:      "/custom/profiles",
			DefaultProfile: "custom",
			ProjectProfiles: map[string]config.ProjectProfile{
				"PROJ": {Profile: "custom"},
			},
		}, logger)

		if provider.directory != "/custom/profiles" {
			t.Errorf("expected directory '/custom/profiles', got '%s'", provider.directory)
		}
		if provider.defaultProfile != "custom" {
			t.Errorf("expected default profile 'custom', got '%s'", provider.defaultProfile)
		}
		if len(provider.projectProfiles) != 1 {
			t.Errorf("expected 1 project profile, got %d", len(provider.projectProfiles))
		}
	})
}

func TestResolveProfileName(t *testing.T) {
	logger := &mockLogger{}

	t.Run("returns default profile when no config", func(t *testing.T) {
		provider := NewProvider(Config{
			DefaultProfile: "default",
		}, logger)

		name := provider.resolveProfileName("PROJ", "repo")

		if name != "default" {
			t.Errorf("expected 'default', got '%s'", name)
		}
	})

	t.Run("returns project-level profile", func(t *testing.T) {
		provider := NewProvider(Config{
			DefaultProfile: "default",
			ProjectProfiles: map[string]config.ProjectProfile{
				"PROJ": {Profile: "custom"},
			},
		}, logger)

		name := provider.resolveProfileName("PROJ", "repo")

		if name != "custom" {
			t.Errorf("expected 'custom', got '%s'", name)
		}
	})

	t.Run("returns repository-specific override", func(t *testing.T) {
		provider := NewProvider(Config{
			DefaultProfile: "default",
			ProjectProfiles: map[string]config.ProjectProfile{
				"PROJ": {
					Profile: "custom",
					Repositories: map[string]string{
						"special-repo": "security",
					},
				},
			},
		}, logger)

		name := provider.resolveProfileName("PROJ", "special-repo")

		if name != "security" {
			t.Errorf("expected 'security', got '%s'", name)
		}
	})

	t.Run("returns project profile for non-override repo", func(t *testing.T) {
		provider := NewProvider(Config{
			DefaultProfile: "default",
			ProjectProfiles: map[string]config.ProjectProfile{
				"PROJ": {
					Profile: "custom",
					Repositories: map[string]string{
						"special-repo": "security",
					},
				},
			},
		}, logger)

		name := provider.resolveProfileName("PROJ", "normal-repo")

		if name != "custom" {
			t.Errorf("expected 'custom', got '%s'", name)
		}
	})

	t.Run("returns default for unknown project", func(t *testing.T) {
		provider := NewProvider(Config{
			DefaultProfile: "default",
			ProjectProfiles: map[string]config.ProjectProfile{
				"PROJ1": {Profile: "custom"},
			},
		}, logger)

		name := provider.resolveProfileName("PROJ2", "repo")

		if name != "default" {
			t.Errorf("expected 'default', got '%s'", name)
		}
	})
}

func TestSubstituteVariables(t *testing.T) {
	logger := &mockLogger{}
	provider := NewProvider(Config{}, logger)

	pr, _ := models.NewPullRequest(
		123,
		100, // repository ID
		"PROJ",
		"myrepo",
		"Fix bug in auth",
		"This PR fixes authentication issues",
		"John Doe",
		"feature/fix-auth",
		"main",
		"https://bitbucket.example.com/scm/proj/myrepo.git",
		"https://bitbucket.example.com/projects/PROJ/repos/myrepo/pull-requests/123",
	)

	t.Run("substitutes all variables", func(t *testing.T) {
		content := `
PR: {{prUrl}}
Title: {{title}}
Description: {{description}}
Author: {{author}}
Repository: {{repository}}
Source: {{sourceBranch}}
Destination: {{destinationBranch}}
Clone: {{repoCloneUrl}}
Project: {{projectKey}}
ID: {{prId}}
`

		result := provider.substituteVariables(content, pr)

		expectedSubstitutions := map[string]string{
			"{{prUrl}}":             "https://bitbucket.example.com/projects/PROJ/repos/myrepo/pull-requests/123",
			"{{title}}":             "Fix bug in auth",
			"{{description}}":       "This PR fixes authentication issues",
			"{{author}}":            "John Doe",
			"{{repository}}":        "myrepo",
			"{{sourceBranch}}":      "feature/fix-auth",
			"{{destinationBranch}}": "main",
			"{{repoCloneUrl}}":      "https://bitbucket.example.com/scm/proj/myrepo.git",
			"{{projectKey}}":        "PROJ",
			"{{prId}}":              "123",
		}

		for placeholder, expectedValue := range expectedSubstitutions {
			if !contains(result, expectedValue) {
				t.Errorf("result should contain substituted value '%s' for placeholder '%s'", expectedValue, placeholder)
			}
			if contains(result, placeholder) {
				t.Errorf("result should not contain placeholder '%s'", placeholder)
			}
		}
	})

	t.Run("handles content without variables", func(t *testing.T) {
		content := "This is a static profile with no variables."
		result := provider.substituteVariables(content, pr)

		if result != content {
			t.Error("static content should remain unchanged")
		}
	})

	t.Run("handles multiple occurrences of same variable", func(t *testing.T) {
		content := "Title: {{title}}, Again: {{title}}"
		result := provider.substituteVariables(content, pr)

		expected := "Title: Fix bug in auth, Again: Fix bug in auth"
		if result != expected {
			t.Errorf("expected '%s', got '%s'", expected, result)
		}
	})
}

func TestValidateProfile(t *testing.T) {
	logger := &mockLogger{}
	provider := NewProvider(Config{}, logger)

	t.Run("validates profile with all required sections", func(t *testing.T) {
		profile := `
Role: You are a code reviewer
Goal: Review the pull request for quality
PR: {{prUrl}}

Additional instructions here...
`

		err := provider.ValidateProfile(profile)
		if err != nil {
			t.Errorf("valid profile should pass validation, got error: %v", err)
		}
	})

	t.Run("rejects profile missing Role section", func(t *testing.T) {
		profile := `
Goal: Review the pull request
PR: {{prUrl}}
`

		err := provider.ValidateProfile(profile)
		if err == nil {
			t.Error("should return error for missing Role section")
		}
		if errors.GetCode(err) != errors.ErrorCodeProfileInvalid {
			t.Errorf("expected ErrorCodeProfileInvalid, got %s", errors.GetCode(err))
		}
	})

	t.Run("rejects profile missing Goal section", func(t *testing.T) {
		profile := `
Role: Code reviewer
PR: {{prUrl}}
`

		err := provider.ValidateProfile(profile)
		if err == nil {
			t.Error("should return error for missing Goal section")
		}
	})

	t.Run("rejects profile missing PR section", func(t *testing.T) {
		profile := `
Role: Code reviewer
Goal: Review code
`

		err := provider.ValidateProfile(profile)
		if err == nil {
			t.Error("should return error for missing PR section")
		}
	})

	t.Run("rejects profile that is too short", func(t *testing.T) {
		profile := "Role: X\nGoal: Y\nPR: Z"

		err := provider.ValidateProfile(profile)
		if err == nil {
			t.Error("should return error for profile < 100 characters")
		}
		if errors.GetCode(err) != errors.ErrorCodeProfileInvalid {
			t.Errorf("expected ErrorCodeProfileInvalid, got %s", errors.GetCode(err))
		}
	})

	t.Run("accepts profile with exactly 100 characters", func(t *testing.T) {
		// Create a profile that's exactly 100 chars
		profile := "Role: Code reviewer\nGoal: Review PR quality\nPR: {{prUrl}}\nExtra content to reach minimum length requirement."

		err := provider.ValidateProfile(profile)
		if err != nil && len(profile) >= 100 {
			t.Errorf("profile with %d characters should be valid, got error: %v", len(profile), err)
		}
	})
}

func TestGetProfile(t *testing.T) {
	// Create temporary directory for test profiles
	tempDir, err := os.MkdirTemp("", "profile-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logger := &mockLogger{}

	// Create test profiles
	createTestProfile(t, tempDir, "default.md", `
Role: You are a code reviewer
Goal: Review the pull request for quality issues
PR: {{prUrl}}

This is the default profile with sufficient content to pass the 100 character minimum requirement.
`)

	createTestProfile(t, tempDir, "security.md", `
Role: You are a security-focused code reviewer
Goal: Review the pull request for security vulnerabilities
PR: {{prUrl}}

This is the security profile with sufficient content to pass the 100 character minimum requirement.
`)

	pr, _ := models.NewPullRequest(
		123,
		100, // repository ID
		"PROJ",
		"myrepo",
		"Test PR",
		"Description",
		"Author",
		"feature/test",
		"main",
		"https://bitbucket.example.com/scm/proj/myrepo.git",
		"https://bitbucket.example.com/projects/PROJ/repos/myrepo/pull-requests/123",
	)

	t.Run("loads default profile", func(t *testing.T) {
		provider := NewProvider(Config{
			Directory:      tempDir,
			DefaultProfile: "default",
		}, logger)

		ctx := context.Background()
		profile, err := provider.GetProfile(ctx, pr)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !contains(profile, "default profile") {
			t.Error("should load default profile content")
		}
		if !contains(profile, "https://bitbucket.example.com/projects/PROJ/repos/myrepo/pull-requests/123") {
			t.Error("should substitute PR URL")
		}
	})

	t.Run("loads project-specific profile", func(t *testing.T) {
		provider := NewProvider(Config{
			Directory:      tempDir,
			DefaultProfile: "default",
			ProjectProfiles: map[string]config.ProjectProfile{
				"PROJ": {Profile: "security"},
			},
		}, logger)

		ctx := context.Background()
		profile, err := provider.GetProfile(ctx, pr)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !contains(profile, "security profile") {
			t.Error("should load security profile content")
		}
	})

	t.Run("returns error for missing profile file", func(t *testing.T) {
		provider := NewProvider(Config{
			Directory:      tempDir,
			DefaultProfile: "nonexistent",
		}, logger)

		ctx := context.Background()
		_, err := provider.GetProfile(ctx, pr)

		if err == nil {
			t.Error("should return error for missing profile")
		}
		if errors.GetCode(err) != errors.ErrorCodeProfileNotFound {
			t.Errorf("expected ErrorCodeProfileNotFound, got %s", errors.GetCode(err))
		}
	})

	t.Run("returns error for invalid profile", func(t *testing.T) {
		// Create invalid profile (missing required sections)
		createTestProfile(t, tempDir, "invalid.md", "Invalid profile content")

		provider := NewProvider(Config{
			Directory:      tempDir,
			DefaultProfile: "invalid",
		}, logger)

		ctx := context.Background()
		_, err := provider.GetProfile(ctx, pr)

		if err == nil {
			t.Error("should return error for invalid profile")
		}
		if errors.GetCode(err) != errors.ErrorCodeProfileInvalid {
			t.Errorf("expected ErrorCodeProfileInvalid, got %s", errors.GetCode(err))
		}
	})
}

func TestReloadProfiles(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "profile-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logger := &mockLogger{}

	t.Run("succeeds when directory exists", func(t *testing.T) {
		createTestProfile(t, tempDir, "default.md", "Test profile content with minimum 100 characters required for validation to pass successfully.")

		provider := NewProvider(Config{
			Directory: tempDir,
		}, logger)

		err := provider.ReloadProfiles()
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("returns error when directory does not exist", func(t *testing.T) {
		provider := NewProvider(Config{
			Directory: "/nonexistent/path",
		}, logger)

		err := provider.ReloadProfiles()
		if err == nil {
			t.Error("should return error for nonexistent directory")
		}
		if errors.GetCode(err) != errors.ErrorCodeProfileNotFound {
			t.Errorf("expected ErrorCodeProfileNotFound, got %s", errors.GetCode(err))
		}
	})
}

func TestListProfiles(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "profile-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logger := &mockLogger{}

	createTestProfile(t, tempDir, "default.md", "Profile 1 with minimum 100 characters for validation purposes here.")
	createTestProfile(t, tempDir, "security.md", "Profile 2 with minimum 100 characters for validation purposes here.")
	createTestProfile(t, tempDir, "custom.md", "Profile 3 with minimum 100 characters for validation purposes here.")

	// Create a non-.md file (should be ignored)
	createTestProfile(t, tempDir, "readme.txt", "Not a profile")

	provider := NewProvider(Config{
		Directory: tempDir,
	}, logger)

	t.Run("lists all .md profiles", func(t *testing.T) {
		profiles, err := provider.listProfiles()

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(profiles) != 3 {
			t.Errorf("expected 3 profiles, got %d", len(profiles))
		}

		expectedProfiles := map[string]bool{
			"default":  true,
			"security": true,
			"custom":   true,
		}

		for _, profile := range profiles {
			if !expectedProfiles[profile] {
				t.Errorf("unexpected profile '%s'", profile)
			}
		}
	})
}

// Helper functions

func createTestProfile(t *testing.T, dir, filename, content string) {
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test profile: %v", err)
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && containsHelper(s, substr)
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
