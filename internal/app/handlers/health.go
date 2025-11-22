package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/ports"
)

// HealthResponse represents the health check response
type HealthResponse struct {
	Status         string                    `json:"status"`
	QueueSize      int                       `json:"queueSize"`
	QueueRunning   bool                      `json:"queueRunning"`
	CircuitBreaker CircuitBreakerStatus      `json:"circuitBreaker"`
	Components     map[string]ComponentStatus `json:"components"`
}

// CircuitBreakerStatus represents circuit breaker status
type CircuitBreakerStatus struct {
	State  string `json:"state"`
	IsOpen bool   `json:"isOpen"`
}

// ComponentStatus represents a component's health status
type ComponentStatus struct {
	Healthy bool   `json:"healthy"`
	Message string `json:"message,omitempty"`
}

// HandleHealth returns a health check handler
func HandleHealth(queue ports.ReviewQueue, circuitBreaker ports.CircuitBreaker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Gather health information
		queueSize := queue.GetSize()
		queueRunning := queue.IsRunning()
		cbState := circuitBreaker.GetState()
		cbOpen := circuitBreaker.IsOpen()

		// Determine overall health
		healthy := queueRunning && !cbOpen

		// Build response
		response := HealthResponse{
			Status:       getStatus(healthy),
			QueueSize:    queueSize,
			QueueRunning: queueRunning,
			CircuitBreaker: CircuitBreakerStatus{
				State:  string(cbState),
				IsOpen: cbOpen,
			},
			Components: map[string]ComponentStatus{
				"queue": {
					Healthy: queueRunning,
					Message: getMessage(queueRunning, "Queue is running", "Queue is not running"),
				},
				"circuit_breaker": {
					Healthy: !cbOpen,
					Message: getMessage(!cbOpen, "Circuit breaker is closed", "Circuit breaker is open"),
				},
			},
		}

		// Set status code based on health
		statusCode := http.StatusOK
		if !healthy {
			statusCode = http.StatusServiceUnavailable
		}

		// Write response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(response)
	}
}

func getStatus(healthy bool) string {
	if healthy {
		return "healthy"
	}
	return "unhealthy"
}

func getMessage(condition bool, trueMsg, falseMsg string) string {
	if condition {
		return trueMsg
	}
	return falseMsg
}
