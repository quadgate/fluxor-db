package main

import (
	"context"
	"fmt"
	"time"
)

// Example showing how to enable backpressure and use the in-memory cache layer.
func Example_CacheAndBackpressure() {
	cfg := NewConfigBuilder().
		WithDatabaseType(DatabaseTypePostgreSQL).
		WithDSN("postgres://user:pass@localhost:5432/dbname?sslmode=disable").
		WithBackpressure("block", 0). // block when max concurrency reached
		Build()

	rt := NewDBRuntime(cfg)
	if err := rt.Connect(); err != nil {
		fmt.Println("connect error:", err)
		return
	}
	defer DisconnectWithLog(rt)

	// Attach a cache layer (capacity 1000 keys, default TTL 60s)
	rt.SetCache(NewInMemoryCache(1000, 60*time.Second))

	ctx := context.Background()
	key := "users:top10"
	columns, rows, fromCache, err := rt.QueryCached(ctx, key, 30*time.Second, "SELECT id, name FROM users ORDER BY id LIMIT 10")
	if err != nil {
		fmt.Println("query error:", err)
		return
	}

	fmt.Println(len(columns) > 0, len(rows) >= 0, fromCache)
}
