package errors

import (
	"errors"
	"testing"
)

func TestNew(t *testing.T) {
	t.Run("creates error with correct code and message", func(t *testing.T) {
		err := New(ErrorCodeTimeout, "operation timed out")

		if err.Code != ErrorCodeTimeout {
			t.Errorf("expected code %s, got %s", ErrorCodeTimeout, err.Code)
		}
		if err.Message != "operation timed out" {
			t.Errorf("expected message 'operation timed out', got '%s'", err.Message)
		}
		if err.Cause != nil {
			t.Errorf("expected nil cause, got %v", err.Cause)
		}
	})

	t.Run("sets retryable flag based on error code", func(t *testing.T) {
		retryableErr := New(ErrorCodeTimeout, "timeout")
		if !retryableErr.Retryable {
			t.Error("ErrorCodeTimeout should be retryable")
		}

		nonRetryableErr := New(ErrorCodeVCSNotFound, "not found")
		if nonRetryableErr.Retryable {
			t.Error("ErrorCodeVCSNotFound should not be retryable")
		}
	})

	t.Run("initializes empty metadata map", func(t *testing.T) {
		err := New(ErrorCodeTimeout, "timeout")

		if err.Metadata == nil {
			t.Error("Metadata should be initialized")
		}
		if len(err.Metadata) != 0 {
			t.Errorf("expected empty metadata, got %d items", len(err.Metadata))
		}
	})
}

func TestWrap(t *testing.T) {
	t.Run("wraps error with code and message", func(t *testing.T) {
		cause := errors.New("underlying error")
		err := Wrap(ErrorCodeNetworkFailure, "network request failed", cause)

		if err.Code != ErrorCodeNetworkFailure {
			t.Errorf("expected code %s, got %s", ErrorCodeNetworkFailure, err.Code)
		}
		if err.Message != "network request failed" {
			t.Errorf("expected message 'network request failed', got '%s'", err.Message)
		}
		if err.Cause != cause {
			t.Errorf("expected cause to be preserved")
		}
	})

	t.Run("preserves cause in error chain", func(t *testing.T) {
		cause := errors.New("root cause")
		err := Wrap(ErrorCodeVCSAPIError, "API error", cause)

		unwrapped := errors.Unwrap(err)
		if unwrapped != cause {
			t.Error("Unwrap should return the cause")
		}
	})
}

func TestError(t *testing.T) {
	t.Run("formats error without cause", func(t *testing.T) {
		err := New(ErrorCodeTimeout, "operation timed out")
		expected := "[TIMEOUT] operation timed out"

		if err.Error() != expected {
			t.Errorf("expected '%s', got '%s'", expected, err.Error())
		}
	})

	t.Run("formats error with cause", func(t *testing.T) {
		cause := errors.New("connection refused")
		err := Wrap(ErrorCodeNetworkFailure, "network error", cause)
		expected := "[NETWORK_FAILURE] network error: connection refused"

		if err.Error() != expected {
			t.Errorf("expected '%s', got '%s'", expected, err.Error())
		}
	})
}

func TestUnwrap(t *testing.T) {
	t.Run("returns cause when present", func(t *testing.T) {
		cause := errors.New("root cause")
		err := Wrap(ErrorCodeVCSAPIError, "API error", cause)

		unwrapped := err.Unwrap()
		if unwrapped != cause {
			t.Error("Unwrap should return the cause")
		}
	})

	t.Run("returns nil when no cause", func(t *testing.T) {
		err := New(ErrorCodeTimeout, "timeout")

		unwrapped := err.Unwrap()
		if unwrapped != nil {
			t.Errorf("expected nil, got %v", unwrapped)
		}
	})
}

func TestIs(t *testing.T) {
	t.Run("matches errors with same code", func(t *testing.T) {
		err1 := New(ErrorCodeTimeout, "timeout 1")
		err2 := New(ErrorCodeTimeout, "timeout 2")

		if !errors.Is(err1, err2) {
			t.Error("errors with same code should match")
		}
	})

	t.Run("does not match errors with different codes", func(t *testing.T) {
		err1 := New(ErrorCodeTimeout, "timeout")
		err2 := New(ErrorCodeNetworkFailure, "network failure")

		if errors.Is(err1, err2) {
			t.Error("errors with different codes should not match")
		}
	})

	t.Run("does not match non-DomainError", func(t *testing.T) {
		err1 := New(ErrorCodeTimeout, "timeout")
		err2 := errors.New("standard error")

		if errors.Is(err1, err2) {
			t.Error("DomainError should not match standard error")
		}
	})

	t.Run("works with wrapped errors", func(t *testing.T) {
		target := New(ErrorCodeVCSNotFound, "not found")
		wrapped := Wrap(ErrorCodeVCSAPIError, "API error", target)

		if !errors.Is(wrapped, target) {
			t.Error("should match wrapped error")
		}
	})
}

func TestWithMetadata(t *testing.T) {
	t.Run("adds metadata to error", func(t *testing.T) {
		err := New(ErrorCodeTimeout, "timeout").
			WithMetadata("url", "http://example.com").
			WithMetadata("retry_count", 3)

		if err.Metadata["url"] != "http://example.com" {
			t.Errorf("expected url metadata, got %v", err.Metadata["url"])
		}
		if err.Metadata["retry_count"] != 3 {
			t.Errorf("expected retry_count metadata, got %v", err.Metadata["retry_count"])
		}
	})

	t.Run("returns error for chaining", func(t *testing.T) {
		err := New(ErrorCodeTimeout, "timeout").
			WithMetadata("key1", "value1").
			WithMetadata("key2", "value2")

		if len(err.Metadata) != 2 {
			t.Errorf("expected 2 metadata entries, got %d", len(err.Metadata))
		}
	})

	t.Run("initializes metadata if nil", func(t *testing.T) {
		err := &DomainError{
			Code:    ErrorCodeTimeout,
			Message: "timeout",
		}

		err.WithMetadata("key", "value")

		if err.Metadata == nil {
			t.Error("metadata should be initialized")
		}
		if err.Metadata["key"] != "value" {
			t.Error("metadata should be set")
		}
	})
}

func TestIsRetryable(t *testing.T) {
	t.Run("returns true for retryable errors", func(t *testing.T) {
		retryableCodes := []ErrorCode{
			ErrorCodeTimeout,
			ErrorCodeNetworkFailure,
			ErrorCodeRateLimitExceeded,
			ErrorCodeVCSAPIError,
			ErrorCodeGitUpdateFailed,
			ErrorCodeReviewerTimeout,
		}

		for _, code := range retryableCodes {
			err := New(code, "error")
			if !IsRetryable(err) {
				t.Errorf("error code %s should be retryable", code)
			}
		}
	})

	t.Run("returns false for non-retryable errors", func(t *testing.T) {
		nonRetryableCodes := []ErrorCode{
			ErrorCodeVCSNotFound,
			ErrorCodeVCSInvalidPayload,
			ErrorCodeProfileNotFound,
			ErrorCodeConfigInvalid,
			ErrorCodeValidationFailed,
		}

		for _, code := range nonRetryableCodes {
			err := New(code, "error")
			if IsRetryable(err) {
				t.Errorf("error code %s should not be retryable", code)
			}
		}
	})

	t.Run("returns false for standard errors", func(t *testing.T) {
		err := errors.New("standard error")
		if IsRetryable(err) {
			t.Error("standard error should not be retryable")
		}
	})

	t.Run("returns false for nil error", func(t *testing.T) {
		if IsRetryable(nil) {
			t.Error("nil should not be retryable")
		}
	})
}

func TestGetCode(t *testing.T) {
	t.Run("extracts code from domain error", func(t *testing.T) {
		err := New(ErrorCodeTimeout, "timeout")
		code := GetCode(err)

		if code != ErrorCodeTimeout {
			t.Errorf("expected code %s, got %s", ErrorCodeTimeout, code)
		}
	})

	t.Run("extracts code from wrapped error", func(t *testing.T) {
		cause := New(ErrorCodeVCSNotFound, "not found")
		wrapped := Wrap(ErrorCodeVCSAPIError, "API error", cause)

		code := GetCode(wrapped)
		if code != ErrorCodeVCSAPIError {
			t.Errorf("expected code %s, got %s", ErrorCodeVCSAPIError, code)
		}
	})

	t.Run("returns ErrorCodeUnknown for standard errors", func(t *testing.T) {
		err := errors.New("standard error")
		code := GetCode(err)

		if code != ErrorCodeUnknown {
			t.Errorf("expected ErrorCodeUnknown, got %s", code)
		}
	})

	t.Run("returns ErrorCodeUnknown for nil", func(t *testing.T) {
		code := GetCode(nil)

		if code != ErrorCodeUnknown {
			t.Errorf("expected ErrorCodeUnknown, got %s", code)
		}
	})
}

func TestGetMetadata(t *testing.T) {
	t.Run("extracts metadata from domain error", func(t *testing.T) {
		err := New(ErrorCodeTimeout, "timeout").
			WithMetadata("key", "value")

		metadata := GetMetadata(err)
		if metadata["key"] != "value" {
			t.Errorf("expected metadata key=value, got %v", metadata)
		}
	})

	t.Run("returns nil for standard errors", func(t *testing.T) {
		err := errors.New("standard error")
		metadata := GetMetadata(err)

		if metadata != nil {
			t.Errorf("expected nil metadata, got %v", metadata)
		}
	})

	t.Run("returns nil for nil error", func(t *testing.T) {
		metadata := GetMetadata(nil)

		if metadata != nil {
			t.Errorf("expected nil metadata, got %v", metadata)
		}
	})
}

func TestPredefinedErrors(t *testing.T) {
	t.Run("ErrCircuitOpen has correct properties", func(t *testing.T) {
		if ErrCircuitOpen.Code != ErrorCodeCircuitOpen {
			t.Errorf("expected code %s, got %s", ErrorCodeCircuitOpen, ErrCircuitOpen.Code)
		}
		if ErrCircuitOpen.Message != "circuit breaker is open" {
			t.Errorf("unexpected message: %s", ErrCircuitOpen.Message)
		}
	})

	t.Run("ErrQueueFull has correct properties", func(t *testing.T) {
		if ErrQueueFull.Code != ErrorCodeQueueFull {
			t.Errorf("expected code %s, got %s", ErrorCodeQueueFull, ErrQueueFull.Code)
		}
		if ErrQueueFull.Message != "queue is full" {
			t.Errorf("unexpected message: %s", ErrQueueFull.Message)
		}
	})

	t.Run("ErrQueueClosed has correct properties", func(t *testing.T) {
		if ErrQueueClosed.Code != ErrorCodeQueueClosed {
			t.Errorf("expected code %s, got %s", ErrorCodeQueueClosed, ErrQueueClosed.Code)
		}
	})

	t.Run("ErrUnauthorized has correct properties", func(t *testing.T) {
		if ErrUnauthorized.Code != ErrorCodeVCSUnauthorized {
			t.Errorf("expected code %s, got %s", ErrorCodeVCSUnauthorized, ErrUnauthorized.Code)
		}
	})

	t.Run("ErrNotFound has correct properties", func(t *testing.T) {
		if ErrNotFound.Code != ErrorCodeVCSNotFound {
			t.Errorf("expected code %s, got %s", ErrorCodeVCSNotFound, ErrNotFound.Code)
		}
	})
}

func TestErrorChaining(t *testing.T) {
	t.Run("chains multiple wrapped errors", func(t *testing.T) {
		root := errors.New("root cause")
		level1 := Wrap(ErrorCodeNetworkFailure, "network error", root)
		level2 := Wrap(ErrorCodeVCSAPIError, "API error", level1)

		// Check level2
		if level2.Code != ErrorCodeVCSAPIError {
			t.Error("level2 should have VCS API error code")
		}

		// Unwrap once
		unwrapped1 := errors.Unwrap(level2)
		domainErr1, ok := unwrapped1.(*DomainError)
		if !ok {
			t.Fatal("first unwrap should return DomainError")
		}
		if domainErr1.Code != ErrorCodeNetworkFailure {
			t.Error("level1 should have network failure code")
		}

		// Unwrap twice
		unwrapped2 := errors.Unwrap(domainErr1)
		if unwrapped2 != root {
			t.Error("second unwrap should return root cause")
		}
	})

	t.Run("errors.Is works through chain", func(t *testing.T) {
		target := New(ErrorCodeVCSNotFound, "not found")
		wrapped1 := Wrap(ErrorCodeVCSAPIError, "API error", target)
		wrapped2 := Wrap(ErrorCodeNetworkFailure, "network error", wrapped1)

		if !errors.Is(wrapped2, target) {
			t.Error("should match target through chain")
		}
	})

	t.Run("errors.As extracts DomainError from chain", func(t *testing.T) {
		root := errors.New("root")
		domainErr := Wrap(ErrorCodeTimeout, "timeout", root)
		wrapped := Wrap(ErrorCodeVCSAPIError, "API error", domainErr)

		var extracted *DomainError
		if !errors.As(wrapped, &extracted) {
			t.Fatal("should extract DomainError")
		}
		if extracted.Code != ErrorCodeVCSAPIError {
			t.Errorf("expected VCS API error, got %s", extracted.Code)
		}
	})
}

func TestRetryableErrorCodes(t *testing.T) {
	t.Run("all expected codes are retryable", func(t *testing.T) {
		// This test ensures the isRetryable function stays in sync
		retryable := []ErrorCode{
			ErrorCodeTimeout,
			ErrorCodeNetworkFailure,
			ErrorCodeRateLimitExceeded,
			ErrorCodeVCSAPIError,
			ErrorCodeGitUpdateFailed,
			ErrorCodeReviewerTimeout,
		}

		for _, code := range retryable {
			err := New(code, "test")
			if !err.Retryable {
				t.Errorf("code %s should be marked retryable", code)
			}
		}
	})

	t.Run("all other codes are not retryable", func(t *testing.T) {
		nonRetryable := []ErrorCode{
			ErrorCodeVCSUnauthorized,
			ErrorCodeVCSNotFound,
			ErrorCodeVCSInvalidPayload,
			ErrorCodeGitCloneFailed,
			ErrorCodeGitAccessDenied,
			ErrorCodeReviewerFailed,
			ErrorCodeReviewerInvalid,
			ErrorCodeProfileNotFound,
			ErrorCodeProfileInvalid,
			ErrorCodeConfigInvalid,
			ErrorCodeConfigMissing,
			ErrorCodeQueueFull,
			ErrorCodeQueueClosed,
			ErrorCodeValidationFailed,
			ErrorCodeCircuitOpen,
			ErrorCodeUnknown,
		}

		for _, code := range nonRetryable {
			err := New(code, "test")
			if err.Retryable {
				t.Errorf("code %s should not be marked retryable", code)
			}
		}
	})
}
