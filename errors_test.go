package main

import (
	"context"
	"errors"
	"testing"
)

func TestNewDatabaseError(t *testing.T) {
	originalErr := errors.New("original error")
	dbErr := NewDatabaseError("TEST_CODE", "test message", originalErr)

	if dbErr == nil {
		t.Fatal("NewDatabaseError returned nil")
	}

	if dbErr.Code != "TEST_CODE" {
		t.Errorf("Expected Code 'TEST_CODE', got '%s'", dbErr.Code)
	}

	if dbErr.Message != "test message" {
		t.Errorf("Expected Message 'test message', got '%s'", dbErr.Message)
	}

	if dbErr.Err != originalErr {
		t.Error("Error wrapping mismatch")
	}
}

func TestDatabaseError_Error(t *testing.T) {
	originalErr := errors.New("original error")
	dbErr := NewDatabaseError("TEST_CODE", "test message", originalErr)

	errStr := dbErr.Error()
	if errStr == "" {
		t.Error("Error() should not return empty string")
	}

	if errStr == "test message" {
		t.Error("Error() should include code and wrapped error")
	}
}

func TestDatabaseError_Unwrap(t *testing.T) {
	originalErr := errors.New("original error")
	dbErr := NewDatabaseError("TEST_CODE", "test message", originalErr)

	unwrapped := dbErr.Unwrap()
	if unwrapped != originalErr {
		t.Error("Unwrap() should return original error")
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "timeout error",
			err:      NewDatabaseError(ErrCodeTimeout, "timeout", nil),
			expected: true,
		},
		{
			name:     "connection failed error",
			err:      NewDatabaseError(ErrCodeConnectionFailed, "connection failed", nil),
			expected: true,
		},
		{
			name:     "query failed error",
			err:      NewDatabaseError(ErrCodeQueryFailed, "query failed", nil),
			expected: false,
		},
		{
			name:     "non-database error",
			err:      errors.New("regular error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryableError(tt.err)
			if result != tt.expected {
				t.Errorf("IsRetryableError() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsCircuitBreakerError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "circuit breaker error",
			err:      NewDatabaseError(ErrCodeCircuitBreakerOpen, "circuit breaker open", nil),
			expected: true,
		},
		{
			name:     "other error",
			err:      NewDatabaseError(ErrCodeQueryFailed, "query failed", nil),
			expected: false,
		},
		{
			name:     "non-database error",
			err:      errors.New("regular error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsCircuitBreakerError(tt.err)
			if result != tt.expected {
				t.Errorf("IsCircuitBreakerError() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestWrapError(t *testing.T) {
	originalErr := errors.New("original error")
	wrapped := WrapError("TEST_CODE", "wrapped message", originalErr)

	if wrapped == nil {
		t.Fatal("WrapError returned nil")
	}

	var dbErr *DatabaseError
	if !errors.As(wrapped, &dbErr) {
		t.Fatal("Wrapped error should be DatabaseError")
	}

	if dbErr.Code != "TEST_CODE" {
		t.Errorf("Expected Code 'TEST_CODE', got '%s'", dbErr.Code)
	}
}

func TestWrapError_Nil(t *testing.T) {
	wrapped := WrapError("TEST_CODE", "message", nil)
	if wrapped != nil {
		t.Error("WrapError should return nil for nil input")
	}
}

func TestNewErrorRecovery(t *testing.T) {
	config := &RuntimeConfig{
		DSN: "test@localhost:1521/XE",
	}
	runtime := NewDBRuntime(config)

	recovery := NewErrorRecovery(runtime)
	if recovery == nil {
		t.Fatal("NewErrorRecovery returned nil")
	}

	if recovery.runtime != runtime {
		t.Error("ErrorRecovery runtime mismatch")
	}
}

func TestErrorRecovery_RecoverConnection_NotConnected(t *testing.T) {
	config := &RuntimeConfig{
		DSN: "test@localhost:1521/XE",
	}
	runtime := NewDBRuntime(config)
	recovery := NewErrorRecovery(runtime)
	ctx := context.Background()

	// Should attempt to reconnect (will fail without actual DB, but should not panic)
	err := recovery.RecoverConnection(ctx)
	if err == nil {
		t.Error("RecoverConnection should return error when not connected and DB unavailable")
	}

	// Verify it's a connection error
	var dbErr *DatabaseError
	if !errors.As(err, &dbErr) {
		t.Error("Error should be DatabaseError")
	}
}

func TestErrorRecovery_HandleError_Nil(t *testing.T) {
	config := &RuntimeConfig{
		DSN: "test@localhost:1521/XE",
	}
	runtime := NewDBRuntime(config)
	recovery := NewErrorRecovery(runtime)
	ctx := context.Background()

	err := recovery.HandleError(ctx, nil)
	if err != nil {
		t.Errorf("HandleError should return nil for nil error, got %v", err)
	}
}

func TestErrorRecovery_HandleError_NonRetryable(t *testing.T) {
	config := &RuntimeConfig{
		DSN: "test@localhost:1521/XE",
	}
	runtime := NewDBRuntime(config)
	recovery := NewErrorRecovery(runtime)
	ctx := context.Background()

	testErr := errors.New("non-retryable error")
	err := recovery.HandleError(ctx, testErr)
	if err != testErr {
		t.Errorf("HandleError should return original error for non-retryable errors")
	}
}
