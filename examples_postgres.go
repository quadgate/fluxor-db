package main

import (
	"context"
	"fmt"
	"log"
	"time"
)

// ExamplePostgreSQLBasicUsage demonstrates basic PostgreSQL usage
func ExamplePostgreSQLBasicUsage() {
	// Create configuration for PostgreSQL
	config := NewConfigBuilder().
		WithDatabaseType(DatabaseTypePostgreSQL).
		WithDSN("postgres://user:password@localhost:5432/dbname?sslmode=disable").
		WithConnectionPool(50, 10).
		WithQuerySettings(200, 1*time.Second, 30*time.Second).
		Build()

	// Validate configuration
	if err := NewConfigBuilder().
		WithDatabaseType(DatabaseTypePostgreSQL).
		WithDSN(config.DSN).
		Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Create and connect
	runtime := NewDBRuntime(config)
	if err := runtime.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
		DisconnectWithLog(runtime)
		return
	}
	defer DisconnectWithLog(runtime)

	// Execute simple query
	ctx := context.Background()
	result, err := runtime.Exec(ctx, "SELECT 1")
	if err != nil {
		log.Fatalf("Failed to execute query: %v", err)
		return
	}

	fmt.Printf("PostgreSQL query executed: %+v\n", result)
}

// ExamplePostgreSQLWithTransaction demonstrates transaction usage
func ExamplePostgreSQLWithTransaction() {
	config := NewConfigBuilder().
		WithDatabaseType(DatabaseTypePostgreSQL).
		WithDSN("postgres://user:password@localhost:5432/dbname?sslmode=disable").
		WithConnectionPool(50, 10).
		Build()

	runtime := NewDBRuntime(config)
	if err := runtime.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer DisconnectWithLog(runtime)

	ctx := context.Background()

	// Start transaction
	tx, err := runtime.Begin(ctx, nil)
	if err != nil {
		log.Fatalf("Failed to begin transaction: %v", err)
	}

	// Execute queries in transaction
	_, err = tx.Exec(ctx, "CREATE TABLE IF NOT EXISTS users (id SERIAL PRIMARY KEY, name TEXT, email TEXT)")
	if err != nil {
		tx.Rollback()
		log.Fatalf("Failed to create table: %v", err)
	}

	_, err = tx.Exec(ctx, "INSERT INTO users (name, email) VALUES ($1, $2)", "John Doe", "john@example.com")
	if err != nil {
		tx.Rollback()
		log.Fatalf("Failed to insert user: %v", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		log.Fatalf("Failed to commit transaction: %v", err)
	}

	fmt.Println("PostgreSQL transaction completed successfully")
}

// ExamplePostgreSQLWithPreparedStatements demonstrates prepared statement caching
func ExamplePostgreSQLWithPreparedStatements() {
	config := NewConfigBuilder().
		WithDatabaseType(DatabaseTypePostgreSQL).
		WithDSN("postgres://user:password@localhost:5432/dbname?sslmode=disable").
		WithConnectionPool(50, 10).
		WithQuerySettings(200, 1*time.Second, 30*time.Second).
		Build()

	runtime := NewDBRuntime(config)
	if err := runtime.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer DisconnectWithLog(runtime)

	ctx := context.Background()

	// Prepare statement (will be cached)
	stmt, err := runtime.Prepare(ctx, "SELECT * FROM users WHERE id = $1")
	if err != nil {
		log.Fatalf("Failed to prepare statement: %v", err)
	}

	// Execute prepared statement multiple times
	for i := 1; i <= 5; i++ {
		rows, err := stmt.QueryContext(ctx, i)
		if err != nil {
			log.Printf("Query failed for id %d: %v", i, err)
			continue
		}
		rows.Close()
		fmt.Printf("Executed prepared statement for user id: %d\n", i)
	}

	fmt.Println("PostgreSQL prepared statements executed successfully")
}

// ExamplePostgreSQLAdvancedConfig demonstrates advanced PostgreSQL configuration
func ExamplePostgreSQLAdvancedConfig() {
	config := NewConfigBuilder().
		WithDatabaseType(DatabaseTypePostgreSQL).
		WithDSN("postgres://user:password@localhost:5432/dbname?sslmode=disable").
		WithConnectionPool(100, 20).
		WithConnectionLifetime(30*time.Minute, 10*time.Minute).
		WithLeakDetection(true, 10*time.Minute).
		WithCircuitBreaker(5, 60*time.Second, 10*time.Second).
		WithRateLimit(2000). // 2000 requests per second
		WithQuerySettings(300, 500*time.Millisecond, 30*time.Second).
		WithRetryPolicy(3, 100*time.Millisecond).
		Build()

	runtime := NewDBRuntime(config)
	if err := runtime.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer DisconnectWithLog(runtime)

	// Perform health check
	ctx := context.Background()
	if err := runtime.HealthCheck(ctx); err != nil {
		log.Fatalf("Health check failed: %v", err)
	}

	fmt.Println("PostgreSQL Advanced Runtime Configuration:")
	fmt.Printf("- Database Type: PostgreSQL\n")
	fmt.Printf("- Max Open Connections: 100\n")
	fmt.Printf("- Max Idle Connections: 20\n")
	fmt.Printf("- Circuit Breaker State: %s\n", runtime.CircuitBreakerState())
	fmt.Printf("- Connection Stats: %+v\n", runtime.Stats())
	fmt.Printf("- Performance Metrics: %+v\n", runtime.Metrics())
}

// ExamplePostgreSQLWithMonitoring demonstrates monitoring capabilities
func ExamplePostgreSQLWithMonitoring() {
	config := NewConfigBuilder().
		WithDatabaseType(DatabaseTypePostgreSQL).
		WithDSN("postgres://user:password@localhost:5432/dbname?sslmode=disable").
		WithConnectionPool(50, 10).
		Build()

	runtime := NewDBRuntime(config)
	if err := runtime.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer DisconnectWithLog(runtime)

	ctx := context.Background()

	// Execute some queries
	for i := 0; i < 10; i++ {
		_, err := runtime.Query(ctx, "SELECT * FROM users LIMIT 10")
		if err != nil {
			log.Printf("Query %d failed: %v", i, err)
			continue
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Get metrics
	metrics := runtime.Metrics()
	fmt.Println("\nPostgreSQL Performance Metrics:")
	fmt.Printf("- Total Queries: %d\n", metrics.TotalQueries)
	fmt.Printf("- Successful Queries: %d\n", metrics.SuccessfulQueries)
	fmt.Printf("- Failed Queries: %d\n", metrics.FailedQueries)
	fmt.Printf("- Slow Queries: %d\n", metrics.SlowQueries)
	fmt.Printf("- Average Query Time: %v\n", metrics.AverageQueryTime)

	// Get connection pool stats
	stats := runtime.Stats()
	fmt.Println("\nConnection Pool Stats:")
	fmt.Printf("- Open Connections: %d\n", stats.OpenConnections)
	fmt.Printf("- In Use: %d\n", stats.InUse)
	fmt.Printf("- Idle: %d\n", stats.Idle)
}

// ExamplePostgreSQLBulkInsert demonstrates bulk insert with batch processing
func ExamplePostgreSQLBulkInsert() {
	config := NewConfigBuilder().
		WithDatabaseType(DatabaseTypePostgreSQL).
		WithDSN("postgres://user:password@localhost:5432/dbname?sslmode=disable").
		WithConnectionPool(50, 10).
		Build()

	runtime := NewDBRuntime(config)
	if err := runtime.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer DisconnectWithLog(runtime)

	ctx := context.Background()

	// Start transaction for bulk insert
	tx, err := runtime.Begin(ctx, nil)
	if err != nil {
		log.Fatalf("Failed to begin transaction: %v", err)
	}

	// Insert multiple records
	users := []struct {
		name  string
		email string
	}{
		{"Alice", "alice@example.com"},
		{"Bob", "bob@example.com"},
		{"Charlie", "charlie@example.com"},
		{"David", "david@example.com"},
		{"Eve", "eve@example.com"},
	}

	for _, user := range users {
		_, err := tx.Exec(ctx, "INSERT INTO users (name, email) VALUES ($1, $2)", user.name, user.email)
		if err != nil {
			tx.Rollback()
			log.Fatalf("Failed to insert user %s: %v", user.name, err)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		log.Fatalf("Failed to commit transaction: %v", err)
	}

	fmt.Printf("Bulk inserted %d users into PostgreSQL database\n", len(users))
}

// ExamplePostgreSQLWithConnectionPool demonstrates connection pool behavior
func ExamplePostgreSQLWithConnectionPool() {
	config := NewConfigBuilder().
		WithDatabaseType(DatabaseTypePostgreSQL).
		WithDSN("postgres://user:password@localhost:5432/dbname?sslmode=disable").
		WithConnectionPool(10, 5). // Small pool for demonstration
		Build()

	runtime := NewDBRuntime(config)
	if err := runtime.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer DisconnectWithLog(runtime)

	fmt.Println("Initial connection pool stats:")
	fmt.Printf("%+v\n\n", runtime.Stats())

	// Simulate concurrent queries
	ctx := context.Background()
	done := make(chan bool)

	for i := 0; i < 15; i++ { // More than max connections
		go func(id int) {
			_, err := runtime.Query(ctx, "SELECT pg_sleep(1)")
			if err != nil {
				log.Printf("Query %d failed: %v", id, err)
			} else {
				fmt.Printf("Query %d completed\n", id)
			}
			done <- true
		}(i)
	}

	// Wait for all queries
	for i := 0; i < 15; i++ {
		<-done
	}

	fmt.Println("\nFinal connection pool stats:")
	fmt.Printf("%+v\n", runtime.Stats())
}
