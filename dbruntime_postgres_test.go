package main

import (
	"testing"
	"time"
)

func TestNewDBRuntimePostgreSQL(t *testing.T) {
	config := &RuntimeConfig{
		DatabaseType: DatabaseTypePostgreSQL,
		DSN:          "postgres://test:test@localhost:5432/testdb?sslmode=disable",
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

	if runtime.config.DatabaseType != DatabaseTypePostgreSQL {
		t.Errorf("Expected database type PostgreSQL, got %s", runtime.config.DatabaseType)
	}

	if runtime.connManager == nil {
		t.Fatal("Connection manager is nil")
	}

	if runtime.gate == nil {
		t.Fatal("Connection gate is nil")
	}
}

func TestConfigBuilderWithDatabaseType(t *testing.T) {
	// Test Oracle
	oracleConfig := NewConfigBuilder().
		WithDatabaseType(DatabaseTypeOracle).
		WithDSN("user/password@localhost:1521/XE").
		Build()

	if oracleConfig.DatabaseType != DatabaseTypeOracle {
		t.Errorf("Expected Oracle database type, got %s", oracleConfig.DatabaseType)
	}

	if oracleConfig.ValidationQuery != "SELECT 1 FROM DUAL" {
		t.Errorf("Expected Oracle validation query, got %s", oracleConfig.ValidationQuery)
	}

	// Test PostgreSQL
	postgresConfig := NewConfigBuilder().
		WithDatabaseType(DatabaseTypePostgreSQL).
		WithDSN("postgres://user:password@localhost:5432/db").
		Build()

	if postgresConfig.DatabaseType != DatabaseTypePostgreSQL {
		t.Errorf("Expected PostgreSQL database type, got %s", postgresConfig.DatabaseType)
	}

	if postgresConfig.ValidationQuery != "SELECT 1" {
		t.Errorf("Expected PostgreSQL validation query, got %s", postgresConfig.ValidationQuery)
	}
}

func TestPostgreSQLValidationQuery(t *testing.T) {
	config := &AdvancedConfig{
		DatabaseType:      DatabaseTypePostgreSQL,
		DSN:               "postgres://test:test@localhost:5432/testdb",
		ValidationQuery:   "", // Should be set automatically
		MaxOpenConns:      10,
		MaxIdleConns:      5,
		ConnMaxLifetime:   30 * time.Minute,
		ConnMaxIdleTime:   10 * time.Minute,
		ConnectionTimeout: 30 * time.Second,
	}

	cm := NewConnectionManager(config)
	
	if cm.config.ValidationQuery != "SELECT 1" {
		t.Errorf("Expected PostgreSQL validation query 'SELECT 1', got '%s'", cm.config.ValidationQuery)
	}
}

func TestOracleValidationQuery(t *testing.T) {
	config := &AdvancedConfig{
		DatabaseType:      DatabaseTypeOracle,
		DSN:               "user/password@localhost:1521/XE",
		ValidationQuery:   "", // Should be set automatically
		MaxOpenConns:      10,
		MaxIdleConns:      5,
		ConnMaxLifetime:   30 * time.Minute,
		ConnMaxIdleTime:   10 * time.Minute,
		ConnectionTimeout: 30 * time.Second,
	}

	cm := NewConnectionManager(config)
	
	if cm.config.ValidationQuery != "SELECT 1 FROM DUAL" {
		t.Errorf("Expected Oracle validation query 'SELECT 1 FROM DUAL', got '%s'", cm.config.ValidationQuery)
	}
}

func TestDefaultDatabaseType(t *testing.T) {
	// When no database type is specified, it should default to Oracle
	config := &AdvancedConfig{
		DSN:               "user/password@localhost:1521/XE",
		MaxOpenConns:      10,
		MaxIdleConns:      5,
		ConnMaxLifetime:   30 * time.Minute,
		ConnMaxIdleTime:   10 * time.Minute,
		ConnectionTimeout: 30 * time.Second,
	}

	cm := NewConnectionManager(config)
	
	if cm.config.DatabaseType != DatabaseTypeOracle {
		t.Errorf("Expected default database type to be Oracle, got %s", cm.config.DatabaseType)
	}
}

func TestConfigBuilderValidation(t *testing.T) {
	tests := []struct {
		name      string
		config    *RuntimeConfig
		expectErr bool
	}{
		{
			name: "Valid Oracle Config",
			config: &RuntimeConfig{
				DatabaseType: DatabaseTypeOracle,
				DSN:          "user/password@localhost:1521/XE",
				MaxOpenConns: 10,
				MaxIdleConns: 5,
			},
			expectErr: false,
		},
		{
			name: "Valid PostgreSQL Config",
			config: &RuntimeConfig{
				DatabaseType: DatabaseTypePostgreSQL,
				DSN:          "postgres://user:pass@localhost:5432/db",
				MaxOpenConns: 10,
				MaxIdleConns: 5,
			},
			expectErr: false,
		},
		{
			name: "Empty DSN",
			config: &RuntimeConfig{
				DatabaseType: DatabaseTypePostgreSQL,
				DSN:          "",
				MaxOpenConns: 10,
				MaxIdleConns: 5,
			},
			expectErr: true,
		},
		{
			name: "Invalid MaxOpenConns",
			config: &RuntimeConfig{
				DatabaseType: DatabaseTypePostgreSQL,
				DSN:          "postgres://user:pass@localhost:5432/db",
				MaxOpenConns: -1,
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

func TestPostgreSQLConnectionManager(t *testing.T) {
	config := &AdvancedConfig{
		DatabaseType:           DatabaseTypePostgreSQL,
		DSN:                    "postgres://test:test@localhost:5432/testdb?sslmode=disable",
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

	if cm.config.DatabaseType != DatabaseTypePostgreSQL {
		t.Errorf("Expected PostgreSQL database type, got %s", cm.config.DatabaseType)
	}

	if cm.validator == nil {
		t.Fatal("Validator is nil")
	}

	if cm.leakDetector == nil {
		t.Fatal("LeakDetector is nil")
	}
}

func TestMultipleDatabaseTypes(t *testing.T) {
	// Test that we can create runtimes for both database types
	oracleRuntime := NewDBRuntime(&RuntimeConfig{
		DatabaseType: DatabaseTypeOracle,
		DSN:          "user/password@localhost:1521/XE",
		MaxOpenConns: 10,
	})

	postgresRuntime := NewDBRuntime(&RuntimeConfig{
		DatabaseType: DatabaseTypePostgreSQL,
		DSN:          "postgres://user:pass@localhost:5432/db",
		MaxOpenConns: 10,
	})

	if oracleRuntime == nil {
		t.Error("Oracle runtime is nil")
	}

	if postgresRuntime == nil {
		t.Error("PostgreSQL runtime is nil")
	}

	if oracleRuntime.config.DatabaseType != DatabaseTypeOracle {
		t.Errorf("Expected Oracle type, got %s", oracleRuntime.config.DatabaseType)
	}

	if postgresRuntime.config.DatabaseType != DatabaseTypePostgreSQL {
		t.Errorf("Expected PostgreSQL type, got %s", postgresRuntime.config.DatabaseType)
	}
}

func TestEnvironmentVariableDBType(t *testing.T) {
	// This would test environment variable support
	// In a real test, you'd set the environment variable first
	config := DefaultConfig()
	
	// Should default to Oracle if not set
	if config.DatabaseType == "" {
		t.Error("DatabaseType should not be empty")
	}
}
