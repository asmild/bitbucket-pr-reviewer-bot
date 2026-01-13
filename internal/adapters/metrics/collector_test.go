package metrics

import (
	"context"
	"sync"
	"testing"
)

// mockLogger implements ports.Logger for testing
type mockLogger struct{}

func (m *mockLogger) Debug(msg string, keysAndValues ...any) {}
func (m *mockLogger) Info(msg string, keysAndValues ...any)  {}
func (m *mockLogger) Warn(msg string, keysAndValues ...any)  {}
func (m *mockLogger) Error(msg string, keysAndValues ...any) {}
func (m *mockLogger) Fatal(msg string, keysAndValues ...any) {}

// TestCollector tests the collector with persistence
// Single test to avoid Prometheus duplicate registration issues
func TestCollector(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := PersistenceConfig{
		Enabled: true,
		Type:    "filesystem",
		Path:    tmpDir,
	}

	c := NewCollector(&mockLogger{}, cfg)
	defer c.Close()

	ctx := context.Background()

	// Test 1: Empty restore should not error
	if err := c.Restore(ctx); err != nil {
		t.Errorf("Restore from empty should not error: %v", err)
	}

	// Test 2: Add values and verify accumulation
	c.AddCostUSD("PROJ1", "sonnet", 1.50)
	c.AddCostUSD("PROJ1", "sonnet", 2.50) // Should accumulate to 4.00
	c.AddCostUSD("PROJ2", "opus", 10.00)

	c.AddTokensUsed("PROJ1", "sonnet", 1000, 500)
	c.AddTokensUsed("PROJ1", "sonnet", 2000, 1000) // Should accumulate

	c.AddReviewIssues("PROJ1", 1, 2, 3)
	c.AddReviewIssues("PROJ1", 2, 1, 0) // Should accumulate

	// Test 3: Verify internal values
	c.valuesMu.RLock()
	if val := findMetricValue(c.values["cost_usd_total"], map[string]string{"project": "PROJ1", "model": "sonnet"}); val != 4.00 {
		t.Errorf("Expected PROJ1/sonnet cost 4.00, got %v", val)
	}
	if val := findMetricValue(c.values["cost_usd_total"], map[string]string{"project": "PROJ2", "model": "opus"}); val != 10.00 {
		t.Errorf("Expected PROJ2/opus cost 10.00, got %v", val)
	}
	if val := findMetricValue(c.values["tokens_input_total"], map[string]string{"project": "PROJ1", "model": "sonnet"}); val != 3000 {
		t.Errorf("Expected input tokens 3000, got %v", val)
	}
	if val := findMetricValue(c.values["tokens_output_total"], map[string]string{"project": "PROJ1", "model": "sonnet"}); val != 1500 {
		t.Errorf("Expected output tokens 1500, got %v", val)
	}
	if val := findMetricValue(c.values["critical_issues_total"], map[string]string{"project": "PROJ1"}); val != 3 {
		t.Errorf("Expected critical issues 3, got %v", val)
	}
	c.valuesMu.RUnlock()

	// Test 4: Save should succeed
	if err := c.Save(ctx); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Test 5: Concurrent access should be safe
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				c.AddCostUSD("PROJ3", "haiku", 1.0)
			}
		}()
	}
	wg.Wait()

	c.valuesMu.RLock()
	expectedCost := 1000.0 // 10 * 100 * 1.0
	if val := findMetricValue(c.values["cost_usd_total"], map[string]string{"project": "PROJ3", "model": "haiku"}); val != expectedCost {
		t.Errorf("Expected concurrent cost %v, got %v", expectedCost, val)
	}
	c.valuesMu.RUnlock()
}

// findMetricValue finds a metric value by matching labels
func findMetricValue(entries []MetricEntry, labels map[string]string) float64 {
	for _, entry := range entries {
		if labelsEqual(entry.Labels, labels) {
			return entry.Value
		}
	}
	return -1 // Not found
}