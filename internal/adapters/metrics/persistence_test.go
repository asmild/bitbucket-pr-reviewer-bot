package metrics

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewPersister_Disabled(t *testing.T) {
	cfg := PersistenceConfig{
		Enabled: false,
		Type:    "filesystem",
		Path:    "/tmp/test",
	}

	p, err := NewPersister(cfg)
	if err != nil {
		t.Fatalf("NewPersister failed: %v", err)
	}

	if _, ok := p.(*noopPersister); !ok {
		t.Errorf("Expected noopPersister when disabled, got %T", p)
	}

	ctx := context.Background()
	if err := p.Save(ctx, &MetricsSnapshot{}); err != nil {
		t.Errorf("noopPersister.Save failed: %v", err)
	}

	snapshot, err := p.Restore(ctx)
	if err != nil {
		t.Errorf("noopPersister.Restore failed: %v", err)
	}
	if snapshot != nil {
		t.Errorf("noopPersister.Restore should return nil snapshot")
	}

	if err := p.Close(); err != nil {
		t.Errorf("noopPersister.Close failed: %v", err)
	}
}

func TestNewPersister_EmptyConfig(t *testing.T) {
	cfg := PersistenceConfig{}

	p, err := NewPersister(cfg)
	if err != nil {
		t.Fatalf("NewPersister failed: %v", err)
	}

	if _, ok := p.(*noopPersister); !ok {
		t.Errorf("Expected noopPersister for empty config, got %T", p)
	}
}

func TestNewPersister_UnknownType(t *testing.T) {
	cfg := PersistenceConfig{
		Enabled: true,
		Type:    "unknown",
		Path:    "/tmp/test",
	}

	_, err := NewPersister(cfg)
	if err == nil {
		t.Error("Expected error for unknown type")
	}
}

func TestNewPersister_Filesystem(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := PersistenceConfig{
		Enabled: true,
		Type:    "filesystem",
		Path:    tmpDir,
	}

	p, err := NewPersister(cfg)
	if err != nil {
		t.Fatalf("NewPersister failed: %v", err)
	}
	defer p.Close()

	if _, ok := p.(*FilePersister); !ok {
		t.Errorf("Expected FilePersister, got %T", p)
	}
}

func TestNewPersister_SQLite(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := PersistenceConfig{
		Enabled: true,
		Type:    "sqlite",
		Path:    tmpDir,
	}

	p, err := NewPersister(cfg)
	if err != nil {
		t.Fatalf("NewPersister failed: %v", err)
	}
	defer p.Close()

	if _, ok := p.(*SQLitePersister); !ok {
		t.Errorf("Expected SQLitePersister, got %T", p)
	}
}

func TestFilePersister_SaveRestore(t *testing.T) {
	tmpDir := t.TempDir()

	p, err := NewFilePersister(tmpDir)
	if err != nil {
		t.Fatalf("NewFilePersister failed: %v", err)
	}
	defer p.Close()

	ctx := context.Background()

	snapshot := &MetricsSnapshot{
		Timestamp: time.Now(),
		Counters: map[string][]MetricEntry{
			"cost_usd_total": {
				{Labels: map[string]string{"project": "PROJ1", "model": "sonnet"}, Value: 12.50},
				{Labels: map[string]string{"project": "PROJ2", "model": "opus"}, Value: 25.00},
			},
			"tokens_input_total": {
				{Labels: map[string]string{"project": "PROJ1", "model": "sonnet"}, Value: 50000},
			},
		},
	}

	if err := p.Save(ctx, snapshot); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	expectedPath := filepath.Join(tmpDir, "metrics.json")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("Expected file %s to exist", expectedPath)
	}

	restored, err := p.Restore(ctx)
	if err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	if restored == nil {
		t.Fatal("Restored snapshot is nil")
	}

	if len(restored.Counters) != 2 {
		t.Errorf("Expected 2 counter types, got %d", len(restored.Counters))
	}

	costEntries := restored.Counters["cost_usd_total"]
	if len(costEntries) != 2 {
		t.Errorf("Expected 2 cost entries, got %d", len(costEntries))
	}
}

func TestFilePersister_RestoreNoFile(t *testing.T) {
	tmpDir := t.TempDir()

	p, err := NewFilePersister(tmpDir)
	if err != nil {
		t.Fatalf("NewFilePersister failed: %v", err)
	}
	defer p.Close()

	ctx := context.Background()

	snapshot, err := p.Restore(ctx)
	if err != nil {
		t.Errorf("Restore should not error on missing file: %v", err)
	}
	if snapshot != nil {
		t.Errorf("Restore should return nil snapshot for missing file")
	}
}

func TestFilePersister_WithExplicitFilePath(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "custom.json")

	p, err := NewFilePersister(filePath)
	if err != nil {
		t.Fatalf("NewFilePersister failed: %v", err)
	}
	defer p.Close()

	ctx := context.Background()
	snapshot := &MetricsSnapshot{
		Timestamp: time.Now(),
		Counters: map[string][]MetricEntry{
			"test": {{Labels: map[string]string{"key": "value"}, Value: 1.0}},
		},
	}

	if err := p.Save(ctx, snapshot); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Errorf("Expected file %s to exist", filePath)
	}
}

func TestSQLitePersister_SaveRestore(t *testing.T) {
	tmpDir := t.TempDir()

	p, err := NewSQLitePersister(tmpDir)
	if err != nil {
		t.Fatalf("NewSQLitePersister failed: %v", err)
	}
	defer p.Close()

	ctx := context.Background()

	snapshot := &MetricsSnapshot{
		Timestamp: time.Now(),
		Counters: map[string][]MetricEntry{
			"cost_usd_total": {
				{Labels: map[string]string{"project": "PROJ1", "model": "sonnet"}, Value: 12.50},
				{Labels: map[string]string{"project": "PROJ2", "model": "opus"}, Value: 25.00},
			},
			"tokens_input_total": {
				{Labels: map[string]string{"project": "PROJ1", "model": "sonnet"}, Value: 50000},
			},
		},
	}

	if err := p.Save(ctx, snapshot); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	expectedPath := filepath.Join(tmpDir, "metrics.db")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("Expected file %s to exist", expectedPath)
	}

	restored, err := p.Restore(ctx)
	if err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	if restored == nil {
		t.Fatal("Restored snapshot is nil")
	}

	if len(restored.Counters) != 2 {
		t.Errorf("Expected 2 counter types, got %d", len(restored.Counters))
	}
}

func TestSQLitePersister_RestoreEmpty(t *testing.T) {
	tmpDir := t.TempDir()

	p, err := NewSQLitePersister(tmpDir)
	if err != nil {
		t.Fatalf("NewSQLitePersister failed: %v", err)
	}
	defer p.Close()

	ctx := context.Background()

	snapshot, err := p.Restore(ctx)
	if err != nil {
		t.Errorf("Restore should not error on empty database: %v", err)
	}
	if snapshot != nil {
		t.Errorf("Restore should return nil snapshot for empty database")
	}
}

func TestSQLitePersister_UpdateExisting(t *testing.T) {
	tmpDir := t.TempDir()

	p, err := NewSQLitePersister(tmpDir)
	if err != nil {
		t.Fatalf("NewSQLitePersister failed: %v", err)
	}
	defer p.Close()

	ctx := context.Background()

	snapshot1 := &MetricsSnapshot{
		Timestamp: time.Now(),
		Counters: map[string][]MetricEntry{
			"cost_usd_total": {
				{Labels: map[string]string{"project": "PROJ1", "model": "sonnet"}, Value: 10.00},
			},
		},
	}
	if err := p.Save(ctx, snapshot1); err != nil {
		t.Fatalf("First save failed: %v", err)
	}

	snapshot2 := &MetricsSnapshot{
		Timestamp: time.Now(),
		Counters: map[string][]MetricEntry{
			"cost_usd_total": {
				{Labels: map[string]string{"project": "PROJ1", "model": "sonnet"}, Value: 20.00},
			},
		},
	}
	if err := p.Save(ctx, snapshot2); err != nil {
		t.Fatalf("Second save failed: %v", err)
	}

	restored, err := p.Restore(ctx)
	if err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	entries := restored.Counters["cost_usd_total"]
	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}
	if entries[0].Value != 20.00 {
		t.Errorf("Expected updated value 20.00, got %v", entries[0].Value)
	}
}

func TestFilePersister_AtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()

	p, err := NewFilePersister(tmpDir)
	if err != nil {
		t.Fatalf("NewFilePersister failed: %v", err)
	}
	defer p.Close()

	ctx := context.Background()

	snapshot := &MetricsSnapshot{
		Timestamp: time.Now(),
		Counters: map[string][]MetricEntry{
			"test": {{Labels: map[string]string{"key": "value"}, Value: 1.0}},
		},
	}
	if err := p.Save(ctx, snapshot); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	files, _ := os.ReadDir(tmpDir)
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".tmp" {
			t.Errorf("Temp file %s should not exist after save", f.Name())
		}
	}
}