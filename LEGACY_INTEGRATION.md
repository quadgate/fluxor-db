# Legacy Database Integration Use Cases

## Common Legacy Database Challenges

### 1. **Legacy Oracle Systems (Pre-12c)**
```go
// Typical legacy Oracle with connection issues
config := NewConfigBuilder().
    WithDatabaseType(DatabaseTypeOracle).
    WithDSN("user/password@legacy-oracle:1521/XE").
    WithBackpressure("timeout", 5*time.Second). // handle slow responses
    WithConnectionPool(20, 5).                   // limited connections for old hardware
    WithCircuitBreaker(3, 60*time.Second, 10*time.Second).
    Build()

runtime := NewDBRuntime(config)
// Add cache to reduce load on legacy system
runtime.SetCache(NewInMemoryCache(500, 300*time.Second)) // 5min TTL
```

### 2. **Legacy PostgreSQL (8.x/9.x)**
```go
// Old PostgreSQL without modern features
config := NewConfigBuilder().
    WithDatabaseType(DatabaseTypePostgreSQL).
    WithDSN("postgres://user:pass@legacy-pg:5432/olddb?sslmode=disable").
    WithBackpressure("block", 0). // block instead of failing
    Build()

runtime := NewDBRuntime(config)
// Cache reference data heavily used in legacy apps
runtime.SetCache(NewInMemoryCache(1000, 600*time.Second)) // 10min TTL

// Cache expensive legacy queries
ctx := context.Background()
cols, rows, hit, _ := runtime.QueryCached(ctx, "users:active", 
    30*time.Minute, 
    "SELECT * FROM users WHERE status = 'ACTIVE' AND legacy_flag = 'Y'")
```

### 3. **Legacy MySQL (5.x)**
```go
// Old MySQL with performance issues
config := NewConfigBuilder().
    WithDatabaseType(DatabaseTypeMySQL).
    WithDSN("user:pass@tcp(legacy-mysql:3306)/olddb").
    WithBackpressure("timeout", 10*time.Second).
    WithQuerySettings(50, 2*time.Second, 30*time.Second).
    Build()

runtime := NewDBRuntime(config)
runtime.SetCache(NewInMemoryCache(2000, 900*time.Second)) // 15min TTL
```

## Migration Patterns

### 1. **Gradual Modernization**
```go
// Step 1: Add runtime wrapper around legacy DB
legacy := NewDBRuntime(legacyConfig)
legacy.SetCache(NewInMemoryCache(1000, 300*time.Second))

// Step 2: Expose as microservice via TCP
server := NewTCPServer(&TCPServerConfig{
    Address: "0.0.0.0:9090",
    Runtime: legacy,
})
server.Start()

// Step 3: Modern services connect via TCP
client := NewTCPClient(&TCPClientConfig{
    Address: "legacy-db-service:9090",
})
```

### 2. **Cache-First Strategy**
```go
func QueryLegacyWithCache(rt *DBRuntime, cacheKey string, query string, args ...interface{}) ([][]interface{}, error) {
    ctx := context.Background()
    
    // Try cache first (important for legacy DB protection)
    _, rows, fromCache, err := rt.QueryCached(ctx, cacheKey, 15*time.Minute, query, args...)
    if err != nil {
        return nil, err
    }
    
    if fromCache {
        fmt.Println("Protected legacy DB by serving from cache")
    }
    
    return rows, nil
}
```

### 3. **Legacy Wrapper Service**
```go
type LegacyDBService struct {
    runtime *DBRuntime
}

func NewLegacyDBService(dsn string) *LegacyDBService {
    config := NewConfigBuilder().
        WithDSN(dsn).
        WithBackpressure("block", 0).        // protect legacy DB
        WithCircuitBreaker(5, 60*time.Second, 15*time.Second).
        Build()
    
    rt := NewDBRuntime(config)
    rt.SetCache(NewInMemoryCache(5000, 600*time.Second)) // aggressive caching
    
    return &LegacyDBService{runtime: rt}
}

func (s *LegacyDBService) GetUser(id int) (*User, error) {
    cacheKey := fmt.Sprintf("user:%d", id)
    ctx := context.Background()
    
    _, rows, _, err := s.runtime.QueryCached(ctx, cacheKey, 30*time.Minute,
        "SELECT id, name, email FROM users WHERE id = ?", id)
    
    if err != nil || len(rows) == 0 {
        return nil, err
    }
    
    return &User{
        ID:    int(rows[0][0].(int64)),
        Name:  rows[0][1].(string),
        Email: rows[0][2].(string),
    }, nil
}
```

## Environment Configuration for Legacy Systems

```bash
# Legacy database protection
export DB_BACKPRESSURE_MODE=block                    # protect legacy DB
export DB_MAX_CONCURRENT_CONNECTIONS=25              # limited for legacy hardware
export DB_CB_MAX_FAILURES=3                         # circuit breaker
export DB_CB_RESET_TIMEOUT=120s                     # longer recovery time
export DB_QUERY_TIMEOUT=60s                         # legacy queries can be slow
export DB_MAX_RETRIES=5                            # retry for flaky connections
export DB_CONN_MAX_LIFETIME=300s                    # shorter for unstable connections

# Cache configuration (Redis alternative)  
export CACHE_CAPACITY=10000
export CACHE_DEFAULT_TTL=900s                       # 15 minutes default
```

## Benefits for Legacy Integration

1. **No Infrastructure Changes** - No Redis/external cache needed
2. **Gradual Migration** - Wrap legacy DB incrementally  
3. **Protection** - Backpressure prevents legacy DB overload
4. **Resilience** - Circuit breaker handles legacy DB failures
5. **Performance** - In-memory cache reduces legacy DB load
6. **Microservices Ready** - TCP layer enables service architecture
7. **Monitoring** - Metrics for legacy DB performance visibility