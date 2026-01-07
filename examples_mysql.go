package main

import (
	"context"
	"fmt"
	"log"
	"time"
)

// ExampleMySQLBasicUsage demonstrates basic MySQL usage
func ExampleMySQLBasicUsage() {
	// Create configuration for MySQL
	config := NewConfigBuilder().
		WithDatabaseType(DatabaseTypeMySQL).
		WithDSN("user:password@tcp(localhost:3306)/dbname?parseTime=true").
		WithConnectionPool(50, 10).
		WithQuerySettings(200, 1*time.Second, 30*time.Second).
		Build()

	// Validate configuration
	if err := NewConfigBuilder().
		WithDatabaseType(DatabaseTypeMySQL).
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

	fmt.Printf("MySQL query executed: %+v\n", result)
}

// ExampleMySQLWithTransaction demonstrates transaction usage
func ExampleMySQLWithTransaction() {
	config := NewConfigBuilder().
		WithDatabaseType(DatabaseTypeMySQL).
		WithDSN("user:password@tcp(localhost:3306)/dbname?parseTime=true").
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
	_, err = tx.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS users (
			id INT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			email VARCHAR(255) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
	`)
	if err != nil {
		tx.Rollback()
		log.Fatalf("Failed to create table: %v", err)
	}

	_, err = tx.Exec(ctx, "INSERT INTO users (name, email) VALUES (?, ?)", "John Doe", "john@example.com")
	if err != nil {
		tx.Rollback()
		log.Fatalf("Failed to insert user: %v", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		log.Fatalf("Failed to commit transaction: %v", err)
	}

	fmt.Println("MySQL transaction completed successfully")
}

// ExampleMySQLWithPreparedStatements demonstrates prepared statement caching
func ExampleMySQLWithPreparedStatements() {
	config := NewConfigBuilder().
		WithDatabaseType(DatabaseTypeMySQL).
		WithDSN("user:password@tcp(localhost:3306)/dbname?parseTime=true").
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
	stmt, err := runtime.Prepare(ctx, "SELECT * FROM users WHERE id = ?")
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

	fmt.Println("MySQL prepared statements executed successfully")
}

// ExampleMySQLAdvancedConfig demonstrates advanced MySQL configuration
func ExampleMySQLAdvancedConfig() {
	config := NewConfigBuilder().
		WithDatabaseType(DatabaseTypeMySQL).
		WithDSN("user:password@tcp(localhost:3306)/dbname?parseTime=true&charset=utf8mb4&loc=Local").
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

	fmt.Println("MySQL Advanced Runtime Configuration:")
	fmt.Printf("- Database Type: MySQL\n")
	fmt.Printf("- Max Open Connections: 100\n")
	fmt.Printf("- Max Idle Connections: 20\n")
	fmt.Printf("- Circuit Breaker State: %s\n", runtime.CircuitBreakerState())
	fmt.Printf("- Connection Stats: %+v\n", runtime.Stats())
	fmt.Printf("- Performance Metrics: %+v\n", runtime.Metrics())
}

// ExampleMySQLWithMonitoring demonstrates monitoring capabilities
func ExampleMySQLWithMonitoring() {
	config := NewConfigBuilder().
		WithDatabaseType(DatabaseTypeMySQL).
		WithDSN("user:password@tcp(localhost:3306)/dbname?parseTime=true").
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
	fmt.Println("\nMySQL Performance Metrics:")
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

// ExampleMySQLBulkInsert demonstrates bulk insert with batch processing
func ExampleMySQLBulkInsert() {
	config := NewConfigBuilder().
		WithDatabaseType(DatabaseTypeMySQL).
		WithDSN("user:password@tcp(localhost:3306)/dbname?parseTime=true").
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
		_, err := tx.Exec(ctx, "INSERT INTO users (name, email) VALUES (?, ?)", user.name, user.email)
		if err != nil {
			tx.Rollback()
			log.Fatalf("Failed to insert user %s: %v", user.name, err)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		log.Fatalf("Failed to commit transaction: %v", err)
	}

	fmt.Printf("Bulk inserted %d users into MySQL database\n", len(users))
}

// ExampleMySQLWithConnectionPool demonstrates connection pool behavior
func ExampleMySQLWithConnectionPool() {
	config := NewConfigBuilder().
		WithDatabaseType(DatabaseTypeMySQL).
		WithDSN("user:password@tcp(localhost:3306)/dbname?parseTime=true").
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
			_, err := runtime.Query(ctx, "SELECT SLEEP(1)")
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

// ExampleMySQLMultiValueInsert demonstrates efficient multi-value insert
func ExampleMySQLMultiValueInsert() {
	config := NewConfigBuilder().
		WithDatabaseType(DatabaseTypeMySQL).
		WithDSN("user:password@tcp(localhost:3306)/dbname?parseTime=true").
		WithConnectionPool(50, 10).
		Build()

	runtime := NewDBRuntime(config)
	if err := runtime.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer DisconnectWithLog(runtime)

	ctx := context.Background()

	// Multi-value insert (more efficient than multiple single inserts)
	result, err := runtime.Exec(ctx, `
		INSERT INTO users (name, email) VALUES 
		('User1', 'user1@example.com'),
		('User2', 'user2@example.com'),
		('User3', 'user3@example.com'),
		('User4', 'user4@example.com'),
		('User5', 'user5@example.com')
	`)
	if err != nil {
		log.Fatalf("Failed to bulk insert: %v", err)
	}

	rowsAffected, _ := result.RowsAffected()
	fmt.Printf("Multi-value insert completed: %d rows affected\n", rowsAffected)
}

// ExampleMySQLWithTimeout demonstrates query timeout handling
func ExampleMySQLWithTimeout() {
	config := NewConfigBuilder().
		WithDatabaseType(DatabaseTypeMySQL).
		WithDSN("user:password@tcp(localhost:3306)/dbname?parseTime=true").
		WithConnectionPool(50, 10).
		WithQuerySettings(200, 1*time.Second, 5*time.Second). // 5 second timeout
		Build()

	runtime := NewDBRuntime(config)
	if err := runtime.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer DisconnectWithLog(runtime)

	ctx := context.Background()

	// This query will timeout if it takes longer than 5 seconds
	_, err := runtime.Query(ctx, "SELECT * FROM large_table WHERE complex_condition = 1")
	if err != nil {
		fmt.Printf("Query failed (possibly timeout): %v\n", err)
	} else {
		fmt.Println("Query completed within timeout")
	}
}
