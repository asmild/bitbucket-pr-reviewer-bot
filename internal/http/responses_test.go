package http

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteErrorResponse_BadRequest(t *testing.T) {
	w := httptest.NewRecorder()

	WriteErrorResponse(w, http.StatusBadRequest, "invalid_input", "The provided input was invalid")

	// Check status code
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	// Check content type
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}

	// Parse response body
	var response map[string]string
	err := json.NewDecoder(w.Body).Decode(&response)
	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Check error fields
	if response["error"] != "invalid_input" {
		t.Errorf("expected error 'invalid_input', got %s", response["error"])
	}

	if response["message"] != "The provided input was invalid" {
		t.Errorf("expected message 'The provided input was invalid', got %s", response["message"])
	}
}

func TestWriteErrorResponse_Unauthorized(t *testing.T) {
	w := httptest.NewRecorder()

	WriteErrorResponse(w, http.StatusUnauthorized, "invalid_signature", "Webhook signature validation failed")

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}

	var response map[string]string
	json.NewDecoder(w.Body).Decode(&response)

	if response["error"] != "invalid_signature" {
		t.Errorf("expected error 'invalid_signature', got %s", response["error"])
	}
}

func TestWriteErrorResponse_Forbidden(t *testing.T) {
	w := httptest.NewRecorder()

	WriteErrorResponse(w, http.StatusForbidden, "project_not_allowed", "This project is not allowed")

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, w.Code)
	}

	var response map[string]string
	json.NewDecoder(w.Body).Decode(&response)

	if response["error"] != "project_not_allowed" {
		t.Errorf("expected error 'project_not_allowed'")
	}
}

func TestWriteErrorResponse_InternalServerError(t *testing.T) {
	w := httptest.NewRecorder()

	WriteErrorResponse(w, http.StatusInternalServerError, "processing_failed", "Failed to process webhook")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	var response map[string]string
	json.NewDecoder(w.Body).Decode(&response)

	if response["error"] != "processing_failed" {
		t.Errorf("expected error 'processing_failed'")
	}
}

func TestWriteErrorResponse_EmptyFields(t *testing.T) {
	w := httptest.NewRecorder()

	// Test with empty error and message
	WriteErrorResponse(w, http.StatusBadRequest, "", "")

	var response map[string]string
	json.NewDecoder(w.Body).Decode(&response)

	if response["error"] != "" {
		t.Errorf("expected empty error field")
	}

	if response["message"] != "" {
		t.Errorf("expected empty message field")
	}
}

func TestWriteErrorResponse_SpecialCharacters(t *testing.T) {
	w := httptest.NewRecorder()

	errorMsg := "error with \"quotes\" and \n newlines"
	message := "message with special chars: @#$%^&*()"

	WriteErrorResponse(w, http.StatusBadRequest, errorMsg, message)

	var response map[string]string
	json.NewDecoder(w.Body).Decode(&response)

	if response["error"] != errorMsg {
		t.Errorf("error field not preserved correctly")
	}

	if response["message"] != message {
		t.Errorf("message field not preserved correctly")
	}
}

func TestWriteErrorResponse_BodyWritten(t *testing.T) {
	w := httptest.NewRecorder()

	WriteErrorResponse(w, http.StatusBadRequest, "error", "message")

	body := w.Body.String()
	if len(body) == 0 {
		t.Errorf("expected response body to be written")
	}

	// Verify it's valid JSON
	var response map[string]string
	err := json.Unmarshal([]byte(body), &response)
	if err != nil {
		t.Errorf("response body is not valid JSON: %v", err)
	}
}

func TestWriteErrorResponse_HeadersSetBeforeStatus(t *testing.T) {
	w := httptest.NewRecorder()

	WriteErrorResponse(w, http.StatusBadRequest, "error", "message")

	// Content-Type header should be set
	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type header not set correctly")
	}
}

func TestWriteSuccessResponse_Success(t *testing.T) {
	w := httptest.NewRecorder()

	WriteSuccessResponse(w, "Fix authentication bug", 1)

	// Check status code
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Check content type
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}

	// Parse response body
	var response map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&response)
	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Check response fields
	if response["message"] != "Webhook received successfully" {
		t.Errorf("expected message 'Webhook received successfully', got %s", response["message"])
	}

	if response["prTitle"] != "Fix authentication bug" {
		t.Errorf("expected prTitle 'Fix authentication bug', got %s", response["prTitle"])
	}

	if response["queuePosition"] != float64(1) {
		t.Errorf("expected queuePosition 1, got %v", response["queuePosition"])
	}
}

func TestWriteSuccessResponse_DifferentQueuePositions(t *testing.T) {
	tests := []struct {
		name          string
		queuePosition int
	}{
		{"First in queue", 1},
		{"Middle of queue", 5},
		{"End of queue", 100},
		{"Large queue", 9999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			WriteSuccessResponse(w, "Test PR", tt.queuePosition)

			var response map[string]interface{}
			json.NewDecoder(w.Body).Decode(&response)

			if response["queuePosition"] != float64(tt.queuePosition) {
				t.Errorf("expected queuePosition %d, got %v", tt.queuePosition, response["queuePosition"])
			}
		})
	}
}

func TestWriteSuccessResponse_DifferentPRTitles(t *testing.T) {
	titles := []string{
		"Simple title",
		"Title with numbers: PROJ-123",
		"Title with special chars: @#$%^&*()",
		"Title with \"quotes\"",
		"Very long title that contains a lot of information about the pull request and what it does",
		"",
	}

	for _, title := range titles {
		t.Run(title, func(t *testing.T) {
			w := httptest.NewRecorder()

			WriteSuccessResponse(w, title, 1)

			var response map[string]interface{}
			json.NewDecoder(w.Body).Decode(&response)

			if response["prTitle"] != title {
				t.Errorf("expected prTitle %q, got %q", title, response["prTitle"])
			}
		})
	}
}

func TestWriteSuccessResponse_BodyWritten(t *testing.T) {
	w := httptest.NewRecorder()

	WriteSuccessResponse(w, "Test PR", 1)

	body := w.Body.String()
	if len(body) == 0 {
		t.Errorf("expected response body to be written")
	}

	// Verify it's valid JSON
	var response map[string]interface{}
	err := json.Unmarshal([]byte(body), &response)
	if err != nil {
		t.Errorf("response body is not valid JSON: %v", err)
	}
}

func TestWriteSuccessResponse_HeadersSetBeforeStatus(t *testing.T) {
	w := httptest.NewRecorder()

	WriteSuccessResponse(w, "Test PR", 1)

	// Content-Type header should be set
	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type header not set correctly")
	}
}

func TestWriteSuccessResponse_ValidJSON(t *testing.T) {
	w := httptest.NewRecorder()

	WriteSuccessResponse(w, "Test PR", 5)

	// Verify the response is valid JSON with correct structure
	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}

	// Verify all required fields are present
	requiredFields := []string{"message", "prTitle", "queuePosition"}
	for _, field := range requiredFields {
		if _, exists := response[field]; !exists {
			t.Errorf("required field %q missing from response", field)
		}
	}
}

func TestWriteSuccessResponse_ResponseStructure(t *testing.T) {
	w := httptest.NewRecorder()

	WriteSuccessResponse(w, "My PR", 3)

	// Read and verify the raw JSON structure
	body, _ := io.ReadAll(w.Body)

	if !bytes.Contains(body, []byte("\"message\":")) {
		t.Errorf("response missing 'message' field")
	}

	if !bytes.Contains(body, []byte("\"prTitle\":")) {
		t.Errorf("response missing 'prTitle' field")
	}

	if !bytes.Contains(body, []byte("\"queuePosition\":")) {
		t.Errorf("response missing 'queuePosition' field")
	}

	if !bytes.Contains(body, []byte("Webhook received successfully")) {
		t.Errorf("response missing expected message text")
	}

	if !bytes.Contains(body, []byte("My PR")) {
		t.Errorf("response missing PR title")
	}

	if !bytes.Contains(body, []byte("3")) {
		t.Errorf("response missing queue position")
	}
}

func TestWriteErrorResponse_AllStatusCodes(t *testing.T) {
	statusCodes := []int{
		http.StatusBadRequest,       // 400
		http.StatusUnauthorized,     // 401
		http.StatusForbidden,        // 403
		http.StatusNotFound,         // 404
		http.StatusConflict,         // 409
		http.StatusInternalServerError, // 500
		http.StatusServiceUnavailable,  // 503
	}

	for _, code := range statusCodes {
		t.Run(http.StatusText(code), func(t *testing.T) {
			w := httptest.NewRecorder()

			WriteErrorResponse(w, code, "error", "message")

			if w.Code != code {
				t.Errorf("expected status %d, got %d", code, w.Code)
			}

			// Should always be able to decode JSON
			var response map[string]string
			if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
				t.Errorf("failed to decode response: %v", err)
			}
		})
	}
}

func TestResponseConsistency_MultipleWrites(t *testing.T) {
	// Test that multiple writes to different response writers work independently
	w1 := httptest.NewRecorder()
	w2 := httptest.NewRecorder()

	WriteSuccessResponse(w1, "PR 1", 1)
	WriteErrorResponse(w2, http.StatusBadRequest, "error", "message")

	// Verify they don't interfere
	var response1 map[string]interface{}
	json.NewDecoder(w1.Body).Decode(&response1)

	if response1["prTitle"] != "PR 1" {
		t.Errorf("first response corrupted")
	}

	var response2 map[string]string
	json.NewDecoder(w2.Body).Decode(&response2)

	if response2["error"] != "error" {
		t.Errorf("second response corrupted")
	}

	if w1.Code != http.StatusOK {
		t.Errorf("first response status changed")
	}

	if w2.Code != http.StatusBadRequest {
		t.Errorf("second response status changed")
	}
}

func TestWriteSuccessResponse_ZeroQueuePosition(t *testing.T) {
	w := httptest.NewRecorder()

	WriteSuccessResponse(w, "Test PR", 0)

	var response map[string]interface{}
	json.NewDecoder(w.Body).Decode(&response)

	if response["queuePosition"] != float64(0) {
		t.Errorf("expected queuePosition 0, got %v", response["queuePosition"])
	}
}

func TestWriteErrorResponse_LongStrings(t *testing.T) {
	w := httptest.NewRecorder()

	longError := "error_" + string(make([]byte, 500))
	longMessage := "message_" + string(make([]byte, 500))

	WriteErrorResponse(w, http.StatusBadRequest, longError, longMessage)

	var response map[string]string
	json.NewDecoder(w.Body).Decode(&response)

	if len(response["error"]) != len(longError) {
		t.Errorf("error field truncated or modified")
	}

	if len(response["message"]) != len(longMessage) {
		t.Errorf("message field truncated or modified")
	}
}

func TestResponseContentType_ConsistentAcrossResponses(t *testing.T) {
	w1 := httptest.NewRecorder()
	w2 := httptest.NewRecorder()

	WriteSuccessResponse(w1, "PR", 1)
	WriteErrorResponse(w2, http.StatusBadRequest, "error", "message")

	if w1.Header().Get("Content-Type") != "application/json" {
		t.Errorf("success response has wrong Content-Type")
	}

	if w2.Header().Get("Content-Type") != "application/json" {
		t.Errorf("error response has wrong Content-Type")
	}
}
