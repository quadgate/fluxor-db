package main

import (
	"fmt"
	"log"
	"time"
)

// ExampleTCPServerBasic demonstrates basic TCP server usage
func ExampleTCPServerBasic() {
	// Create database runtime
	config := NewConfigBuilder().
		WithDatabaseType(DatabaseTypeMySQL).
		WithDSN("user:password@tcp(localhost:3306)/dbname?parseTime=true").
		WithConnectionPool(50, 10).
		Build()

	runtime := NewDBRuntime(config)
	if err := runtime.Connect(); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer runtime.Disconnect()

	// Create and start TCP server
	serverConfig := &TCPServerConfig{
		Address: "localhost:9090",
		Runtime: runtime,
	}

	server := NewTCPServer(serverConfig)
	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start TCP server: %v", err)
	}
	defer server.Stop()

	fmt.Printf("TCP server listening on %s\n", server.GetAddress())
	
	// Keep server running
	time.Sleep(5 * time.Minute)
}

// ExampleTCPClientBasic demonstrates basic TCP client usage
func ExampleTCPClientBasic() {
	// Create TCP client
	clientConfig := &TCPClientConfig{
		Address: "localhost:9090",
		Timeout: 30 * time.Second,
	}

	client := NewTCPClient(clientConfig)
	if err := client.Connect(); err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	defer client.Disconnect()

	// Ping server
	if err := client.Ping(); err != nil {
		log.Fatalf("Ping failed: %v", err)
	}
	fmt.Println("Ping successful!")

	// Execute query
	result, err := client.Exec("INSERT INTO users (name, email) VALUES (?, ?)", "John Doe", "john@example.com")
	if err != nil {
		log.Fatalf("Exec failed: %v", err)
	}
	fmt.Printf("Insert successful: %d rows affected\n", result.RowsAffected)

	// Query data
	queryResult, err := client.Query("SELECT * FROM users WHERE name = ?", "John Doe")
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}
	fmt.Printf("Found %d rows with columns: %v\n", len(queryResult.Rows), queryResult.Columns)

	// Get statistics
	stats, err := client.Stats()
	if err != nil {
		log.Fatalf("Stats failed: %v", err)
	}
	fmt.Printf("Connection pool: %d open, %d in use, %d idle\n", 
		stats.OpenConnections, stats.InUse, stats.Idle)

	// Get metrics
	metrics, err := client.Metrics()
	if err != nil {
		log.Fatalf("Metrics failed: %v", err)
	}
	fmt.Printf("Queries: %d total, %d successful, %d failed\n",
		metrics.TotalQueries, metrics.SuccessfulQueries, metrics.FailedQueries)
}

// ExampleTCPServerMultiClient demonstrates handling multiple clients
func ExampleTCPServerMultiClient() {
	// Setup server
	config := NewConfigBuilder().
		WithDatabaseType(DatabaseTypePostgreSQL).
		WithDSN("postgres://user:password@localhost:5432/dbname").
		WithConnectionPool(100, 20).
		Build()

	runtime := NewDBRuntime(config)
	if err := runtime.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer runtime.Disconnect()

	server := NewTCPServer(&TCPServerConfig{
		Address: "localhost:9090",
		Runtime: runtime,
	})

	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Simulate multiple clients
	for i := 0; i < 5; i++ {
		go func(clientID int) {
			client := NewTCPClient(&TCPClientConfig{
				Address: "localhost:9090",
			})

			if err := client.Connect(); err != nil {
				log.Printf("Client %d failed to connect: %v", clientID, err)
				return
			}
			defer client.Disconnect()

			// Each client performs operations
			for j := 0; j < 10; j++ {
				result, err := client.Query("SELECT * FROM users LIMIT 10")
				if err != nil {
					log.Printf("Client %d query %d failed: %v", clientID, j, err)
					continue
				}
				fmt.Printf("Client %d query %d: found %d rows\n", clientID, j, len(result.Rows))
				time.Sleep(100 * time.Millisecond)
			}
		}(i)
	}

	// Monitor server
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		fmt.Printf("Active clients: %d\n", server.GetClientCount())
		if server.GetClientCount() == 0 {
			break
		}
	}
}

// ExampleTCPServerWithOracle demonstrates TCP server with Oracle database
func ExampleTCPServerWithOracle() {
	config := NewConfigBuilder().
		WithDatabaseType(DatabaseTypeOracle).
		WithDSN("user/password@localhost:1521/XE").
		WithConnectionPool(50, 10).
		WithCircuitBreaker(5, 60*time.Second, 10*time.Second).
		WithRateLimit(1000).
		Build()

	runtime := NewDBRuntime(config)
	if err := runtime.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer runtime.Disconnect()

	server := NewTCPServer(&TCPServerConfig{
		Address: "0.0.0.0:9090", // Listen on all interfaces
		Runtime: runtime,
	})

	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	fmt.Printf("Oracle TCP server running on %s\n", server.GetAddress())
	fmt.Println("Circuit breaker and rate limiting enabled")
	
	// Keep running
	select {}
}

// ExampleTCPClientWithRetry demonstrates client with retry logic
func ExampleTCPClientWithRetry() {
	client := NewTCPClient(&TCPClientConfig{
		Address: "localhost:9090",
		Timeout: 10 * time.Second,
	})

	// Retry connection
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		if err := client.Connect(); err != nil {
			log.Printf("Connection attempt %d failed: %v", i+1, err)
			time.Sleep(time.Duration(i+1) * time.Second)
			continue
		}
		fmt.Println("Connected successfully!")
		break
	}

	if !client.IsConnected() {
		log.Fatal("Failed to connect after retries")
	}
	defer client.Disconnect()

	// Execute with retry
	var result *ExecResult
	var err error
	for i := 0; i < maxRetries; i++ {
		result, err = client.Exec("UPDATE users SET active = 1 WHERE id = ?", 123)
		if err == nil {
			break
		}
		log.Printf("Exec attempt %d failed: %v", i+1, err)
		time.Sleep(time.Duration(i+1) * time.Second)
	}

	if err != nil {
		log.Fatalf("Failed to execute after retries: %v", err)
	}

	fmt.Printf("Update successful: %d rows affected\n", result.RowsAffected)
}

// ExampleTCPClientBatchOperations demonstrates batch operations over TCP
func ExampleTCPClientBatchOperations() {
	client := NewTCPClient(&TCPClientConfig{
		Address: "localhost:9090",
	})

	if err := client.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer client.Disconnect()

	// Batch inserts
	users := []struct {
		name  string
		email string
	}{
		{"Alice", "alice@example.com"},
		{"Bob", "bob@example.com"},
		{"Charlie", "charlie@example.com"},
	}

	totalAffected := int64(0)
	for _, user := range users {
		result, err := client.Exec(
			"INSERT INTO users (name, email) VALUES (?, ?)",
			user.name, user.email,
		)
		if err != nil {
			log.Printf("Failed to insert %s: %v", user.name, err)
			continue
		}
		totalAffected += result.RowsAffected
		fmt.Printf("Inserted %s (ID: %d)\n", user.name, result.LastInsertID)
	}

	fmt.Printf("Total rows inserted: %d\n", totalAffected)
}

// ExampleTCPServerMonitoring demonstrates server monitoring
func ExampleTCPServerMonitoring() {
	config := NewConfigBuilder().
		WithDatabaseType(DatabaseTypeMySQL).
		WithDSN("user:password@tcp(localhost:3306)/dbname").
		WithConnectionPool(50, 10).
		Build()

	runtime := NewDBRuntime(config)
	if err := runtime.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer runtime.Disconnect()

	server := NewTCPServer(&TCPServerConfig{
		Address: "localhost:9090",
		Runtime: runtime,
	})

	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Monitoring loop
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		clientCount := server.GetClientCount()
		
		// Get runtime metrics
		metrics := runtime.Metrics()
		stats := runtime.Stats()
		
		fmt.Printf("\n=== Server Status ===\n")
		fmt.Printf("Connected clients: %d\n", clientCount)
		fmt.Printf("Connection pool: %d/%d (in use: %d)\n",
			stats.OpenConnections, stats.MaxOpenConnections, stats.InUse)
		fmt.Printf("Total queries: %d (success: %d, failed: %d)\n",
			metrics.TotalQueries, metrics.SuccessfulQueries, metrics.FailedQueries)
		fmt.Printf("Circuit breaker: %s\n", runtime.CircuitBreakerState())
	}
}

// ExampleTCPClientQueryIteration demonstrates iterating query results
func ExampleTCPClientQueryIteration() {
	client := NewTCPClient(&TCPClientConfig{
		Address: "localhost:9090",
	})

	if err := client.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer client.Disconnect()

	// Query data
	result, err := client.Query("SELECT id, name, email FROM users LIMIT 100")
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}

	fmt.Printf("Columns: %v\n", result.Columns)
	fmt.Printf("Found %d users:\n", len(result.Rows))

	// Iterate through rows
	for i, row := range result.Rows {
		if len(row) >= 3 {
			fmt.Printf("%d. ID: %v, Name: %v, Email: %v\n",
				i+1, row[0], row[1], row[2])
		}
	}
}
