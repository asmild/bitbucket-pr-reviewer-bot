package ports

import "context"

// RateLimiter defines the interface for rate limiting
type RateLimiter interface {
	// Allow returns true if a request should be allowed, false if rate limited
	// This is a non-blocking check
	Allow() bool

	// Wait blocks until the request can proceed or context is cancelled
	Wait(ctx context.Context) error

	// Execute runs a function with rate limiting
	Execute(ctx context.Context, fn func() error) error

	// TryExecute attempts to run a function, returns error if rate limited
	TryExecute(fn func() error) error
}
