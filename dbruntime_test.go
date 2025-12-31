package main

import (
	"context"
	"testing"
)

func TestNewDBRuntime(t *testing.T) {
	config := &RuntimeConfig{
		DSN:          "test@localhost:1521/XE",
		MaxOpenConns: 10,
		MaxIdleConns: 5,
	}

	runtime := NewDBRuntime(config)
	if runtime == nil {
		t.Fatal("NewDBRuntime returned nil")
	}

	if runtime.config == nil {
		t.Fatal("Runtime config is nil")
	}

	if runtime.connManager == nil {
		t.Fatal("Connection manager is nil")
	}

	if runtime.gate == nil {
		t.Fatal("Connection gate is nil")
	}
}

func TestDBRuntime_IsConnected(t *testing.T) {
	config := &RuntimeConfig{
		DSN: "test@localhost:1521/XE",
	}

	runtime := NewDBRuntime(config)
	if runtime.IsConnected() {
		t.Error("Runtime should not be connected before Connect()")
	}
}

func TestDBRuntime_CircuitBreakerState(t *testing.T) {
	config := &RuntimeConfig{
		DSN: "test@localhost:1521/XE",
	}

	runtime := NewDBRuntime(config)
	state := runtime.CircuitBreakerState()

	if state == "" {
		t.Error("Circuit breaker state should not be empty")
	}

	validStates := map[string]bool{
		CircuitStateClosed:   true,
		CircuitStateOpen:     true,
		CircuitStateHalfOpen: true,
	}

	if !validStates[state] {
		t.Errorf("Invalid circuit breaker state: %s", state)
	}
}

func TestDBRuntime_Stats(t *testing.T) {
	config := &RuntimeConfig{
		DSN: "test@localhost:1521/XE",
	}

	runtime := NewDBRuntime(config)
	stats := runtime.Stats()

	// Stats should be empty when not connected
	if stats.OpenConnections != 0 {
		t.Errorf("Expected 0 open connections, got %d", stats.OpenConnections)
	}
}

func TestDBRuntime_Metrics(t *testing.T) {
	config := &RuntimeConfig{
		DSN: "test@localhost:1521/XE",
	}

	runtime := NewDBRuntime(config)
	metrics := runtime.Metrics()

	// Metrics should be empty when not connected
	if metrics.TotalQueries != 0 {
		t.Errorf("Expected 0 total queries, got %d", metrics.TotalQueries)
	}
}

func TestDBRuntime_HealthCheck_NotConnected(t *testing.T) {
	config := &RuntimeConfig{
		DSN: "test@localhost:1521/XE",
	}

	runtime := NewDBRuntime(config)
	ctx := context.Background()

	err := runtime.HealthCheck(ctx)
	if err == nil {
		t.Error("HealthCheck should fail when not connected")
	}
}

func TestDBRuntime_Disconnect_NotConnected(t *testing.T) {
	config := &RuntimeConfig{
		DSN: "test@localhost:1521/XE",
	}

	runtime := NewDBRuntime(config)
	err := runtime.Disconnect()
	if err != nil {
		t.Errorf("Disconnect should not fail when not connected: %v", err)
	}
}
