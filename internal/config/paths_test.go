package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigSearchPaths(t *testing.T) {
	paths := ConfigSearchPaths()

	// Should have at least 2 paths (current dir + system-wide)
	if len(paths) < 2 {
		t.Errorf("Expected at least 2 search paths, got %d", len(paths))
	}

	// First path should be current directory
	expectedFirst := filepath.Join(".", ConfigFileName)
	if paths[0] != expectedFirst {
		t.Errorf("First path should be %s, got %s", expectedFirst, paths[0])
	}

	// Last path should be system-wide
	expectedLast := filepath.Join("/etc", AppName, ConfigFileName)
	if paths[len(paths)-1] != expectedLast {
		t.Errorf("Last path should be %s, got %s", expectedLast, paths[len(paths)-1])
	}

	// Check if home directory path exists (if UserHomeDir succeeds)
	if homeDir, err := os.UserHomeDir(); err == nil {
		expectedHome := filepath.Join(homeDir, "."+AppName, ConfigFileName)
		found := false
		for _, p := range paths {
			if p == expectedHome {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected home directory path %s to be in search paths", expectedHome)
		}
	}
}

func TestProfilesSearchPaths(t *testing.T) {
	paths := ProfilesSearchPaths()

	// Should have at least 2 paths
	if len(paths) < 2 {
		t.Errorf("Expected at least 2 search paths, got %d", len(paths))
	}

	// First path should be current directory
	expectedFirst := filepath.Join(".", ProfilesDirName)
	if paths[0] != expectedFirst {
		t.Errorf("First path should be %s, got %s", expectedFirst, paths[0])
	}

	// Last path should be system-wide
	expectedLast := filepath.Join("/etc", AppName, ProfilesDirName)
	if paths[len(paths)-1] != expectedLast {
		t.Errorf("Last path should be %s, got %s", expectedLast, paths[len(paths)-1])
	}
}

func TestFindConfigFile(t *testing.T) {
	// Save and restore environment
	oldConfigPath := os.Getenv("CONFIG_PATH")
	defer os.Setenv("CONFIG_PATH", oldConfigPath)

	// Create temporary directory structure
	tmpDir := t.TempDir()

	tests := []struct {
		name         string
		setup        func() string
		expectFound  bool
		expectedPath string
	}{
		{
			name: "config in current directory",
			setup: func() string {
				// This test assumes we're running in a directory with templates
				// We'll just check if FindConfigFile works without error
				return ""
			},
			expectFound: false, // May or may not find in actual directory
		},
		{
			name: "config via CONFIG_PATH env var",
			setup: func() string {
				configPath := filepath.Join(tmpDir, "custom-config.yaml")
				// Create the file
				f, _ := os.Create(configPath)
				f.Close()
				os.Setenv("CONFIG_PATH", configPath)
				return configPath
			},
			expectFound:  true,
			expectedPath: filepath.Join(tmpDir, "custom-config.yaml"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			expectedPath := tt.setup()

			// Test
			result := FindConfigFile()

			if tt.expectFound {
				if result == "" {
					t.Error("Expected to find config file but got empty string")
				}
				if expectedPath != "" && result != expectedPath {
					t.Errorf("Expected path %s, got %s", expectedPath, result)
				}
			}

			// Cleanup
			os.Unsetenv("CONFIG_PATH")
		})
	}
}

func TestFindProfilesDirectory(t *testing.T) {
	// Save and restore environment
	oldProfilesDir := os.Getenv("PROFILES_DIRECTORY")
	defer os.Setenv("PROFILES_DIRECTORY", oldProfilesDir)

	// Create temporary directory structure
	tmpDir := t.TempDir()

	tests := []struct {
		name         string
		setup        func() string
		expectFound  bool
		expectedPath string
	}{
		{
			name: "profiles via PROFILES_DIRECTORY env var",
			setup: func() string {
				profilesPath := filepath.Join(tmpDir, "custom-profiles")
				os.MkdirAll(profilesPath, 0755)
				os.Setenv("PROFILES_DIRECTORY", profilesPath)
				return profilesPath
			},
			expectFound:  true,
			expectedPath: filepath.Join(tmpDir, "custom-profiles"),
		},
		{
			name: "non-existent env var path",
			setup: func() string {
				os.Setenv("PROFILES_DIRECTORY", "/nonexistent/path")
				return ""
			},
			expectFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			expectedPath := tt.setup()

			// Test
			result := FindProfilesDirectory()

			if tt.expectFound {
				if result == "" {
					t.Error("Expected to find profiles directory but got empty string")
				}
				if expectedPath != "" && result != expectedPath {
					t.Errorf("Expected path %s, got %s", expectedPath, result)
				}
			}

			// Cleanup
			os.Unsetenv("PROFILES_DIRECTORY")
		})
	}
}

func TestConstants(t *testing.T) {
	if AppName == "" {
		t.Error("AppName should not be empty")
	}

	if ConfigFileName == "" {
		t.Error("ConfigFileName should not be empty")
	}

	if ProfilesDirName == "" {
		t.Error("ProfilesDirName should not be empty")
	}

	// Verify expected values
	expectedAppName := "bb-pr-reviewer"
	if AppName != expectedAppName {
		t.Errorf("Expected AppName to be %s, got %s", expectedAppName, AppName)
	}

	expectedConfigFileName := "config.yaml"
	if ConfigFileName != expectedConfigFileName {
		t.Errorf("Expected ConfigFileName to be %s, got %s", expectedConfigFileName, ConfigFileName)
	}

	expectedProfilesDirName := "profiles"
	if ProfilesDirName != expectedProfilesDirName {
		t.Errorf("Expected ProfilesDirName to be %s, got %s", expectedProfilesDirName, ProfilesDirName)
	}
}
