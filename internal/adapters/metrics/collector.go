package metrics

import (
	"context"
	"time"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/ports"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	// MetricsNamespace is the prefix for all Prometheus metrics
	MetricsNamespace = "pr_reviewer"
)

// Collector implements ports.MetricsCollector using Prometheus
type Collector struct {
	webhookReceived      *prometheus.CounterVec
	reviewStarted        *prometheus.CounterVec
	reviewCompleted      *prometheus.CounterVec
	reviewFailed         *prometheus.CounterVec
	uniquePRReviewed     *prometheus.CounterVec
	reviewDuration       *prometheus.HistogramVec
	queueSize            prometheus.Gauge
	gitCloneDuration     prometheus.Histogram
	circuitBreakerState  *prometheus.GaugeVec
	circuitBreakerTrans  *prometheus.CounterVec
	logger               ports.Logger
}

// NewCollector creates a new Prometheus metrics collector
func NewCollector(logger ports.Logger) *Collector {
	return &Collector{
		webhookReceived: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: MetricsNamespace,
				Name:      "webhook_received_total",
				Help:      "Total number of webhooks received by event type",
			},
			[]string{"event_type"},
		),
		reviewStarted: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: MetricsNamespace,
				Name:      "review_started_total",
				Help:      "Total number of reviews started",
			},
			[]string{"project"},
		),
		reviewCompleted: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: MetricsNamespace,
				Name:      "review_completed_total",
				Help:      "Total number of reviews completed",
			},
			[]string{"project", "status"},
		),
		reviewFailed: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: MetricsNamespace,
				Name:      "review_failed_total",
				Help:      "Total number of failed reviews",
			},
			[]string{"project", "error_type"},
		),
		uniquePRReviewed: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: MetricsNamespace,
				Name:      "unique_pr_reviewed_total",
				Help:      "Total number of unique PRs reviewed (each PR counted once regardless of retries)",
			},
			[]string{"project", "repo"},
		),
		reviewDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: MetricsNamespace,
				Name:      "review_duration_seconds",
				Help:      "Duration of reviews in seconds",
				Buckets:   []float64{10, 30, 60, 120, 180, 300, 600},
			},
			[]string{"project"},
		),
		queueSize: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: MetricsNamespace,
				Name:      "queue_size",
				Help:      "Current number of items in the review queue",
			},
		),
		gitCloneDuration: promauto.NewHistogram(
			prometheus.HistogramOpts{
				Namespace: MetricsNamespace,
				Name:      "git_clone_duration_seconds",
				Help:      "Duration of git clone/update operations",
				Buckets:   []float64{1, 5, 10, 30, 60, 120},
			},
		),
		circuitBreakerState: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: MetricsNamespace,
				Name:      "circuit_breaker_state",
				Help:      "Circuit breaker state (0=closed, 1=half-open, 2=open)",
			},
			[]string{"name"},
		),
		circuitBreakerTrans: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: MetricsNamespace,
				Name:      "circuit_breaker_transitions_total",
				Help:      "Total number of circuit breaker state transitions",
			},
			[]string{"from", "to"},
		),
		logger: logger,
	}
}

// IncrementWebhookReceived increments webhook received counter
func (c *Collector) IncrementWebhookReceived(eventType string) {
	c.webhookReceived.WithLabelValues(eventType).Inc()
}

// IncrementReviewStarted increments review started counter
func (c *Collector) IncrementReviewStarted(projectKey string) {
	c.reviewStarted.WithLabelValues(projectKey).Inc()
}

// IncrementReviewCompleted increments review completed counter
func (c *Collector) IncrementReviewCompleted(projectKey, status string) {
	c.reviewCompleted.WithLabelValues(projectKey, status).Inc()
}

// IncrementReviewFailed increments review failed counter
func (c *Collector) IncrementReviewFailed(projectKey, errorType string) {
	c.reviewFailed.WithLabelValues(projectKey, errorType).Inc()
}

// IncrementUniquePRReviewed increments unique PR reviewed counter
func (c *Collector) IncrementUniquePRReviewed(projectKey, repoSlug string, prID int) {
	c.uniquePRReviewed.WithLabelValues(projectKey, repoSlug).Inc()
}

// ObserveReviewDuration observes review duration
func (c *Collector) ObserveReviewDuration(projectKey string, duration time.Duration) {
	c.reviewDuration.WithLabelValues(projectKey).Observe(duration.Seconds())
}

// ObserveQueueSize sets the current queue size
func (c *Collector) ObserveQueueSize(size int) {
	c.queueSize.Set(float64(size))
}

// ObserveGitCloneDuration observes git clone duration
func (c *Collector) ObserveGitCloneDuration(duration time.Duration) {
	c.gitCloneDuration.Observe(duration.Seconds())
}

// RecordCircuitBreakerState records the circuit breaker state
func (c *Collector) RecordCircuitBreakerState(state string) {
	// Map state to numeric value
	var stateValue float64
	switch state {
	case "closed":
		stateValue = 0
	case "half_open":
		stateValue = 1
	case "open":
		stateValue = 2
	default:
		stateValue = -1
	}

	c.circuitBreakerState.WithLabelValues("main").Set(stateValue)
}

// IncrementCircuitBreakerTransition increments circuit breaker transition counter
func (c *Collector) IncrementCircuitBreakerTransition(from, to string) {
	c.circuitBreakerTrans.WithLabelValues(from, to).Inc()
}

// Save saves metrics to persistent storage (not implemented for Prometheus)
func (c *Collector) Save(ctx context.Context) error {
	// Prometheus metrics are stored in memory and scraped by Prometheus server
	// For persistence across restarts, we would need to implement a custom solution
	c.logger.Debug("Metrics save requested (no-op for Prometheus)")
	return nil
}

// Restore restores metrics from persistent storage (not implemented for Prometheus)
func (c *Collector) Restore(ctx context.Context) error {
	// Prometheus metrics cannot be restored from disk in the standard implementation
	// This would require a custom persistence layer
	c.logger.Debug("Metrics restore requested (no-op for Prometheus)")
	return nil
}

// GetRegistry returns the Prometheus registry for HTTP handler
func GetRegistry() *prometheus.Registry {
	return prometheus.DefaultRegisterer.(*prometheus.Registry)
}
