package main

import (
	"context"
	"fmt"
	"time"
)

// Example showing pure in-memory database runtime for maximum performance
func Example_InMemoryRuntime() {
	// Create in-memory SQLite database with aggressive caching
	config := NewConfigBuilder().
		WithInMemoryMode(true). // Enables all in-memory optimizations
		Build()

	runtime := NewDBRuntime(config)
	runtime.Connect()
	defer runtime.Disconnect()

	ctx := context.Background()

	// Create tables in memory
	runtime.Exec(ctx, `
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			email TEXT UNIQUE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)

	runtime.Exec(ctx, `
		CREATE TABLE orders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER,
			amount DECIMAL(10,2),
			status TEXT DEFAULT 'PENDING',
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)

	// Insert test data
	runtime.Exec(ctx, "INSERT INTO users (name, email) VALUES (?, ?)", "Alice", "alice@example.com")
	runtime.Exec(ctx, "INSERT INTO users (name, email) VALUES (?, ?)", "Bob", "bob@example.com")
	runtime.Exec(ctx, "INSERT INTO orders (user_id, amount) VALUES (?, ?)", 1, 99.99)
	runtime.Exec(ctx, "INSERT INTO orders (user_id, amount) VALUES (?, ?)", 2, 149.99)

	// Query with aggressive caching (cached for 10 minutes by default)
	cols, rows, hit, _ := runtime.QueryCached(ctx, "user_orders_summary", 0, `
		SELECT u.name, COUNT(o.id) as order_count, COALESCE(SUM(o.amount), 0) as total_amount
		FROM users u
		LEFT JOIN orders o ON u.id = o.user_id
		GROUP BY u.id, u.name
		ORDER BY total_amount DESC
	`)

	fmt.Printf("Columns: %v, Rows: %d, From Cache: %v\n", len(cols), len(rows), hit)
	// Output: Columns: 3, Rows: 2, From Cache: false
}

// Example showing in-memory mode for testing and development
func Example_InMemoryTesting() {
	// Perfect for unit tests - no external dependencies
	config := NewConfigBuilder().
		WithDatabaseType(DatabaseTypeSQLite).
		WithDSN(":memory:"). // Pure in-memory SQLite
		WithAggressiveCaching(5000, 60*time.Second).
		Build()

	runtime := NewDBRuntime(config)
	runtime.Connect()
	defer runtime.Disconnect()

	ctx := context.Background()

	// Setup test schema
	runtime.Exec(ctx, "CREATE TABLE test_table (id INTEGER PRIMARY KEY, value TEXT)")

	// Fast in-memory operations
	for i := 0; i < 1000; i++ {
		runtime.Exec(ctx, "INSERT INTO test_table (value) VALUES (?)", fmt.Sprintf("value_%d", i))
	}

	// Cached queries for repeated operations
	_, rows, _, _ := runtime.QueryCached(ctx, "count_test", 30*time.Second, "SELECT COUNT(*) FROM test_table")
	
	fmt.Printf("Total records: %v\n", rows[0][0])
	// Output: Total records: 1000
}

// Example showing hybrid mode - SQLite in-memory with legacy data sync
func Example_HybridInMemoryLegacy() {
	// In-memory runtime for performance
	memConfig := NewConfigBuilder().
		WithInMemoryMode(true).
		Build()

	memRuntime := NewDBRuntime(memConfig)
	memRuntime.Connect()
	defer memRuntime.Disconnect()

	// Legacy database connection (when needed)
	legacyConfig := NewConfigBuilder().
		WithDatabaseType(DatabaseTypeOracle).
		WithDSN("user/pass@legacy-oracle:1521/XE").
		WithBackpressure("timeout", 5*time.Second).
		Build()

	legacyRuntime := NewDBRuntime(legacyConfig)
	// Connect only when needed to reduce legacy DB load

	ctx := context.Background()

	// Setup in-memory tables
	memRuntime.Exec(ctx, `
		CREATE TABLE cached_products (
			id INTEGER PRIMARY KEY,
			name TEXT,
			price DECIMAL(10,2),
			category TEXT,
			last_updated DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)

	// Sync function to load from legacy DB into memory
	syncFromLegacy := func() error {
		if err := legacyRuntime.Connect(); err != nil {
			return err
		}
		defer legacyRuntime.Disconnect()

		// Query legacy DB
		rows, err := legacyRuntime.Query(ctx, "SELECT id, name, price, category FROM products WHERE active = 1")
		if err != nil {
			return err
		}
		defer rows.Close()

		// Clear in-memory cache
		memRuntime.Exec(ctx, "DELETE FROM cached_products")

		// Load into memory
		for rows.Next() {
			var id int
			var name, category string
			var price float64
			
			rows.Scan(&id, &name, &price, &category)
			memRuntime.Exec(ctx, 
				"INSERT INTO cached_products (id, name, price, category) VALUES (?, ?, ?, ?)",
				id, name, price, category)
		}
		
		return nil
	}

	// Sync data (do this periodically)
	syncFromLegacy()

	// All queries now run in-memory
	_, products, _, _ := memRuntime.QueryCached(ctx, "products_by_category", 5*time.Minute,
		"SELECT category, COUNT(*), AVG(price) FROM cached_products GROUP BY category")

	fmt.Printf("Product categories: %d\n", len(products))
}

// Example showing in-memory TCP server for ultra-fast responses
func Example_InMemoryTCPServer() {
	// Ultra-fast in-memory database server
	config := NewConfigBuilder().
		WithInMemoryMode(true).
		Build()

	runtime := NewDBRuntime(config)
	runtime.Connect()
	defer runtime.Disconnect()

	ctx := context.Background()

	// Setup reference data in memory
	runtime.Exec(ctx, `
		CREATE TABLE countries (
			code TEXT PRIMARY KEY,
			name TEXT,
			region TEXT,
			population INTEGER
		)
	`)

	// Load reference data
	countries := [][]string{
		{"US", "United States", "North America", "331900000"},
		{"CN", "China", "Asia", "1439323776"},
		{"IN", "India", "Asia", "1380004385"},
		{"BR", "Brazil", "South America", "212559417"},
	}

	for _, country := range countries {
		runtime.Exec(ctx, "INSERT INTO countries VALUES (?, ?, ?, ?)",
			country[0], country[1], country[2], country[3])
	}

	// TCP server with in-memory database
	serverConfig := &TCPServerConfig{
		Address:              "0.0.0.0:9090",
		Runtime:              runtime,
		EnableDDoSProtection: true,
		EnableIdempotency:    true,
		MaxConnectionsPerIP:  100, // Can handle more with in-memory
		RateLimitPerIP:       1000, // Much higher rate limits
	}

	server := NewTCPServer(serverConfig)
	server.Start()
	defer server.Stop()

	fmt.Println("Ultra-fast in-memory database server running on :9090")
}

// Example showing performance comparison: legacy vs in-memory
func Example_PerformanceComparison() {
	// Legacy database setup
	legacyConfig := NewConfigBuilder().
		WithDatabaseType(DatabaseTypePostgreSQL).
		WithDSN("postgres://user:pass@legacy-db:5432/db").
		WithConnectionPool(10, 2).
		Build()

	// In-memory setup
	memoryConfig := NewConfigBuilder().
		WithInMemoryMode(true).
		Build()

	legacyRuntime := NewDBRuntime(legacyConfig)
	memoryRuntime := NewDBRuntime(memoryConfig)

	// Setup identical schemas
	schema := "CREATE TABLE benchmark (id INTEGER PRIMARY KEY, data TEXT)"
	
	if legacyRuntime.Connect() == nil {
		legacyRuntime.Exec(context.Background(), schema)
		defer legacyRuntime.Disconnect()
	}
	
	memoryRuntime.Connect()
	memoryRuntime.Exec(context.Background(), schema)
	defer memoryRuntime.Disconnect()

	ctx := context.Background()

	// Benchmark inserts
	start := time.Now()
	for i := 0; i < 1000; i++ {
		memoryRuntime.Exec(ctx, "INSERT INTO benchmark (data) VALUES (?)", fmt.Sprintf("data_%d", i))
	}
	memoryTime := time.Since(start)

	// Query with caching
	_, _, hit, _ := memoryRuntime.QueryCached(ctx, "count_bench", 60*time.Second, "SELECT COUNT(*) FROM benchmark")

	fmt.Printf("In-memory 1000 inserts: %v, Cache hit: %v\n", memoryTime, hit)
}