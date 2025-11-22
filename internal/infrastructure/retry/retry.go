package retry

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/errors"
)

// Config holds retry configuration
type Config struct {
	MaxAttempts     int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	Multiplier      float64
	RetryableErrors []errors.ErrorCode
}

// DefaultConfig returns a default retry configuration
func DefaultConfig() Config {
	return Config{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		RetryableErrors: []errors.ErrorCode{
			errors.ErrorCodeTimeout,
			errors.ErrorCodeNetworkFailure,
			errors.ErrorCodeRateLimitExceeded,
			errors.ErrorCodeVCSAPIError,
			errors.ErrorCodeGitUpdateFailed,
			errors.ErrorCodeReviewerTimeout,
		},
	}
}

// Retrier handles retry logic with exponential backoff
type Retrier struct {
	config Config
}

// New creates a new Retrier
func New(config Config) *Retrier {
	return &Retrier{config: config}
}

// Execute executes a function with retry logic
func (r *Retrier) Execute(ctx context.Context, fn func() error) error {
	var lastErr error

	for attempt := 1; attempt <= r.config.MaxAttempts; attempt++ {
		// Execute the function
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if we should retry
		if !r.shouldRetry(err) {
			return err
		}

		// Check if we've exhausted all attempts
		if attempt >= r.config.MaxAttempts {
			return fmt.Errorf("max retry attempts (%d) exceeded: %w", r.config.MaxAttempts, lastErr)
		}

		// Calculate backoff delay
		delay := r.calculateDelay(attempt)

		// Check context cancellation before waiting
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	return lastErr
}

// ExecuteWithCallback executes a function with retry logic and calls a callback on each attempt
func (r *Retrier) ExecuteWithCallback(
	ctx context.Context,
	fn func() error,
	onRetry func(attempt int, err error, delay time.Duration),
) error {
	var lastErr error

	for attempt := 1; attempt <= r.config.MaxAttempts; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		if !r.shouldRetry(err) {
			return err
		}

		if attempt >= r.config.MaxAttempts {
			return fmt.Errorf("max retry attempts (%d) exceeded: %w", r.config.MaxAttempts, lastErr)
		}

		delay := r.calculateDelay(attempt)

		// Call the callback
		if onRetry != nil {
			onRetry(attempt, err, delay)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}

	return lastErr
}

// shouldRetry determines if an error should trigger a retry
func (r *Retrier) shouldRetry(err error) bool {
	// Use the domain error package to check if retryable
	if errors.IsRetryable(err) {
		return true
	}

	// Additionally check against configured error codes
	errorCode := errors.GetCode(err)
	for _, retryableCode := range r.config.RetryableErrors {
		if errorCode == retryableCode {
			return true
		}
	}

	return false
}

// calculateDelay calculates the delay for a given attempt using exponential backoff
func (r *Retrier) calculateDelay(attempt int) time.Duration {
	// Exponential backoff: initialDelay * multiplier^(attempt-1)
	delay := float64(r.config.InitialDelay) * math.Pow(r.config.Multiplier, float64(attempt-1))

	// Cap at max delay
	if delay > float64(r.config.MaxDelay) {
		delay = float64(r.config.MaxDelay)
	}

	return time.Duration(delay)
}

// Do is a convenience function that creates a retrier and executes a function
func Do(ctx context.Context, fn func() error) error {
	retrier := New(DefaultConfig())
	return retrier.Execute(ctx, fn)
}

// DoWithConfig is a convenience function that executes with custom config
func DoWithConfig(ctx context.Context, config Config, fn func() error) error {
	retrier := New(config)
	return retrier.Execute(ctx, fn)
}
