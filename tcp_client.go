package main

import (
	"bufio"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// TCPClient represents a TCP client for database runtime
type TCPClient struct {
	address    string
	conn       net.Conn
	messageID  uint64
	mu         sync.Mutex
	timeout    time.Duration
	connected  bool
	connMu     sync.RWMutex
}

// TCPClientConfig configures the TCP client
type TCPClientConfig struct {
	Address string
	Timeout time.Duration
}

// NewTCPClient creates a new TCP client
func NewTCPClient(config *TCPClientConfig) *TCPClient {
	timeout := 30 * time.Second
	if config.Timeout > 0 {
		timeout = config.Timeout
	}

	return &TCPClient{
		address: config.Address,
		timeout: timeout,
	}
}

// Connect connects to the TCP server
func (c *TCPClient) Connect() error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.connected {
		return fmt.Errorf("already connected")
	}

	conn, err := net.DialTimeout("tcp", c.address, c.timeout)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", c.address, err)
	}

	c.conn = conn
	c.connected = true
	return nil
}

// Disconnect disconnects from the TCP server
func (c *TCPClient) Disconnect() error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if !c.connected {
		return fmt.Errorf("not connected")
	}

	// Send close message
	msg := &TCPMessage{
		Type: MessageTypeClose,
		ID:   c.nextID(),
	}

	c.sendMessage(msg)

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}

	c.connected = false
	return nil
}

// IsConnected returns whether the client is connected
func (c *TCPClient) IsConnected() bool {
	c.connMu.RLock()
	defer c.connMu.RUnlock()
	return c.connected
}

// Ping sends a ping message to check server health
func (c *TCPClient) Ping() error {
	msg := &TCPMessage{
		Type: MessageTypePing,
		ID:   c.nextID(),
	}

	resp, err := c.sendAndReceive(msg)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("ping failed: %s", resp.Error)
	}

	return nil
}

// Exec executes a query without returning rows
func (c *TCPClient) Exec(query string, args ...interface{}) (*ExecResult, error) {
	msg := &TCPMessage{
		Type:  MessageTypeExec,
		ID:    c.nextID(),
		Query: query,
		Args:  args,
	}

	resp, err := c.sendAndReceive(msg)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("exec failed: %s", resp.Error)
	}

	return ParseExecResult(resp.Data)
}

// Query executes a query that returns rows
func (c *TCPClient) Query(query string, args ...interface{}) (*QueryResult, error) {
	msg := &TCPMessage{
		Type:  MessageTypeQuery,
		ID:    c.nextID(),
		Query: query,
		Args:  args,
	}

	resp, err := c.sendAndReceive(msg)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("query failed: %s", resp.Error)
	}

	return ParseQueryResult(resp.Data)
}

// Stats retrieves connection pool statistics
func (c *TCPClient) Stats() (*StatsResult, error) {
	msg := &TCPMessage{
		Type: MessageTypeStats,
		ID:   c.nextID(),
	}

	resp, err := c.sendAndReceive(msg)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("stats failed: %s", resp.Error)
	}

	return ParseStatsResult(resp.Data)
}

// Metrics retrieves performance metrics
func (c *TCPClient) Metrics() (*MetricsResult, error) {
	msg := &TCPMessage{
		Type: MessageTypeMetrics,
		ID:   c.nextID(),
	}

	resp, err := c.sendAndReceive(msg)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("metrics failed: %s", resp.Error)
	}

	return ParseMetricsResult(resp.Data)
}

// sendAndReceive sends a message and waits for response
func (c *TCPClient) sendAndReceive(msg *TCPMessage) (*TCPResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}

	// Set write deadline
	if err := c.conn.SetWriteDeadline(time.Now().Add(c.timeout)); err != nil {
		return nil, fmt.Errorf("failed to set write deadline: %w", err)
	}

	// Send message
	data, err := EncodeTCPMessage(msg)
	if err != nil {
		return nil, err
	}

	if _, err := c.conn.Write(data); err != nil {
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	// Set read deadline
	if err := c.conn.SetReadDeadline(time.Now().Add(c.timeout)); err != nil {
		return nil, fmt.Errorf("failed to set read deadline: %w", err)
	}

	// Read response
	scanner := bufio.NewScanner(c.conn)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer

	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}
		return nil, fmt.Errorf("connection closed")
	}

	resp, err := DecodeTCPResponse(scanner.Bytes())
	if err != nil {
		return nil, err
	}

	// Verify response ID matches request ID
	if resp.ID != msg.ID {
		return nil, fmt.Errorf("response ID mismatch: expected %s, got %s", msg.ID, resp.ID)
	}

	return resp, nil
}

// sendMessage sends a message without waiting for response
func (c *TCPClient) sendMessage(msg *TCPMessage) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}

	data, err := EncodeTCPMessage(msg)
	if err != nil {
		return err
	}

	if _, err := c.conn.Write(data); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

// nextID generates the next message ID
func (c *TCPClient) nextID() string {
	id := atomic.AddUint64(&c.messageID, 1)
	return fmt.Sprintf("%d", id)
}

// GetAddress returns the server address
func (c *TCPClient) GetAddress() string {
	return c.address
}

// SetTimeout sets the timeout for operations
func (c *TCPClient) SetTimeout(timeout time.Duration) {
	c.timeout = timeout
}
