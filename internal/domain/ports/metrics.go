package ports

import (
	"context"
	"time"
)

// MetricsCollector defines the interface for metrics collection
type MetricsCollector interface {
	// Counters
	IncrementWebhookReceived(eventType string)
	IncrementReviewStarted(projectKey string)
	IncrementReviewCompleted(projectKey, status string)
	IncrementReviewFailed(projectKey, errorType string)

	// Histograms
	ObserveReviewDuration(projectKey string, duration time.Duration)
	ObserveQueueSize(size int)
	ObserveGitCloneDuration(duration time.Duration)

	// Circuit breaker metrics
	RecordCircuitBreakerState(state string)
	IncrementCircuitBreakerTransition(from, to string)

	// Save and restore
	Save(ctx context.Context) error
	Restore(ctx context.Context) error
}

// HealthChecker defines the interface for health checking
type HealthChecker interface {
	// Check performs a health check
	Check(ctx context.Context) error

	// GetStatus returns the current health status
	GetStatus() HealthStatus
}

// HealthStatus represents the health status of the application
type HealthStatus struct {
	Healthy    bool
	Components map[string]ComponentHealth
}

// ComponentHealth represents the health of a specific component
type ComponentHealth struct {
	Healthy bool
	Message string
}
