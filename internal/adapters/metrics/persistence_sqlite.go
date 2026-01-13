package metrics

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// SQLitePersister implements Persister using SQLite
type SQLitePersister struct {
	db *sql.DB
}

// NewSQLitePersister creates a new SQLite-based persister
func NewSQLitePersister(path string) (*SQLitePersister, error) {
	resolved, err := resolvePath(path, "metrics.db")
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", resolved)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite database: %w", err)
	}

	if err := initSQLiteSchema(db); err != nil {
		db.Close()
		return nil, err
	}

	return &SQLitePersister{db: db}, nil
}

func initSQLiteSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS metrics (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		labels_json TEXT NOT NULL,
		value REAL NOT NULL,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(name, labels_json)
	);
	CREATE INDEX IF NOT EXISTS idx_metrics_name ON metrics(name);
	`
	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}
	return nil
}

func (p *SQLitePersister) Save(ctx context.Context, snapshot *MetricsSnapshot) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO metrics (name, labels_json, value, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(name, labels_json) DO UPDATE SET value = ?, updated_at = ?
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for metricName, entries := range snapshot.Counters {
		for _, entry := range entries {
			labelsJSON, err := json.Marshal(entry.Labels)
			if err != nil {
				return fmt.Errorf("failed to marshal labels: %w", err)
			}
			_, err = stmt.ExecContext(ctx, metricName, string(labelsJSON), entry.Value, snapshot.Timestamp, entry.Value, snapshot.Timestamp)
			if err != nil {
				return fmt.Errorf("failed to upsert metric %s: %w", metricName, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (p *SQLitePersister) Restore(ctx context.Context) (*MetricsSnapshot, error) {
	rows, err := p.db.QueryContext(ctx, `SELECT name, labels_json, value, updated_at FROM metrics`)
	if err != nil {
		return nil, fmt.Errorf("failed to query metrics: %w", err)
	}
	defer rows.Close()

	snapshot := &MetricsSnapshot{
		Counters: make(map[string][]MetricEntry),
	}

	var latestTime time.Time
	for rows.Next() {
		var name, labelsJSON string
		var value float64
		var updatedAt time.Time

		if err := rows.Scan(&name, &labelsJSON, &value, &updatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		var labels map[string]string
		if err := json.Unmarshal([]byte(labelsJSON), &labels); err != nil {
			return nil, fmt.Errorf("failed to unmarshal labels: %w", err)
		}

		snapshot.Counters[name] = append(snapshot.Counters[name], MetricEntry{
			Labels: labels,
			Value:  value,
		})

		if updatedAt.After(latestTime) {
			latestTime = updatedAt
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	if len(snapshot.Counters) == 0 {
		return nil, nil // No data
	}

	snapshot.Timestamp = latestTime
	return snapshot, nil
}

func (p *SQLitePersister) Close() error {
	return p.db.Close()
}
