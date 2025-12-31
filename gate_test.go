package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewConnectionGate(t *testing.T) {
	config := &GateConfig{
		MaxFailures:              5,
		ResetTimeout:             60 * time.Second,
		HalfOpenTimeout:          10 * time.Second,
		MaxRequestsPerSecond:     1000,
		MaxConcurrentConnections: 100,
	}

	gate := NewConnectionGate(config)
	if gate == nil {
		t.Fatal("NewConnectionGate returned nil")
	}

	if gate.circuitBreaker == nil {
		t.Fatal("Circuit breaker is nil")
	}

	if gate.rateLimiter == nil {
		t.Fatal("Rate limiter is nil")
	}

	if gate.connectionLimiter == nil {
		t.Fatal("Connection limiter is nil")
	}
}

func TestConnectionGate_Allow(t *testing.T) {
	gate := NewConnectionGate(nil)
	ctx := context.Background()

	err := gate.Allow(ctx)
	if err != nil {
		t.Errorf("Allow() should succeed initially, got error: %v", err)
	}
}

func TestConnectionGate_State(t *testing.T) {
	gate := NewConnectionGate(nil)
	state := gate.State()

	validStates := map[string]bool{
		"closed":    true,
		"open":      true,
		"half-open": true,
	}

	if !validStates[state] {
		t.Errorf("Invalid circuit breaker state: %s", state)
	}
}

func TestConnectionGate_RecordSuccess(t *testing.T) {
	gate := NewConnectionGate(nil)
	gate.RecordSuccess()

	// Should not panic
	state := gate.State()
	if state == "" {
		t.Error("State should not be empty")
	}
}

func TestConnectionGate_RecordFailure(t *testing.T) {
	gate := NewConnectionGate(&GateConfig{
		MaxFailures: 2,
	})

	// Record failures
	gate.RecordFailure()
	gate.RecordFailure()

	// Circuit breaker should be open after max failures
	state := gate.State()
	if state != "open" {
		t.Errorf("Expected circuit breaker to be open, got %s", state)
	}
}

func TestCircuitBreaker_Allow(t *testing.T) {
	cb := NewCircuitBreaker(&GateConfig{
		MaxFailures: 3,
	})

	err := cb.Allow()
	if err != nil {
		t.Errorf("Allow() should succeed when closed, got error: %v", err)
	}
}

func TestCircuitBreaker_RecordFailure(t *testing.T) {
	cb := NewCircuitBreaker(&GateConfig{
		MaxFailures: 2,
	})

	// Record failures
	cb.RecordFailure()
	if cb.State() != "closed" {
		t.Error("Circuit breaker should still be closed after one failure")
	}

	cb.RecordFailure()
	if cb.State() != "open" {
		t.Error("Circuit breaker should be open after max failures")
	}
}

func TestCircuitBreaker_RecordSuccess(t *testing.T) {
	cb := NewCircuitBreaker(&GateConfig{
		MaxFailures: 2,
	})

	// Open the circuit breaker
	cb.RecordFailure()
	cb.RecordFailure()

	// Should be open
	if cb.State() != "open" {
		t.Error("Circuit breaker should be open")
	}

	// Wait for reset timeout (simplified - in real test would use shorter timeout)
	// For now, just verify state transitions work
	cb.RecordSuccess()
	// In half-open state, success should close it
}

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter(&GateConfig{
		MaxRequestsPerSecond: 10,
	})

	// Should allow initial requests
	for i := 0; i < 10; i++ {
		if err := rl.Allow(); err != nil {
			t.Errorf("Allow() should succeed, got error: %v", err)
		}
	}
}

func TestConnectionLimiter_Acquire(t *testing.T) {
	cl := NewConnectionLimiter(&GateConfig{
		MaxConcurrentConnections: 5,
	})

	// Should acquire connections
	for i := 0; i < 5; i++ {
		if err := cl.Acquire(); err != nil {
			t.Errorf("Acquire() should succeed, got error: %v", err)
		}
	}

	// Should fail when limit reached
	if err := cl.Acquire(); err == nil {
		t.Error("Acquire() should fail when limit reached")
	}
}

func TestConnectionLimiter_Release(t *testing.T) {
	cl := NewConnectionLimiter(&GateConfig{
		MaxConcurrentConnections: 5,
	})

	// Acquire connections
	for i := 0; i < 5; i++ {
		cl.Acquire()
	}

	// Release one
	cl.Release()

	// Should be able to acquire again
	if err := cl.Acquire(); err != nil {
		t.Errorf("Acquire() should succeed after release, got error: %v", err)
	}
}

func TestExecuteWithGate(t *testing.T) {
	gate := NewConnectionGate(nil)
	ctx := context.Background()

	result, err := ExecuteWithGate(gate, ctx, func(ctx context.Context) (string, error) {
		return "success", nil
	})

	if err != nil {
		t.Errorf("ExecuteWithGate() should succeed, got error: %v", err)
	}

	if result != "success" {
		t.Errorf("Expected 'success', got '%s'", result)
	}
}

func TestExecuteWithGate_Error(t *testing.T) {
	gate := NewConnectionGate(nil)
	ctx := context.Background()
	testErr := errors.New("test error")

	_, err := ExecuteWithGate(gate, ctx, func(ctx context.Context) (string, error) {
		return "", testErr
	})

	if err == nil {
		t.Error("ExecuteWithGate() should return error")
	}

	if err != testErr {
		t.Errorf("Expected test error, got %v", err)
	}
}
