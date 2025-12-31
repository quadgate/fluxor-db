package main

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// DatabaseError represents a database-related error
type DatabaseError struct {
	Code    string
	Message string
	Err     error
}

func (e *DatabaseError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *DatabaseError) Unwrap() error {
	return e.Err
}

// Error codes
const (
	ErrCodeConnectionFailed   = "CONNECTION_FAILED"
	ErrCodeQueryFailed        = "QUERY_FAILED"
	ErrCodeTransactionFailed  = "TRANSACTION_FAILED"
	ErrCodeCircuitBreakerOpen = "CIRCUIT_BREAKER_OPEN"
	ErrCodeRateLimitExceeded  = "RATE_LIMIT_EXCEEDED"
	ErrCodeConnectionLeak     = "CONNECTION_LEAK"
	ErrCodeValidationFailed   = "VALIDATION_FAILED"
	ErrCodeTimeout            = "TIMEOUT"
	ErrCodeRetryExhausted     = "RETRY_EXHAUSTED"
)

// NewDatabaseError creates a new database error
func NewDatabaseError(code, message string, err error) *DatabaseError {
	return &DatabaseError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// IsRetryableError checks if an error is retryable
func IsRetryableError(err error) bool {
	var dbErr *DatabaseError
	if errors.As(err, &dbErr) {
		switch dbErr.Code {
		case ErrCodeTimeout, ErrCodeConnectionFailed:
			return true
		}
	}
	return false
}

// IsCircuitBreakerError checks if error is due to circuit breaker
func IsCircuitBreakerError(err error) bool {
	var dbErr *DatabaseError
	if errors.As(err, &dbErr) {
		return dbErr.Code == ErrCodeCircuitBreakerOpen
	}
	return false
}

// WrapError wraps an error with database error context
func WrapError(code, message string, err error) error {
	if err == nil {
		return nil
	}
	return NewDatabaseError(code, message, err)
}

// ErrorRecovery provides error recovery strategies
type ErrorRecovery struct {
	runtime *DBRuntime
}

// NewErrorRecovery creates a new error recovery handler
func NewErrorRecovery(runtime *DBRuntime) *ErrorRecovery {
	return &ErrorRecovery{runtime: runtime}
}

// RecoverConnection attempts to recover from connection errors
func (er *ErrorRecovery) RecoverConnection(ctx context.Context) error {
	if !er.runtime.IsConnected() {
		// Try to reconnect
		if err := er.runtime.Connect(); err != nil {
			return WrapError(ErrCodeConnectionFailed, "failed to reconnect", err)
		}
	}

	// Verify connection is healthy
	if err := er.runtime.HealthCheck(ctx); err != nil {
		return WrapError(ErrCodeConnectionFailed, "connection health check failed", err)
	}

	return nil
}

// HandleError handles errors with appropriate recovery strategies
func (er *ErrorRecovery) HandleError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}

	// Check if it's a circuit breaker error
	if IsCircuitBreakerError(err) {
		// Wait for circuit breaker to potentially recover
		time.Sleep(1 * time.Second)
		if er.runtime.CircuitBreakerState() != "open" {
			return nil // Circuit breaker recovered
		}
		return err
	}

	// Check if it's a connection error
	if IsRetryableError(err) {
		// Attempt connection recovery
		if recoverErr := er.RecoverConnection(ctx); recoverErr != nil {
			return fmt.Errorf("recovery failed: %w (original error: %v)", recoverErr, err)
		}
		// Connection recovered, return original error for retry
		return err
	}

	return err
}
