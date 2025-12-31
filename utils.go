package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"
)

// QueryExecutor provides a convenient interface for executing queries
type QueryExecutor struct {
	runtime *DBRuntime
}

// NewQueryExecutor creates a new query executor
func NewQueryExecutor(runtime *DBRuntime) *QueryExecutor {
	return &QueryExecutor{runtime: runtime}
}

// Select executes a SELECT query and scans results into a slice
func (qe *QueryExecutor) Select(ctx context.Context, query string, args []interface{}, scanFunc func(*sql.Rows) error) error {
	rows, err := qe.runtime.Query(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		if err := scanFunc(rows); err != nil {
			return fmt.Errorf("scan failed: %w", err)
		}
	}

	return rows.Err()
}

// SelectOne executes a SELECT query expecting exactly one row
func (qe *QueryExecutor) SelectOne(ctx context.Context, query string, args []interface{}, scanFunc func(*sql.Row) error) error {
	row := qe.runtime.QueryRow(ctx, query, args...)
	return scanFunc(row)
}

// Execute executes a non-query SQL statement
func (qe *QueryExecutor) Execute(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return qe.runtime.Exec(ctx, query, args...)
}

// Transaction executes a function within a transaction
func (qe *QueryExecutor) Transaction(ctx context.Context, fn func(*AdvancedTx) error) error {
	tx, err := qe.runtime.Begin(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		} else if err != nil {
			tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()

	err = fn(tx)
	return err
}

// Diagnostics provides diagnostic information about the runtime
type Diagnostics struct {
	Runtime         *DBRuntime
	ConnectionStats sql.DBStats
	Metrics         MetricsStats
	CircuitBreaker  string
	Timestamp       time.Time
}

// GetDiagnostics returns comprehensive diagnostic information
func GetDiagnostics(runtime *DBRuntime) *Diagnostics {
	return &Diagnostics{
		Runtime:         runtime,
		ConnectionStats: runtime.Stats(),
		Metrics:         runtime.Metrics(),
		CircuitBreaker:  runtime.CircuitBreakerState(),
		Timestamp:       time.Now(),
	}
}

// String returns a formatted string representation of diagnostics
func (d *Diagnostics) String() string {
	return fmt.Sprintf(`Database Runtime Diagnostics
==========================
Timestamp: %s
Circuit Breaker: %s

Connection Pool:
  Open Connections: %d
  In Use: %d
  Idle: %d
  Wait Count: %d
  Wait Duration: %v
  Max Idle Closed: %d
  Max Idle Time Closed: %d
  Max Lifetime Closed: %d

Performance Metrics:
  Total Queries: %d
  Successful: %d
  Failed: %d
  Success Rate: %.2f%%
  Average Query Time: %v
  Slow Queries: %d
`,
		d.Timestamp.Format(time.RFC3339),
		d.CircuitBreaker,
		d.ConnectionStats.OpenConnections,
		d.ConnectionStats.InUse,
		d.ConnectionStats.Idle,
		d.ConnectionStats.WaitCount,
		d.ConnectionStats.WaitDuration,
		d.ConnectionStats.MaxIdleClosed,
		d.ConnectionStats.MaxIdleTimeClosed,
		d.ConnectionStats.MaxLifetimeClosed,
		d.Metrics.TotalQueries,
		d.Metrics.SuccessfulQueries,
		d.Metrics.FailedQueries,
		d.Metrics.SuccessRate,
		d.Metrics.AverageQueryTime,
		d.Metrics.SlowQueries,
	)
}

// HealthStatus represents the health status of the runtime
type HealthStatus struct {
	Healthy          bool
	Message          string
	LastCheck        time.Time
	ConnectionOK     bool
	CircuitBreakerOK bool
}

// CheckHealth performs a comprehensive health check
func CheckHealth(ctx context.Context, runtime *DBRuntime) *HealthStatus {
	status := &HealthStatus{
		LastCheck: time.Now(),
	}

	// Check connection
	if err := runtime.HealthCheck(ctx); err != nil {
		status.ConnectionOK = false
		status.Message = fmt.Sprintf("Connection check failed: %v", err)
		status.Healthy = false
		return status
	}
	status.ConnectionOK = true

	// Check circuit breaker
	cbState := runtime.CircuitBreakerState()
	if cbState == "open" {
		status.CircuitBreakerOK = false
		status.Message = "Circuit breaker is open"
		status.Healthy = false
		return status
	}
	status.CircuitBreakerOK = true

	// Check connection pool stats
	stats := runtime.Stats()
	if stats.OpenConnections >= stats.MaxOpenConnections {
		status.Message = "Connection pool is at capacity"
		status.Healthy = false
		return status
	}

	status.Healthy = true
	status.Message = "All systems operational"
	return status
}

// WithTimeout wraps a context with timeout
func WithTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, timeout)
}

// WithRetry executes a function with retry logic
func WithRetry(ctx context.Context, maxRetries int, backoff time.Duration, fn func() error) error {
	var lastErr error
	currentBackoff := backoff

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(currentBackoff):
			}
			currentBackoff *= 2 // Exponential backoff
		}

		if err := fn(); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}

	return fmt.Errorf("failed after %d attempts: %w", maxRetries+1, lastErr)
}

// DisconnectWithLog disconnects the runtime and logs any errors
// This is a helper for defer statements where error checking is needed
func DisconnectWithLog(runtime *DBRuntime) {
	if err := runtime.Disconnect(); err != nil {
		log.Printf("Error disconnecting database runtime: %v", err)
	}
}
