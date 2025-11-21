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

	// TemplatesDirName is the default templates directory name
	TemplatesDirName = "templates"
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

// TemplatesSearchPaths returns the list of paths to search for templates directory
// Priority order (highest to lowest):
// 1. Current directory (./templates)
// 2. User home directory (~/.bb-pr-reviewer/templates)
// 3. System-wide directory (/etc/bb-pr-reviewer/templates)
func TemplatesSearchPaths() []string {
	paths := []string{
		// 1. Current directory
		filepath.Join(".", TemplatesDirName),
	}

	// 2. User home directory
	if homeDir, err := os.UserHomeDir(); err == nil {
		userTemplatesPath := filepath.Join(homeDir, "."+AppName, TemplatesDirName)
		paths = append(paths, userTemplatesPath)
	}

	// 3. System-wide directory (Unix/Linux only)
	systemTemplatesPath := filepath.Join("/etc", AppName, TemplatesDirName)
	paths = append(paths, systemTemplatesPath)

	return paths
}

// FindConfigFile searches for config file in predefined locations
// Returns the first existing config file path or empty string if not found
func FindConfigFile() string {
	// Check if CONFIG_PATH environment variable is set
	if configPath := os.Getenv("CONFIG_PATH"); configPath != "" {
		if _, err := os.Stat(configPath); err == nil {
			return configPath
		}
	}

	// Search in predefined locations
	for _, path := range ConfigSearchPaths() {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// FindTemplatesDirectory searches for templates directory in predefined locations
// Returns the first existing templates directory or empty string if not found
func FindTemplatesDirectory() string {
	// Check if TEMPLATES_DIRECTORY environment variable is set
	if templatesDir := os.Getenv("TEMPLATES_DIRECTORY"); templatesDir != "" {
		if stat, err := os.Stat(templatesDir); err == nil && stat.IsDir() {
			return templatesDir
		}
	}

	// Search in predefined locations
	for _, path := range TemplatesSearchPaths() {
		if stat, err := os.Stat(path); err == nil && stat.IsDir() {
			return path
		}
	}

	return ""
}

// GetConfigDirectory returns the directory containing the config file
func GetConfigDirectory(configPath string) string {
	if configPath == "" {
		return ""
	}
	return filepath.Dir(configPath)
}
