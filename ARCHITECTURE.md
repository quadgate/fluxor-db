# Kiến trúc Fluxor-DB

## Tổng quan

Fluxor-DB là một advanced database runtime hỗ trợ Oracle, PostgreSQL và MySQL, được thiết kế với kiến trúc phân lớp, tích hợp các pattern hiện đại để đảm bảo độ tin cậy, hiệu suất và khả năng mở rộng cho môi trường production.

## Kiến trúc tổng thể

```
┌─────────────────────────────────────────────────────────────┐
│                        Application Layer                     │
│                    (User Application Code)                   │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                      DBRuntime (dbruntime.go)               │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  • Unified API                                       │   │
│  │  • Connection lifecycle management                   │   │
│  │  • Health checks & monitoring                        │   │
│  │  • Metrics aggregation                               │   │
│  └─────────────────────────────────────────────────────┘   │
└───────┬──────────────┬──────────────┬─────────────┬─────────┘
        │              │              │             │
        ▼              ▼              ▼             ▼
┌──────────────┐ ┌──────────┐ ┌───────────┐ ┌──────────────┐
│ Connection   │ │ Connection│ │ AdvancedDB│ │   Monitor    │
│  Manager     │ │   Gate    │ │  (db.go)  │ │(monitor.go)  │
│ (open.go)    │ │ (gate.go) │ └───────────┘ └──────────────┘
└──────────────┘ └──────────┘       │
      │                │             │
      ▼                ▼             ▼
┌──────────────┐ ┌──────────┐ ┌───────────┐
│ • Leak       │ │ • Circuit │ │ • Stmt    │
│   Detector   │ │   Breaker │ │   Cache   │
│ • Validator  │ │ • Rate    │ │ • Metrics │
│ • Warmup     │ │   Limiter │ │ • Retry   │
└──────────────┘ └──────────┘ └───────────┘
        │                │             │
        └────────────────┴─────────────┘
                         │
                         ▼
              ┌──────────────────────────┐
              │  sql.DB                 │
              │  - godror (Oracle)      │
              │  - lib/pq (PostgreSQL)  │
              │  - mysql (MySQL)        │
              │  Database               │
              └──────────────────────────┘
```

## Các thành phần chính

### 1. DBRuntime - Core Orchestrator

**File**: [`dbruntime.go`](dbruntime.go)

**Trách nhiệm**:
- Điều phối tất cả các component
- Cung cấp unified API cho application layer
- Quản lý lifecycle của connection
- Tổng hợp metrics và health status

**Design Pattern**: Facade Pattern

```go
type DBRuntime struct {
    connManager *ConnectionManager  // Quản lý connection pool
    gate        *ConnectionGate     // Access control
    advancedDB  *AdvancedDB        // Query operations
    config      *RuntimeConfig      // Configuration
}
```

**Key Methods**:
- `Connect()` / `Disconnect()` - Lifecycle management
- `Exec()` / `Query()` / `QueryRow()` - Query operations
- `Begin()` - Transaction management
- `HealthCheck()` - Health monitoring
- `Metrics()` / `Stats()` - Performance monitoring

---

### 2. ConnectionManager - Connection Lifecycle

**File**: [`open.go`](open.go)

**Trách nhiệm**:
- Quản lý connection pool lifecycle
- Detect và report connection leaks
- Validate connections trước khi sử dụng
- Warm-up connections để giảm latency

**Design Pattern**: Object Pool Pattern + Observer Pattern

**Components**:

#### 2.1 LeakDetector
```go
type LeakDetector struct {
    maxConnectionAge time.Duration
    checkInterval    time.Duration
    stopChan         chan struct{}
    leakCallback     func(connID uint64, age time.Duration)
}
```
- Chạy background goroutine kiểm tra connections định kỳ
- Detect connections sống quá lâu (potential leaks)
- Trigger callback để log/alert

#### 2.2 ConnectionValidator
```go
type ConnectionValidator struct {
    validationQuery string
    timeout         time.Duration
    maxRetries      int
    retryBackoff    time.Duration
}
```
- Validate connection trước khi trả về cho client
- Retry với exponential backoff nếu validation fail
- Oracle-specific: `SELECT 1 FROM DUAL`

#### 2.3 Connection Warmup
- Pre-create connections khi khởi động
- Giảm cold start latency
- Chạy async để không block startup

**Configuration**:
```go
type AdvancedConfig struct {
    DSN                    string
    MaxOpenConns           int
    MaxIdleConns           int
    ConnMaxLifetime        time.Duration
    LeakDetectionThreshold time.Duration
    ValidationQuery        string
    WarmupConnections      int
}
```

---

### 3. ConnectionGate - Access Control & Resilience

**File**: [`gate.go`](gate.go)

**Trách nhiệm**:
- Bảo vệ database khỏi overload
- Prevent cascading failures
- Rate limiting và connection limiting

**Design Patterns**: 
- Circuit Breaker Pattern
- Token Bucket Pattern
- Semaphore Pattern

**Components**:

#### 3.1 Circuit Breaker
```
States:
┌────────┐  failures > threshold   ┌──────┐
│ CLOSED │─────────────────────────▶│ OPEN │
└────────┘                          └──────┘
    ▲                                   │
    │                                   │ reset timeout
    │                               ┌───▼───────┐
    └───────── test success ────────│ HALF-OPEN │
                                    └───────────┘
```

**States**:
- **CLOSED**: Normal operation, requests pass through
- **OPEN**: Too many failures, reject all requests
- **HALF-OPEN**: Test period, allow limited requests

#### 3.2 Rate Limiter (Token Bucket)
```go
type RateLimiter struct {
    tokens     int64          // Current tokens
    maxTokens  int64          // Bucket capacity
    refillRate int64          // Tokens per second
    lastRefill time.Time
}
```

**Algorithm**:
1. Refill tokens based on elapsed time
2. Check if tokens available
3. Consume token if available
4. Reject if no tokens

#### 3.3 Connection Limiter
```go
type ConnectionLimiter struct {
    currentConnections int64
    maxConnections     int64
}
```
- Semaphore-based limiting
- Track concurrent connections
- Block when limit reached

---

### 4. AdvancedDB - Query Operations

**File**: [`db.go`](db.go)

**Trách nhiệm**:
- Execute queries với advanced features
- Cache prepared statements
- Collect performance metrics
- Retry failed operations

**Design Patterns**:
- Decorator Pattern (wraps sql.DB)
- Cache Pattern
- Retry Pattern

**Components**:

#### 4.1 PreparedStatementCache
```go
type PreparedStatementCache struct {
    cache   map[string]*sql.Stmt
    maxSize int
    mu      sync.RWMutex
}
```

**Cache Strategy**:
- LRU (Least Recently Used) eviction
- Thread-safe với RWMutex
- Configurable size (default: 200)

**Benefits**:
- Giảm parsing overhead
- Tăng performance cho repeated queries
- Memory efficient

#### 4.2 DBMetrics
```go
type DBMetrics struct {
    TotalQueries      int64
    SuccessfulQueries int64
    FailedQueries     int64
    TotalQueryTime    int64  // nanoseconds
    SlowQueries       int64
}
```

**Metrics Collection**:
- Real-time query statistics
- Slow query detection
- Performance trends
- Thread-safe với atomic operations

#### 4.3 RetryPolicy
```go
type RetryPolicy struct {
    MaxRetries        int
    InitialBackoff    time.Duration
    BackoffMultiplier float64
    RetryableErrors   []error
}
```

**Retry Strategy**:
- Exponential backoff: `delay = initial * multiplier^attempt`
- Selective retry (only retryable errors)
- Context-aware (respect timeouts)

**Flow**:
```
Attempt 1 ──fail──▶ Wait 100ms ──▶ Attempt 2 ──fail──▶ Wait 200ms ──▶ Attempt 3
                                                                         │
                                                                     success/fail
```

#### 4.4 Transaction Support
```go
type AdvancedTx struct {
    tx      *sql.Tx
    metrics *TxMetrics
}
```
- Wrap sql.Tx with metrics
- Track transaction duration
- Monitor commit/rollback rates

---

### 5. Configuration Management

**Files**: [`config.go`](config.go), [`utils.go`](utils.go)

**Design Pattern**: Builder Pattern

```go
config := NewConfigBuilder().
    WithDSN("user/password@localhost:1521/XE").
    WithConnectionPool(50, 10).
    WithCircuitBreaker(5, 60*time.Second, 10*time.Second).
    WithRateLimit(1000).
    Build()
```

**Features**:
- Fluent API
- Environment variable support
- Validation
- Sensible defaults

---

### 6. Error Handling

**File**: [`errors.go`](errors.go)

**Error Hierarchy**:
```go
type DBError struct {
    Code       ErrorCode
    Message    string
    Err        error
    Retryable  bool
}
```

**Error Codes**:
- `ErrCodeConnection` - Connection failures
- `ErrCodeQuery` - Query execution errors
- `ErrCodeTimeout` - Timeout errors
- `ErrCodeCircuitOpen` - Circuit breaker open
- `ErrCodeRateLimit` - Rate limit exceeded

---

### 7. Monitoring System

**File**: [`monitor.go`](monitor.go)

**Design Pattern**: Observer Pattern

```go
type Monitor struct {
    interval time.Duration
    stopChan chan struct{}
    callback func(MonitorEvent)
}
```

**Monitor Events**:
- `HealthCheckEvent` - Health status changes
- `MetricsEvent` - Periodic metrics
- `CircuitBreakerEvent` - State changes
- `LeakDetectionEvent` - Connection leaks

---

## Data Flow

### Query Execution Flow

```
Application
    │
    │ 1. runtime.Exec(ctx, query, args)
    ▼
DBRuntime
    │
    │ 2. Check connection status
    ▼
AdvancedDB
    │
    │ 3. Apply query timeout
    ▼
ExecuteWithGate()
    │
    │ 4. Circuit breaker check
    │ 5. Rate limiter check
    │ 6. Connection limiter check
    ▼
ConnectionGate ──[ALLOWED]──▶ RetryExec()
    │                              │
    │                              │ 7. Execute with retry
    │                              │ 8. Exponential backoff
    │                              ▼
    │                         sql.DB.ExecContext()
    │                              │
    │                              ▼
    │                         Oracle Database
    │                              │
    │◀─────────[RESULT]────────────┘
    │
    │ 9. Record metrics
    │ 10. Update circuit breaker
    ▼
Return to Application
```

### Connection Acquisition Flow

```
Application Request
    │
    │ 1. Get connection
    ▼
ConnectionManager
    │
    │ 2. Check pool availability
    ▼
ConnectionValidator
    │
    │ 3. Validate connection health
    │ 4. Retry if invalid
    ▼
LeakDetector
    │
    │ 5. Track connection acquisition
    │ 6. Start leak detection timer
    ▼
Return connection to Application
    │
    │ Usage...
    ▼
Connection Release
    │
    │ 7. Update last used time
    │ 8. Return to pool
    ▼
ConnectionManager
```

---

## Concurrency Model

### Thread Safety

**Synchronization Primitives**:

1. **RWMutex** (Read-Write Lock)
   - Used in: PreparedStatementCache, ConnectionManager, ConnectionGate
   - Allow multiple readers, single writer
   - Optimize for read-heavy workloads

2. **Atomic Operations**
   - Used in: Metrics, Circuit Breaker state
   - Lock-free counters
   - Better performance than mutex for simple operations

3. **Channels**
   - Used in: LeakDetector, Monitor
   - Goroutine communication
   - Graceful shutdown

**Goroutine Management**:
```
DBRuntime
    ├─▶ LeakDetector (background goroutine)
    │   └─▶ Check connections every N seconds
    │
    ├─▶ Monitor (background goroutine)
    │   └─▶ Collect metrics every N seconds
    │
    └─▶ ConnectionWarmup (one-time goroutine)
        └─▶ Pre-create connections on startup
```

---

## Performance Optimizations

### 1. Connection Pooling
- **Max Open**: 50 (default)
- **Max Idle**: 10 (default)
- **Lifetime**: 30 minutes
- **Idle Time**: 10 minutes

### 2. Prepared Statement Caching
- **Cache Size**: 200 statements
- **Eviction**: LRU (Least Recently Used)
- **Performance Gain**: 10-100x for repeated queries

### 3. Query Timeout
- **Default**: 30 seconds
- **Context-based**: Respect parent context deadlines
- **Prevents**: Hanging queries

### 4. Connection Warmup
- **Pre-create**: 5 connections (configurable)
- **Timeout**: 30 seconds
- **Reduces**: First request latency

### 5. Rate Limiting
- **Token Bucket**: 1000 req/sec (default)
- **Burst**: Allowed based on bucket size
- **Protects**: Database from overload

---

## Resilience Strategies

### 1. Circuit Breaker
- **Purpose**: Prevent cascading failures
- **Threshold**: 5 failures (default)
- **Reset**: 60 seconds
- **Half-Open**: 10 seconds test period

### 2. Retry with Exponential Backoff
- **Max Retries**: 3 (default)
- **Initial Backoff**: 100ms
- **Multiplier**: 2.0
- **Max Backoff**: 5 seconds

### 3. Connection Validation
- **Pre-use validation**: Ping before use
- **Retry**: 3 attempts
- **Backoff**: 100ms, 200ms, 400ms

### 4. Leak Detection
- **Threshold**: 10 minutes
- **Check Interval**: 1 minute
- **Action**: Log warning + callback

---

## Configuration Best Practices

### Production Settings

```go
config := NewConfigBuilder().
    // Connection Pool
    WithConnectionPool(
        maxOpen: 50,    // Based on expected concurrency
        maxIdle: 10,    // 20% of maxOpen
    ).
    WithConnectionLifetime(
        maxLifetime: 30 * time.Minute,
        maxIdleTime: 10 * time.Minute,
    ).
    
    // Resilience
    WithCircuitBreaker(
        maxFailures: 5,
        resetTimeout: 60 * time.Second,
        halfOpenTimeout: 10 * time.Second,
    ).
    WithRateLimit(1000).  // Requests per second
    
    // Performance
    WithQuerySettings(
        stmtCacheSize: 200,
        slowQueryThreshold: 1 * time.Second,
        queryTimeout: 30 * time.Second,
    ).
    
    // Monitoring
    WithLeakDetection(true, 10 * time.Minute).
    Build()
```

### Development Settings

```go
config := NewConfigBuilder().
    WithConnectionPool(10, 2).
    WithQuerySettings(50, 500*time.Millisecond, 10*time.Second).
    WithLeakDetection(true, 1*time.Minute).
    Build()
```

---

## Monitoring & Observability

### Metrics Exported

1. **Connection Pool Metrics**:
   - `db.connections.open` - Current open connections
   - `db.connections.idle` - Current idle connections
   - `db.connections.max_open` - Max open connections
   - `db.connections.wait_count` - Connection wait count
   - `db.connections.wait_duration` - Total wait duration

2. **Query Metrics**:
   - `db.queries.total` - Total queries executed
   - `db.queries.success` - Successful queries
   - `db.queries.failed` - Failed queries
   - `db.queries.slow` - Slow queries (above threshold)
   - `db.queries.duration` - Query duration histogram

3. **Circuit Breaker Metrics**:
   - `db.circuit.state` - Current state (closed/open/half-open)
   - `db.circuit.failures` - Failure count
   - `db.circuit.rejected` - Rejected requests

4. **Rate Limiter Metrics**:
   - `db.ratelimit.allowed` - Allowed requests
   - `db.ratelimit.rejected` - Rejected requests
   - `db.ratelimit.tokens` - Current tokens

### Health Check Endpoints

```go
// Simple health check
err := runtime.HealthCheck(ctx)

// Detailed diagnostics
diagnostics := runtime.Diagnostics()
// Returns:
// - Connection pool status
// - Circuit breaker state
// - Recent errors
// - Performance metrics
```

---

## Extension Points

### 1. Custom Metrics Callback
```go
runtime.SetMetricsCallback(func(metrics MetricsStats) {
    // Export to Prometheus, StatsD, etc.
})
```

### 2. Custom Leak Detector Callback
```go
runtime.SetLeakCallback(func(connID uint64, age time.Duration) {
    // Custom alerting logic
})
```

### 3. Custom Circuit Breaker State Change
```go
runtime.SetCircuitBreakerCallback(func(from, to string) {
    // Log state changes
})
```

### 4. Custom Monitor Events
```go
monitor := NewMonitor(1*time.Minute, func(event MonitorEvent) {
    // Process monitoring events
})
```

---

## Testing Strategy

### Unit Tests
- **Files**: `*_test.go`
- **Coverage**: 30.1% (baseline)
- **Focus**: Individual component behavior

### Integration Tests
- **Mock database**: In-memory SQLite or testcontainers
- **Test scenarios**: Connection failures, retries, circuit breaker

### Performance Tests
- **Benchmarks**: Query performance, cache hit rates
- **Load tests**: Concurrent connection handling

### Race Detection
```bash
go test -race ./...
```

---

## Dependencies

```
github.com/godror/godror v0.49.6        # Oracle driver
github.com/lib/pq v1.10.9               # PostgreSQL driver
github.com/go-sql-driver/mysql v1.9.3   # MySQL driver
golang.org/x/exp                         # Experimental features
```

**Minimalist approach**: Zero unnecessary dependencies

---

## Future Enhancements

1. **Distributed Tracing** - OpenTelemetry integration
2. **Query Builder** - Type-safe query construction
3. **Schema Migration** - Built-in migration tool
4. **Read Replicas** - Automatic read/write splitting
5. **Connection Sharding** - Multi-database support
6. **Metrics Export** - Prometheus exporter
7. **Admin UI** - Web-based monitoring dashboard

---

## References

### Design Patterns
- **Circuit Breaker**: Martin Fowler's Circuit Breaker Pattern
- **Object Pool**: Gang of Four Design Patterns
- **Retry**: AWS Architecture Blog - Exponential Backoff and Jitter

### Inspirations
- **HikariCP**: Java connection pool
- **c3p0**: Java connection pool
- **DBCP**: Apache Commons DBCP
- **pgx**: Go PostgreSQL driver

### Oracle Documentation
- Oracle Database Performance Tuning Guide
- Oracle Call Interface (OCI) Programming Guide
