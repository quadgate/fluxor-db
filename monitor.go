package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Monitor provides continuous monitoring of the database runtime
type Monitor struct {
	runtime   *DBRuntime
	interval  time.Duration
	stopChan  chan struct{}
	callbacks []MonitorCallback
	mu        sync.RWMutex
	running   bool
}

// MonitorCallback is called when monitoring events occur
type MonitorCallback func(event MonitorEvent)

// MonitorEvent represents a monitoring event
type MonitorEvent struct {
	Type        string
	Timestamp   time.Time
	Diagnostics *Diagnostics
	Health      *HealthStatus
	Message     string
}

// NewMonitor creates a new monitor
func NewMonitor(runtime *DBRuntime, interval time.Duration) *Monitor {
	return &Monitor{
		runtime:   runtime,
		interval:  interval,
		stopChan:  make(chan struct{}),
		callbacks: []MonitorCallback{},
	}
}

// AddCallback adds a callback function to be called on monitoring events
func (m *Monitor) AddCallback(callback MonitorCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callbacks = append(m.callbacks, callback)
}

// Start starts the monitoring loop
func (m *Monitor) Start(ctx context.Context) {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.mu.Unlock()

	go m.monitorLoop(ctx)
}

// Stop stops the monitoring loop
func (m *Monitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return
	}
	close(m.stopChan)
	m.running = false
}

// monitorLoop runs the monitoring loop
func (m *Monitor) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.checkAndNotify(ctx)
		case <-m.stopChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

// checkAndNotify performs checks and notifies callbacks
func (m *Monitor) checkAndNotify(ctx context.Context) {
	// Get diagnostics
	diagnostics := GetDiagnostics(m.runtime)

	// Perform health check
	health := CheckHealth(ctx, m.runtime)

	// Notify callbacks
	m.mu.RLock()
	callbacks := m.callbacks
	m.mu.RUnlock()

	event := MonitorEvent{
		Type:        "periodic_check",
		Timestamp:   time.Now(),
		Diagnostics: diagnostics,
		Health:      health,
	}

	for _, callback := range callbacks {
		callback(event)
	}

	// Check for warnings
	if !health.Healthy {
		warningEvent := MonitorEvent{
			Type:        "health_warning",
			Timestamp:   time.Now(),
			Diagnostics: diagnostics,
			Health:      health,
			Message:     health.Message,
		}
		for _, callback := range callbacks {
			callback(warningEvent)
		}
	}

	// Check for slow queries
	metrics := diagnostics.Metrics
	if metrics.SlowQueries > 0 {
		slowQueryEvent := MonitorEvent{
			Type:        "slow_queries",
			Timestamp:   time.Now(),
			Diagnostics: diagnostics,
			Message:     fmt.Sprintf("Detected %d slow queries", metrics.SlowQueries),
		}
		for _, callback := range callbacks {
			callback(slowQueryEvent)
		}
	}

	// Check circuit breaker state
	if diagnostics.CircuitBreaker == "open" {
		cbEvent := MonitorEvent{
			Type:        "circuit_breaker_open",
			Timestamp:   time.Now(),
			Diagnostics: diagnostics,
			Message:     "Circuit breaker is open",
		}
		for _, callback := range callbacks {
			callback(cbEvent)
		}
	}
}

// DefaultLoggingCallback logs monitoring events
func DefaultLoggingCallback(event MonitorEvent) {
	switch event.Type {
	case "health_warning":
		fmt.Printf("[WARN] %s: %s\n", event.Timestamp.Format(time.RFC3339), event.Message)
	case "circuit_breaker_open":
		fmt.Printf("[ERROR] %s: Circuit breaker is open\n", event.Timestamp.Format(time.RFC3339))
	case "slow_queries":
		fmt.Printf("[WARN] %s: %s\n", event.Timestamp.Format(time.RFC3339), event.Message)
	default:
		// Periodic check - log diagnostics summary
		if event.Diagnostics != nil {
			stats := event.Diagnostics.ConnectionStats
			fmt.Printf("[INFO] %s: Connections=%d/%d, Queries=%d, SuccessRate=%.2f%%\n",
				event.Timestamp.Format(time.RFC3339),
				stats.InUse,
				stats.OpenConnections,
				event.Diagnostics.Metrics.TotalQueries,
				event.Diagnostics.Metrics.SuccessRate,
			)
		}
	}
}
