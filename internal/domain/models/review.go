package models

import (
	"time"
)

// ReviewResult represents the outcome of a PR review
type ReviewResult struct {
	// Review content
	Comment string

	// Metrics extracted from the review
	Metrics ReviewMetrics

	// Execution metadata
	StartTime    time.Time
	EndTime      time.Time
	Duration     time.Duration
	ReviewerType string // e.g., "claude-sonnet-4"
}

// ReviewMetrics contains structured metrics extracted from the review
type ReviewMetrics struct {
	// File statistics
	FilesChanged    int
	LinesAdded      int
	LinesRemoved    int

	// Issue counts
	IssuesFound     int
	CriticalIssues  int
	WarningIssues   int
	SuggestionCount int

	// Quality indicators (extracted from AI response if present)
	SecurityScore   *float64 // 0-10 scale
	CodeQuality     *float64 // 0-10 scale
	TestCoverage    *float64 // percentage
	ComplexityScore *float64 // 0-10 scale
}

// NewReviewResult creates a new ReviewResult
func NewReviewResult(comment, reviewerType string) *ReviewResult {
	now := time.Now()
	return &ReviewResult{
		Comment:      comment,
		StartTime:    now,
		ReviewerType: reviewerType,
		Metrics:      ReviewMetrics{},
	}
}

// Complete marks the review as completed and calculates duration
func (r *ReviewResult) Complete() {
	r.EndTime = time.Now()
	r.Duration = r.EndTime.Sub(r.StartTime)
}

// WithMetrics sets the review metrics
func (r *ReviewResult) WithMetrics(metrics ReviewMetrics) *ReviewResult {
	r.Metrics = metrics
	return r
}

// IsSuccessful checks if the review completed successfully
func (r *ReviewResult) IsSuccessful() bool {
	return r.Comment != "" && !r.EndTime.IsZero()
}

// HasCriticalIssues returns true if critical issues were found
func (r *ReviewResult) HasCriticalIssues() bool {
	return r.Metrics.CriticalIssues > 0
}
