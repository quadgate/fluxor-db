# Fluxor-DB: Legacy Database Integration Runtime
## Multi-Database Support with Modern Patterns

A specialized database runtime for **legacy database modernization** in Go, supporting Oracle, PostgreSQL, and MySQL. Designed for organizations migrating from legacy systems to microservices architecture without external dependencies like Redis.

## 🎯 Legacy Database Integration

Perfect for:
- **Legacy system modernization** - Wrap old databases with modern patterns
- **Gradual migration** - Add caching and resilience without infrastructure changes  
- **No Redis environments** - In-memory cache as Redis alternative
- **Unpredictable legacy DBs** - Backpressure and circuit breakers for stability
- **Microservices transition** - TCP layer for service-oriented architecture

## Features

### 🚀 Legacy Integration Features
- **In-Memory Cache Layer** - Redis alternative for legacy environments
- **Backpressure Handling** - Block/timeout modes for legacy DB performance limits
- **Advanced Connection Pooling** - Manage unreliable legacy connections
- **Connection Leak Detection** - Critical for legacy systems with connection issues
- **Connection Validation** - Pre-use validation with retry logic for flaky connections
- **TCP Network Layer** - Expose legacy databases as microservices

### 🌐 TCP Network Layer (Microservices Ready)
- **Legacy DB as Service** - Expose legacy database runtime over TCP
- **Multiple Clients** - Support distributed applications connecting to legacy DB
- **JSON Protocol** - Simple protocol for cross-platform legacy integration
- **Remote Operations** - Execute queries, transactions, get metrics remotely

### 🛡️ Legacy Database Resilience
- **Circuit Breaker** - Essential for unreliable legacy database connections
- **Backpressure Control** - Block/timeout modes when legacy DB can't handle load
- **Rate Limiting** - Protect legacy systems from overload  
- **Connection Limiting** - Prevent resource exhaustion on legacy hardware
- **Automatic Retry** - Handle transient failures common in legacy systems

### ⚡ Performance for Legacy Systems
- **In-Memory Caching** - Redis replacement for legacy environments
- **Prepared Statement Caching** - Optimize repeated queries on slow legacy DBs
- **Query Timeout Management** - Prevent hanging on unresponsive legacy systems
- **Performance Metrics** - Comprehensive metrics collection and reporting
- **Slow Query Detection** - Automatic detection of slow queries

### 📊 Monitoring & Diagnostics
- **Health Checks** - Comprehensive health check functionality
- **Diagnostics** - Detailed diagnostic information
- **Metrics Collection** - Query performance and connection pool metrics
- **Monitoring** - Continuous monitoring with callbacks

## Installation

```bash
# For Oracle Database
go get github.com/godror/godror

# For PostgreSQL
go get github.com/lib/pq

# For MySQL
go get github.com/go-sql-driver/mysql
```

## Quick Start

### Oracle Database

```go
package main

import (
    "context"
    "fmt"
    "time"
)

func main() {
    // Create configuration for Oracle
    config := NewConfigBuilder().
        WithDatabaseType(DatabaseTypeOracle).
        WithDSN("user/password@localhost:1521/XE").
        WithConnectionPool(50, 10).
        WithQuerySettings(200, 1*time.Second, 30*time.Second).
        Build()

    // Create and connect
    runtime := NewDBRuntime(config)
    if err := runtime.Connect(); err != nil {
        panic(err)
    }
    defer runtime.Disconnect()

    // Execute query
    ctx := context.Background()
    result, err := runtime.Exec(ctx, "SELECT 1 FROM DUAL")
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Query executed: %+v\n", result)
}
```

### PostgreSQL Database

```go
package main

import (
    "context"
    "fmt"
    "time"
)

func main() {
    // Create configuration for PostgreSQL
    config := NewConfigBuilder().
        WithDatabaseType(DatabaseTypePostgreSQL).
        WithDSN("postgres://user:password@localhost:5432/dbname?sslmode=disable").
        WithConnectionPool(50, 10).
        WithQuerySettings(200, 1*time.Second, 30*time.Second).
        Build()

    // Create and connect
    runtime := NewDBRuntime(config)
    if err := runtime.Connect(); err != nil {
        panic(err)
    }
    defer runtime.Disconnect()

    // Execute query
    ctx := context.Background()
    result, err := runtime.Exec(ctx, "SELECT 1")
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Query executed: %+v\n", result)
}
```

### MySQL Database

```go
package main

import (
    "context"
    "fmt"
    "time"
)

func main() {
    // Create configuration for MySQL
    config := NewConfigBuilder().
        WithDatabaseType(DatabaseTypeMySQL).
        WithDSN("user:password@tcp(localhost:3306)/dbname?parseTime=true").
        WithConnectionPool(50, 10).
        WithQuerySettings(200, 1*time.Second, 30*time.Second).
        Build()

    // Create and connect
    runtime := NewDBRuntime(config)
    if err := runtime.Connect(); err != nil {
        panic(err)
    }
    defer runtime.Disconnect()

    // Execute query
    ctx := context.Background()
    result, err := runtime.Exec(ctx, "SELECT 1")
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Query executed: %+v\n", result)
}
```

## Configuration

### Using ConfigBuilder (Recommended)

```go
// For Oracle
config := NewConfigBuilder().
    WithDatabaseType(DatabaseTypeOracle).
    WithDSN("user/password@localhost:1521/XE").
    WithConnectionPool(50, 10).
    WithConnectionLifetime(30*time.Minute, 10*time.Minute).
    WithLeakDetection(true, 10*time.Minute).
    WithCircuitBreaker(5, 60*time.Second, 10*time.Second).
    WithRateLimit(1000).
    WithQuerySettings(200, 1*time.Second, 30*time.Second).
    WithRetryPolicy(3, 100*time.Millisecond).
    Build()

// For PostgreSQL
config := NewConfigBuilder().
    WithDatabaseType(DatabaseTypePostgreSQL).
    WithDSN("postgres://user:password@localhost:5432/dbname?sslmode=disable").
    WithConnectionPool(50, 10).
    WithConnectionLifetime(30*time.Minute, 10*time.Minute).
    WithLeakDetection(true, 10*time.Minute).
    WithCircuitBreaker(5, 60*time.Second, 10*time.Second).
    WithRateLimit(1000).
    WithQuerySettings(200, 1*time.Second, 30*time.Second).
    WithRetryPolicy(3, 100*time.Millisecond).
    Build()

// For MySQL
config := NewConfigBuilder().
    WithDatabaseType(DatabaseTypeMySQL).
    WithDSN("user:password@tcp(localhost:3306)/dbname?parseTime=true").
    WithConnectionPool(50, 10).
    WithConnectionLifetime(30*time.Minute, 10*time.Minute).
    WithLeakDetection(true, 10*time.Minute).
    WithCircuitBreaker(5, 60*time.Second, 10*time.Second).
    WithRateLimit(1000).
    WithQuerySettings(200, 1*time.Second, 30*time.Second).
    WithRetryPolicy(3, 100*time.Millisecond).
    Build()
```

### Using Environment Variables

All configuration options can be set via environment variables:

```bash
# Database type: oracle, postgres, or mysql
export DB_TYPE=mysql

# For Oracle
export DB_DSN="user/password@localhost:1521/XE"

# For PostgreSQL
export DB_DSN="postgres://user:password@localhost:5432/dbname?sslmode=disable"

# For MySQL
export DB_DSN="user:password@tcp(localhost:3306)/dbname?parseTime=true"

export DB_MAX_OPEN_CONNS=50
export DB_MAX_IDLE_CONNS=10
export DB_CONN_MAX_LIFETIME=30m
export DB_ENABLE_LEAK_DETECTION=true
export DB_CB_MAX_FAILURES=5
export DB_MAX_REQUESTS_PER_SEC=1000
export DB_STMT_CACHE_SIZE=200
export DB_SLOW_QUERY_THRESHOLD=1s
export DB_QUERY_TIMEOUT=30s
```

Then use:
```go
config := DefaultConfig()
```

## Usage Examples

### Basic Query Execution

```go
ctx := context.Background()
result, err := runtime.Exec(ctx, "INSERT INTO users (name) VALUES (:1)", "John")
if err != nil {
    log.Fatal(err)
}
```

### Query with Results

```go
rows, err := runtime.Query(ctx, "SELECT id, name FROM users")
if err != nil {
    log.Fatal(err)
}
defer rows.Close()

for rows.Next() {
    var id int
    var name string
    if err := rows.Scan(&id, &name); err != nil {
        log.Fatal(err)
    }
    fmt.Printf("User %d: %s\n", id, name)
}
```

### Transactions

```go
executor := NewQueryExecutor(runtime)
err := executor.Transaction(ctx, func(tx *AdvancedTx) error {
    _, err := tx.Exec(ctx, "INSERT INTO users (name) VALUES (:1)", "John")
    if err != nil {
        return err
    }
    _, err = tx.Exec(ctx, "UPDATE users SET last_login = SYSDATE WHERE name = :1", "John")
    return err
})
```

### Prepared Statements

```go
stmt, err := runtime.Prepare(ctx, "SELECT name FROM users WHERE id = :1")
if err != nil {
    log.Fatal(err)
}
defer stmt.Close()

row := stmt.QueryRow(1)
var name string
if err := row.Scan(&name); err != nil {
    log.Fatal(err)
}
```

### Health Checks

```go
health := CheckHealth(ctx, runtime)
if !health.Healthy {
    log.Printf("Database unhealthy: %s", health.Message)
}
```

### Diagnostics

```go
diagnostics := GetDiagnostics(runtime)
fmt.Println(diagnostics.String())
```

### Monitoring

```go
monitor := NewMonitor(runtime, 30*time.Second)
monitor.AddCallback(DefaultLoggingCallback)
monitor.Start(ctx)
defer monitor.Stop()
```

### Metrics

```go
metrics := runtime.Metrics()
fmt.Printf("Total Queries: %d\n", metrics.TotalQueries)
fmt.Printf("Success Rate: %.2f%%\n", metrics.SuccessRate)
fmt.Printf("Average Query Time: %v\n", metrics.AverageQueryTime)
```

## Advanced Features

### Circuit Breaker

The circuit breaker automatically opens when failures exceed the threshold, preventing cascading failures:

```go
state := runtime.CircuitBreakerState()
// Returns: "closed", "open", or "half-open"
```

### Connection Leak Detection

Automatically detects connections that have been held for too long:

```go
config := NewConfigBuilder().
    WithLeakDetection(true, 10*time.Minute).
    Build()
```

### Rate Limiting

Protects against overload with configurable rate limits:

```go
config := NewConfigBuilder().
    WithRateLimit(1000). // 1000 requests per second
    Build()
```

### Error Recovery

Automatic error recovery for transient failures:

```go
recovery := NewErrorRecovery(runtime)
if err := recovery.HandleError(ctx, err); err != nil {
    log.Fatal(err)
}
```

## Configuration Reference

### RuntimeConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| DSN | string | "" | Database connection string |
| MaxOpenConns | int | 50 | Maximum open connections |
| MaxIdleConns | int | 10 | Maximum idle connections |
| ConnMaxLifetime | time.Duration | 30m | Maximum connection lifetime |
| ConnMaxIdleTime | time.Duration | 10m | Maximum idle time |
| LeakDetectionThreshold | time.Duration | 10m | Leak detection threshold |
| EnableLeakDetection | bool | true | Enable leak detection |
| CircuitBreakerMaxFailures | int | 5 | Circuit breaker failure threshold |
| CircuitBreakerResetTimeout | time.Duration | 60s | Circuit breaker reset timeout |
| MaxRequestsPerSecond | int64 | 1000 | Rate limit (requests/sec) |
| MaxConcurrentConnections | int64 | 100 | Maximum concurrent connections |
| StmtCacheSize | int | 200 | Prepared statement cache size |
| SlowQueryThreshold | time.Duration | 1s | Slow query threshold |
| QueryTimeout | time.Duration | 30s | Query timeout |
| MaxRetries | int | 3 | Maximum retry attempts |
| RetryBackoff | time.Duration | 100ms | Retry backoff duration |

## Performance Considerations

1. **Connection Pool Size**: Set `MaxOpenConns` based on your database server capacity
2. **Statement Cache**: Increase `StmtCacheSize` for applications with many repeated queries
3. **Query Timeout**: Set appropriate `QueryTimeout` to prevent hanging queries
4. **Circuit Breaker**: Tune `CircuitBreakerMaxFailures` based on your failure tolerance
5. **Rate Limiting**: Set `MaxRequestsPerSecond` based on your database capacity

## Best Practices

1. Always use context with timeout for queries
2. Enable leak detection in development
3. Monitor metrics regularly
4. Set appropriate connection pool sizes
5. Use prepared statements for repeated queries
6. Handle circuit breaker states appropriately
7. Implement health checks in your application
8. Use transactions for multi-step operations

## Comparison with HikariCP

| Feature | HikariCP | This Runtime |
|---------|----------|-------------|
| Connection Pooling | ✅ | ✅ |
| Connection Validation | ✅ | ✅ |
| Leak Detection | ✅ | ✅ |
| Metrics | ✅ | ✅ |
| Circuit Breaker | ❌ | ✅ |
| Rate Limiting | ❌ | ✅ |
| Prepared Statement Cache | ❌ | ✅ |
| Advanced Retry | ❌ | ✅ |
| Connection Warm-up | ❌ | ✅ |
| Health Checks | ❌ | ✅ |
| Monitoring | ❌ | ✅ |

## License

[Your License Here]

## Contributing

[Contributing Guidelines]
