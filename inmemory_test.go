package main

import (
	"context"
	"testing"
	"time"
)

func TestInMemoryRuntime(t *testing.T) {
	config := NewConfigBuilder().
		WithDatabaseType(DatabaseTypeSQLite).
		WithDSN(":memory:").
		WithInMemoryMode(true).
		Build()

	runtime := NewDBRuntime(config)
	
	if err := runtime.Connect(); err != nil {
		t.Fatalf("Failed to connect to in-memory database: %v", err)
	}
	defer runtime.Disconnect()

	if runtime.config.DatabaseType != DatabaseTypeSQLite {
		t.Errorf("Expected SQLite database type, got %s", runtime.config.DatabaseType)
	}

	if runtime.config.DSN != ":memory:" {
		t.Errorf("Expected :memory: DSN, got %s", runtime.config.DSN)
	}

	if !runtime.config.EnableAggressiveCaching {
		t.Error("Expected aggressive caching to be enabled")
	}

	if runtime.cache == nil {
		t.Error("Expected cache to be configured automatically")
	}

	ctx := context.Background()

	// Test table creation
	_, err := runtime.Exec(ctx, "CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Test insert
	result, err := runtime.Exec(ctx, "INSERT INTO test (name) VALUES (?)", "test_value")
	if err != nil {
		t.Fatalf("Failed to insert: %v", err)
	}

	affected, _ := result.RowsAffected()
	if affected != 1 {
		t.Errorf("Expected 1 row affected, got %d", affected)
	}

	// Test cached query
	cols, rows, fromCache, err := runtime.QueryCached(ctx, "test_query", 60*time.Second, "SELECT * FROM test")
	if err != nil {
		t.Fatalf("Failed to query: %v", err)
	}

	if len(cols) != 2 {
		t.Errorf("Expected 2 columns, got %d", len(cols))
	}

	if len(rows) != 1 {
		t.Errorf("Expected 1 row, got %d", len(rows))
	}

	if fromCache {
		t.Error("First query should not be from cache")
	}

	// Test cache hit
	_, _, fromCache2, err := runtime.QueryCached(ctx, "test_query", 60*time.Second, "SELECT * FROM test")
	if err != nil {
		t.Fatalf("Failed second query: %v", err)
	}

	if !fromCache2 {
		t.Error("Second query should be from cache")
	}
}

func TestSQLiteValidationQuery(t *testing.T) {
	config := NewConfigBuilder().
		WithDatabaseType(DatabaseTypeSQLite).
		WithDSN(":memory:").
		Build()

	if config.ValidationQuery != "SELECT 1" {
		t.Errorf("Expected 'SELECT 1' validation query for SQLite, got '%s'", config.ValidationQuery)
	}
}

func TestAggressiveCaching(t *testing.T) {
	config := NewConfigBuilder().
		WithAggressiveCaching(5000, 120*time.Second).
		Build()

	if !config.EnableAggressiveCaching {
		t.Error("Expected aggressive caching to be enabled")
	}

	if config.CacheCapacity != 5000 {
		t.Errorf("Expected cache capacity 5000, got %d", config.CacheCapacity)
	}

	if config.CacheDefaultTTL != 120*time.Second {
		t.Errorf("Expected cache TTL 120s, got %v", config.CacheDefaultTTL)
	}
}

func BenchmarkInMemoryInserts(b *testing.B) {
	config := NewConfigBuilder().
		WithInMemoryMode(true).
		Build()

	runtime := NewDBRuntime(config)
	runtime.Connect()
	defer runtime.Disconnect()

	ctx := context.Background()
	runtime.Exec(ctx, "CREATE TABLE bench (id INTEGER PRIMARY KEY, value TEXT)")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runtime.Exec(ctx, "INSERT INTO bench (value) VALUES (?)", "test_value")
	}
}

func BenchmarkInMemoryQueries(b *testing.B) {
	config := NewConfigBuilder().
		WithInMemoryMode(true).
		Build()

	runtime := NewDBRuntime(config)
	runtime.Connect()
	defer runtime.Disconnect()

	ctx := context.Background()
	runtime.Exec(ctx, "CREATE TABLE bench (id INTEGER PRIMARY KEY, value TEXT)")
	
	// Insert test data
	for i := 0; i < 1000; i++ {
		runtime.Exec(ctx, "INSERT INTO bench (value) VALUES (?)", "test_value")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runtime.Query(ctx, "SELECT COUNT(*) FROM bench")
	}
}