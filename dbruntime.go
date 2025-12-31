package main

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "github.com/godror/godror"
)

// DBRuntime manages database connections and provides runtime services
type DBRuntime struct {
	db        *sql.DB
	config    *DBConfig
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	connected bool
}

// DBConfig holds database configuration
type DBConfig struct {
	DSN          string
	MaxOpenConns int
	MaxIdleConns int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// NewDBRuntime creates a new database runtime instance
func NewDBRuntime(config *DBConfig) *DBRuntime {
	ctx, cancel := context.WithCancel(context.Background())
	
	if config.MaxOpenConns == 0 {
		config.MaxOpenConns = 25
	}
	if config.MaxIdleConns == 0 {
		config.MaxIdleConns = 5
	}
	if config.ConnMaxLifetime == 0 {
		config.ConnMaxLifetime = 5 * time.Minute
	}
	if config.ConnMaxIdleTime == 0 {
		config.ConnMaxIdleTime = 10 * time.Minute
	}

	return &DBRuntime{
		config:    config,
		ctx:       ctx,
		cancel:    cancel,
		connected: false,
	}
}

// Connect establishes a connection to the database
func (r *DBRuntime) Connect() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.connected {
		return nil
	}

	db, err := sql.Open("godror", r.config.DSN)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(r.config.MaxOpenConns)
	db.SetMaxIdleConns(r.config.MaxIdleConns)
	db.SetConnMaxLifetime(r.config.ConnMaxLifetime)
	db.SetConnMaxIdleTime(r.config.ConnMaxIdleTime)

	// Test the connection
	ctx, cancel := context.WithTimeout(r.ctx, 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping database: %w", err)
	}

	r.db = db
	r.connected = true
	return nil
}

// Disconnect closes the database connection
func (r *DBRuntime) Disconnect() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.connected {
		return nil
	}

	r.cancel()
	if r.db != nil {
		if err := r.db.Close(); err != nil {
			return fmt.Errorf("failed to close database: %w", err)
		}
	}

	r.connected = false
	return nil
}

// DB returns the underlying database connection
func (r *DBRuntime) DB() *sql.DB {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.db
}

// IsConnected returns whether the runtime is connected to the database
func (r *DBRuntime) IsConnected() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.connected
}

// Exec executes a query without returning rows
func (r *DBRuntime) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if !r.IsConnected() {
		return nil, fmt.Errorf("database not connected")
	}
	return r.db.ExecContext(ctx, query, args...)
}

// Query executes a query that returns rows
func (r *DBRuntime) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if !r.IsConnected() {
		return nil, fmt.Errorf("database not connected")
	}
	return r.db.QueryContext(ctx, query, args...)
}

// QueryRow executes a query that returns at most one row
func (r *DBRuntime) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if !r.IsConnected() {
		return nil
	}
	return r.db.QueryRowContext(ctx, query, args...)
}

// Begin starts a new transaction
func (r *DBRuntime) Begin(ctx context.Context) (*sql.Tx, error) {
	if !r.IsConnected() {
		return nil, fmt.Errorf("database not connected")
	}
	return r.db.BeginTx(ctx, nil)
}

// Stats returns database connection pool statistics
func (r *DBRuntime) Stats() sql.DBStats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.db == nil {
		return sql.DBStats{}
	}
	return r.db.Stats()
}

// HealthCheck performs a health check on the database connection
func (r *DBRuntime) HealthCheck(ctx context.Context) error {
	if !r.IsConnected() {
		return fmt.Errorf("database not connected")
	}
	return r.db.PingContext(ctx)
}

// Example usage
func main() {
	// Create a new database runtime with configuration
	config := &DBConfig{
		DSN: "user/password@localhost:1521/XE", // Example Oracle DSN
	}
	
	runtime := NewDBRuntime(config)
	
	// Connect to the database
	if err := runtime.Connect(); err != nil {
		fmt.Printf("Failed to connect: %v\n", err)
		return
	}
	defer runtime.Disconnect()
	
	// Perform health check
	ctx := context.Background()
	if err := runtime.HealthCheck(ctx); err != nil {
		fmt.Printf("Health check failed: %v\n", err)
		return
	}
	
	fmt.Println("Database runtime is ready!")
	fmt.Printf("Connection stats: %+v\n", runtime.Stats())
}
