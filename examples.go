package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"
)

// ExampleBasicUsage demonstrates basic usage of the database runtime
func ExampleBasicUsage() {
	// Create configuration using builder
	config := NewConfigBuilder().
		WithDSN("user/password@localhost:1521/XE").
		WithConnectionPool(50, 10).
		WithQuerySettings(200, 1*time.Second, 30*time.Second).
		Build()

	// Validate configuration
	if err := NewConfigBuilder().WithDSN(config.DSN).Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Create and connect
	runtime := NewDBRuntime(config)
	if err := runtime.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer runtime.Disconnect()

	ctx := context.Background()

	// Execute a simple query
	result, err := runtime.Exec(ctx, "SELECT 1 FROM DUAL")
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}
	fmt.Printf("Query result: %+v\n", result)
}

// ExampleWithTransaction demonstrates transaction usage
func ExampleWithTransaction() {
	config := NewConfigBuilder().
		WithDSN("user/password@localhost:1521/XE").
		Build()

	runtime := NewDBRuntime(config)
	if err := runtime.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer runtime.Disconnect()

	ctx := context.Background()
	executor := NewQueryExecutor(runtime)

	// Execute within a transaction
	err := executor.Transaction(ctx, func(tx *AdvancedTx) error {
		// Insert operation
		_, err := tx.Exec(ctx, "INSERT INTO users (name, email) VALUES (:1, :2)", "John Doe", "john@example.com")
		if err != nil {
			return err
		}

		// Update operation
		_, err = tx.Exec(ctx, "UPDATE users SET last_login = SYSDATE WHERE email = :1", "john@example.com")
		return err
	})

	if err != nil {
		log.Fatalf("Transaction failed: %v", err)
	}
}

// ExampleWithPreparedStatements demonstrates prepared statement caching
func ExampleWithPreparedStatements() {
	config := NewConfigBuilder().
		WithDSN("user/password@localhost:1521/XE").
		WithQuerySettings(200, 1*time.Second, 30*time.Second).
		Build()

	runtime := NewDBRuntime(config)
	if err := runtime.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer runtime.Disconnect()

	ctx := context.Background()

	// Prepare statement (will be cached)
	stmt, err := runtime.Prepare(ctx, "SELECT id, name FROM users WHERE id = :1")
	if err != nil {
		log.Fatalf("Failed to prepare statement: %v", err)
	}
	defer stmt.Close()

	// Execute multiple times - statement is reused from cache
	for i := 1; i <= 10; i++ {
		row := stmt.QueryRow(i)
		var id int
		var name string
		if err := row.Scan(&id, &name); err != nil {
			if err == sql.ErrNoRows {
				continue
			}
			log.Printf("Query failed: %v", err)
			continue
		}
		fmt.Printf("User %d: %s\n", id, name)
	}
}

// ExampleWithMetrics demonstrates metrics collection
func ExampleWithMetrics() {
	config := NewConfigBuilder().
		WithDSN("user/password@localhost:1521/XE").
		Build()

	runtime := NewDBRuntime(config)
	if err := runtime.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer runtime.Disconnect()

	ctx := context.Background()

	// Execute some queries
	for i := 0; i < 100; i++ {
		runtime.Exec(ctx, "SELECT 1 FROM DUAL")
	}

	// Get metrics
	metrics := runtime.Metrics()
	fmt.Printf("Total Queries: %d\n", metrics.TotalQueries)
	fmt.Printf("Successful: %d\n", metrics.SuccessfulQueries)
	fmt.Printf("Failed: %d\n", metrics.FailedQueries)
	fmt.Printf("Success Rate: %.2f%%\n", metrics.SuccessRate)
	fmt.Printf("Average Query Time: %v\n", metrics.AverageQueryTime)
	fmt.Printf("Slow Queries: %d\n", metrics.SlowQueries)
}

// ExampleWithHealthCheck demonstrates health checking
func ExampleWithHealthCheck() {
	config := NewConfigBuilder().
		WithDSN("user/password@localhost:1521/XE").
		Build()

	runtime := NewDBRuntime(config)
	if err := runtime.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer runtime.Disconnect()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Perform health check
	health := CheckHealth(ctx, runtime)
	fmt.Printf("Health Status: %+v\n", health)

	// Get diagnostics
	diagnostics := GetDiagnostics(runtime)
	fmt.Println(diagnostics.String())
}

// ExampleWithCircuitBreaker demonstrates circuit breaker behavior
func ExampleWithCircuitBreaker() {
	config := NewConfigBuilder().
		WithDSN("user/password@localhost:1521/XE").
		WithCircuitBreaker(3, 60*time.Second, 10*time.Second).
		Build()

	runtime := NewDBRuntime(config)
	if err := runtime.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer runtime.Disconnect()

	ctx := context.Background()

	// Monitor circuit breaker state
	for i := 0; i < 10; i++ {
		state := runtime.CircuitBreakerState()
		fmt.Printf("Circuit Breaker State: %s\n", state)

		// Try to execute query
		_, err := runtime.Exec(ctx, "SELECT 1 FROM DUAL")
		if err != nil {
			fmt.Printf("Query failed: %v\n", err)
		}

		time.Sleep(1 * time.Second)
	}
}

// ExampleWithQueryExecutor demonstrates the QueryExecutor helper
func ExampleWithQueryExecutor() {
	config := NewConfigBuilder().
		WithDSN("user/password@localhost:1521/XE").
		Build()

	runtime := NewDBRuntime(config)
	if err := runtime.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer runtime.Disconnect()

	ctx := context.Background()
	executor := NewQueryExecutor(runtime)

	// Select multiple rows
	type User struct {
		ID    int
		Name  string
		Email string
	}

	var users []User
	err := executor.Select(ctx, "SELECT id, name, email FROM users WHERE active = 1", nil, func(rows *sql.Rows) error {
		var u User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email); err != nil {
			return err
		}
		users = append(users, u)
		return nil
	})

	if err != nil {
		log.Fatalf("Select failed: %v", err)
	}

	fmt.Printf("Found %d active users\n", len(users))
}
