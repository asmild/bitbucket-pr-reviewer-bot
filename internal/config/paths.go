package config

import (
	"os"
	"path/filepath"
)

const (
	// AppName is the application name used in paths
	AppName = "bb-pr-reviewer"

	// ConfigFileName is the default configuration file name
	ConfigFileName = "config.yaml"

	// ProfilesDirName is the default profiles directory name
	ProfilesDirName = "profiles"
)

// ConfigSearchPaths returns the list of paths to search for configuration files
// Priority order (highest to lowest):
// 1. Current directory (./config.yaml)
// 2. User home directory (~/.bb-pr-reviewer/config.yaml)
// 3. System-wide directory (/etc/bb-pr-reviewer/config.yaml)
func ConfigSearchPaths() []string {
	paths := []string{
		// 1. Current directory
		filepath.Join(".", ConfigFileName),
	}

	// 2. User home directory
	if homeDir, err := os.UserHomeDir(); err == nil {
		userConfigPath := filepath.Join(homeDir, "."+AppName, ConfigFileName)
		paths = append(paths, userConfigPath)
	}

	// 3. System-wide directory (Unix/Linux only)
	systemConfigPath := filepath.Join("/etc", AppName, ConfigFileName)
	paths = append(paths, systemConfigPath)

	return paths
}

// ProfilesSearchPaths returns the list of paths to search for profiles directory
// Priority order (highest to lowest):
// 1. Current directory (./profiles)
// 2. User home directory (~/.bb-pr-reviewer/profiles)
// 3. System-wide directory (/etc/bb-pr-reviewer/profiles)
func ProfilesSearchPaths() []string {
	paths := []string{
		// 1. Current directory
		filepath.Join(".", ProfilesDirName),
	}

	// 2. User home directory
	if homeDir, err := os.UserHomeDir(); err == nil {
		userProfilesPath := filepath.Join(homeDir, "."+AppName, ProfilesDirName)
		paths = append(paths, userProfilesPath)
	}

	// 3. System-wide directory (Unix/Linux only)
	systemProfilesPath := filepath.Join("/etc", AppName, ProfilesDirName)
	paths = append(paths, systemProfilesPath)

	return paths
}

// FindConfigFile searches for config file in predefined locations
// Returns the first existing config file path or empty string if not found
func FindConfigFile() string {
	// Search in predefined locations
	for _, path := range ConfigSearchPaths() {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// FindProfilesDirectory searches for profiles directory in predefined locations
// Returns the first existing profiles directory or empty string if not found
func FindProfilesDirectory() string {
	// Check if PROFILES_DIRECTORY environment variable is set
	if profilesDir := os.Getenv("PROFILES_DIRECTORY"); profilesDir != "" {
		if stat, err := os.Stat(profilesDir); err == nil && stat.IsDir() {
			return profilesDir
		}
	}

	// Search in predefined locations
	for _, path := range ProfilesSearchPaths() {
		if stat, err := os.Stat(path); err == nil && stat.IsDir() {
			return path
		}
	}

	return ""
}
