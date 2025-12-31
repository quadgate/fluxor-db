package main

import (
	"os"
	"testing"
	"time"
)

func TestNewConfigBuilder(t *testing.T) {
	builder := NewConfigBuilder()
	if builder == nil {
		t.Fatal("NewConfigBuilder returned nil")
	}

	if builder.config == nil {
		t.Fatal("Config is nil")
	}
}

func TestConfigBuilder_WithDSN(t *testing.T) {
	builder := NewConfigBuilder()
	builder.WithDSN("test@localhost:1521/XE")

	if builder.config.DSN != "test@localhost:1521/XE" {
		t.Errorf("Expected DSN 'test@localhost:1521/XE', got '%s'", builder.config.DSN)
	}
}

func TestConfigBuilder_WithConnectionPool(t *testing.T) {
	builder := NewConfigBuilder()
	builder.WithConnectionPool(100, 20)

	if builder.config.MaxOpenConns != 100 {
		t.Errorf("Expected MaxOpenConns 100, got %d", builder.config.MaxOpenConns)
	}

	if builder.config.MaxIdleConns != 20 {
		t.Errorf("Expected MaxIdleConns 20, got %d", builder.config.MaxIdleConns)
	}
}

func TestConfigBuilder_WithConnectionLifetime(t *testing.T) {
	builder := NewConfigBuilder()
	maxLifetime := 60 * time.Minute
	maxIdleTime := 20 * time.Minute
	builder.WithConnectionLifetime(maxLifetime, maxIdleTime)

	if builder.config.ConnMaxLifetime != maxLifetime {
		t.Errorf("Expected ConnMaxLifetime %v, got %v", maxLifetime, builder.config.ConnMaxLifetime)
	}

	if builder.config.ConnMaxIdleTime != maxIdleTime {
		t.Errorf("Expected ConnMaxIdleTime %v, got %v", maxIdleTime, builder.config.ConnMaxIdleTime)
	}
}

func TestConfigBuilder_WithLeakDetection(t *testing.T) {
	builder := NewConfigBuilder()
	threshold := 15 * time.Minute
	builder.WithLeakDetection(true, threshold)

	if !builder.config.EnableLeakDetection {
		t.Error("Expected EnableLeakDetection to be true")
	}

	if builder.config.LeakDetectionThreshold != threshold {
		t.Errorf("Expected LeakDetectionThreshold %v, got %v", threshold, builder.config.LeakDetectionThreshold)
	}
}

func TestConfigBuilder_WithCircuitBreaker(t *testing.T) {
	builder := NewConfigBuilder()
	maxFailures := 10
	resetTimeout := 120 * time.Second
	halfOpenTimeout := 20 * time.Second
	builder.WithCircuitBreaker(maxFailures, resetTimeout, halfOpenTimeout)

	if builder.config.CircuitBreakerMaxFailures != maxFailures {
		t.Errorf("Expected CircuitBreakerMaxFailures %d, got %d", maxFailures, builder.config.CircuitBreakerMaxFailures)
	}

	if builder.config.CircuitBreakerResetTimeout != resetTimeout {
		t.Errorf("Expected CircuitBreakerResetTimeout %v, got %v", resetTimeout, builder.config.CircuitBreakerResetTimeout)
	}

	if builder.config.CircuitBreakerHalfOpenTimeout != halfOpenTimeout {
		t.Errorf("Expected CircuitBreakerHalfOpenTimeout %v, got %v", halfOpenTimeout, builder.config.CircuitBreakerHalfOpenTimeout)
	}
}

func TestConfigBuilder_WithRateLimit(t *testing.T) {
	builder := NewConfigBuilder()
	builder.WithRateLimit(2000)

	if builder.config.MaxRequestsPerSecond != 2000 {
		t.Errorf("Expected MaxRequestsPerSecond 2000, got %d", builder.config.MaxRequestsPerSecond)
	}
}

func TestConfigBuilder_WithQuerySettings(t *testing.T) {
	builder := NewConfigBuilder()
	stmtCacheSize := 500
	slowQueryThreshold := 2 * time.Second
	queryTimeout := 60 * time.Second
	builder.WithQuerySettings(stmtCacheSize, slowQueryThreshold, queryTimeout)

	if builder.config.StmtCacheSize != stmtCacheSize {
		t.Errorf("Expected StmtCacheSize %d, got %d", stmtCacheSize, builder.config.StmtCacheSize)
	}

	if builder.config.SlowQueryThreshold != slowQueryThreshold {
		t.Errorf("Expected SlowQueryThreshold %v, got %v", slowQueryThreshold, builder.config.SlowQueryThreshold)
	}

	if builder.config.QueryTimeout != queryTimeout {
		t.Errorf("Expected QueryTimeout %v, got %v", queryTimeout, builder.config.QueryTimeout)
	}
}

func TestConfigBuilder_WithRetryPolicy(t *testing.T) {
	builder := NewConfigBuilder()
	maxRetries := 5
	backoff := 200 * time.Millisecond
	builder.WithRetryPolicy(maxRetries, backoff)

	if builder.config.MaxRetries != maxRetries {
		t.Errorf("Expected MaxRetries %d, got %d", maxRetries, builder.config.MaxRetries)
	}

	if builder.config.RetryBackoff != backoff {
		t.Errorf("Expected RetryBackoff %v, got %v", backoff, builder.config.RetryBackoff)
	}
}

func TestConfigBuilder_Build(t *testing.T) {
	builder := NewConfigBuilder().
		WithDSN("test@localhost:1521/XE").
		WithConnectionPool(50, 10)

	config := builder.Build()
	if config == nil {
		t.Fatal("Build returned nil")
	}

	if config.DSN != "test@localhost:1521/XE" {
		t.Errorf("Expected DSN 'test@localhost:1521/XE', got '%s'", config.DSN)
	}
}

func TestConfigBuilder_Validate(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*ConfigBuilder)
		wantErr bool
	}{
		{
			name: "valid config",
			setup: func(b *ConfigBuilder) {
				b.WithDSN("test@localhost:1521/XE").WithConnectionPool(10, 5)
			},
			wantErr: false,
		},
		{
			name: "missing DSN",
			setup: func(b *ConfigBuilder) {
				b.WithConnectionPool(10, 5)
			},
			wantErr: true,
		},
		{
			name: "zero MaxOpenConns",
			setup: func(b *ConfigBuilder) {
				b.WithDSN("test@localhost:1521/XE").WithConnectionPool(0, 5)
			},
			wantErr: true,
		},
		{
			name: "MaxIdleConns exceeds MaxOpenConns",
			setup: func(b *ConfigBuilder) {
				b.WithDSN("test@localhost:1521/XE").WithConnectionPool(10, 20)
			},
			wantErr: true,
		},
		{
			name: "zero MaxIdleConns",
			setup: func(b *ConfigBuilder) {
				b.WithDSN("test@localhost:1521/XE").WithConnectionPool(10, 0)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewConfigBuilder()
			tt.setup(builder)
			err := builder.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	if config == nil {
		t.Fatal("DefaultConfig returned nil")
	}

	// Check that defaults are set
	if config.MaxOpenConns == 0 {
		t.Error("MaxOpenConns should have a default value")
	}

	if config.MaxIdleConns == 0 {
		t.Error("MaxIdleConns should have a default value")
	}
}

func TestGetEnv(t *testing.T) {
	// Test with environment variable set
	os.Setenv("TEST_ENV_VAR", "test_value")
	defer os.Unsetenv("TEST_ENV_VAR")

	value := getEnv("TEST_ENV_VAR", "default")
	if value != "test_value" {
		t.Errorf("Expected 'test_value', got '%s'", value)
	}

	// Test with default value
	value = getEnv("NONEXISTENT_VAR", "default")
	if value != "default" {
		t.Errorf("Expected 'default', got '%s'", value)
	}
}

func TestGetEnvInt(t *testing.T) {
	os.Setenv("TEST_INT_VAR", "42")
	defer os.Unsetenv("TEST_INT_VAR")

	value := getEnvInt("TEST_INT_VAR", 0)
	if value != 42 {
		t.Errorf("Expected 42, got %d", value)
	}

	value = getEnvInt("NONEXISTENT_INT_VAR", 100)
	if value != 100 {
		t.Errorf("Expected 100, got %d", value)
	}
}

func TestGetEnvBool(t *testing.T) {
	os.Setenv("TEST_BOOL_VAR", "true")
	defer os.Unsetenv("TEST_BOOL_VAR")

	value := getEnvBool("TEST_BOOL_VAR", false)
	if !value {
		t.Error("Expected true, got false")
	}

	value = getEnvBool("NONEXISTENT_BOOL_VAR", false)
	if value {
		t.Error("Expected false, got true")
	}
}

func TestGetEnvDuration(t *testing.T) {
	os.Setenv("TEST_DURATION_VAR", "5m")
	defer os.Unsetenv("TEST_DURATION_VAR")

	value := getEnvDuration("TEST_DURATION_VAR", time.Minute)
	if value != 5*time.Minute {
		t.Errorf("Expected 5m, got %v", value)
	}

	value = getEnvDuration("NONEXISTENT_DURATION_VAR", 10*time.Second)
	if value != 10*time.Second {
		t.Errorf("Expected 10s, got %v", value)
	}
}
