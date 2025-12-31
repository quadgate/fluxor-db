package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// DBRuntime is an advanced Oracle database runtime that exceeds HikariCP capabilities
type DBRuntime struct {
	connManager *ConnectionManager
	gate        *ConnectionGate
	advancedDB  *AdvancedDB
	config      *RuntimeConfig
}

// RuntimeConfig configures the entire database runtime
type RuntimeConfig struct {
	// Connection configuration
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration

	// Advanced connection features
	LeakDetectionThreshold time.Duration
	ValidationQuery        string
	ValidationTimeout      time.Duration
	WarmupConnections      int
	WarmupTimeout          time.Duration
	ConnectionTimeout      time.Duration
	EnableLeakDetection    bool

	// Gate configuration
	CircuitBreakerMaxFailures     int
	CircuitBreakerResetTimeout    time.Duration
	CircuitBreakerHalfOpenTimeout time.Duration
	MaxRequestsPerSecond          int64
	MaxConcurrentConnections      int64

	// Database operation configuration
	StmtCacheSize      int
	SlowQueryThreshold time.Duration
	QueryTimeout       time.Duration
	MaxRetries         int
	RetryBackoff       time.Duration
}

// NewDBRuntime creates a new advanced database runtime
func NewDBRuntime(config *RuntimeConfig) *DBRuntime {
	if config == nil {
		config = &RuntimeConfig{}
	}

	// Create connection manager
	connConfig := &AdvancedConfig{
		DSN:                    config.DSN,
		MaxOpenConns:           config.MaxOpenConns,
		MaxIdleConns:           config.MaxIdleConns,
		ConnMaxLifetime:        config.ConnMaxLifetime,
		ConnMaxIdleTime:        config.ConnMaxIdleTime,
		LeakDetectionThreshold: config.LeakDetectionThreshold,
		ValidationQuery:        config.ValidationQuery,
		ValidationTimeout:      config.ValidationTimeout,
		WarmupConnections:      config.WarmupConnections,
		WarmupTimeout:          config.WarmupTimeout,
		ConnectionTimeout:      config.ConnectionTimeout,
		EnableLeakDetection:    config.EnableLeakDetection,
	}

	connManager := NewConnectionManager(connConfig)

	// Create connection gate
	gateConfig := &GateConfig{
		MaxFailures:              config.CircuitBreakerMaxFailures,
		ResetTimeout:             config.CircuitBreakerResetTimeout,
		HalfOpenTimeout:          config.CircuitBreakerHalfOpenTimeout,
		MaxRequestsPerSecond:     config.MaxRequestsPerSecond,
		MaxConcurrentConnections: config.MaxConcurrentConnections,
	}

	gate := NewConnectionGate(gateConfig)

	// AdvancedDB will be created after connection is opened
	runtime := &DBRuntime{
		connManager: connManager,
		gate:        gate,
		config:      config,
	}

	return runtime
}

// Connect establishes connection to the database
func (r *DBRuntime) Connect() error {
	if err := r.connManager.Open(); err != nil {
		return fmt.Errorf("failed to open connection: %w", err)
	}

	// Create advanced DB wrapper
	dbConfig := &DBAdvancedConfig{
		StmtCacheSize:      r.config.StmtCacheSize,
		SlowQueryThreshold: r.config.SlowQueryThreshold,
		QueryTimeout:       r.config.QueryTimeout,
		MaxRetries:         r.config.MaxRetries,
		RetryBackoff:       r.config.RetryBackoff,
	}

	r.advancedDB = NewAdvancedDB(r.connManager.DB(), r.gate, dbConfig)

	return nil
}

// Disconnect closes all connections and cleans up resources
func (r *DBRuntime) Disconnect() error {
	if r.advancedDB != nil && r.advancedDB.stmtCache != nil {
		r.advancedDB.stmtCache.Clear()
	}
	return r.connManager.Close()
}

// DB returns the underlying sql.DB connection pool
func (r *DBRuntime) DB() *sql.DB {
	return r.connManager.DB()
}

// AdvancedDB returns the advanced database wrapper
func (r *DBRuntime) AdvancedDB() *AdvancedDB {
	return r.advancedDB
}

// IsConnected returns whether the runtime is connected
func (r *DBRuntime) IsConnected() bool {
	return r.connManager.db != nil
}

// Exec executes a query without returning rows (with all advanced features)
func (r *DBRuntime) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if !r.IsConnected() {
		return nil, fmt.Errorf("database not connected")
	}
	return r.advancedDB.Exec(ctx, query, args...)
}

// Query executes a query that returns rows (with all advanced features)
func (r *DBRuntime) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if !r.IsConnected() {
		return nil, fmt.Errorf("database not connected")
	}
	return r.advancedDB.Query(ctx, query, args...)
}

// QueryRow executes a query that returns at most one row
func (r *DBRuntime) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if !r.IsConnected() {
		return nil
	}
	return r.advancedDB.QueryRow(ctx, query, args...)
}

// Prepare creates or retrieves a cached prepared statement
func (r *DBRuntime) Prepare(ctx context.Context, query string) (*sql.Stmt, error) {
	if !r.IsConnected() {
		return nil, fmt.Errorf("database not connected")
	}
	return r.advancedDB.Prepare(ctx, query)
}

// Begin starts a new transaction
func (r *DBRuntime) Begin(ctx context.Context, opts *sql.TxOptions) (*AdvancedTx, error) {
	if !r.IsConnected() {
		return nil, fmt.Errorf("database not connected")
	}
	return r.advancedDB.Begin(ctx, opts)
}

// Stats returns connection pool statistics
func (r *DBRuntime) Stats() sql.DBStats {
	if !r.IsConnected() {
		return sql.DBStats{}
	}
	return r.advancedDB.Stats()
}

// Metrics returns performance metrics
func (r *DBRuntime) Metrics() MetricsStats {
	if !r.IsConnected() {
		return MetricsStats{}
	}
	return r.advancedDB.Metrics().GetStats()
}

// HealthCheck performs a health check on the database connection
func (r *DBRuntime) HealthCheck(ctx context.Context) error {
	if !r.IsConnected() {
		return fmt.Errorf("database not connected")
	}
	return r.advancedDB.HealthCheck(ctx)
}

// CircuitBreakerState returns the current circuit breaker state
func (r *DBRuntime) CircuitBreakerState() string {
	return r.gate.State()
}

// Example usage demonstrating advanced features
func main() {
	// Create runtime with advanced configuration
	config := &RuntimeConfig{
		// Basic connection settings
		DSN:             "user/password@localhost:1521/XE",
		MaxOpenConns:    50,
		MaxIdleConns:    10,
		ConnMaxLifetime: 30 * time.Minute,
		ConnMaxIdleTime: 10 * time.Minute,

		// Advanced connection features
		LeakDetectionThreshold: 10 * time.Minute,
		ValidationQuery:        "SELECT 1 FROM DUAL",
		ValidationTimeout:      5 * time.Second,
		WarmupConnections:      5,
		WarmupTimeout:          30 * time.Second,
		ConnectionTimeout:      30 * time.Second,
		EnableLeakDetection:    true,

		// Circuit breaker settings
		CircuitBreakerMaxFailures:     5,
		CircuitBreakerResetTimeout:    60 * time.Second,
		CircuitBreakerHalfOpenTimeout: 10 * time.Second,
		MaxRequestsPerSecond:          1000,
		MaxConcurrentConnections:      100,

		// Query settings
		StmtCacheSize:      200,
		SlowQueryThreshold: 1 * time.Second,
		QueryTimeout:       30 * time.Second,
		MaxRetries:         3,
		RetryBackoff:       100 * time.Millisecond,
	}

	runtime := NewDBRuntime(config)

	// Connect to database
	if err := runtime.Connect(); err != nil {
		fmt.Printf("Failed to connect: %v\n", err)
		return
	}
	defer DisconnectWithLog(runtime)

	// Perform health check
	ctx := context.Background()
	if err := runtime.HealthCheck(ctx); err != nil {
		fmt.Printf("Health check failed: %v\n", err)
		return
	}

	fmt.Println("Advanced Oracle Database Runtime is ready!")
	fmt.Printf("Circuit Breaker State: %s\n", runtime.CircuitBreakerState())
	fmt.Printf("Connection Stats: %+v\n", runtime.Stats())

	// Example query execution
	result, err := runtime.Exec(ctx, "SELECT 1 FROM DUAL")
	if err != nil {
		fmt.Printf("Query failed: %v\n", err)
		return
	}

	fmt.Printf("Query executed successfully: %+v\n", result)
	fmt.Printf("Performance Metrics: %+v\n", runtime.Metrics())
}
