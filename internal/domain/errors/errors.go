package errors

import (
	"errors"
	"fmt"
)

// ErrorCode represents specific error categories for better error handling
type ErrorCode string

const (
	// Infrastructure errors
	ErrorCodeTimeout           ErrorCode = "TIMEOUT"
	ErrorCodeNetworkFailure    ErrorCode = "NETWORK_FAILURE"
	ErrorCodeRateLimitExceeded ErrorCode = "RATE_LIMIT_EXCEEDED"

	// VCS (Version Control System) errors
	ErrorCodeVCSUnauthorized   ErrorCode = "VCS_UNAUTHORIZED"
	ErrorCodeVCSNotFound       ErrorCode = "VCS_NOT_FOUND"
	ErrorCodeVCSInvalidPayload ErrorCode = "VCS_INVALID_PAYLOAD"
	ErrorCodeVCSAPIError       ErrorCode = "VCS_API_ERROR"

	// Git repository errors
	ErrorCodeGitCloneFailed  ErrorCode = "GIT_CLONE_FAILED"
	ErrorCodeGitUpdateFailed ErrorCode = "GIT_UPDATE_FAILED"
	ErrorCodeGitAccessDenied ErrorCode = "GIT_ACCESS_DENIED"

	// AI Reviewer errors
	ErrorCodeReviewerTimeout ErrorCode = "REVIEWER_TIMEOUT"
	ErrorCodeReviewerFailed  ErrorCode = "REVIEWER_FAILED"
	ErrorCodeReviewerInvalid ErrorCode = "REVIEWER_INVALID_RESPONSE"

	// Profile errors
	ErrorCodeProfileNotFound ErrorCode = "PROFILE_NOT_FOUND"
	ErrorCodeProfileInvalid  ErrorCode = "PROFILE_INVALID"

	// Configuration errors
	ErrorCodeConfigInvalid ErrorCode = "CONFIG_INVALID"
	ErrorCodeConfigMissing ErrorCode = "CONFIG_MISSING"

	// Queue errors
	ErrorCodeQueueFull   ErrorCode = "QUEUE_FULL"
	ErrorCodeQueueClosed ErrorCode = "QUEUE_CLOSED"

	// Validation errors
	ErrorCodeValidationFailed ErrorCode = "VALIDATION_FAILED"

	// Circuit breaker errors
	ErrorCodeCircuitOpen ErrorCode = "CIRCUIT_OPEN"

	// Unknown/unexpected errors
	ErrorCodeUnknown ErrorCode = "UNKNOWN"
)

// DomainError represents a structured error with context and metadata
type DomainError struct {
	Code      ErrorCode
	Message   string
	Cause     error
	Retryable bool
	Metadata  map[string]interface{}
}

// Error implements the error interface
func (e *DomainError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap implements error unwrapping for Go 1.13+
func (e *DomainError) Unwrap() error {
	return e.Cause
}

// Is implements error comparison for Go 1.13+
func (e *DomainError) Is(target error) bool {
	t, ok := target.(*DomainError)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

// WithMetadata adds metadata to the error
func (e *DomainError) WithMetadata(key string, value interface{}) *DomainError {
	if e.Metadata == nil {
		e.Metadata = make(map[string]interface{})
	}
	e.Metadata[key] = value
	return e
}

// New creates a new DomainError
func New(code ErrorCode, message string) *DomainError {
	return &DomainError{
		Code:      code,
		Message:   message,
		Retryable: isRetryable(code),
		Metadata:  make(map[string]interface{}),
	}
}

// Wrap wraps an existing error with a DomainError
func Wrap(code ErrorCode, message string, cause error) *DomainError {
	return &DomainError{
		Code:      code,
		Message:   message,
		Cause:     cause,
		Retryable: isRetryable(code),
		Metadata:  make(map[string]interface{}),
	}
}

// isRetryable determines if an error code represents a retryable error
func isRetryable(code ErrorCode) bool {
	switch code {
	case ErrorCodeTimeout,
		ErrorCodeNetworkFailure,
		ErrorCodeRateLimitExceeded,
		ErrorCodeVCSAPIError,
		ErrorCodeGitUpdateFailed,
		ErrorCodeReviewerTimeout:
		return true
	default:
		return false
	}
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		return domainErr.Retryable
	}
	return false
}

// GetCode extracts the error code from an error
func GetCode(err error) ErrorCode {
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		return domainErr.Code
	}
	return ErrorCodeUnknown
}

// GetMetadata extracts metadata from an error
func GetMetadata(err error) map[string]interface{} {
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		return domainErr.Metadata
	}
	return nil
}

// Common pre-defined errors
var (
	ErrCircuitOpen  = New(ErrorCodeCircuitOpen, "circuit breaker is open")
	ErrQueueFull    = New(ErrorCodeQueueFull, "queue is full")
	ErrQueueClosed  = New(ErrorCodeQueueClosed, "queue is closed")
	ErrUnauthorized = New(ErrorCodeVCSUnauthorized, "unauthorized access")
	ErrNotFound     = New(ErrorCodeVCSNotFound, "resource not found")
)
