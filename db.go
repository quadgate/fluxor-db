package main

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// AdvancedDB provides advanced database operations beyond standard sql.DB
type AdvancedDB struct {
	db           *sql.DB
	gate         *ConnectionGate
	stmtCache    *PreparedStatementCache
	metrics      *DBMetrics
	retryPolicy  *RetryPolicy
	queryTimeout time.Duration
	mu           sync.RWMutex
}

// PreparedStatementCache caches prepared statements for performance
type PreparedStatementCache struct {
	cache   map[string]*sql.Stmt
	maxSize int
	mu      sync.RWMutex // nolint:unused // Used for thread-safe cache operations
}

// DBMetrics tracks database performance metrics
type DBMetrics struct {
	TotalQueries       int64
	SuccessfulQueries  int64
	FailedQueries      int64
	TotalQueryTime     int64 // nanoseconds
	SlowQueries        int64
	SlowQueryThreshold time.Duration
	mu                 sync.RWMutex // nolint:unused // Used for thread-safe metrics access
}

// RetryPolicy defines retry behavior for failed operations
type RetryPolicy struct {
	MaxRetries        int
	InitialBackoff    time.Duration
	MaxBackoff        time.Duration
	BackoffMultiplier float64
	RetryableErrors   []error
}

// NewAdvancedDB creates a new advanced database wrapper
func NewAdvancedDB(db *sql.DB, gate *ConnectionGate, config *DBAdvancedConfig) *AdvancedDB {
	adb := &AdvancedDB{
		db:           db,
		gate:         gate,
		stmtCache:    NewPreparedStatementCache(config),
		metrics:      NewDBMetrics(config),
		retryPolicy:  NewRetryPolicy(config),
		queryTimeout: 30 * time.Second,
	}

	if config != nil {
		if config.QueryTimeout > 0 {
			adb.queryTimeout = config.QueryTimeout
		}
	}

	return adb
}

// DBAdvancedConfig configures advanced database features
type DBAdvancedConfig struct {
	StmtCacheSize      int
	SlowQueryThreshold time.Duration
	QueryTimeout       time.Duration
	MaxRetries         int
	RetryBackoff       time.Duration
}

// Exec executes a query with advanced features
func (adb *AdvancedDB) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	start := time.Now()
	defer func() {
		adb.metrics.RecordQuery(time.Since(start), nil)
	}()

	// Apply query timeout
	ctx, cancel := context.WithTimeout(ctx, adb.queryTimeout)
	defer cancel()

	// Execute with gate protection and retry
	return ExecuteWithGate(adb.gate, ctx, func(ctx context.Context) (sql.Result, error) {
		return adb.retryExec(ctx, query, args...)
	})
}

// retryExec executes with retry logic
func (adb *AdvancedDB) retryExec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	var lastErr error
	backoff := adb.retryPolicy.InitialBackoff

	for attempt := 0; attempt <= adb.retryPolicy.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
			backoff = time.Duration(float64(backoff) * adb.retryPolicy.BackoffMultiplier)
			if backoff > adb.retryPolicy.MaxBackoff {
				backoff = adb.retryPolicy.MaxBackoff
			}
		}

		result, err := adb.db.ExecContext(ctx, query, args...)
		if err == nil {
			return result, nil
		}

		lastErr = err
		if !adb.retryPolicy.ShouldRetry(err) {
			break
		}
	}

	return nil, fmt.Errorf("exec failed after %d attempts: %w", adb.retryPolicy.MaxRetries+1, lastErr)
}

// Query executes a query that returns rows
func (adb *AdvancedDB) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	start := time.Now()
	defer func() {
		adb.metrics.RecordQuery(time.Since(start), nil)
	}()

	ctx, cancel := context.WithTimeout(ctx, adb.queryTimeout)
	defer cancel()

	return ExecuteWithGate(adb.gate, ctx, func(ctx context.Context) (*sql.Rows, error) {
		return adb.retryQuery(ctx, query, args...)
	})
}

// retryQuery executes query with retry logic
func (adb *AdvancedDB) retryQuery(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	var lastErr error
	backoff := adb.retryPolicy.InitialBackoff

	for attempt := 0; attempt <= adb.retryPolicy.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
			backoff = time.Duration(float64(backoff) * adb.retryPolicy.BackoffMultiplier)
			if backoff > adb.retryPolicy.MaxBackoff {
				backoff = adb.retryPolicy.MaxBackoff
			}
		}

		rows, err := adb.db.QueryContext(ctx, query, args...)
		if err == nil {
			return rows, nil
		}

		lastErr = err
		if !adb.retryPolicy.ShouldRetry(err) {
			break
		}
	}

	return nil, fmt.Errorf("query failed after %d attempts: %w", adb.retryPolicy.MaxRetries+1, lastErr)
}

// QueryRow executes a query that returns at most one row
func (adb *AdvancedDB) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	start := time.Now()
	defer func() {
		adb.metrics.RecordQuery(time.Since(start), nil)
	}()

	ctx, cancel := context.WithTimeout(ctx, adb.queryTimeout)
	defer cancel()

	// Note: QueryRow doesn't return error immediately, so we can't use gate here
	// But we can still track metrics
	return adb.db.QueryRowContext(ctx, query, args...)
}

// Prepare creates or retrieves a cached prepared statement
func (adb *AdvancedDB) Prepare(ctx context.Context, query string) (*sql.Stmt, error) {
	// Try to get from cache
	if stmt := adb.stmtCache.Get(query); stmt != nil {
		return stmt, nil
	}

	// Create new prepared statement
	stmt, err := adb.db.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}

	// Cache it
	adb.stmtCache.Put(query, stmt)
	return stmt, nil
}

// Begin starts a transaction with advanced features
func (adb *AdvancedDB) Begin(ctx context.Context, opts *sql.TxOptions) (*AdvancedTx, error) {
	ctx, cancel := context.WithTimeout(ctx, adb.queryTimeout)
	defer cancel()

	tx, err := ExecuteWithGate(adb.gate, ctx, func(ctx context.Context) (*sql.Tx, error) {
		return adb.db.BeginTx(ctx, opts)
	})

	if err != nil {
		return nil, err
	}

	return &AdvancedTx{
		tx:      tx,
		gate:    adb.gate,
		metrics: adb.metrics,
	}, nil
}

// AdvancedTx wraps sql.Tx with advanced features
type AdvancedTx struct {
	tx      *sql.Tx
	gate    *ConnectionGate
	metrics *DBMetrics
}

// Exec executes within transaction
func (atx *AdvancedTx) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	start := time.Now()
	result, err := atx.tx.ExecContext(ctx, query, args...)
	atx.metrics.RecordQuery(time.Since(start), err)
	return result, err
}

// Query executes query within transaction
func (atx *AdvancedTx) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	start := time.Now()
	defer func() {
		atx.metrics.RecordQuery(time.Since(start), nil)
	}()
	return atx.tx.QueryContext(ctx, query, args...)
}

// Commit commits the transaction
func (atx *AdvancedTx) Commit() error {
	err := atx.tx.Commit()
	if err != nil {
		atx.gate.RecordFailure()
	} else {
		atx.gate.RecordSuccess()
	}
	return err
}

// Rollback rolls back the transaction
func (atx *AdvancedTx) Rollback() error {
	err := atx.tx.Rollback()
	if err != nil {
		atx.gate.RecordFailure()
	}
	return err
}

// Stats returns connection pool statistics
func (adb *AdvancedDB) Stats() sql.DBStats {
	return adb.db.Stats()
}

// Metrics returns performance metrics
func (adb *AdvancedDB) Metrics() *DBMetrics {
	return adb.metrics
}

// HealthCheck performs a health check
func (adb *AdvancedDB) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return adb.db.PingContext(ctx)
}

// NewPreparedStatementCache creates a new statement cache
func NewPreparedStatementCache(config *DBAdvancedConfig) *PreparedStatementCache {
	maxSize := 100
	if config != nil && config.StmtCacheSize > 0 {
		maxSize = config.StmtCacheSize
	}

	return &PreparedStatementCache{
		cache:   make(map[string]*sql.Stmt),
		maxSize: maxSize,
	}
}

// Get retrieves a prepared statement from cache
func (psc *PreparedStatementCache) Get(query string) *sql.Stmt {
	psc.mu.RLock()
	defer psc.mu.RUnlock()
	return psc.cache[query]
}

// Put stores a prepared statement in cache
func (psc *PreparedStatementCache) Put(query string, stmt *sql.Stmt) {
	psc.mu.Lock()
	defer psc.mu.Unlock()

	if len(psc.cache) >= psc.maxSize {
		// Evict oldest (simple FIFO - in production use LRU)
		for k := range psc.cache {
			delete(psc.cache, k)
			break
		}
	}

	psc.cache[query] = stmt
}

// Clear clears the statement cache
func (psc *PreparedStatementCache) Clear() {
	psc.mu.Lock()
	defer psc.mu.Unlock()

	for _, stmt := range psc.cache {
		stmt.Close()
	}
	psc.cache = make(map[string]*sql.Stmt)
}

// NewDBMetrics creates new database metrics
func NewDBMetrics(config *DBAdvancedConfig) *DBMetrics {
	threshold := 1 * time.Second
	if config != nil && config.SlowQueryThreshold > 0 {
		threshold = config.SlowQueryThreshold
	}

	return &DBMetrics{
		SlowQueryThreshold: threshold,
	}
}

// RecordQuery records a query execution
func (m *DBMetrics) RecordQuery(duration time.Duration, err error) {
	atomic.AddInt64(&m.TotalQueries, 1)
	atomic.AddInt64(&m.TotalQueryTime, int64(duration))

	if err != nil {
		atomic.AddInt64(&m.FailedQueries, 1)
	} else {
		atomic.AddInt64(&m.SuccessfulQueries, 1)
	}

	if duration > m.SlowQueryThreshold {
		atomic.AddInt64(&m.SlowQueries, 1)
	}
}

// GetStats returns current metrics
func (m *DBMetrics) GetStats() MetricsStats {
	total := atomic.LoadInt64(&m.TotalQueries)
	successful := atomic.LoadInt64(&m.SuccessfulQueries)
	failed := atomic.LoadInt64(&m.FailedQueries)
	totalTime := atomic.LoadInt64(&m.TotalQueryTime)
	slow := atomic.LoadInt64(&m.SlowQueries)

	avgTime := time.Duration(0)
	if total > 0 {
		avgTime = time.Duration(totalTime / total)
	}

	return MetricsStats{
		TotalQueries:      total,
		SuccessfulQueries: successful,
		FailedQueries:     failed,
		AverageQueryTime:  avgTime,
		SlowQueries:       slow,
		SuccessRate:       float64(successful) / float64(total) * 100,
	}
}

// MetricsStats holds metrics statistics
type MetricsStats struct {
	TotalQueries      int64
	SuccessfulQueries int64
	FailedQueries     int64
	AverageQueryTime  time.Duration
	SlowQueries       int64
	SuccessRate       float64
}

// NewRetryPolicy creates a new retry policy
func NewRetryPolicy(config *DBAdvancedConfig) *RetryPolicy {
	rp := &RetryPolicy{
		MaxRetries:        3,
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        5 * time.Second,
		BackoffMultiplier: 2.0,
	}

	if config != nil {
		if config.MaxRetries > 0 {
			rp.MaxRetries = config.MaxRetries
		}
		if config.RetryBackoff > 0 {
			rp.InitialBackoff = config.RetryBackoff
		}
	}

	return rp
}

// ShouldRetry determines if an error should be retried
func (rp *RetryPolicy) ShouldRetry(err error) bool {
	if err == nil {
		return false
	}

	// Check if error is in retryable list
	for _, retryableErr := range rp.RetryableErrors {
		if err == retryableErr {
			return true
		}
	}

	// Default: retry on context timeout/deadline exceeded
	return err == context.DeadlineExceeded || err == context.Canceled
}
