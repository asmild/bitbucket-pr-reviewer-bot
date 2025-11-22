package circuitbreaker

import (
	"context"
	"sync"
	"time"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/errors"
	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/ports"
)

// Breaker implements ports.CircuitBreaker
type Breaker struct {
	mu               sync.Mutex
	state            ports.CircuitState
	failureCount     int
	failureThreshold int
	resetTimeout     time.Duration
	lastFailureTime  time.Time
	onStateChange    func(from, to ports.CircuitState)
}

// Config holds circuit breaker configuration
type Config struct {
	FailureThreshold int
	ResetTimeout     time.Duration
	OnStateChange    func(from, to ports.CircuitState)
}

// NewBreaker creates a new circuit breaker
func NewBreaker(cfg Config) *Breaker {
	if cfg.FailureThreshold == 0 {
		cfg.FailureThreshold = 3
	}
	if cfg.ResetTimeout == 0 {
		cfg.ResetTimeout = 30 * time.Second
	}

	return &Breaker{
		state:            ports.CircuitStateClosed,
		failureCount:     0,
		failureThreshold: cfg.FailureThreshold,
		resetTimeout:     cfg.ResetTimeout,
		onStateChange:    cfg.OnStateChange,
	}
}

// Execute executes a function with circuit breaker protection
func (b *Breaker) Execute(ctx context.Context, fn func() error) error {
	b.mu.Lock()

	// Check if we should transition from Open to HalfOpen
	if b.state == ports.CircuitStateOpen && time.Since(b.lastFailureTime) > b.resetTimeout {
		b.transitionTo(ports.CircuitStateHalfOpen)
		b.failureCount = 0
	}

	// If circuit is open, reject immediately
	if b.state == ports.CircuitStateOpen {
		b.mu.Unlock()
		return errors.ErrCircuitOpen
	}

	b.mu.Unlock()

	// Check context cancellation
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Execute the function
	err := fn()

	b.mu.Lock()
	defer b.mu.Unlock()

	if err != nil {
		b.recordFailure()
	} else {
		b.recordSuccess()
	}

	return err
}

// GetState returns the current state of the circuit breaker
func (b *Breaker) GetState() ports.CircuitState {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

// Reset manually resets the circuit breaker to closed state
func (b *Breaker) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.state != ports.CircuitStateClosed {
		b.transitionTo(ports.CircuitStateClosed)
	}
	b.failureCount = 0
}

// IsOpen returns true if the circuit is open
func (b *Breaker) IsOpen() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state == ports.CircuitStateOpen
}

// recordFailure records a failure and potentially opens the circuit (must be called with lock held)
func (b *Breaker) recordFailure() {
	b.failureCount++
	b.lastFailureTime = time.Now()

	if b.failureCount >= b.failureThreshold && b.state != ports.CircuitStateOpen {
		b.transitionTo(ports.CircuitStateOpen)
	}
}

// recordSuccess records a success and resets the circuit breaker (must be called with lock held)
func (b *Breaker) recordSuccess() {
	if b.state == ports.CircuitStateHalfOpen {
		b.transitionTo(ports.CircuitStateClosed)
	}
	b.failureCount = 0
}

// transitionTo transitions to a new state and calls the callback (must be called with lock held)
func (b *Breaker) transitionTo(newState ports.CircuitState) {
	oldState := b.state
	b.state = newState

	if b.onStateChange != nil && oldState != newState {
		// Call callback without holding lock to prevent deadlock
		go b.onStateChange(oldState, newState)
	}
}

// GetMetrics returns current circuit breaker metrics
func (b *Breaker) GetMetrics() Metrics {
	b.mu.Lock()
	defer b.mu.Unlock()

	return Metrics{
		State:           b.state,
		FailureCount:    b.failureCount,
		LastFailureTime: b.lastFailureTime,
	}
}

// Metrics holds circuit breaker metrics
type Metrics struct {
	State           ports.CircuitState
	FailureCount    int
	LastFailureTime time.Time
}
