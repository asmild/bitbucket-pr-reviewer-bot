package metrics

import (
	"context"
	"sync"
	"time"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/ports"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	// MetricsNamespace is the prefix for all Prometheus metrics
	MetricsNamespace = "pr_reviewer"

	// Persisted counter metric names (cumulative counters only, not gauges/histograms)
	metricWebhookReceived  = "webhook_received_total"
	metricReviewStarted    = "review_started_total"
	metricReviewCompleted  = "review_completed_total"
	metricReviewFailed     = "review_failed_total"
	metricUniquePRReviewed = "unique_pr_reviewed_total"
	metricTokensInput      = "tokens_input_total"
	metricTokensOutput     = "tokens_output_total"
	metricCostUSD          = "cost_usd_total"
	metricCriticalIssues   = "critical_issues_total"
	metricWarningIssues    = "warning_issues_total"
	metricSuggestions      = "suggestions_total"
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
	tokensInputTotal     *prometheus.CounterVec
	tokensOutputTotal    *prometheus.CounterVec
	costUSDTotal         *prometheus.CounterVec
	criticalIssuesTotal  *prometheus.CounterVec
	warningIssuesTotal   *prometheus.CounterVec
	suggestionsTotal     *prometheus.CounterVec

	// Persistence
	persister Persister
	values    map[string][]MetricEntry // metric_name -> entries
	valuesMu  sync.RWMutex

	logger ports.Logger
}

// NewCollector creates a new Prometheus metrics collector
func NewCollector(logger ports.Logger, persistCfg PersistenceConfig) *Collector {
	persister, err := NewPersister(persistCfg)
	if err != nil {
		logger.Warn("Failed to create persister, persistence disabled", "error", err)
		persister = &noopPersister{}
	}

	return &Collector{
		persister: persister,
		values:    make(map[string][]MetricEntry),
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
		tokensInputTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: MetricsNamespace,
				Name:      "tokens_input_total",
				Help:      "Total number of input tokens used for Claude reviews",
			},
			[]string{"project", "model"},
		),
		tokensOutputTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: MetricsNamespace,
				Name:      "tokens_output_total",
				Help:      "Total number of output tokens used for Claude reviews",
			},
			[]string{"project", "model"},
		),
		costUSDTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: MetricsNamespace,
				Name:      "cost_usd_total",
				Help:      "Total cost in USD for Claude reviews",
			},
			[]string{"project", "model"},
		),
		criticalIssuesTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: MetricsNamespace,
				Name:      "critical_issues_total",
				Help:      "Total number of critical issues found in reviews",
			},
			[]string{"project"},
		),
		warningIssuesTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: MetricsNamespace,
				Name:      "warning_issues_total",
				Help:      "Total number of warning issues found in reviews",
			},
			[]string{"project"},
		),
		suggestionsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: MetricsNamespace,
				Name:      "suggestions_total",
				Help:      "Total number of suggestions found in reviews",
			},
			[]string{"project"},
		),
		logger: logger,
	}
}

// IncrementWebhookReceived increments webhook received counter
func (c *Collector) IncrementWebhookReceived(eventType string) {
	c.trackValue(metricWebhookReceived, map[string]string{"event_type": eventType}, 1)
	c.webhookReceived.WithLabelValues(eventType).Inc()
}

// IncrementReviewStarted increments review started counter
func (c *Collector) IncrementReviewStarted(projectKey string) {
	c.trackValue(metricReviewStarted, map[string]string{"project": projectKey}, 1)
	c.reviewStarted.WithLabelValues(projectKey).Inc()
}

// IncrementReviewCompleted increments review completed counter
func (c *Collector) IncrementReviewCompleted(projectKey, status string) {
	c.trackValue(metricReviewCompleted, map[string]string{"project": projectKey, "status": status}, 1)
	c.reviewCompleted.WithLabelValues(projectKey, status).Inc()
}

// IncrementReviewFailed increments review failed counter
func (c *Collector) IncrementReviewFailed(projectKey, errorType string) {
	c.trackValue(metricReviewFailed, map[string]string{"project": projectKey, "error_type": errorType}, 1)
	c.reviewFailed.WithLabelValues(projectKey, errorType).Inc()
}

// IncrementUniquePRReviewed increments unique PR reviewed counter
func (c *Collector) IncrementUniquePRReviewed(projectKey, repoSlug string, prID int) {
	c.trackValue(metricUniquePRReviewed, map[string]string{"project": projectKey, "repo": repoSlug}, 1)
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

// AddTokensUsed adds token counts to the metrics
func (c *Collector) AddTokensUsed(projectKey, model string, inputTokens, outputTokens int) {
	labels := map[string]string{"project": projectKey, "model": model}
	c.trackValue(metricTokensInput, labels, float64(inputTokens))
	c.trackValue(metricTokensOutput, labels, float64(outputTokens))
	c.tokensInputTotal.WithLabelValues(projectKey, model).Add(float64(inputTokens))
	c.tokensOutputTotal.WithLabelValues(projectKey, model).Add(float64(outputTokens))
}

// AddCostUSD adds cost in USD to the metrics
func (c *Collector) AddCostUSD(projectKey, model string, costUSD float64) {
	labels := map[string]string{"project": projectKey, "model": model}
	c.trackValue(metricCostUSD, labels, costUSD)
	c.costUSDTotal.WithLabelValues(projectKey, model).Add(costUSD)
}

// AddReviewIssues adds review issue counts to the metrics
func (c *Collector) AddReviewIssues(projectKey string, critical, warning, suggestions int) {
	labels := map[string]string{"project": projectKey}
	c.trackValue(metricCriticalIssues, labels, float64(critical))
	c.trackValue(metricWarningIssues, labels, float64(warning))
	c.trackValue(metricSuggestions, labels, float64(suggestions))
	c.criticalIssuesTotal.WithLabelValues(projectKey).Add(float64(critical))
	c.warningIssuesTotal.WithLabelValues(projectKey).Add(float64(warning))
	c.suggestionsTotal.WithLabelValues(projectKey).Add(float64(suggestions))
}

// labelsEqual checks if two label maps are equal
func labelsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// trackValue tracks a counter value for persistence
func (c *Collector) trackValue(metricName string, labels map[string]string, delta float64) {
	c.valuesMu.Lock()
	defer c.valuesMu.Unlock()

	// Find existing entry with same labels or create new one
	entries := c.values[metricName]
	for i, entry := range entries {
		if labelsEqual(entry.Labels, labels) {
			c.values[metricName][i].Value += delta
			return
		}
	}
	// No existing entry found, append new one
	c.values[metricName] = append(entries, MetricEntry{Labels: labels, Value: delta})
}

// Save saves metrics to persistent storage
func (c *Collector) Save(ctx context.Context) error {
	c.valuesMu.RLock()
	snapshot := &MetricsSnapshot{
		Timestamp: time.Now(),
		Counters:  make(map[string][]MetricEntry),
	}
	for metricName, entries := range c.values {
		snapshot.Counters[metricName] = make([]MetricEntry, len(entries))
		copy(snapshot.Counters[metricName], entries)
	}
	c.valuesMu.RUnlock()

	if err := c.persister.Save(ctx, snapshot); err != nil {
		c.logger.Warn("Failed to save metrics", "error", err)
		return err
	}

	c.logger.Debug("Metrics saved successfully")
	return nil
}

// Restore restores metrics from persistent storage
func (c *Collector) Restore(ctx context.Context) error {
	snapshot, err := c.persister.Restore(ctx)
	if err != nil {
		c.logger.Warn("Failed to restore metrics", "error", err)
		return err
	}

	if snapshot == nil {
		c.logger.Debug("No metrics to restore")
		return nil
	}

	c.valuesMu.Lock()
	defer c.valuesMu.Unlock()

	// Restore values and add to Prometheus counters
	for metricName, entries := range snapshot.Counters {
		c.values[metricName] = make([]MetricEntry, len(entries))
		copy(c.values[metricName], entries)

		for _, entry := range entries {
			c.restoreCounter(metricName, entry)
		}
	}

	c.logger.Info("Metrics restored successfully", "timestamp", snapshot.Timestamp)
	return nil
}

// restoreCounter restores a single counter entry to Prometheus
func (c *Collector) restoreCounter(metricName string, entry MetricEntry) {
	switch metricName {
	case metricWebhookReceived:
		c.webhookReceived.WithLabelValues(entry.Labels["event_type"]).Add(entry.Value)
	case metricReviewStarted:
		c.reviewStarted.WithLabelValues(entry.Labels["project"]).Add(entry.Value)
	case metricReviewCompleted:
		c.reviewCompleted.WithLabelValues(entry.Labels["project"], entry.Labels["status"]).Add(entry.Value)
	case metricReviewFailed:
		c.reviewFailed.WithLabelValues(entry.Labels["project"], entry.Labels["error_type"]).Add(entry.Value)
	case metricUniquePRReviewed:
		c.uniquePRReviewed.WithLabelValues(entry.Labels["project"], entry.Labels["repo"]).Add(entry.Value)
	case metricTokensInput:
		c.tokensInputTotal.WithLabelValues(entry.Labels["project"], entry.Labels["model"]).Add(entry.Value)
	case metricTokensOutput:
		c.tokensOutputTotal.WithLabelValues(entry.Labels["project"], entry.Labels["model"]).Add(entry.Value)
	case metricCostUSD:
		c.costUSDTotal.WithLabelValues(entry.Labels["project"], entry.Labels["model"]).Add(entry.Value)
	case metricCriticalIssues:
		c.criticalIssuesTotal.WithLabelValues(entry.Labels["project"]).Add(entry.Value)
	case metricWarningIssues:
		c.warningIssuesTotal.WithLabelValues(entry.Labels["project"]).Add(entry.Value)
	case metricSuggestions:
		c.suggestionsTotal.WithLabelValues(entry.Labels["project"]).Add(entry.Value)
	}
}

// Close closes the persister
func (c *Collector) Close() error {
	return c.persister.Close()
}

// GetRegistry returns the Prometheus registry for HTTP handler
func GetRegistry() *prometheus.Registry {
	return prometheus.DefaultRegisterer.(*prometheus.Registry)
}
