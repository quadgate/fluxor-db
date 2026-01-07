package main

import (
	"encoding/json"
	"fmt"
)

// MessageType represents the type of TCP message
type MessageType string

const (
	// MessageTypeExec executes a query without returning rows
	MessageTypeExec MessageType = "EXEC"
	// MessageTypeQuery executes a query that returns rows
	MessageTypeQuery MessageType = "QUERY"
	// MessageTypePing checks server health
	MessageTypePing MessageType = "PING"
	// MessageTypeStats returns connection pool statistics
	MessageTypeStats MessageType = "STATS"
	// MessageTypeMetrics returns performance metrics
	MessageTypeMetrics MessageType = "METRICS"
	// MessageTypeClose closes the connection
	MessageTypeClose MessageType = "CLOSE"
)

// TCPMessage represents a message sent over TCP
type TCPMessage struct {
	Type    MessageType     `json:"type"`
	ID      string          `json:"id"`
	Query   string          `json:"query,omitempty"`
	Args    []interface{}   `json:"args,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// TCPResponse represents a response sent over TCP
type TCPResponse struct {
	ID      string          `json:"id"`
	Success bool            `json:"success"`
	Error   string          `json:"error,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// ExecResult represents the result of an EXEC operation
type ExecResult struct {
	RowsAffected int64 `json:"rows_affected"`
	LastInsertID int64 `json:"last_insert_id"`
}

// QueryResult represents the result of a QUERY operation
type QueryResult struct {
	Columns []string        `json:"columns"`
	Rows    [][]interface{} `json:"rows"`
}

// StatsResult represents connection pool statistics
type StatsResult struct {
	MaxOpenConnections int `json:"max_open_connections"`
	OpenConnections    int `json:"open_connections"`
	InUse              int `json:"in_use"`
	Idle               int `json:"idle"`
	WaitCount          int64 `json:"wait_count"`
	WaitDuration       int64 `json:"wait_duration_ns"`
	MaxIdleClosed      int64 `json:"max_idle_closed"`
	MaxIdleTimeClosed  int64 `json:"max_idle_time_closed"`
	MaxLifetimeClosed  int64 `json:"max_lifetime_closed"`
}

// MetricsResult represents performance metrics
type MetricsResult struct {
	TotalQueries      int64 `json:"total_queries"`
	SuccessfulQueries int64 `json:"successful_queries"`
	FailedQueries     int64 `json:"failed_queries"`
	SlowQueries       int64 `json:"slow_queries"`
	AverageQueryTime  int64 `json:"average_query_time_ns"`
}

// EncodeTCPMessage encodes a TCP message to JSON bytes
func EncodeTCPMessage(msg *TCPMessage) ([]byte, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to encode message: %w", err)
	}
	// Add newline delimiter
	return append(data, '\n'), nil
}

// DecodeTCPMessage decodes JSON bytes to a TCP message
func DecodeTCPMessage(data []byte) (*TCPMessage, error) {
	var msg TCPMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("failed to decode message: %w", err)
	}
	return &msg, nil
}

// EncodeTCPResponse encodes a TCP response to JSON bytes
func EncodeTCPResponse(resp *TCPResponse) ([]byte, error) {
	data, err := json.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to encode response: %w", err)
	}
	// Add newline delimiter
	return append(data, '\n'), nil
}

// DecodeTCPResponse decodes JSON bytes to a TCP response
func DecodeTCPResponse(data []byte) (*TCPResponse, error) {
	var resp TCPResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &resp, nil
}

// NewSuccessResponse creates a successful response
func NewSuccessResponse(id string, data interface{}) (*TCPResponse, error) {
	payload, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return &TCPResponse{
		ID:      id,
		Success: true,
		Data:    payload,
	}, nil
}

// NewErrorResponse creates an error response
func NewErrorResponse(id string, err error) *TCPResponse {
	return &TCPResponse{
		ID:      id,
		Success: false,
		Error:   err.Error(),
	}
}
