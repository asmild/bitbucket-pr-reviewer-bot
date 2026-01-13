package metrics

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// MetricEntry represents a single metric value with its labels
type MetricEntry struct {
	Labels map[string]string `json:"labels"` // e.g., {"project": "CI", "model": "sonnet"}
	Value  float64           `json:"value"`
}

// MetricsSnapshot represents a snapshot of all counter metrics for persistence
type MetricsSnapshot struct {
	Timestamp time.Time                `json:"timestamp"`
	Counters  map[string][]MetricEntry `json:"counters"` // metric_name -> entries
}

// Persister defines the interface for metrics persistence
type Persister interface {
	Save(ctx context.Context, snapshot *MetricsSnapshot) error
	Restore(ctx context.Context) (*MetricsSnapshot, error)
	Close() error
}

// PersistenceConfig holds configuration for metrics persistence
type PersistenceConfig struct {
	Enabled      bool
	Type         string // "filesystem" or "sqlite"
	Path         string
	SaveInterval time.Duration
}

// NewPersister creates a new persister based on configuration
func NewPersister(cfg PersistenceConfig) (Persister, error) {
	if !cfg.Enabled {
		return &noopPersister{}, nil
	}

	switch cfg.Type {
	case "filesystem":
		return NewFilePersister(cfg.Path)
	case "sqlite":
		return NewSQLitePersister(cfg.Path)
	default:
		return nil, fmt.Errorf("unknown persistence type: %s", cfg.Type)
	}
}

// noopPersister is a no-op implementation when persistence is disabled
type noopPersister struct{}

func (p *noopPersister) Save(ctx context.Context, snapshot *MetricsSnapshot) error {
	return nil
}

func (p *noopPersister) Restore(ctx context.Context) (*MetricsSnapshot, error) {
	return nil, nil
}

func (p *noopPersister) Close() error {
	return nil
}

// resolvePath ensures the path is a file path, creating directories as needed.
// If path is or looks like a directory, appends defaultFilename.
func resolvePath(path, defaultFilename string) (string, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	info, err := os.Stat(path)
	if err == nil && info.IsDir() {
		return filepath.Join(path, defaultFilename), nil
	}
	if os.IsNotExist(err) && filepath.Ext(path) == "" {
		if err := os.MkdirAll(path, 0755); err != nil {
			return "", fmt.Errorf("failed to create directory: %w", err)
		}
		return filepath.Join(path, defaultFilename), nil
	}

	return path, nil
}