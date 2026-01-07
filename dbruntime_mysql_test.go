package main

import (
	"testing"
	"time"
)

func TestNewDBRuntimeMySQL(t *testing.T) {
	config := &RuntimeConfig{
		DatabaseType: DatabaseTypeMySQL,
		DSN:          "user:password@tcp(localhost:3306)/testdb?parseTime=true",
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

	if runtime.config.DatabaseType != DatabaseTypeMySQL {
		t.Errorf("Expected database type MySQL, got %s", runtime.config.DatabaseType)
	}

	if runtime.connManager == nil {
		t.Fatal("Connection manager is nil")
	}

	if runtime.gate == nil {
		t.Fatal("Connection gate is nil")
	}
}

func TestConfigBuilderWithMySQLDatabaseType(t *testing.T) {
	// Test MySQL
	mysqlConfig := NewConfigBuilder().
		WithDatabaseType(DatabaseTypeMySQL).
		WithDSN("user:password@tcp(localhost:3306)/dbname").
		Build()

	if mysqlConfig.DatabaseType != DatabaseTypeMySQL {
		t.Errorf("Expected MySQL database type, got %s", mysqlConfig.DatabaseType)
	}

	if mysqlConfig.ValidationQuery != "SELECT 1" {
		t.Errorf("Expected MySQL validation query, got %s", mysqlConfig.ValidationQuery)
	}
}

func TestMySQLValidationQuery(t *testing.T) {
	config := &AdvancedConfig{
		DatabaseType:      DatabaseTypeMySQL,
		DSN:               "user:password@tcp(localhost:3306)/testdb",
		ValidationQuery:   "", // Should be set automatically
		MaxOpenConns:      10,
		MaxIdleConns:      5,
		ConnMaxLifetime:   30 * time.Minute,
		ConnMaxIdleTime:   10 * time.Minute,
		ConnectionTimeout: 30 * time.Second,
	}

	cm := NewConnectionManager(config)
	
	if cm.config.ValidationQuery != "SELECT 1" {
		t.Errorf("Expected MySQL validation query 'SELECT 1', got '%s'", cm.config.ValidationQuery)
	}
}

func TestMySQLConnectionManager(t *testing.T) {
	config := &AdvancedConfig{
		DatabaseType:           DatabaseTypeMySQL,
		DSN:                    "user:password@tcp(localhost:3306)/testdb?parseTime=true",
		MaxOpenConns:           10,
		MaxIdleConns:           5,
		ConnMaxLifetime:        30 * time.Minute,
		ConnMaxIdleTime:        10 * time.Minute,
		LeakDetectionThreshold: 10 * time.Minute,
		ValidationQuery:        "SELECT 1",
		ValidationTimeout:      5 * time.Second,
		ConnectionTimeout:      30 * time.Second,
		EnableLeakDetection:    true,
	}

	cm := NewConnectionManager(config)
	
	if cm == nil {
		t.Fatal("ConnectionManager is nil")
	}

	if cm.config.DatabaseType != DatabaseTypeMySQL {
		t.Errorf("Expected MySQL database type, got %s", cm.config.DatabaseType)
	}

	if cm.validator == nil {
		t.Fatal("Validator is nil")
	}

	if cm.leakDetector == nil {
		t.Fatal("LeakDetector is nil")
	}
}

func TestMultipleDatabaseTypesWithMySQL(t *testing.T) {
	// Test that we can create runtimes for all three database types
	oracleRuntime := NewDBRuntime(&RuntimeConfig{
		DatabaseType: DatabaseTypeOracle,
		DSN:          "user/password@localhost:1521/XE",
		MaxOpenConns: 10,
		MaxIdleConns: 5,
	})

	postgresRuntime := NewDBRuntime(&RuntimeConfig{
		DatabaseType: DatabaseTypePostgreSQL,
		DSN:          "postgres://user:pass@localhost:5432/db",
		MaxOpenConns: 10,
		MaxIdleConns: 5,
	})

	mysqlRuntime := NewDBRuntime(&RuntimeConfig{
		DatabaseType: DatabaseTypeMySQL,
		DSN:          "user:pass@tcp(localhost:3306)/db",
		MaxOpenConns: 10,
		MaxIdleConns: 5,
	})

	if oracleRuntime == nil {
		t.Error("Oracle runtime is nil")
	}

	if postgresRuntime == nil {
		t.Error("PostgreSQL runtime is nil")
	}

	if mysqlRuntime == nil {
		t.Error("MySQL runtime is nil")
	}

	if oracleRuntime.config.DatabaseType != DatabaseTypeOracle {
		t.Errorf("Expected Oracle type, got %s", oracleRuntime.config.DatabaseType)
	}

	if postgresRuntime.config.DatabaseType != DatabaseTypePostgreSQL {
		t.Errorf("Expected PostgreSQL type, got %s", postgresRuntime.config.DatabaseType)
	}

	if mysqlRuntime.config.DatabaseType != DatabaseTypeMySQL {
		t.Errorf("Expected MySQL type, got %s", mysqlRuntime.config.DatabaseType)
	}
}

func TestConfigBuilderValidationMySQL(t *testing.T) {
	tests := []struct {
		name      string
		config    *RuntimeConfig
		expectErr bool
	}{
		{
			name: "Valid MySQL Config",
			config: &RuntimeConfig{
				DatabaseType: DatabaseTypeMySQL,
				DSN:          "user:password@tcp(localhost:3306)/dbname",
				MaxOpenConns: 10,
				MaxIdleConns: 5,
			},
			expectErr: false,
		},
		{
			name: "MySQL with parseTime",
			config: &RuntimeConfig{
				DatabaseType: DatabaseTypeMySQL,
				DSN:          "user:password@tcp(localhost:3306)/dbname?parseTime=true",
				MaxOpenConns: 10,
				MaxIdleConns: 5,
			},
			expectErr: false,
		},
		{
			name: "MySQL Empty DSN",
			config: &RuntimeConfig{
				DatabaseType: DatabaseTypeMySQL,
				DSN:          "",
				MaxOpenConns: 10,
				MaxIdleConns: 5,
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb := NewConfigBuilder()
			cb.config = tt.config
			err := cb.Validate()
			
			if tt.expectErr && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestAllDatabaseTypesValidationQueries(t *testing.T) {
	tests := []struct {
		dbType          DatabaseType
		expectedQuery   string
	}{
		{DatabaseTypeOracle, "SELECT 1 FROM DUAL"},
		{DatabaseTypePostgreSQL, "SELECT 1"},
		{DatabaseTypeMySQL, "SELECT 1"},
	}

	for _, tt := range tests {
		t.Run(string(tt.dbType), func(t *testing.T) {
			config := &AdvancedConfig{
				DatabaseType:      tt.dbType,
				DSN:               "test",
				ValidationQuery:   "", // Should be set automatically
				MaxOpenConns:      10,
				MaxIdleConns:      5,
				ConnMaxLifetime:   30 * time.Minute,
				ConnMaxIdleTime:   10 * time.Minute,
				ConnectionTimeout: 30 * time.Second,
			}

			cm := NewConnectionManager(config)
			
			if cm.config.ValidationQuery != tt.expectedQuery {
				t.Errorf("Expected validation query '%s', got '%s'", 
					tt.expectedQuery, cm.config.ValidationQuery)
			}
		})
	}
}

func TestMySQLDSNFormats(t *testing.T) {
	validDSNs := []string{
		"user:password@tcp(localhost:3306)/dbname",
		"user:password@tcp(127.0.0.1:3306)/dbname",
		"user:password@tcp(localhost:3306)/dbname?parseTime=true",
		"user:password@tcp(localhost:3306)/dbname?charset=utf8mb4",
		"user:password@tcp(localhost:3306)/dbname?parseTime=true&charset=utf8mb4&loc=Local",
		"user:password@/dbname", // Unix socket
	}

	for _, dsn := range validDSNs {
		t.Run(dsn, func(t *testing.T) {
			config := &RuntimeConfig{
				DatabaseType: DatabaseTypeMySQL,
				DSN:          dsn,
				MaxOpenConns: 10,
				MaxIdleConns: 5,
			}

			runtime := NewDBRuntime(config)
			if runtime == nil {
				t.Error("Failed to create runtime with valid DSN")
			}

			if runtime.config.DSN != dsn {
				t.Errorf("DSN mismatch: expected %s, got %s", dsn, runtime.config.DSN)
			}
		})
	}
}

func TestMySQLConfigWithAllFeatures(t *testing.T) {
	config := NewConfigBuilder().
		WithDatabaseType(DatabaseTypeMySQL).
		WithDSN("user:password@tcp(localhost:3306)/dbname?parseTime=true").
		WithConnectionPool(100, 20).
		WithConnectionLifetime(30*time.Minute, 10*time.Minute).
		WithLeakDetection(true, 10*time.Minute).
		WithCircuitBreaker(5, 60*time.Second, 10*time.Second).
		WithRateLimit(2000).
		WithQuerySettings(300, 500*time.Millisecond, 30*time.Second).
		WithRetryPolicy(3, 100*time.Millisecond).
		Build()

	if config.DatabaseType != DatabaseTypeMySQL {
		t.Errorf("Expected MySQL type, got %s", config.DatabaseType)
	}

	if config.MaxOpenConns != 100 {
		t.Errorf("Expected MaxOpenConns 100, got %d", config.MaxOpenConns)
	}

	if config.MaxIdleConns != 20 {
		t.Errorf("Expected MaxIdleConns 20, got %d", config.MaxIdleConns)
	}

	if config.StmtCacheSize != 300 {
		t.Errorf("Expected StmtCacheSize 300, got %d", config.StmtCacheSize)
	}

	if config.MaxRequestsPerSecond != 2000 {
		t.Errorf("Expected MaxRequestsPerSecond 2000, got %d", config.MaxRequestsPerSecond)
	}
}
