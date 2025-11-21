package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// PRCreatedTotal tracks total PRs created by repository
	PRCreatedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pr_created_total",
			Help: "Total number of pull requests created",
		},
		[]string{"repository"},
	)

	// PRUpdatedTotal tracks total PRs updated by repository
	PRUpdatedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pr_updated_total",
			Help: "Total number of pull requests updated",
		},
		[]string{"repository"},
	)

	// ClaudeLGTMTotal tracks total LGTM reviews by repository
	ClaudeLGTMTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "claude_lgtm_total",
			Help: "Total number of Claude LGTM reviews",
		},
		[]string{"repository"},
	)

	// ClaudeIssuesFoundTotal tracks total issues found by repository
	ClaudeIssuesFoundTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "claude_issues_found_total",
			Help: "Total number of issues found by Claude reviews",
		},
		[]string{"repository"},
	)

	// ClaudeReviewSuccessTotal tracks successful reviews by repository
	ClaudeReviewSuccessTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "claude_review_success_total",
			Help: "Total number of successful Claude reviews",
		},
		[]string{"repository"},
	)

	// ClaudeReviewFailureTotal tracks failed reviews by repository and error type
	ClaudeReviewFailureTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "claude_review_failure_total",
			Help: "Total number of failed Claude reviews",
		},
		[]string{"repository", "error_type"},
	)

	// ClaudeReviewDuration tracks review duration histogram
	ClaudeReviewDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "claude_review_duration_seconds",
			Help:    "Duration of Claude reviews in seconds",
			Buckets: []float64{5, 10, 30, 60, 120, 180, 300},
		},
		[]string{"repository", "status"},
	)
)

// RecordPRCreated records a PR creation event
func RecordPRCreated(repository string) {
	PRCreatedTotal.WithLabelValues(repository).Inc()
}

// RecordPRUpdated records a PR update event
func RecordPRUpdated(repository string) {
	PRUpdatedTotal.WithLabelValues(repository).Inc()
}

// RecordLGTM records a LGTM review
func RecordLGTM(repository string) {
	ClaudeLGTMTotal.WithLabelValues(repository).Inc()
}

// RecordIssuesFound records issues found
func RecordIssuesFound(repository string, count int) {
	ClaudeIssuesFoundTotal.WithLabelValues(repository).Add(float64(count))
}

// RecordReviewSuccess records a successful review
func RecordReviewSuccess(repository string, duration float64) {
	ClaudeReviewSuccessTotal.WithLabelValues(repository).Inc()
	ClaudeReviewDuration.WithLabelValues(repository, "success").Observe(duration)
}

// RecordReviewFailure records a failed review
func RecordReviewFailure(repository string, errorType string, duration float64) {
	ClaudeReviewFailureTotal.WithLabelValues(repository, errorType).Inc()
	ClaudeReviewDuration.WithLabelValues(repository, "failure").Observe(duration)
}
