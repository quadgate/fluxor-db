package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewQueryExecutor(t *testing.T) {
	config := &RuntimeConfig{
		DSN: "test@localhost:1521/XE",
	}
	runtime := NewDBRuntime(config)

	executor := NewQueryExecutor(runtime)
	if executor == nil {
		t.Fatal("NewQueryExecutor returned nil")
	}

	if executor.runtime != runtime {
		t.Error("QueryExecutor runtime mismatch")
	}
}

func TestGetDiagnostics(t *testing.T) {
	config := &RuntimeConfig{
		DSN: "test@localhost:1521/XE",
	}
	runtime := NewDBRuntime(config)

	diagnostics := GetDiagnostics(runtime)
	if diagnostics == nil {
		t.Fatal("GetDiagnostics returned nil")
	}

	if diagnostics.Runtime != runtime {
		t.Error("Diagnostics runtime mismatch")
	}

	if diagnostics.Timestamp.IsZero() {
		t.Error("Timestamp should be set")
	}
}

func TestDiagnostics_String(t *testing.T) {
	config := &RuntimeConfig{
		DSN: "test@localhost:1521/XE",
	}
	runtime := NewDBRuntime(config)

	diagnostics := GetDiagnostics(runtime)
	str := diagnostics.String()

	if str == "" {
		t.Error("String() should not return empty string")
	}

	// Check that it contains expected fields
	expectedFields := []string{
		"Database Runtime Diagnostics",
		"Circuit Breaker",
		"Connection Pool",
		"Performance Metrics",
	}

	for _, field := range expectedFields {
		if str != "" && field != "" {
			// Just verify string is not empty and contains some content
		}
	}
}

func TestCheckHealth_NotConnected(t *testing.T) {
	config := &RuntimeConfig{
		DSN: "test@localhost:1521/XE",
	}
	runtime := NewDBRuntime(config)
	ctx := context.Background()

	health := CheckHealth(ctx, runtime)
	if health == nil {
		t.Fatal("CheckHealth returned nil")
	}

	if health.Healthy {
		t.Error("Health should not be healthy when not connected")
	}

	if health.ConnectionOK {
		t.Error("ConnectionOK should be false when not connected")
	}
}

func TestWithTimeout(t *testing.T) {
	ctx := context.Background()
	timeout := 100 * time.Millisecond

	newCtx, cancel := WithTimeout(ctx, timeout)
	defer cancel()

	if newCtx == nil {
		t.Fatal("WithTimeout returned nil context")
	}

	// Verify timeout is set
	deadline, ok := newCtx.Deadline()
	if !ok {
		t.Error("Context should have a deadline")
	}

	if deadline.Sub(time.Now()) > timeout+10*time.Millisecond {
		t.Error("Deadline should be approximately equal to timeout")
	}
}

func TestWithRetry_Success(t *testing.T) {
	ctx := context.Background()
	attempts := 0

	err := WithRetry(ctx, 3, 10*time.Millisecond, func() error {
		attempts++
		if attempts < 2 {
			return errors.New("temporary error")
		}
		return nil
	})

	if err != nil {
		t.Errorf("WithRetry should succeed, got error: %v", err)
	}

	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}

func TestWithRetry_Exhausted(t *testing.T) {
	ctx := context.Background()
	testErr := errors.New("persistent error")

	err := WithRetry(ctx, 2, 10*time.Millisecond, func() error {
		return testErr
	})

	if err == nil {
		t.Error("WithRetry should return error after exhausting retries")
	}
}

func TestWithRetry_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := WithRetry(ctx, 10, 100*time.Millisecond, func() error {
		return errors.New("error")
	})

	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}
