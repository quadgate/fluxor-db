package main

import (
	"fmt"
	"log"
	"time"
)

// Example showing DDoS protection and idempotency for legacy database protection
func Example_DDosProtectionAndIdempotency() {
	// Configure legacy database runtime
	config := NewConfigBuilder().
		WithDatabaseType(DatabaseTypePostgreSQL).
		WithDSN("postgres://user:pass@legacy-db:5432/db?sslmode=disable").
		WithBackpressure("timeout", 5*time.Second).
		WithConnectionPool(10, 2). // Limited for legacy hardware
		Build()

	runtime := NewDBRuntime(config)
	runtime.Connect()
	defer runtime.Disconnect()

	// Configure TCP server with DDoS protection and idempotency
	serverConfig := &TCPServerConfig{
		Address:              "0.0.0.0:9090",
		Runtime:              runtime,
		EnableDDoSProtection: true,
		EnableIdempotency:    true,
		MaxRequestSize:       1024 * 1024, // 1MB max request
		MaxConnectionsPerIP:  5,           // Max 5 connections per IP
		RateLimitPerIP:       10,          // 10 requests/sec per IP
		BlacklistedIPs:       []string{"192.168.1.100", "10.0.0.50"},
		WhitelistedIPs:       []string{"192.168.1.0/24"}, // Allow local network
	}

	server := NewTCPServer(serverConfig)
	server.Start()
	defer server.Stop()

	fmt.Println("Legacy database protected with DDoS and idempotency")
}

// Example showing idempotent operations to prevent duplicate transactions
func Example_IdempotentTransactions() {
	clientConfig := &TCPClientConfig{
		Address: "legacy-db-service:9090",
		Timeout: 30 * time.Second,
	}

	client := NewTCPClient(clientConfig)
	client.Connect()
	defer client.Disconnect()

	// Critical transaction with idempotency key
	idempotencyKey := fmt.Sprintf("payment-%d-%d", 12345, time.Now().Unix())

	// This will execute only once, even if called multiple times
	result1, err1 := client.ExecWithIdempotency(
		"INSERT INTO payments (user_id, amount, status) VALUES (?, ?, 'PENDING')",
		idempotencyKey,
		12345, 100.00,
	)

	// Duplicate call - will return cached result, not execute again
	result2, err2 := client.ExecWithIdempotency(
		"INSERT INTO payments (user_id, amount, status) VALUES (?, ?, 'PENDING')",
		idempotencyKey,
		12345, 100.00,
	)

	if err1 == nil && err2 == nil {
		fmt.Printf("First call: %d rows affected\n", result1.RowsAffected)
		fmt.Printf("Second call: %d rows affected (cached)\n", result2.RowsAffected)
	}
}

// Example showing rate limiting protection for legacy database
func Example_RateLimitProtection() {
	// Client trying to overwhelm legacy database
	client := NewTCPClient(&TCPClientConfig{
		Address: "legacy-db:9090",
	})
	client.Connect()
	defer client.Disconnect()

	successCount := 0
	rateLimitCount := 0

	// Try to send 100 requests rapidly
	for i := 0; i < 100; i++ {
		_, err := client.Query("SELECT COUNT(*) FROM users")
		if err != nil {
			if fmt.Sprintf("%v", err) == "rate limit exceeded" {
				rateLimitCount++
			}
		} else {
			successCount++
		}
	}

	fmt.Printf("Successful: %d, Rate limited: %d\n", successCount, rateLimitCount)
}

// Example showing connection limit protection
func Example_ConnectionLimitProtection() {
	// Legacy database can only handle 5 concurrent connections
	serverConfig := &TCPServerConfig{
		Address:              "0.0.0.0:9090",
		EnableDDoSProtection: true,
		MaxConnectionsPerIP:  5, // Protect legacy DB from connection exhaustion
	}

	server := NewTCPServer(serverConfig)
	server.Start()
	defer server.Stop()

	// Try to create 10 connections from same IP
	var clients []*TCPClient
	defer func() {
		for _, c := range clients {
			c.Disconnect()
		}
	}()

	for i := 0; i < 10; i++ {
		client := NewTCPClient(&TCPClientConfig{
			Address: "localhost:9090",
		})

		err := client.Connect()
		if err != nil {
			log.Printf("Connection %d failed: %v", i+1, err)
		} else {
			clients = append(clients, client)
			log.Printf("Connection %d successful", i+1)
		}
	}

	fmt.Printf("Created %d connections (limit: 5)\n", len(clients))
}

// Example showing IP blacklisting for security
func Example_IPBlacklisting() {
	// Protect legacy database from known bad IPs
	serverConfig := &TCPServerConfig{
		Address:              "0.0.0.0:9090",
		EnableDDoSProtection: true,
		BlacklistedIPs: []string{
			"192.168.1.100", // Malicious IP
			"10.0.0.50",     // Compromised host
			"172.16.0.0/16", // Entire suspicious subnet
		},
		WhitelistedIPs: []string{
			"192.168.1.0/24", // Company network
			"10.0.1.0/24",    // VPN network
		},
	}

	server := NewTCPServer(serverConfig)
	server.Start()
	defer server.Stop()

	fmt.Println("Legacy database protected with IP filtering")
}

// Example showing comprehensive legacy database protection
func Example_ComprehensiveLegacyProtection() {
	// Legacy Oracle database with full protection
	config := NewConfigBuilder().
		WithDatabaseType(DatabaseTypeOracle).
		WithDSN("user/password@legacy-oracle:1521/XE").
		WithBackpressure("block", 0).                     // Block when overloaded
		WithConnectionPool(20, 5).                        // Limited connections
		WithCircuitBreaker(3, 120*time.Second, 30*time.Second). // Aggressive circuit breaker
		WithQuerySettings(50, 5*time.Second, 60*time.Second).   // Slow query tolerance
		Build()

	runtime := NewDBRuntime(config)
	runtime.SetCache(NewInMemoryCache(5000, 900*time.Second)) // 15min cache
	runtime.Connect()
	defer runtime.Disconnect()

	// TCP server with comprehensive DDoS protection
	serverConfig := &TCPServerConfig{
		Address:              "0.0.0.0:9090",
		Runtime:              runtime,
		EnableDDoSProtection: true,
		EnableIdempotency:    true,
		MaxRequestSize:       512 * 1024, // 512KB limit
		MaxConnectionsPerIP:  3,          // Very restrictive for legacy DB
		RateLimitPerIP:       5,          // 5 req/sec max
		BlacklistedIPs: []string{
			// Add known attack sources
		},
	}

	server := NewTCPServer(serverConfig)
	server.Start()
	defer server.Stop()

	log.Println("Legacy Oracle database fully protected and ready")
}