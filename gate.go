package main

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

const (
	CircuitStateClosed   = "closed"
	CircuitStateOpen     = "open"
	CircuitStateHalfOpen = "half-open"
)

var (
	ErrCircuitOpen       = errors.New("circuit breaker is open")
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
	ErrConnectionLimit   = errors.New("connection limit exceeded")
)

// ConnectionGate manages connection access with advanced features
type ConnectionGate struct {
	circuitBreaker    *CircuitBreaker
	rateLimiter       *RateLimiter
	connectionLimiter *ConnectionLimiter
	mu                sync.RWMutex
}

// CircuitBreaker implements circuit breaker pattern
type CircuitBreaker struct {
	maxFailures     int
	resetTimeout    time.Duration
	halfOpenTimeout time.Duration
	failureCount    int64
	lastFailureTime time.Time
	state           int32 // 0: closed, 1: open, 2: half-open
	mu              sync.RWMutex
	onStateChange   func(from, to string)
}

const (
	circuitClosed = iota
	circuitOpen
	circuitHalfOpen
)

// RateLimiter implements token bucket rate limiting
type RateLimiter struct {
	tokens     int64
	maxTokens  int64
	refillRate int64 // tokens per second
	lastRefill time.Time
	mu         sync.Mutex
}

// ConnectionLimiter limits concurrent connections
type ConnectionLimiter struct {
	currentConnections int64
	maxConnections     int64
	mu                 sync.RWMutex
	// backpressure support
	mode    string
	timeout time.Duration
	sem     chan struct{}
}

// NewConnectionGate creates a new connection gate
func NewConnectionGate(config *GateConfig) *ConnectionGate {
	return &ConnectionGate{
		circuitBreaker:    NewCircuitBreaker(config),
		rateLimiter:       NewRateLimiter(config),
		connectionLimiter: NewConnectionLimiter(config),
	}
}

// GateConfig configures the connection gate
type GateConfig struct {
	// Circuit breaker
	MaxFailures     int
	ResetTimeout    time.Duration
	HalfOpenTimeout time.Duration

	// Rate limiting
	MaxRequestsPerSecond int64

	// Connection limiting
	MaxConcurrentConnections int64

	// Backpressure behavior when hitting connection limit
	// Modes:
	//   "drop"   - return error immediately (default, backwards compatible)
	//   "block"  - block until a slot is available or context is canceled
	//   "timeout"- wait up to BackpressureTimeout
	BackpressureMode    string
	BackpressureTimeout time.Duration
}

// Allow checks if a connection request should be allowed
func (cg *ConnectionGate) Allow(ctx context.Context) error {
	// Check circuit breaker
	if err := cg.circuitBreaker.Allow(ctx); err != nil {
		return err
	}

	// Check rate limiter
	if err := cg.rateLimiter.Allow(); err != nil {
		cg.circuitBreaker.RecordFailure()
		return err
	}

	// Check connection limiter
	if err := cg.connectionLimiter.AcquireWithContext(ctx); err != nil {
		cg.circuitBreaker.RecordFailure()
		return err
	}

	return nil
}

// Release releases a connection slot
func (cg *ConnectionGate) Release() {
	cg.connectionLimiter.Release()
}

// RecordSuccess records a successful operation
func (cg *ConnectionGate) RecordSuccess() {
	cg.circuitBreaker.RecordSuccess()
}

// RecordFailure records a failed operation
func (cg *ConnectionGate) RecordFailure() {
	cg.circuitBreaker.RecordFailure()
	cg.connectionLimiter.Release()
}

// State returns the current circuit breaker state
func (cg *ConnectionGate) State() string {
	return cg.circuitBreaker.State()
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config *GateConfig) *CircuitBreaker {
	cb := &CircuitBreaker{
		maxFailures:     5,
		resetTimeout:    60 * time.Second,
		halfOpenTimeout: 10 * time.Second,
		state:           circuitClosed,
	}

	if config != nil {
		if config.MaxFailures > 0 {
			cb.maxFailures = config.MaxFailures
		}
		if config.ResetTimeout > 0 {
			cb.resetTimeout = config.ResetTimeout
		}
		if config.HalfOpenTimeout > 0 {
			cb.halfOpenTimeout = config.HalfOpenTimeout
		}
	}

	return cb
}

// Allow checks if the circuit breaker allows the operation
func (cb *CircuitBreaker) Allow(_ context.Context) error {
	cb.mu.RLock()
	state := atomic.LoadInt32(&cb.state)
	cb.mu.RUnlock()

	switch state {
	case circuitOpen:
		// Check if we should transition to half-open
		cb.mu.Lock()
		if time.Since(cb.lastFailureTime) > cb.resetTimeout {
			atomic.StoreInt32(&cb.state, circuitHalfOpen)
			atomic.StoreInt64(&cb.failureCount, 0)
			if cb.onStateChange != nil {
				cb.onStateChange(CircuitStateOpen, CircuitStateHalfOpen)
			}
			cb.mu.Unlock()
			return nil
		}
		cb.mu.Unlock()
		return ErrCircuitOpen

	case circuitHalfOpen:
		return nil

	case circuitClosed:
		return nil
	}

	return nil
}

// RecordSuccess records a successful operation
func (cb *CircuitBreaker) RecordSuccess() {
	state := atomic.LoadInt32(&cb.state)

	if state == circuitHalfOpen {
		cb.mu.Lock()
		atomic.StoreInt32(&cb.state, circuitClosed)
		atomic.StoreInt64(&cb.failureCount, 0)
		if cb.onStateChange != nil {
			cb.onStateChange(CircuitStateHalfOpen, CircuitStateClosed)
		}
		cb.mu.Unlock()
	} else if state == circuitClosed {
		atomic.StoreInt64(&cb.failureCount, 0)
	}
}

// RecordFailure records a failed operation
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	state := atomic.LoadInt32(&cb.state)
	failures := atomic.AddInt64(&cb.failureCount, 1)
	cb.lastFailureTime = time.Now()

	if state == circuitHalfOpen {
		// Immediately open on failure in half-open state
		atomic.StoreInt32(&cb.state, circuitOpen)
		if cb.onStateChange != nil {
			cb.onStateChange(CircuitStateHalfOpen, CircuitStateOpen)
		}
	} else if state == circuitClosed && int(failures) >= cb.maxFailures {
		atomic.StoreInt32(&cb.state, circuitOpen)
		if cb.onStateChange != nil {
			cb.onStateChange(CircuitStateClosed, CircuitStateOpen)
		}
	}
}

// State returns the current state as a string
func (cb *CircuitBreaker) State() string {
	state := atomic.LoadInt32(&cb.state)
	switch state {
	case circuitClosed:
		return CircuitStateClosed
	case circuitOpen:
		return CircuitStateOpen
	case circuitHalfOpen:
		return CircuitStateHalfOpen
	}
	return "unknown"
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(config *GateConfig) *RateLimiter {
	rl := &RateLimiter{
		maxTokens:  1000,
		refillRate: 100,
		lastRefill: time.Now(),
	}

	if config != nil && config.MaxRequestsPerSecond > 0 {
		rl.maxTokens = config.MaxRequestsPerSecond * 10 // 10 seconds worth
		rl.refillRate = config.MaxRequestsPerSecond
	}

	rl.tokens = rl.maxTokens
	return rl
}

// Allow checks if a request is allowed under rate limiting
func (rl *RateLimiter) Allow() error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastRefill)

	// Refill tokens
	tokensToAdd := int64(elapsed.Seconds() * float64(rl.refillRate))
	if tokensToAdd > 0 {
		rl.tokens = min(rl.tokens+tokensToAdd, rl.maxTokens)
		rl.lastRefill = now
	}

	if rl.tokens > 0 {
		rl.tokens--
		return nil
	}

	return ErrRateLimitExceeded
}

// NewConnectionLimiter creates a new connection limiter
func NewConnectionLimiter(config *GateConfig) *ConnectionLimiter {
	cl := &ConnectionLimiter{
		maxConnections: 100,
	}

	if config != nil && config.MaxConcurrentConnections > 0 {
		cl.maxConnections = config.MaxConcurrentConnections
	}

	if config != nil {
		cl.mode = config.BackpressureMode
		cl.timeout = config.BackpressureTimeout
		if cl.mode == "block" || cl.mode == "timeout" {
			// semaphore with capacity = maxConnections to block when full
			cl.sem = make(chan struct{}, cl.maxConnections)
		}
	}

	return cl
}

// Acquire acquires a connection slot
func (cl *ConnectionLimiter) Acquire() error {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	if atomic.LoadInt64(&cl.currentConnections) >= cl.maxConnections {
		return ErrConnectionLimit
	}

	atomic.AddInt64(&cl.currentConnections, 1)
	return nil
}

// AcquireWithContext acquires a connection slot with backpressure behavior
func (cl *ConnectionLimiter) AcquireWithContext(ctx context.Context) error {
	// If no backpressure semaphore configured, use legacy non-blocking path
	if cl.sem == nil {
		return cl.Acquire()
	}

	// Fast path: if below maxConnections we also increment counters
	// We rely on semaphore to represent in-flight usage consistently
	select {
	case cl.sem <- struct{}{}:
		atomic.AddInt64(&cl.currentConnections, 1)
		return nil
	default:
		// Full: apply backpressure according to mode
	}

	switch cl.mode {
	case "block":
		select {
		case cl.sem <- struct{}{}:
			atomic.AddInt64(&cl.currentConnections, 1)
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	case "timeout":
		timeout := cl.timeout
		if timeout <= 0 {
			// fallback to immediate failure if timeout not set
			return ErrConnectionLimit
		}
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		select {
		case cl.sem <- struct{}{}:
			atomic.AddInt64(&cl.currentConnections, 1)
			return nil
		case <-timer.C:
			return ErrConnectionLimit
		case <-ctx.Done():
			return ctx.Err()
		}
	default:
		// drop (legacy behavior)
		return ErrConnectionLimit
	}
}

// Release releases a connection slot
func (cl *ConnectionLimiter) Release() {
	if cl.sem != nil {
		// release semaphore token if acquired
		select {
		case <-cl.sem:
		default:
		}
	}
	atomic.AddInt64(&cl.currentConnections, -1)
}

// CurrentConnections returns the current number of connections
func (cl *ConnectionLimiter) CurrentConnections() int64 {
	return atomic.LoadInt64(&cl.currentConnections)
}

// ExecuteWithGate executes a database operation with gate protection
func ExecuteWithGate[T any](
	gate *ConnectionGate,
	ctx context.Context,
	operation func(context.Context) (T, error),
) (T, error) {
	var zero T

	// Check gate
	if err := gate.Allow(ctx); err != nil {
		return zero, err
	}

	// Execute operation
	result, err := operation(ctx)

	if err != nil {
		gate.RecordFailure()
		return zero, err
	}

	gate.RecordSuccess()
	return result, nil
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
