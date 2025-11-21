package http

import (
	"encoding/json"
	"net/http"
)

// WriteErrorResponse writes an error response
func WriteErrorResponse(w http.ResponseWriter, statusCode int, err string, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{
		"error":   err,
		"message": message,
	})
}

// WriteSuccessResponse writes a success response
func WriteSuccessResponse(w http.ResponseWriter, prTitle string, queuePosition int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":       "Webhook received successfully",
		"prTitle":       prTitle,
		"queuePosition": queuePosition,
	})
}
