package main

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ConnectionManager handles advanced connection lifecycle management
type ConnectionManager struct {
	db                *sql.DB
	config            *AdvancedConfig
	mu                sync.RWMutex
	activeConnections map[uint64]*TrackedConnection
	connectionID      uint64
	leakDetector      *LeakDetector
	validator         *ConnectionValidator
	warmupDone        atomic.Bool
}

// TrackedConnection tracks individual connections for leak detection
type TrackedConnection struct {
	ID         uint64
	AcquiredAt time.Time
	LastUsedAt time.Time
	QueryCount int64
	StackTrace string
	mu         sync.RWMutex
}

// LeakDetector monitors for connection leaks
type LeakDetector struct {
	maxConnectionAge time.Duration
	checkInterval    time.Duration
	stopChan         chan struct{}
	leakCallback     func(connID uint64, age time.Duration)
}

// ConnectionValidator validates connections before use
type ConnectionValidator struct {
	validationQuery string
	timeout         time.Duration
	maxRetries      int
	retryBackoff    time.Duration
}

// AdvancedConfig extends basic configuration with advanced features
type AdvancedConfig struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration

	// Advanced features
	LeakDetectionThreshold time.Duration
	ValidationQuery        string
	ValidationTimeout      time.Duration
	WarmupConnections      int
	WarmupTimeout          time.Duration
	ConnectionTimeout      time.Duration
	EnableMetrics          bool
	EnableLeakDetection    bool
}

// NewConnectionManager creates a new advanced connection manager
func NewConnectionManager(config *AdvancedConfig) *ConnectionManager {
	cm := &ConnectionManager{
		config:            config,
		activeConnections: make(map[uint64]*TrackedConnection),
		leakDetector:      NewLeakDetector(config),
		validator:         NewConnectionValidator(config),
	}

	// Set defaults
	if config.MaxOpenConns == 0 {
		config.MaxOpenConns = 25
	}
	if config.MaxIdleConns == 0 {
		config.MaxIdleConns = 5
	}
	if config.ConnMaxLifetime == 0 {
		config.ConnMaxLifetime = 30 * time.Minute
	}
	if config.ConnMaxIdleTime == 0 {
		config.ConnMaxIdleTime = 10 * time.Minute
	}
	if config.LeakDetectionThreshold == 0 {
		config.LeakDetectionThreshold = 10 * time.Minute
	}
	if config.ValidationQuery == "" {
		config.ValidationQuery = "SELECT 1 FROM DUAL"
	}
	if config.ValidationTimeout == 0 {
		config.ValidationTimeout = 5 * time.Second
	}
	if config.ConnectionTimeout == 0 {
		config.ConnectionTimeout = 30 * time.Second
	}

	return cm
}

// Open creates and configures the database connection pool
func (cm *ConnectionManager) Open() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.db != nil {
		return nil
	}

	// Open database connection - godror handles Oracle-specific pooling
	db, err := sql.Open("godror", cm.config.DSN)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(cm.config.MaxOpenConns)
	db.SetMaxIdleConns(cm.config.MaxIdleConns)
	db.SetConnMaxLifetime(cm.config.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cm.config.ConnMaxIdleTime)

	// Validate initial connection
	ctx, cancel := context.WithTimeout(context.Background(), cm.config.ConnectionTimeout)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping database: %w", err)
	}

	cm.db = db

	// Start leak detection if enabled
	if cm.config.EnableLeakDetection {
		cm.leakDetector.Start(cm)
	}

	// Warm up connections
	if cm.config.WarmupConnections > 0 {
		go cm.warmupConnections()
	}

	return nil
}

// warmupConnections pre-creates connections to reduce latency
func (cm *ConnectionManager) warmupConnections() {
	if cm.warmupDone.Load() {
		return
	}
	defer cm.warmupDone.Store(true)

	ctx, cancel := context.WithTimeout(context.Background(), cm.config.WarmupTimeout)
	defer cancel()

	if cm.config.WarmupTimeout == 0 {
		ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
	}

	// Create warmup connections
	for i := 0; i < cm.config.WarmupConnections && i < cm.config.MaxIdleConns; i++ {
		conn, err := cm.db.Conn(ctx)
		if err != nil {
			continue
		}
		conn.Close()
	}
}

// AcquireConnection acquires a connection and tracks it
func (cm *ConnectionManager) AcquireConnection(ctx context.Context) (*sql.Conn, error) {
	if cm.db == nil {
		return nil, fmt.Errorf("database not opened")
	}

	conn, err := cm.db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire connection: %w", err)
	}

	// Validate connection if validator is configured
	if cm.validator != nil {
		if err := cm.validator.Validate(ctx, conn); err != nil {
			conn.Close()
			return nil, fmt.Errorf("connection validation failed: %w", err)
		}
	}

	// Track connection for leak detection
	if cm.config.EnableLeakDetection {
		cm.trackConnection(conn)
	}

	return conn, nil
}

// trackConnection tracks a connection for leak detection
func (cm *ConnectionManager) trackConnection(conn *sql.Conn) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	id := atomic.AddUint64(&cm.connectionID, 1)
	tracked := &TrackedConnection{
		ID:         id,
		AcquiredAt: time.Now(),
		LastUsedAt: time.Now(),
	}

	cm.activeConnections[id] = tracked
}

// ReleaseConnection releases a tracked connection
func (cm *ConnectionManager) ReleaseConnection(conn *sql.Conn) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Remove from tracking (simplified - in real implementation would use connection ID)
	conn.Close()
}

// Close closes all connections and stops monitoring
func (cm *ConnectionManager) Close() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.leakDetector != nil {
		cm.leakDetector.Stop()
	}

	if cm.db != nil {
		if err := cm.db.Close(); err != nil {
			return fmt.Errorf("failed to close database: %w", err)
		}
		cm.db = nil
	}

	cm.activeConnections = make(map[uint64]*TrackedConnection)
	return nil
}

// DB returns the underlying database connection pool
func (cm *ConnectionManager) DB() *sql.DB {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.db
}

// NewLeakDetector creates a new leak detector
func NewLeakDetector(config *AdvancedConfig) *LeakDetector {
	if !config.EnableLeakDetection {
		return nil
	}

	return &LeakDetector{
		maxConnectionAge: config.LeakDetectionThreshold,
		checkInterval:    30 * time.Second,
		stopChan:         make(chan struct{}),
	}
}

// Start begins leak detection monitoring
func (ld *LeakDetector) Start(cm *ConnectionManager) {
	if ld == nil {
		return
	}

	go func() {
		ticker := time.NewTicker(ld.checkInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				ld.checkLeaks(cm)
			case <-ld.stopChan:
				return
			}
		}
	}()
}

// Stop stops leak detection
func (ld *LeakDetector) Stop() {
	if ld == nil {
		return
	}
	close(ld.stopChan)
}

// checkLeaks checks for connection leaks
func (ld *LeakDetector) checkLeaks(cm *ConnectionManager) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	now := time.Now()
	for id, conn := range cm.activeConnections {
		age := now.Sub(conn.AcquiredAt)
		if age > ld.maxConnectionAge {
			if ld.leakCallback != nil {
				ld.leakCallback(id, age)
			}
		}
	}
}

// NewConnectionValidator creates a new connection validator
func NewConnectionValidator(config *AdvancedConfig) *ConnectionValidator {
	return &ConnectionValidator{
		validationQuery: config.ValidationQuery,
		timeout:         config.ValidationTimeout,
		maxRetries:      3,
		retryBackoff:    100 * time.Millisecond,
	}
}

// Validate validates a connection
func (cv *ConnectionValidator) Validate(ctx context.Context, conn *sql.Conn) error {
	ctx, cancel := context.WithTimeout(ctx, cv.timeout)
	defer cancel()

	var lastErr error
	for i := 0; i < cv.maxRetries; i++ {
		var result int
		err := conn.QueryRowContext(ctx, cv.validationQuery).Scan(&result)
		if err == nil {
			return nil
		}
		lastErr = err
		time.Sleep(cv.retryBackoff * time.Duration(i+1))
	}

	return fmt.Errorf("validation failed after %d retries: %w", cv.maxRetries, lastErr)
}
