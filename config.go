package main

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// ConfigBuilder provides a fluent interface for building RuntimeConfig
type ConfigBuilder struct {
	config *RuntimeConfig
}

// NewConfigBuilder creates a new configuration builder with sensible defaults
func NewConfigBuilder() *ConfigBuilder {
	return &ConfigBuilder{
		config: DefaultConfig(),
	}
}

// DefaultConfig returns a configuration with production-ready defaults
func DefaultConfig() *RuntimeConfig {
	dbType := DatabaseType(getEnv("DB_TYPE", string(DatabaseTypeSQLite)))
	validationQuery := "SELECT 1"
	if dbType == DatabaseTypeOracle {
		validationQuery = "SELECT 1 FROM DUAL"
	}

	dsn := getEnv("DB_DSN", "")
	if dsn == "" && dbType == DatabaseTypeSQLite {
		dsn = ":memory:" // Default to in-memory SQLite
	}

	return &RuntimeConfig{
		// Database type
		DatabaseType:    dbType,

		// Basic connection settings
		DSN:             dsn,
		MaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 50),
		MaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 10),
		ConnMaxLifetime: getEnvDuration("DB_CONN_MAX_LIFETIME", 30*time.Minute),
		ConnMaxIdleTime: getEnvDuration("DB_CONN_MAX_IDLE_TIME", 10*time.Minute),

		// Advanced connection features
		LeakDetectionThreshold: getEnvDuration("DB_LEAK_DETECTION_THRESHOLD", 10*time.Minute),
		ValidationQuery:        getEnv("DB_VALIDATION_QUERY", validationQuery),
		ValidationTimeout:      getEnvDuration("DB_VALIDATION_TIMEOUT", 5*time.Second),
		WarmupConnections:      getEnvInt("DB_WARMUP_CONNECTIONS", 5),
		WarmupTimeout:          getEnvDuration("DB_WARMUP_TIMEOUT", 30*time.Second),
		ConnectionTimeout:      getEnvDuration("DB_CONNECTION_TIMEOUT", 30*time.Second),
		EnableLeakDetection:    getEnvBool("DB_ENABLE_LEAK_DETECTION", true),

		// Circuit breaker settings
		CircuitBreakerMaxFailures:     getEnvInt("DB_CB_MAX_FAILURES", 5),
		CircuitBreakerResetTimeout:    getEnvDuration("DB_CB_RESET_TIMEOUT", 60*time.Second),
		CircuitBreakerHalfOpenTimeout: getEnvDuration("DB_CB_HALF_OPEN_TIMEOUT", 10*time.Second),
		MaxRequestsPerSecond:          getEnvInt64("DB_MAX_REQUESTS_PER_SEC", 1000),
		MaxConcurrentConnections:      getEnvInt64("DB_MAX_CONCURRENT_CONNECTIONS", 100),

		// Query settings
		StmtCacheSize:      getEnvInt("DB_STMT_CACHE_SIZE", 200),
		SlowQueryThreshold: getEnvDuration("DB_SLOW_QUERY_THRESHOLD", 1*time.Second),
		QueryTimeout:       getEnvDuration("DB_QUERY_TIMEOUT", 30*time.Second),
		MaxRetries:         getEnvInt("DB_MAX_RETRIES", 3),
		RetryBackoff:       getEnvDuration("DB_RETRY_BACKOFF", 100*time.Millisecond),

		// Backpressure defaults (drop by default for backward compatibility)
		BackpressureMode:    getEnv("DB_BACKPRESSURE_MODE", "drop"),
		BackpressureTimeout: getEnvDuration("DB_BACKPRESSURE_TIMEOUT", 0),

		// In-memory optimizations
		EnableAggressiveCaching: getEnvBool("DB_AGGRESSIVE_CACHING", false),
		CacheDefaultTTL:         getEnvDuration("DB_CACHE_DEFAULT_TTL", 300*time.Second),
		CacheCapacity:           getEnvInt("DB_CACHE_CAPACITY", 10000),
		InMemoryMode:            getEnvBool("DB_IN_MEMORY_MODE", false),
	}
}

// WithDatabaseType sets the database type (oracle, postgres, or mysql)
func (cb *ConfigBuilder) WithDatabaseType(dbType DatabaseType) *ConfigBuilder {
	cb.config.DatabaseType = dbType
	// Update validation query based on database type
	if dbType == DatabaseTypePostgreSQL || dbType == DatabaseTypeMySQL || dbType == DatabaseTypeSQLite {
		if cb.config.ValidationQuery == "SELECT 1 FROM DUAL" {
			cb.config.ValidationQuery = "SELECT 1"
		}
	} else if dbType == DatabaseTypeOracle {
		if cb.config.ValidationQuery == "SELECT 1" {
			cb.config.ValidationQuery = "SELECT 1 FROM DUAL"
		}
	}
	return cb
}

// WithDSN sets the database DSN
func (cb *ConfigBuilder) WithDSN(dsn string) *ConfigBuilder {
	cb.config.DSN = dsn
	return cb
}

// WithConnectionPool sets connection pool settings
func (cb *ConfigBuilder) WithConnectionPool(maxOpen, maxIdle int) *ConfigBuilder {
	cb.config.MaxOpenConns = maxOpen
	cb.config.MaxIdleConns = maxIdle
	return cb
}

// WithConnectionLifetime sets connection lifetime settings
func (cb *ConfigBuilder) WithConnectionLifetime(maxLifetime, maxIdleTime time.Duration) *ConfigBuilder {
	cb.config.ConnMaxLifetime = maxLifetime
	cb.config.ConnMaxIdleTime = maxIdleTime
	return cb
}

// WithLeakDetection enables/disables leak detection
func (cb *ConfigBuilder) WithLeakDetection(enabled bool, threshold time.Duration) *ConfigBuilder {
	cb.config.EnableLeakDetection = enabled
	cb.config.LeakDetectionThreshold = threshold
	return cb
}

// WithCircuitBreaker configures circuit breaker
func (cb *ConfigBuilder) WithCircuitBreaker(maxFailures int, resetTimeout, halfOpenTimeout time.Duration) *ConfigBuilder {
	cb.config.CircuitBreakerMaxFailures = maxFailures
	cb.config.CircuitBreakerResetTimeout = resetTimeout
	cb.config.CircuitBreakerHalfOpenTimeout = halfOpenTimeout
	return cb
}

// WithRateLimit sets rate limiting
func (cb *ConfigBuilder) WithRateLimit(maxRequestsPerSecond int64) *ConfigBuilder {
	cb.config.MaxRequestsPerSecond = maxRequestsPerSecond
	return cb
}

// WithBackpressure configures backpressure behavior when reaching concurrency limit
// mode: "drop" | "block" | "timeout"; timeout used only for "timeout" mode
func (cb *ConfigBuilder) WithBackpressure(mode string, timeout time.Duration) *ConfigBuilder {
	cb.config.BackpressureMode = mode
	cb.config.BackpressureTimeout = timeout
	return cb
}

// WithInMemoryMode enables in-memory optimizations for maximum performance
func (cb *ConfigBuilder) WithInMemoryMode(enabled bool) *ConfigBuilder {
	cb.config.InMemoryMode = enabled
	if enabled {
		// Auto-configure for in-memory performance
		cb.config.EnableAggressiveCaching = true
		cb.config.CacheDefaultTTL = 600 * time.Second // 10 minutes
		cb.config.CacheCapacity = 50000              // Large cache
		// Use SQLite in-memory if no DSN specified
		if cb.config.DSN == "" {
			cb.config.DatabaseType = DatabaseTypeSQLite
			cb.config.DSN = ":memory:"
		}
	}
	return cb
}

// WithAggressiveCaching enables aggressive caching with custom settings
func (cb *ConfigBuilder) WithAggressiveCaching(capacity int, defaultTTL time.Duration) *ConfigBuilder {
	cb.config.EnableAggressiveCaching = true
	cb.config.CacheCapacity = capacity
	cb.config.CacheDefaultTTL = defaultTTL
	return cb
}

// WithQuerySettings configures query-related settings
func (cb *ConfigBuilder) WithQuerySettings(stmtCacheSize int, slowQueryThreshold, queryTimeout time.Duration) *ConfigBuilder {
	cb.config.StmtCacheSize = stmtCacheSize
	cb.config.SlowQueryThreshold = slowQueryThreshold
	cb.config.QueryTimeout = queryTimeout
	return cb
}

// WithRetryPolicy configures retry policy
func (cb *ConfigBuilder) WithRetryPolicy(maxRetries int, backoff time.Duration) *ConfigBuilder {
	cb.config.MaxRetries = maxRetries
	cb.config.RetryBackoff = backoff
	return cb
}

// Build returns the configured RuntimeConfig
func (cb *ConfigBuilder) Build() *RuntimeConfig {
	return cb.config
}

// Validate validates the configuration
func (cb *ConfigBuilder) Validate() error {
	if cb.config.DSN == "" {
		return fmt.Errorf("DSN is required")
	}
	if cb.config.MaxOpenConns <= 0 {
		return fmt.Errorf("MaxOpenConns must be greater than 0")
	}
	if cb.config.MaxIdleConns > cb.config.MaxOpenConns {
		return fmt.Errorf("MaxIdleConns cannot exceed MaxOpenConns")
	}
	if cb.config.MaxIdleConns <= 0 {
		return fmt.Errorf("MaxIdleConns must be greater than 0")
	}
	return nil
}

// Helper functions for environment variables
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
