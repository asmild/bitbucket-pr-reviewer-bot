package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Helper function to check if git is available
func isGitAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// Helper function to create a test git repository
func createTestGitRepo(t *testing.T, path string) {
	t.Helper()

	// Create directory
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = path
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Configure git user for test repo
	exec.Command("git", "-C", path, "config", "user.name", "Test User").Run()
	exec.Command("git", "-C", path, "config", "user.email", "test@example.com").Run()

	// Create initial commit
	testFile := filepath.Join(path, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	exec.Command("git", "-C", path, "add", ".").Run()
	exec.Command("git", "-C", path, "commit", "-m", "Initial commit").Run()
}

func TestValidateBranchName_ValidBranches(t *testing.T) {
	validBranches := []string{
		"main",
		"master",
		"develop",
		"feature/test",
		"feature/PROJ-123",
		"bugfix/issue-456",
		"release/v1.0.0",
		"hotfix/critical",
		"feature_branch",
		"feature.branch",
		"my-feature-branch",
		"feature/test-123",
		"release_v1.0",
		"dev.branch",
		"branch123",
		"UPPERCASE",
		"MixedCase",
		"feature/sub/path",
	}

	for _, branch := range validBranches {
		t.Run(branch, func(t *testing.T) {
			err := ValidateBranchName(branch)
			if err != nil {
				t.Errorf("expected branch %q to be valid, got error: %v", branch, err)
			}
		})
	}
}

func TestValidateBranchName_InvalidBranches(t *testing.T) {
	invalidBranches := []string{
		"feature@test",
		"branch name",      // space
		"feature:test",
		"feature|test",
		"feature\\test",
		"feature#test",
		"branch$name",
		"test&branch",
		"branch*name",
		"test(branch)",
		"branch[name]",
		"test{branch}",
		"branch;name",
		"test'branch",
		"branch\"name",
		"test<branch>",
		"branch?name",
		"test!branch",
		"branch%name",
		"test+branch",
		"branch=name",
		"test~branch",
		"branch`name",
	}

	for _, branch := range invalidBranches {
		t.Run(branch, func(t *testing.T) {
			err := ValidateBranchName(branch)
			if err == nil {
				t.Errorf("expected branch %q to be invalid, but it was accepted", branch)
			}
			if !strings.Contains(err.Error(), "invalid branch name") {
				t.Errorf("expected error to contain 'invalid branch name', got: %v", err)
			}
		})
	}
}

func TestValidateBranchName_EmptyString(t *testing.T) {
	err := ValidateBranchName("")
	if err != nil {
		t.Errorf("expected empty string to be valid (no characters to validate), got error: %v", err)
	}
}

func TestValidateBranchName_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		branch  string
		isValid bool
	}{
		{"Single char", "a", true},
		{"Single number", "1", true},
		{"Single underscore", "_", true},
		{"Single dot", ".", true},
		{"Single slash", "/", true},
		{"Single hyphen", "-", true},
		{"Only numbers", "12345", true},
		{"Only underscores", "___", true},
		{"Only dots", "...", true},
		{"Only slashes", "///", true},
		{"Only hyphens", "---", true},
		{"Mixed valid", "a1_.-/", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBranchName(tt.branch)
			if tt.isValid && err != nil {
				t.Errorf("expected branch %q to be valid, got error: %v", tt.branch, err)
			}
			if !tt.isValid && err == nil {
				t.Errorf("expected branch %q to be invalid", tt.branch)
			}
		})
	}
}

func TestBuildAuthURL_HTTPS(t *testing.T) {
	tests := []struct {
		name     string
		cloneURL string
		username string
		token    string
		expected string
	}{
		{
			name:     "Basic HTTPS URL",
			cloneURL: "https://git.example.com/repo.git",
			username: "user",
			token:    "token123",
			expected: "https://user:token123@git.example.com/repo.git",
		},
		{
			name:     "URL with path",
			cloneURL: "https://git.example.com/scm/project/repo.git",
			username: "myuser",
			token:    "mytoken",
			expected: "https://myuser:mytoken@git.example.com/scm/project/repo.git",
		},
		{
			name:     "URL with port",
			cloneURL: "https://git.example.com:443/repo.git",
			username: "admin",
			token:    "pass",
			expected: "https://admin:pass@git.example.com:443/repo.git",
		},
		{
			name:     "Empty credentials",
			cloneURL: "https://git.example.com/repo.git",
			username: "",
			token:    "",
			expected: "https://:@git.example.com/repo.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildAuthURL(tt.cloneURL, tt.username, tt.token)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestBuildAuthURL_NonHTTPS(t *testing.T) {
	tests := []struct {
		name     string
		cloneURL string
		username string
		token    string
	}{
		{
			name:     "SSH URL",
			cloneURL: "git@github.com:user/repo.git",
			username: "user",
			token:    "token",
		},
		{
			name:     "HTTP URL",
			cloneURL: "http://git.example.com/repo.git",
			username: "user",
			token:    "token",
		},
		{
			name:     "File path",
			cloneURL: "/path/to/repo",
			username: "user",
			token:    "token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildAuthURL(tt.cloneURL, tt.username, tt.token)
			// For non-HTTPS URLs, should return unchanged
			if result != tt.cloneURL {
				t.Errorf("expected URL to remain unchanged for non-HTTPS, got %q", result)
			}
		})
	}
}

func TestBuildAuthURL_SpecialCharacters(t *testing.T) {
	// Test with special characters in credentials
	cloneURL := "https://git.example.com/repo.git"
	username := "user@email.com"
	token := "token!@#$%"

	result := buildAuthURL(cloneURL, username, token)
	expected := "https://user@email.com:token!@#$%@git.example.com/repo.git"

	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestGetProjectPath_Basic(t *testing.T) {
	tests := []struct {
		name       string
		projectKey string
		repository string
		expected   string
	}{
		{
			name:       "Simple case",
			projectKey: "PROJ",
			repository: "repo",
			expected:   filepath.Join(".", "projects", "PROJ", "repo"),
		},
		{
			name:       "With hyphens",
			projectKey: "MY-PROJ",
			repository: "my-repo",
			expected:   filepath.Join(".", "projects", "MY-PROJ", "my-repo"),
		},
		{
			name:       "Uppercase",
			projectKey: "CI",
			repository: "BUILD",
			expected:   filepath.Join(".", "projects", "CI", "BUILD"),
		},
		{
			name:       "With numbers",
			projectKey: "PROJ123",
			repository: "repo456",
			expected:   filepath.Join(".", "projects", "PROJ123", "repo456"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetProjectPath(tt.projectKey, tt.repository)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestGetProjectPath_EmptyValues(t *testing.T) {
	tests := []struct {
		name       string
		projectKey string
		repository string
	}{
		{"Empty project key", "", "repo"},
		{"Empty repository", "PROJ", ""},
		{"Both empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetProjectPath(tt.projectKey, tt.repository)
			// Should still construct path, even if empty
			expected := filepath.Join(".", "projects", tt.projectKey, tt.repository)
			if result != expected {
				t.Errorf("expected %q, got %q", expected, result)
			}
		})
	}
}

func TestGetProjectPath_PathSeparators(t *testing.T) {
	result := GetProjectPath("PROJ", "repo")

	// Should use OS-appropriate path separator
	expectedSep := string(filepath.Separator)
	if !strings.Contains(result, expectedSep) && runtime.GOOS != "windows" {
		t.Errorf("expected path to contain OS separator %q", expectedSep)
	}

	// Should contain projects directory
	if !strings.Contains(result, "projects") {
		t.Errorf("expected path to contain 'projects', got %q", result)
	}

	// Should end with PROJ/repo or PROJ\repo
	expectedEnd := "PROJ" + expectedSep + "repo"
	if !strings.HasSuffix(result, expectedEnd) {
		t.Errorf("expected path to end with %q, got %q", expectedEnd, result)
	}
}

func TestCloneRepository_InvalidPath(t *testing.T) {
	if !isGitAvailable() {
		t.Skip("git not available")
	}

	// Try to clone to an invalid/impossible path
	invalidPath := "/root/impossible/path/repo"
	err := CloneRepository("https://github.com/test/repo.git", invalidPath, "user", "token")

	if err == nil {
		t.Errorf("expected error when cloning to invalid path")
		// Cleanup if somehow succeeded
		os.RemoveAll(invalidPath)
	}

	if err != nil && !strings.Contains(err.Error(), "failed to clone") {
		t.Errorf("expected error to contain 'failed to clone', got: %v", err)
	}
}

func TestCloneRepository_InvalidURL(t *testing.T) {
	if !isGitAvailable() {
		t.Skip("git not available")
	}

	// Create temp directory
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "test-repo")

	err := CloneRepository("invalid-url", repoPath, "user", "token")

	if err == nil {
		t.Errorf("expected error when cloning invalid URL")
	}

	if err != nil && !strings.Contains(err.Error(), "failed to clone") {
		t.Errorf("expected error to contain 'failed to clone', got: %v", err)
	}
}

func TestUpdateRepository_NonExistentPath(t *testing.T) {
	if !isGitAvailable() {
		t.Skip("git not available")
	}

	err := UpdateRepository("/non/existent/path", "main", "user", "token")

	if err == nil {
		t.Errorf("expected error when updating non-existent repository")
	}

	if err != nil && !strings.Contains(err.Error(), "failed to fetch") {
		t.Errorf("expected error to contain 'failed to fetch', got: %v", err)
	}
}

func TestUpdateRepository_NotAGitRepo(t *testing.T) {
	if !isGitAvailable() {
		t.Skip("git not available")
	}

	// Create temp directory that's not a git repo
	tempDir := t.TempDir()

	err := UpdateRepository(tempDir, "main", "user", "token")

	if err == nil {
		t.Errorf("expected error when updating non-git directory")
	}
}

func TestEnsureRepository_NewClone(t *testing.T) {
	if !isGitAvailable() {
		t.Skip("git not available")
	}

	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "new-repo")

	// This will fail because we're using an invalid URL, but tests the code path
	err := EnsureRepository("invalid-url", repoPath, "main", "user", "token")

	// Should try to clone since .git doesn't exist
	if err == nil {
		t.Errorf("expected error when cloning invalid URL")
	}

	// Verify it tried to clone (will fail, but tests the logic)
	if _, statErr := os.Stat(filepath.Join(repoPath, ".git")); !os.IsNotExist(statErr) {
		// If .git exists, cleanup happened incorrectly
		t.Errorf("unexpected .git directory created")
	}
}

func TestEnsureRepository_ExistingRepo(t *testing.T) {
	if !isGitAvailable() {
		t.Skip("git not available")
	}

	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "existing-repo")

	// Create a git repo
	createTestGitRepo(t, repoPath)

	// Verify .git exists
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); os.IsNotExist(err) {
		t.Fatalf(".git directory not created")
	}

	// Try to ensure - should attempt update (will fail with invalid branch)
	err := EnsureRepository("https://github.com/test/repo.git", repoPath, "non-existent-branch", "user", "token")

	// Even if update fails, EnsureRepository continues
	if err != nil {
		t.Logf("Update failed as expected: %v", err)
	}

	// Verify repo still exists
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); os.IsNotExist(err) {
		t.Errorf(".git directory should still exist after failed update")
	}
}

func TestEnsureRepository_UpdateContinuesOnError(t *testing.T) {
	if !isGitAvailable() {
		t.Skip("git not available")
	}

	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "repo-with-update-error")

	// Create a git repo
	createTestGitRepo(t, repoPath)

	// EnsureRepository should not return error even if update fails
	err := EnsureRepository("https://github.com/test/repo.git", repoPath, "invalid-branch", "user", "token")

	// Should not error (continues despite update failure)
	if err != nil {
		t.Errorf("EnsureRepository should not error when update fails, got: %v", err)
	}
}

func TestValidateBranchName_Unicode(t *testing.T) {
	unicodeBranches := []string{
		"branch-中文",
		"feature-日本語",
		"развитие",
		"característica",
		"branch-émoji",
	}

	for _, branch := range unicodeBranches {
		t.Run(branch, func(t *testing.T) {
			err := ValidateBranchName(branch)
			if err == nil {
				t.Errorf("expected unicode branch %q to be invalid", branch)
			}
		})
	}
}

func TestBuildAuthURL_OnlyOneReplacement(t *testing.T) {
	// Ensure only the first https:// is replaced
	cloneURL := "https://git.example.com/repos/https://other.git"
	username := "user"
	token := "token"

	result := buildAuthURL(cloneURL, username, token)
	expected := "https://user:token@git.example.com/repos/https://other.git"

	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}

	// Count occurrences of https://
	count := strings.Count(result, "https://")
	if count != 2 {
		t.Errorf("expected 2 occurrences of https://, got %d", count)
	}
}

func TestGetProjectPath_Consistency(t *testing.T) {
	// Test that calling multiple times returns same result
	result1 := GetProjectPath("PROJ", "repo")
	result2 := GetProjectPath("PROJ", "repo")

	if result1 != result2 {
		t.Errorf("GetProjectPath should be deterministic, got different results: %q vs %q", result1, result2)
	}
}

func TestGetProjectPath_DifferentInputs(t *testing.T) {
	// Test that different inputs produce different paths
	path1 := GetProjectPath("PROJ1", "repo")
	path2 := GetProjectPath("PROJ2", "repo")
	path3 := GetProjectPath("PROJ1", "other-repo")

	if path1 == path2 {
		t.Errorf("expected different paths for different project keys")
	}

	if path1 == path3 {
		t.Errorf("expected different paths for different repositories")
	}
}

func TestValidateBranchName_VeryLongBranch(t *testing.T) {
	// Test with very long valid branch name
	longBranch := strings.Repeat("a", 1000)
	err := ValidateBranchName(longBranch)
	if err != nil {
		t.Errorf("expected very long valid branch to be accepted, got error: %v", err)
	}

	// Test with very long invalid branch name
	longInvalidBranch := strings.Repeat("a", 500) + "@" + strings.Repeat("b", 500)
	err = ValidateBranchName(longInvalidBranch)
	if err == nil {
		t.Errorf("expected very long invalid branch to be rejected")
	}
}
