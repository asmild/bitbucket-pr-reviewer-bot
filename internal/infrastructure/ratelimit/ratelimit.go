package ratelimit

import (
	"context"
	"sync"
	"time"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/errors"
)

// RateLimiter implements a token bucket rate limiter
type RateLimiter struct {
	rate        int           // requests allowed per interval
	interval    time.Duration // interval duration
	capacity    int           // current available requests
	maxCapacity int           // maximum capacity
	lastRefill  time.Time     // last refill time
	mu          sync.Mutex
}

// New creates a new RateLimiter
// rate: number of requests allowed per interval
// interval: time period for the rate (e.g., 1 second, 1 minute)
func New(rate int, interval time.Duration) *RateLimiter {
	return &RateLimiter{
		rate:        rate,
		interval:    interval,
		capacity:    rate,
		maxCapacity: rate,
		lastRefill:  time.Now(),
	}
}

// Allow checks if a request is allowed under the rate limit
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.refill()

	if rl.capacity > 0 {
		rl.capacity--
		return true
	}

	return false
}

// Wait blocks until a request can be allowed under the rate limit
func (rl *RateLimiter) Wait(ctx context.Context) error {
	for {
		if rl.Allow() {
			return nil
		}

		// Calculate wait time until next token is available
		waitTime := rl.getWaitTime()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
			// Continue to next iteration
		}
	}
}

// Execute executes a function with rate limiting
func (rl *RateLimiter) Execute(ctx context.Context, fn func() error) error {
	if err := rl.Wait(ctx); err != nil {
		return err
	}

	return fn()
}

// TryExecute attempts to execute a function if rate limit allows, returns error if limit exceeded
func (rl *RateLimiter) TryExecute(fn func() error) error {
	if !rl.Allow() {
		return errors.New(errors.ErrorCodeRateLimitExceeded, "rate limit exceeded")
	}

	return fn()
}

// refill adds capacity based on time elapsed (must be called with lock held)
func (rl *RateLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill)

	// Calculate how much capacity to add based on elapsed time
	// Use float64 for accurate calculation, then convert to int
	elapsedFraction := float64(elapsed) / float64(rl.interval)
	capacityToAdd := int(elapsedFraction * float64(rl.rate))

	if capacityToAdd > 0 {
		rl.capacity += capacityToAdd
		if rl.capacity > rl.maxCapacity {
			rl.capacity = rl.maxCapacity
		}
		rl.lastRefill = now
	}
}

// getWaitTime calculates how long to wait until next request can be processed
func (rl *RateLimiter) getWaitTime() time.Duration {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if rl.capacity > 0 {
		return 0
	}

	// Calculate time until capacity is available for one more request
	timePerRequest := rl.interval / time.Duration(rl.rate)
	return timePerRequest
}

// GetAvailableCapacity returns the current number of available request slots
func (rl *RateLimiter) GetAvailableCapacity() int {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.refill()
	return rl.capacity
}

// Reset resets the rate limiter to full capacity
func (rl *RateLimiter) Reset() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.capacity = rl.maxCapacity
	rl.lastRefill = time.Now()
}

// PerSecond is a convenience function to create a rate limiter with per-second rate
func PerSecond(rate int) *RateLimiter {
	return New(rate, time.Second)
}

// PerMinute is a convenience function to create a rate limiter with per-minute rate
func PerMinute(rate int) *RateLimiter {
	return New(rate, time.Minute)
}

// PerHour is a convenience function to create a rate limiter with per-hour rate
func PerHour(rate int) *RateLimiter {
	return New(rate, time.Hour)
}
