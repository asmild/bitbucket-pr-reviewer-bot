package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// FilePersister implements Persister using filesystem (JSON file)
type FilePersister struct {
	path string
	mu   sync.Mutex
}

// NewFilePersister creates a new filesystem-based persister
func NewFilePersister(path string) (*FilePersister, error) {
	resolved, err := resolvePath(path, "metrics.json")
	if err != nil {
		return nil, err
	}
	return &FilePersister{path: resolved}, nil
}

func (p *FilePersister) Save(_ context.Context, snapshot *MetricsSnapshot) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	// Write to temp file first, then rename for atomicity
	tmpPath := p.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tmpPath, p.path); err != nil {
		os.Remove(tmpPath) // Clean up temp file
		return fmt.Errorf("failed to rename file: %w", err)
	}

	return nil
}

func (p *FilePersister) Restore(_ context.Context) (*MetricsSnapshot, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	data, err := os.ReadFile(p.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No previous data
		}
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var snapshot MetricsSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("failed to unmarshal snapshot: %w", err)
	}

	return &snapshot, nil
}

func (p *FilePersister) Close() error {
	return nil
}