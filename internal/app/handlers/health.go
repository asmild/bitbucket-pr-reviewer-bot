package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/domain/ports"
)

// StartupStatusGetter interface for getting startup status
type StartupStatusGetter interface {
	Get() (state interface{}, step string, errMsg string)
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status         string      `json:"status"`
	State          string      `json:"state"`
	Message        string      `json:"message,omitempty"`
	Queue          QueueStatus `json:"queue"`
	CircuitBreaker string      `json:"circuitBreaker"`
}

// QueueStatus represents queue status
type QueueStatus struct {
	Size   int    `json:"size"`
	Active int    `json:"active"`
	Status string `json:"status"`
}

// HandleHealth returns a health check handler
func HandleHealth(queue ports.ReviewQueue, circuitBreaker ports.CircuitBreaker, startupStatus StartupStatusGetter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get startup status
		appStateRaw, _, errorMessage := startupStatus.Get()
		appState := ""
		if appStateRaw != nil {
			appState = fmt.Sprintf("%v", appStateRaw)
		}

		// Gather health information
		queueSize := queue.GetSize()
		queueRunning := queue.IsRunning()
		activeWorkers := queue.GetActiveWorkers()
		cbState := circuitBreaker.GetState()
		cbOpen := circuitBreaker.IsOpen()

		// Determine overall health based on state
		var healthy bool
		switch appState {
		case "failed":
			healthy = false
		case "running":
			// Only healthy if all components are operational
			healthy = queueRunning && !cbOpen
		case "starting":
			// During startup, we're not yet healthy (but not failing either)
			healthy = true // Return 200 OK during startup
		default:
			healthy = false
		}

		// Determine queue status - only if queue is actually running
		queueStatus := "not_started"
		if queueRunning {
			if activeWorkers > 0 {
				queueStatus = "busy"
			} else {
				queueStatus = "idle"
			}
		}

		// Build response
		response := HealthResponse{
			Status:  getStatus(healthy && appState == "running"), // Only "healthy" when fully running
			State:   appState,
			Message: errorMessage,
			Queue: QueueStatus{
				Size:   queueSize,
				Active: activeWorkers,
				Status: queueStatus,
			},
			CircuitBreaker: string(cbState),
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
