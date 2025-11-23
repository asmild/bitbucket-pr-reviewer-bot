package retry

import (
	"context"
	"errors"
	"testing"
	"time"

	domainErrors "github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/errors"
)

func TestNew(t *testing.T) {
	t.Run("creates retrier with correct config", func(t *testing.T) {
		config := Config{
			MaxAttempts:  5,
			InitialDelay: 2 * time.Second,
			MaxDelay:     60 * time.Second,
			Multiplier:   3.0,
		}

		r := New(config)

		if r.config.MaxAttempts != 5 {
			t.Errorf("expected MaxAttempts 5, got %d", r.config.MaxAttempts)
		}
		if r.config.InitialDelay != 2*time.Second {
			t.Errorf("expected InitialDelay 2s, got %v", r.config.InitialDelay)
		}
		if r.config.MaxDelay != 60*time.Second {
			t.Errorf("expected MaxDelay 60s, got %v", r.config.MaxDelay)
		}
		if r.config.Multiplier != 3.0 {
			t.Errorf("expected Multiplier 3.0, got %f", r.config.Multiplier)
		}
	})
}

func TestDefaultConfig(t *testing.T) {
	t.Run("returns sensible defaults", func(t *testing.T) {
		config := DefaultConfig()

		if config.MaxAttempts != 3 {
			t.Errorf("expected MaxAttempts 3, got %d", config.MaxAttempts)
		}
		if config.InitialDelay != 1*time.Second {
			t.Errorf("expected InitialDelay 1s, got %v", config.InitialDelay)
		}
		if config.MaxDelay != 30*time.Second {
			t.Errorf("expected MaxDelay 30s, got %v", config.MaxDelay)
		}
		if config.Multiplier != 2.0 {
			t.Errorf("expected Multiplier 2.0, got %f", config.Multiplier)
		}
		if len(config.RetryableErrors) == 0 {
			t.Error("expected RetryableErrors to be populated")
		}
	})
}

func TestExecute(t *testing.T) {
	t.Run("succeeds on first attempt", func(t *testing.T) {
		r := New(Config{
			MaxAttempts:  3,
			InitialDelay: 100 * time.Millisecond,
			Multiplier:   2.0,
		})

		attemptCount := 0
		fn := func() error {
			attemptCount++
			return nil
		}

		ctx := context.Background()
		err := r.Execute(ctx, fn)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if attemptCount != 1 {
			t.Errorf("expected 1 attempt, got %d", attemptCount)
		}
	})

	t.Run("retries on retryable error and eventually succeeds", func(t *testing.T) {
		r := New(Config{
			MaxAttempts:  3,
			InitialDelay: 10 * time.Millisecond,
			Multiplier:   2.0,
			RetryableErrors: []domainErrors.ErrorCode{
				domainErrors.ErrorCodeTimeout,
			},
		})

		attemptCount := 0
		fn := func() error {
			attemptCount++
			if attemptCount < 3 {
				return domainErrors.New(domainErrors.ErrorCodeTimeout, "timeout")
			}
			return nil
		}

		ctx := context.Background()
		err := r.Execute(ctx, fn)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if attemptCount != 3 {
			t.Errorf("expected 3 attempts, got %d", attemptCount)
		}
	})

	t.Run("fails after max attempts with retryable error", func(t *testing.T) {
		r := New(Config{
			MaxAttempts:  2,
			InitialDelay: 10 * time.Millisecond,
			Multiplier:   2.0,
			RetryableErrors: []domainErrors.ErrorCode{
				domainErrors.ErrorCodeNetworkFailure,
			},
		})

		attemptCount := 0
		fn := func() error {
			attemptCount++
			return domainErrors.New(domainErrors.ErrorCodeNetworkFailure, "network error")
		}

		ctx := context.Background()
		err := r.Execute(ctx, fn)

		if err == nil {
			t.Error("expected error, got nil")
		}
		if attemptCount != 2 {
			t.Errorf("expected 2 attempts, got %d", attemptCount)
		}
	})

	t.Run("does not retry on non-retryable error", func(t *testing.T) {
		r := New(Config{
			MaxAttempts:  3,
			InitialDelay: 10 * time.Millisecond,
			Multiplier:   2.0,
			RetryableErrors: []domainErrors.ErrorCode{
				domainErrors.ErrorCodeTimeout,
			},
		})

		attemptCount := 0
		fn := func() error {
			attemptCount++
			return domainErrors.New(domainErrors.ErrorCodeVCSNotFound, "not found")
		}

		ctx := context.Background()
		err := r.Execute(ctx, fn)

		if err == nil {
			t.Error("expected error, got nil")
		}
		if attemptCount != 1 {
			t.Errorf("expected 1 attempt (no retries), got %d", attemptCount)
		}
	})

	t.Run("respects already cancelled context", func(t *testing.T) {
		r := New(Config{
			MaxAttempts:  10,
			InitialDelay: 100 * time.Millisecond,
			Multiplier:   2.0,
			RetryableErrors: []domainErrors.ErrorCode{
				domainErrors.ErrorCodeTimeout,
			},
		})

		attemptCount := 0
		fn := func() error {
			attemptCount++
			return domainErrors.New(domainErrors.ErrorCodeTimeout, "timeout")
		}

		// Create already cancelled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := r.Execute(ctx, fn)

		if err == nil {
			t.Error("expected error, got nil")
		}
		// Should detect cancelled context during wait
		if attemptCount < 1 {
			t.Errorf("expected at least 1 attempt, got %d", attemptCount)
		}
	})

	t.Run("handles standard errors", func(t *testing.T) {
		r := New(Config{
			MaxAttempts:  3,
			InitialDelay: 10 * time.Millisecond,
			Multiplier:   2.0,
		})

		attemptCount := 0
		fn := func() error {
			attemptCount++
			return errors.New("standard error")
		}

		ctx := context.Background()
		err := r.Execute(ctx, fn)

		if err == nil {
			t.Error("expected error, got nil")
		}
		// Standard errors are not retryable by default
		if attemptCount != 1 {
			t.Errorf("expected 1 attempt (no retries for standard error), got %d", attemptCount)
		}
	})
}

func TestExecuteWithCallback(t *testing.T) {
	t.Run("calls callback on each retry", func(t *testing.T) {
		r := New(Config{
			MaxAttempts:  3,
			InitialDelay: 10 * time.Millisecond,
			Multiplier:   2.0,
			RetryableErrors: []domainErrors.ErrorCode{
				domainErrors.ErrorCodeTimeout,
			},
		})

		attemptCount := 0
		fn := func() error {
			attemptCount++
			if attemptCount < 3 {
				return domainErrors.New(domainErrors.ErrorCodeTimeout, "timeout")
			}
			return nil
		}

		callbackCount := 0
		var callbackAttempts []int
		var callbackDelays []time.Duration

		onRetry := func(attempt int, err error, delay time.Duration) {
			callbackCount++
			callbackAttempts = append(callbackAttempts, attempt)
			callbackDelays = append(callbackDelays, delay)
		}

		ctx := context.Background()
		err := r.ExecuteWithCallback(ctx, fn, onRetry)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if attemptCount != 3 {
			t.Errorf("expected 3 attempts, got %d", attemptCount)
		}
		if callbackCount != 2 {
			t.Errorf("expected 2 callbacks (for 2 retries), got %d", callbackCount)
		}
		if len(callbackAttempts) != 2 {
			t.Errorf("expected 2 callback attempts recorded, got %d", len(callbackAttempts))
		}
		// First retry is after attempt 1
		if callbackAttempts[0] != 1 {
			t.Errorf("expected first callback attempt 1, got %d", callbackAttempts[0])
		}
		// Second retry is after attempt 2
		if callbackAttempts[1] != 2 {
			t.Errorf("expected second callback attempt 2, got %d", callbackAttempts[1])
		}
		// Verify delays are recorded (exact timing can vary in tests)
		if len(callbackDelays) != 2 {
			t.Errorf("expected 2 callback delays recorded, got %d", len(callbackDelays))
		}
	})

	t.Run("handles nil callback gracefully", func(t *testing.T) {
		r := New(Config{
			MaxAttempts:  2,
			InitialDelay: 10 * time.Millisecond,
			Multiplier:   2.0,
			RetryableErrors: []domainErrors.ErrorCode{
				domainErrors.ErrorCodeTimeout,
			},
		})

		attemptCount := 0
		fn := func() error {
			attemptCount++
			if attemptCount < 2 {
				return domainErrors.New(domainErrors.ErrorCodeTimeout, "timeout")
			}
			return nil
		}

		ctx := context.Background()
		err := r.ExecuteWithCallback(ctx, fn, nil)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if attemptCount != 2 {
			t.Errorf("expected 2 attempts, got %d", attemptCount)
		}
	})
}

func TestCalculateDelay(t *testing.T) {
	t.Run("calculates exponential backoff correctly", func(t *testing.T) {
		r := New(Config{
			InitialDelay: 100 * time.Millisecond,
			MaxDelay:     10 * time.Second,
			Multiplier:   2.0,
		})

		tests := []struct {
			attempt       int
			expectedDelay time.Duration
		}{
			{1, 100 * time.Millisecond},  // 100 * 2^0 = 100ms
			{2, 200 * time.Millisecond},  // 100 * 2^1 = 200ms
			{3, 400 * time.Millisecond},  // 100 * 2^2 = 400ms
			{4, 800 * time.Millisecond},  // 100 * 2^3 = 800ms
			{5, 1600 * time.Millisecond}, // 100 * 2^4 = 1600ms
		}

		for _, tt := range tests {
			delay := r.calculateDelay(tt.attempt)
			if delay != tt.expectedDelay {
				t.Errorf("attempt %d: expected delay %v, got %v",
					tt.attempt, tt.expectedDelay, delay)
			}
		}
	})

	t.Run("caps delay at max delay", func(t *testing.T) {
		r := New(Config{
			InitialDelay: 1 * time.Second,
			MaxDelay:     5 * time.Second,
			Multiplier:   2.0,
		})

		// Attempt 4: 1s * 2^3 = 8s, should be capped at 5s
		delay := r.calculateDelay(4)
		if delay != 5*time.Second {
			t.Errorf("expected delay capped at 5s, got %v", delay)
		}

		// Attempt 10 would be huge, should still be capped at 5s
		delay = r.calculateDelay(10)
		if delay != 5*time.Second {
			t.Errorf("expected delay capped at 5s, got %v", delay)
		}
	})

	t.Run("handles different multipliers", func(t *testing.T) {
		r := New(Config{
			InitialDelay: 100 * time.Millisecond,
			MaxDelay:     10 * time.Second,
			Multiplier:   3.0,
		})

		tests := []struct {
			attempt       int
			expectedDelay time.Duration
		}{
			{1, 100 * time.Millisecond},  // 100 * 3^0 = 100ms
			{2, 300 * time.Millisecond},  // 100 * 3^1 = 300ms
			{3, 900 * time.Millisecond},  // 100 * 3^2 = 900ms
			{4, 2700 * time.Millisecond}, // 100 * 3^3 = 2700ms
		}

		for _, tt := range tests {
			delay := r.calculateDelay(tt.attempt)
			if delay != tt.expectedDelay {
				t.Errorf("attempt %d: expected delay %v, got %v",
					tt.attempt, tt.expectedDelay, delay)
			}
		}
	})
}

func TestShouldRetry(t *testing.T) {
	t.Run("retries error marked as retryable in domain", func(t *testing.T) {
		r := New(Config{
			RetryableErrors: []domainErrors.ErrorCode{},
		})

		// ErrorCodeTimeout is marked as retryable in domain/errors/errors.go
		err := domainErrors.New(domainErrors.ErrorCodeTimeout, "timeout")
		if !r.shouldRetry(err) {
			t.Error("should retry error marked as retryable")
		}
	})

	t.Run("retries error in configured list", func(t *testing.T) {
		r := New(Config{
			RetryableErrors: []domainErrors.ErrorCode{
				domainErrors.ErrorCodeVCSAPIError,
			},
		})

		err := domainErrors.New(domainErrors.ErrorCodeVCSAPIError, "API error")
		if !r.shouldRetry(err) {
			t.Error("should retry error in configured list")
		}
	})

	t.Run("does not retry error not in configured list", func(t *testing.T) {
		r := New(Config{
			RetryableErrors: []domainErrors.ErrorCode{
				domainErrors.ErrorCodeTimeout,
			},
		})

		err := domainErrors.New(domainErrors.ErrorCodeVCSNotFound, "not found")
		if r.shouldRetry(err) {
			t.Error("should not retry error not in configured list")
		}
	})

	t.Run("does not retry standard Go errors", func(t *testing.T) {
		r := New(Config{
			RetryableErrors: []domainErrors.ErrorCode{},
		})

		err := errors.New("standard error")
		if r.shouldRetry(err) {
			t.Error("should not retry standard Go error")
		}
	})

	t.Run("does not retry nil error", func(t *testing.T) {
		r := New(Config{
			RetryableErrors: []domainErrors.ErrorCode{},
		})

		if r.shouldRetry(nil) {
			t.Error("should not retry nil error")
		}
	})
}

func TestConvenienceFunctions(t *testing.T) {
	t.Run("Do uses default config", func(t *testing.T) {
		attemptCount := 0
		fn := func() error {
			attemptCount++
			return nil
		}

		ctx := context.Background()
		err := Do(ctx, fn)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if attemptCount != 1 {
			t.Errorf("expected 1 attempt, got %d", attemptCount)
		}
	})

	t.Run("DoWithConfig uses provided config", func(t *testing.T) {
		config := Config{
			MaxAttempts:  2,
			InitialDelay: 10 * time.Millisecond,
			Multiplier:   2.0,
			RetryableErrors: []domainErrors.ErrorCode{
				domainErrors.ErrorCodeTimeout,
			},
		}

		attemptCount := 0
		fn := func() error {
			attemptCount++
			if attemptCount < 2 {
				return domainErrors.New(domainErrors.ErrorCodeTimeout, "timeout")
			}
			return nil
		}

		ctx := context.Background()
		err := DoWithConfig(ctx, config, fn)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if attemptCount != 2 {
			t.Errorf("expected 2 attempts, got %d", attemptCount)
		}
	})
}

func TestRetryIntegration(t *testing.T) {
	t.Run("realistic retry scenario with exponential backoff", func(t *testing.T) {
		r := New(Config{
			MaxAttempts:  4,
			InitialDelay: 50 * time.Millisecond,
			MaxDelay:     500 * time.Millisecond,
			Multiplier:   2.0,
			RetryableErrors: []domainErrors.ErrorCode{
				domainErrors.ErrorCodeNetworkFailure,
			},
		})

		attemptCount := 0
		var attemptTimes []time.Time

		fn := func() error {
			attemptTimes = append(attemptTimes, time.Now())
			attemptCount++
			if attemptCount < 4 {
				return domainErrors.New(domainErrors.ErrorCodeNetworkFailure, "network failure")
			}
			return nil
		}

		ctx := context.Background()
		start := time.Now()
		err := r.Execute(ctx, fn)
		totalTime := time.Since(start)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if attemptCount != 4 {
			t.Errorf("expected 4 attempts, got %d", attemptCount)
		}

		// Verify delays between attempts
		// Delay 1: 50ms, Delay 2: 100ms, Delay 3: 200ms
		// Total should be at least 350ms
		if totalTime < 350*time.Millisecond {
			t.Errorf("expected total time >= 350ms, got %v", totalTime)
		}

		// Verify increasing delays between attempts
		if len(attemptTimes) == 4 {
			delay1 := attemptTimes[1].Sub(attemptTimes[0])
			delay2 := attemptTimes[2].Sub(attemptTimes[1])
			delay3 := attemptTimes[3].Sub(attemptTimes[2])

			// Each delay should be approximately double the previous (with some tolerance)
			if delay2 < delay1 {
				t.Errorf("delay2 (%v) should be >= delay1 (%v)", delay2, delay1)
			}
			if delay3 < delay2 {
				t.Errorf("delay3 (%v) should be >= delay2 (%v)", delay3, delay2)
			}
		}
	})
}
