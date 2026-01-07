package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// TCPServer represents a TCP server for database runtime
type TCPServer struct {
	config        *TCPServerConfig
	runtime       *DBRuntime
	address       string
	listener      net.Listener
	clients       sync.Map
	clientCounter uint64
	shutdown      chan struct{}
	wg            sync.WaitGroup
	mu            sync.RWMutex
	// DDoS protection
	ipConnections map[string]int
	ipRateLimits  map[string]*time.Time
	blacklistMap  map[string]bool
	whitelistMap  map[string]bool
	// Idempotency
	idempotencyCache Cache
}

// TCPServerConfig configures the TCP server
type TCPServerConfig struct {
	Address              string
	Runtime              *DBRuntime
	EnableIdempotency    bool
	EnableDDoSProtection bool
	MaxRequestSize       int64
	MaxConnectionsPerIP  int
	RateLimitPerIP       int64  // requests per second per IP
	BlacklistedIPs       []string
	WhitelistedIPs       []string
}

// NewTCPServer creates a new TCP server
func NewTCPServer(config *TCPServerConfig) *TCPServer {
	server := &TCPServer{
		config:        config,
		runtime:       config.Runtime,
		address:       config.Address,
		shutdown:      make(chan struct{}),
		ipConnections: make(map[string]int),
		ipRateLimits:  make(map[string]*time.Time),
		blacklistMap:  make(map[string]bool),
		whitelistMap:  make(map[string]bool),
	}

	// Initialize blacklist
	for _, ip := range config.BlacklistedIPs {
		server.blacklistMap[ip] = true
	}

	// Initialize whitelist
	for _, ip := range config.WhitelistedIPs {
		server.whitelistMap[ip] = true
	}

	// Initialize idempotency cache if enabled
	if config.EnableIdempotency {
		server.idempotencyCache = NewInMemoryCache(10000, 300*time.Second) // 5min TTL
	}

	return server
}

// Start starts the TCP server
func (s *TCPServer) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.listener != nil {
		return fmt.Errorf("server already started")
	}

	listener, err := net.Listen("tcp", s.address)
	if err != nil {
		return fmt.Errorf("failed to start TCP server: %w", err)
	}

	s.listener = listener
	log.Printf("TCP server listening on %s", s.address)

	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

// Stop stops the TCP server
func (s *TCPServer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.listener == nil {
		return fmt.Errorf("server not started")
	}

	close(s.shutdown)
	s.listener.Close()

	// Close all client connections
	s.clients.Range(func(key, value interface{}) bool {
		if conn, ok := value.(net.Conn); ok {
			conn.Close()
		}
		return true
	})

	s.wg.Wait()
	log.Printf("TCP server stopped")
	return nil
}

// acceptLoop accepts incoming connections
func (s *TCPServer) acceptLoop() {
	defer s.wg.Done()

	for {
		select {
		case <-s.shutdown:
			return
		default:
		}

		// Set accept deadline to allow periodic shutdown checks
		s.listener.(*net.TCPListener).SetDeadline(time.Now().Add(1 * time.Second))

		conn, err := s.listener.Accept()
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			select {
			case <-s.shutdown:
				return
			default:
				log.Printf("Accept error: %v", err)
				continue
			}
		}

		clientID := atomic.AddUint64(&s.clientCounter, 1)
		s.clients.Store(clientID, conn)

		s.wg.Add(1)
		go s.handleClient(clientID, conn)
	}
}

// handleClient handles a client connection
func (s *TCPServer) handleClient(clientID uint64, conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()
	defer s.clients.Delete(clientID)

	clientIP := s.getClientIP(conn)
	log.Printf("Client %d connected from %s (IP: %s)", clientID, conn.RemoteAddr(), clientIP)

	// DDoS protection checks
	if s.config.EnableDDoSProtection && !s.allowConnection(clientIP) {
		log.Printf("Connection from %s blocked by DDoS protection", clientIP)
		return
	}

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer

	for scanner.Scan() {
		select {
		case <-s.shutdown:
			return
		default:
		}

		data := scanner.Bytes()
		
		// DDoS protection - track request size
		requestSize := int64(len(data))
		
		msg, err := DecodeTCPMessage(data)
		if err != nil {
			log.Printf("Failed to decode message from client %d: %v", clientID, err)
			s.sendError(conn, "", err)
			continue
		}
		
		msg.RequestSize = requestSize
		msg.ClientIP = clientIP

		s.handleMessage(conn, msg)

		if msg.Type == MessageTypeClose {
			log.Printf("Client %d requested close", clientID)
			return
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Scanner error for client %d: %v", clientID, err)
	}

	log.Printf("Client %d disconnected", clientID)
}

// handleMessage handles a single message
func (s *TCPServer) handleMessage(conn net.Conn, msg *TCPMessage) {
	clientIP := s.getClientIP(conn)
	
	// Set client IP for tracking
	msg.ClientIP = clientIP
	
	// DDoS protection - request size check
	if s.config.EnableDDoSProtection && s.config.MaxRequestSize > 0 {
		if msg.RequestSize > s.config.MaxRequestSize {
			s.sendError(conn, msg.ID, fmt.Errorf("request too large: %d bytes", msg.RequestSize))
			return
		}
	}
	
	// DDoS protection - rate limiting per IP
	if s.config.EnableDDoSProtection && !s.checkRateLimit(clientIP) {
		s.sendError(conn, msg.ID, fmt.Errorf("rate limit exceeded for IP: %s", clientIP))
		return
	}
	
	// Idempotency check
	if s.config.EnableIdempotency && msg.IdempotencyKey != "" {
		if result := s.checkIdempotency(msg); result != nil {
			s.sendResponse(conn, result)
			return
		}
	}

	ctx := context.Background()

	switch msg.Type {
	case MessageTypePing:
		s.handlePing(conn, msg)

	case MessageTypeExec:
		response := s.handleExec(ctx, conn, msg)
		if s.config.EnableIdempotency && msg.IdempotencyKey != "" {
			s.storeIdempotency(msg, response)
		}

	case MessageTypeQuery:
		response := s.handleQuery(ctx, conn, msg)
		if s.config.EnableIdempotency && msg.IdempotencyKey != "" {
			s.storeIdempotency(msg, response)
		}

	case MessageTypeStats:
		s.handleStats(conn, msg)

	case MessageTypeMetrics:
		s.handleMetrics(conn, msg)

	default:
		s.sendError(conn, msg.ID, fmt.Errorf("unknown message type: %s", msg.Type))
	}
}

// handlePing handles a ping message
func (s *TCPServer) handlePing(conn net.Conn, msg *TCPMessage) {
	resp, err := NewSuccessResponse(msg.ID, map[string]string{"status": "ok"})
	if err != nil {
		s.sendError(conn, msg.ID, err)
		return
	}
	s.sendResponse(conn, resp)
}

// handleExec handles an exec message
func (s *TCPServer) handleExec(ctx context.Context, conn net.Conn, msg *TCPMessage) {
	result, err := s.runtime.Exec(ctx, msg.Query, msg.Args...)
	if err != nil {
		s.sendError(conn, msg.ID, err)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	lastInsertID, _ := result.LastInsertId()

	execResult := ExecResult{
		RowsAffected: rowsAffected,
		LastInsertID: lastInsertID,
	}

	resp, err := NewSuccessResponse(msg.ID, execResult)
	if err != nil {
		s.sendError(conn, msg.ID, err)
		return
	}

	s.sendResponse(conn, resp)
}

// handleQuery handles a query message
func (s *TCPServer) handleQuery(ctx context.Context, conn net.Conn, msg *TCPMessage) {
	rows, err := s.runtime.Query(ctx, msg.Query, msg.Args...)
	if err != nil {
		s.sendError(conn, msg.ID, err)
		return
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		s.sendError(conn, msg.ID, err)
		return
	}

	var results [][]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			s.sendError(conn, msg.ID, err)
			return
		}

		// Convert []byte to string for JSON serialization
		for i, v := range values {
			if b, ok := v.([]byte); ok {
				values[i] = string(b)
			}
		}

		results = append(results, values)
	}

	if err := rows.Err(); err != nil {
		s.sendError(conn, msg.ID, err)
		return
	}

	queryResult := QueryResult{
		Columns: columns,
		Rows:    results,
	}

	resp, err := NewSuccessResponse(msg.ID, queryResult)
	if err != nil {
		s.sendError(conn, msg.ID, err)
		return
	}

	s.sendResponse(conn, resp)
}

// handleStats handles a stats message
func (s *TCPServer) handleStats(conn net.Conn, msg *TCPMessage) {
	stats := s.runtime.Stats()

	statsResult := StatsResult{
		MaxOpenConnections: stats.MaxOpenConnections,
		OpenConnections:    stats.OpenConnections,
		InUse:              stats.InUse,
		Idle:               stats.Idle,
		WaitCount:          stats.WaitCount,
		WaitDuration:       stats.WaitDuration.Nanoseconds(),
		MaxIdleClosed:      stats.MaxIdleClosed,
		MaxIdleTimeClosed:  stats.MaxIdleTimeClosed,
		MaxLifetimeClosed:  stats.MaxLifetimeClosed,
	}

	resp, err := NewSuccessResponse(msg.ID, statsResult)
	if err != nil {
		s.sendError(conn, msg.ID, err)
		return
	}

	s.sendResponse(conn, resp)
}

// handleMetrics handles a metrics message
func (s *TCPServer) handleMetrics(conn net.Conn, msg *TCPMessage) {
	metrics := s.runtime.Metrics()

	metricsResult := MetricsResult{
		TotalQueries:      metrics.TotalQueries,
		SuccessfulQueries: metrics.SuccessfulQueries,
		FailedQueries:     metrics.FailedQueries,
		SlowQueries:       metrics.SlowQueries,
		AverageQueryTime:  metrics.AverageQueryTime.Nanoseconds(),
	}

	resp, err := NewSuccessResponse(msg.ID, metricsResult)
	if err != nil {
		s.sendError(conn, msg.ID, err)
		return
	}

	s.sendResponse(conn, resp)
}

// sendResponse sends a response to the client
func (s *TCPServer) sendResponse(conn net.Conn, resp *TCPResponse) {
	data, err := EncodeTCPResponse(resp)
	if err != nil {
		log.Printf("Failed to encode response: %v", err)
		return
	}

	if _, err := conn.Write(data); err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}

// getClientIP extracts the real client IP address
func (s *TCPServer) getClientIP(conn net.Conn) string {
	addr := conn.RemoteAddr().String()
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}

// allowConnection checks if a connection from the IP should be allowed
func (s *TCPServer) allowConnection(clientIP string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check blacklist
	if s.blacklistMap[clientIP] {
		return false
	}

	// If whitelist exists and IP not in it, deny
	if len(s.whitelistMap) > 0 && !s.whitelistMap[clientIP] {
		return false
	}

	// Check connections per IP limit
	if s.config.MaxConnectionsPerIP > 0 {
		if s.ipConnections[clientIP] >= s.config.MaxConnectionsPerIP {
			return false
		}
		s.ipConnections[clientIP]++
	}

	return true
}

// checkRateLimit checks if request is within rate limit for IP
func (s *TCPServer) checkRateLimit(clientIP string) bool {
	if s.config.RateLimitPerIP <= 0 {
		return true
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	lastRequest, exists := s.ipRateLimits[clientIP]
	
	if !exists || lastRequest == nil {
		s.ipRateLimits[clientIP] = &now
		return true
	}

	// Simple rate limiting - one request per second per IP
	if now.Sub(*lastRequest) < time.Second {
		return false
	}

	s.ipRateLimits[clientIP] = &now
	return true
}

// checkIdempotency checks if request has been processed before
func (s *TCPServer) checkIdempotency(msg *TCPMessage) *TCPResponse {
	if s.idempotencyCache == nil || msg.IdempotencyKey == "" {
		return nil
	}

	ctx := context.Background()
	if cached, ok := s.idempotencyCache.Get(ctx, msg.IdempotencyKey); ok {
		if response, ok := cached.(*TCPResponse); ok {
			log.Printf("Returning cached response for idempotency key: %s", msg.IdempotencyKey)
			return response
		}
	}
	return nil
}

// storeIdempotency stores the response for future idempotency checks
func (s *TCPServer) storeIdempotency(msg *TCPMessage, response *TCPResponse) {
	if s.idempotencyCache == nil || msg.IdempotencyKey == "" || response == nil {
		return
	}

	ctx := context.Background()
	s.idempotencyCache.Set(ctx, msg.IdempotencyKey, response, 300*time.Second) // 5 minutes
}

// sendError sends an error response to the client
func (s *TCPServer) sendError(conn net.Conn, id string, err error) {
	resp := NewErrorResponse(id, err)
	s.sendResponse(conn, resp)
}

// GetAddress returns the server address
func (s *TCPServer) GetAddress() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return s.address
}

// GetClientCount returns the number of connected clients
func (s *TCPServer) GetClientCount() int {
	count := 0
	s.clients.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}

// ParseExecResult parses exec result from response data
func ParseExecResult(data json.RawMessage) (*ExecResult, error) {
	var result ExecResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ParseQueryResult parses query result from response data
func ParseQueryResult(data json.RawMessage) (*QueryResult, error) {
	var result QueryResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ParseStatsResult parses stats result from response data
func ParseStatsResult(data json.RawMessage) (*StatsResult, error) {
	var result StatsResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ParseMetricsResult parses metrics result from response data
func ParseMetricsResult(data json.RawMessage) (*MetricsResult, error) {
	var result MetricsResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
