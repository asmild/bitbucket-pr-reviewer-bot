package metrics

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/config"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/logger"
	_ "github.com/mattn/go-sqlite3"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

type Persistence struct {
	cfg        *config.Config
	mu         sync.Mutex
	db         *sql.DB
	stopCh     chan struct{}
	wg         sync.WaitGroup
}

type MetricsSnapshot struct {
	Counters   map[string]CounterValue   `json:"counters"`
	Histograms map[string]HistogramValue `json:"histograms"`
}

type CounterValue struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels"`
	Value  float64           `json:"value"`
}

type HistogramValue struct {
	Name    string            `json:"name"`
	Labels  map[string]string `json:"labels"`
	Buckets map[string]uint64 `json:"buckets"`
}

var persistenceInstance *Persistence

// InitPersistence initializes metrics persistence
func InitPersistence(cfg *config.Config) error {
	if !cfg.Metrics.Persistence.Enabled {
		logger.Info("Metrics persistence is disabled")
		return nil
	}

	persistenceInstance = &Persistence{
		cfg:    cfg,
		stopCh: make(chan struct{}),
	}

	// Create storage directory
	if err := os.MkdirAll(cfg.Metrics.Persistence.Path, 0755); err != nil {
		return fmt.Errorf("failed to create metrics storage directory: %w", err)
	}

	// Initialize storage backend
	if cfg.Metrics.Persistence.Type == "sqlite" {
		if err := persistenceInstance.initSQLite(); err != nil {
			logger.Warnf("Failed to initialize SQLite, falling back to filesystem: %v", err)
			cfg.Metrics.Persistence.Type = "filesystem"
		}
	}

	// Load existing metrics
	if err := persistenceInstance.load(); err != nil {
		logger.Warnf("Failed to load persisted metrics: %v", err)
	}

	// Start periodic save
	persistenceInstance.wg.Add(1)
	go persistenceInstance.periodicSave()

	logger.Infof("Metrics persistence initialized (type: %s)", cfg.Metrics.Persistence.Type)
	return nil
}

// Shutdown gracefully shuts down persistence
func ShutdownPersistence() {
	if persistenceInstance == nil {
		return
	}

	close(persistenceInstance.stopCh)
	persistenceInstance.wg.Wait()

	// Save one last time
	if err := persistenceInstance.save(); err != nil {
		logger.Errorf("Failed to save metrics on shutdown: %v", err)
	}

	if persistenceInstance.db != nil {
		persistenceInstance.db.Close()
	}

	logger.Info("Metrics persistence shut down")
}

// initSQLite initializes SQLite database
func (p *Persistence) initSQLite() error {
	dbPath := filepath.Join(p.cfg.Metrics.Persistence.Path, "metrics.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}

	// Create tables
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS counter_metrics (
			name TEXT NOT NULL,
			labels TEXT NOT NULL,
			value REAL NOT NULL,
			PRIMARY KEY (name, labels)
		);

		CREATE TABLE IF NOT EXISTS histogram_metrics (
			name TEXT NOT NULL,
			labels TEXT NOT NULL,
			bucket TEXT NOT NULL,
			count INTEGER NOT NULL,
			PRIMARY KEY (name, labels, bucket)
		);
	`)
	if err != nil {
		return err
	}

	p.db = db
	return nil
}

// periodicSave saves metrics periodically
func (p *Persistence) periodicSave() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.cfg.Metrics.Persistence.SaveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := p.save(); err != nil {
				logger.Errorf("Failed to save metrics: %v", err)
			}
		case <-p.stopCh:
			return
		}
	}
}

// save saves current metrics to storage
func (p *Persistence) save() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	snapshot, err := p.collectMetrics()
	if err != nil {
		return err
	}

	if p.cfg.Metrics.Persistence.Type == "sqlite" && p.db != nil {
		return p.saveToSQLite(snapshot)
	}
	return p.saveToFilesystem(snapshot)
}

// load loads metrics from storage
func (p *Persistence) load() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cfg.Metrics.Persistence.Type == "sqlite" && p.db != nil {
		return p.loadFromSQLite()
	}
	return p.loadFromFilesystem()
}

// collectMetrics collects current metrics from Prometheus registry
func (p *Persistence) collectMetrics() (*MetricsSnapshot, error) {
	snapshot := &MetricsSnapshot{
		Counters:   make(map[string]CounterValue),
		Histograms: make(map[string]HistogramValue),
	}

	metricFamilies, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		return nil, err
	}

	for _, mf := range metricFamilies {
		for _, m := range mf.GetMetric() {
			labels := make(map[string]string)
			for _, label := range m.GetLabel() {
				labels[label.GetName()] = label.GetValue()
			}

			key := fmt.Sprintf("%s:%v", mf.GetName(), labels)

			switch mf.GetType() {
			case dto.MetricType_COUNTER:
				snapshot.Counters[key] = CounterValue{
					Name:   mf.GetName(),
					Labels: labels,
					Value:  m.GetCounter().GetValue(),
				}
			case dto.MetricType_HISTOGRAM:
				buckets := make(map[string]uint64)
				for _, bucket := range m.GetHistogram().GetBucket() {
					buckets[fmt.Sprintf("%v", bucket.GetUpperBound())] = bucket.GetCumulativeCount()
				}
				snapshot.Histograms[key] = HistogramValue{
					Name:    mf.GetName(),
					Labels:  labels,
					Buckets: buckets,
				}
			}
		}
	}

	return snapshot, nil
}

// saveToFilesystem saves metrics to JSON file
func (p *Persistence) saveToFilesystem(snapshot *MetricsSnapshot) error {
	filePath := filepath.Join(p.cfg.Metrics.Persistence.Path, "metrics.json")

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0644)
}

// loadFromFilesystem loads metrics from JSON file
func (p *Persistence) loadFromFilesystem() error {
	filePath := filepath.Join(p.cfg.Metrics.Persistence.Path, "metrics.json")

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No existing metrics
		}
		return err
	}

	var snapshot MetricsSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return err
	}

	// Note: In a real implementation, we would restore these values to Prometheus metrics
	// This is simplified for demonstration
	logger.Infof("Loaded %d counters and %d histograms from filesystem", len(snapshot.Counters), len(snapshot.Histograms))
	return nil
}

// saveToSQLite saves metrics to SQLite database
func (p *Persistence) saveToSQLite(snapshot *MetricsSnapshot) error {
	tx, err := p.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Save counters
	for _, counter := range snapshot.Counters {
		labelsJSON, _ := json.Marshal(counter.Labels)
		_, err := tx.Exec(`
			INSERT OR REPLACE INTO counter_metrics (name, labels, value)
			VALUES (?, ?, ?)
		`, counter.Name, string(labelsJSON), counter.Value)
		if err != nil {
			return err
		}
	}

	// Save histograms
	for _, histogram := range snapshot.Histograms {
		labelsJSON, _ := json.Marshal(histogram.Labels)
		for bucket, count := range histogram.Buckets {
			_, err := tx.Exec(`
				INSERT OR REPLACE INTO histogram_metrics (name, labels, bucket, count)
				VALUES (?, ?, ?, ?)
			`, histogram.Name, string(labelsJSON), bucket, count)
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

// loadFromSQLite loads metrics from SQLite database
func (p *Persistence) loadFromSQLite() error {
	// Load counters
	rows, err := p.db.Query(`SELECT name, labels, value FROM counter_metrics`)
	if err != nil {
		return err
	}
	defer rows.Close()

	counterCount := 0
	for rows.Next() {
		var name, labelsJSON string
		var value float64
		if err := rows.Scan(&name, &labelsJSON, &value); err != nil {
			return err
		}
		counterCount++
	}

	// Load histograms
	rows, err = p.db.Query(`SELECT DISTINCT name, labels FROM histogram_metrics`)
	if err != nil {
		return err
	}
	defer rows.Close()

	histogramCount := 0
	for rows.Next() {
		var name, labelsJSON string
		if err := rows.Scan(&name, &labelsJSON); err != nil {
			return err
		}
		histogramCount++
	}

	logger.Infof("Loaded %d counters and %d histograms from SQLite", counterCount, histogramCount)
	return nil
}
