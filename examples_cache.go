package main

import (
	"context"
	"fmt"
	"time"
)

// Example showing legacy database integration with backpressure and in-memory cache.
// This approach is ideal for legacy systems that:
// - Don't have Redis available
// - Have unpredictable performance
// - Need gradual modernization
func Example_LegacyDatabaseIntegration() {
	cfg := NewConfigBuilder().
		WithDatabaseType(DatabaseTypePostgreSQL).
		WithDSN("postgres://user:pass@localhost:5432/dbname?sslmode=disable").
		.WithBackpressure("block", 0). // block when legacy DB can't handle load
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
