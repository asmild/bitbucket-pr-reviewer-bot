package ports

import (
	"context"
)

// CircuitBreaker defines the interface for circuit breaker pattern
type CircuitBreaker interface {
	// Execute executes a function with circuit breaker protection
	Execute(ctx context.Context, fn func() error) error

	// GetState returns the current state of the circuit breaker
	GetState() CircuitState

	// Reset manually resets the circuit breaker to closed state
	Reset()

	// IsOpen returns true if the circuit is open
	IsOpen() bool
}

// CircuitState represents the state of a circuit breaker
type CircuitState string

const (
	CircuitStateClosed   CircuitState = "closed"
	CircuitStateOpen     CircuitState = "open"
	CircuitStateHalfOpen CircuitState = "half_open"
)
