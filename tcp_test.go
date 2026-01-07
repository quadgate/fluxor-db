package main

import (
	"fmt"
	"testing"
	"time"
)

func TestTCPProtocol_EncodeDecode(t *testing.T) {
	msg := &TCPMessage{
		Type:  MessageTypeExec,
		ID:    "123",
		Query: "INSERT INTO users (name) VALUES (?)",
		Args:  []interface{}{"John"},
	}

	// Encode
	data, err := EncodeTCPMessage(msg)
	if err != nil {
		t.Fatalf("Failed to encode: %v", err)
	}

	// Decode
	decoded, err := DecodeTCPMessage(data[:len(data)-1]) // Remove newline
	if err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}

	if decoded.Type != msg.Type {
		t.Errorf("Type mismatch: expected %s, got %s", msg.Type, decoded.Type)
	}

	if decoded.ID != msg.ID {
		t.Errorf("ID mismatch: expected %s, got %s", msg.ID, decoded.ID)
	}

	if decoded.Query != msg.Query {
		t.Errorf("Query mismatch: expected %s, got %s", msg.Query, decoded.Query)
	}
}

func TestTCPProtocol_Response(t *testing.T) {
	// Success response
	execResult := &ExecResult{
		RowsAffected: 1,
		LastInsertID: 123,
	}

	resp, err := NewSuccessResponse("123", execResult)
	if err != nil {
		t.Fatalf("Failed to create success response: %v", err)
	}

	if !resp.Success {
		t.Error("Expected success response")
	}

	if resp.ID != "123" {
		t.Errorf("ID mismatch: expected 123, got %s", resp.ID)
	}

	// Parse result
	parsed, err := ParseExecResult(resp.Data)
	if err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	if parsed.RowsAffected != 1 {
		t.Errorf("RowsAffected mismatch: expected 1, got %d", parsed.RowsAffected)
	}

	if parsed.LastInsertID != 123 {
		t.Errorf("LastInsertID mismatch: expected 123, got %d", parsed.LastInsertID)
	}

	// Error response
	errResp := NewErrorResponse("456", fmt.Errorf("test error"))
	if errResp.Success {
		t.Error("Expected error response")
	}

	if errResp.Error != "test error" {
		t.Errorf("Error message mismatch: expected 'test error', got '%s'", errResp.Error)
	}
}

func TestTCPServer_CreateAndStop(t *testing.T) {
	config := &RuntimeConfig{
		DatabaseType: DatabaseTypeMySQL,
		DSN:          "user:password@tcp(localhost:3306)/testdb",
		MaxOpenConns: 10,
		MaxIdleConns: 5,
	}

	runtime := NewDBRuntime(config)

	serverConfig := &TCPServerConfig{
		Address: "localhost:19090", // Use different port for testing
		Runtime: runtime,
	}

	server := NewTCPServer(serverConfig)
	if server == nil {
		t.Fatal("Failed to create TCP server")
	}

	// Note: Not starting server in test as it requires actual database
	// Just test creation

	if server.GetClientCount() != 0 {
		t.Errorf("Expected 0 clients, got %d", server.GetClientCount())
	}
}

func TestTCPClient_Create(t *testing.T) {
	clientConfig := &TCPClientConfig{
		Address: "localhost:19090",
		Timeout: 10 * time.Second,
	}

	client := NewTCPClient(clientConfig)
	if client == nil {
		t.Fatal("Failed to create TCP client")
	}

	if client.GetAddress() != "localhost:19090" {
		t.Errorf("Address mismatch: expected localhost:19090, got %s", client.GetAddress())
	}

	if client.IsConnected() {
		t.Error("Client should not be connected initially")
	}
}

func TestTCPClient_SetTimeout(t *testing.T) {
	client := NewTCPClient(&TCPClientConfig{
		Address: "localhost:19090",
	})

	newTimeout := 5 * time.Second
	client.SetTimeout(newTimeout)

	if client.timeout != newTimeout {
		t.Errorf("Timeout mismatch: expected %v, got %v", newTimeout, client.timeout)
	}
}

func TestTCPMessage_AllTypes(t *testing.T) {
	types := []MessageType{
		MessageTypeExec,
		MessageTypeQuery,
		MessageTypePing,
		MessageTypeStats,
		MessageTypeMetrics,
		MessageTypeClose,
	}

	for _, msgType := range types {
		t.Run(string(msgType), func(t *testing.T) {
			msg := &TCPMessage{
				Type: msgType,
				ID:   "test-id",
			}

			data, err := EncodeTCPMessage(msg)
			if err != nil {
				t.Fatalf("Failed to encode %s: %v", msgType, err)
			}

			decoded, err := DecodeTCPMessage(data[:len(data)-1])
			if err != nil {
				t.Fatalf("Failed to decode %s: %v", msgType, err)
			}

			if decoded.Type != msgType {
				t.Errorf("Type mismatch: expected %s, got %s", msgType, decoded.Type)
			}
		})
	}
}

func TestQueryResult_Encoding(t *testing.T) {
	queryResult := &QueryResult{
		Columns: []string{"id", "name", "email"},
		Rows: [][]interface{}{
			{1, "Alice", "alice@example.com"},
			{2, "Bob", "bob@example.com"},
		},
	}

	resp, err := NewSuccessResponse("test", queryResult)
	if err != nil {
		t.Fatalf("Failed to create response: %v", err)
	}

	parsed, err := ParseQueryResult(resp.Data)
	if err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	if len(parsed.Columns) != 3 {
		t.Errorf("Columns length mismatch: expected 3, got %d", len(parsed.Columns))
	}

	if len(parsed.Rows) != 2 {
		t.Errorf("Rows length mismatch: expected 2, got %d", len(parsed.Rows))
	}

	if parsed.Columns[0] != "id" {
		t.Errorf("Column 0 mismatch: expected 'id', got '%s'", parsed.Columns[0])
	}
}

func TestStatsResult_Encoding(t *testing.T) {
	statsResult := &StatsResult{
		MaxOpenConnections: 50,
		OpenConnections:    10,
		InUse:              5,
		Idle:               5,
		WaitCount:          100,
		WaitDuration:       1000000,
	}

	resp, err := NewSuccessResponse("test", statsResult)
	if err != nil {
		t.Fatalf("Failed to create response: %v", err)
	}

	parsed, err := ParseStatsResult(resp.Data)
	if err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	if parsed.MaxOpenConnections != 50 {
		t.Errorf("MaxOpenConnections mismatch: expected 50, got %d", parsed.MaxOpenConnections)
	}

	if parsed.InUse != 5 {
		t.Errorf("InUse mismatch: expected 5, got %d", parsed.InUse)
	}
}

func TestMetricsResult_Encoding(t *testing.T) {
	metricsResult := &MetricsResult{
		TotalQueries:      1000,
		SuccessfulQueries: 950,
		FailedQueries:     50,
		SlowQueries:       10,
		AverageQueryTime:  5000000,
	}

	resp, err := NewSuccessResponse("test", metricsResult)
	if err != nil {
		t.Fatalf("Failed to create response: %v", err)
	}

	parsed, err := ParseMetricsResult(resp.Data)
	if err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	if parsed.TotalQueries != 1000 {
		t.Errorf("TotalQueries mismatch: expected 1000, got %d", parsed.TotalQueries)
	}

	if parsed.SuccessfulQueries != 950 {
		t.Errorf("SuccessfulQueries mismatch: expected 950, got %d", parsed.SuccessfulQueries)
	}
}

func TestTCPClient_NextID(t *testing.T) {
	client := NewTCPClient(&TCPClientConfig{
		Address: "localhost:19090",
	})

	id1 := client.nextID()
	id2 := client.nextID()

	if id1 == id2 {
		t.Error("nextID should return unique IDs")
	}

	if id1 != "1" {
		t.Errorf("First ID should be '1', got '%s'", id1)
	}

	if id2 != "2" {
		t.Errorf("Second ID should be '2', got '%s'", id2)
	}
}

func TestTCPMessage_WithArgs(t *testing.T) {
	msg := &TCPMessage{
		Type:  MessageTypeExec,
		ID:    "123",
		Query: "INSERT INTO users (name, age, active) VALUES (?, ?, ?)",
		Args:  []interface{}{"John", 30, true},
	}

	data, err := EncodeTCPMessage(msg)
	if err != nil {
		t.Fatalf("Failed to encode: %v", err)
	}

	decoded, err := DecodeTCPMessage(data[:len(data)-1])
	if err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}

	if len(decoded.Args) != 3 {
		t.Errorf("Args length mismatch: expected 3, got %d", len(decoded.Args))
	}
}

func TestTCPServer_GetAddress(t *testing.T) {
	server := NewTCPServer(&TCPServerConfig{
		Address: "localhost:19090",
		Runtime: NewDBRuntime(&RuntimeConfig{
			DatabaseType: DatabaseTypeMySQL,
			DSN:          "test",
			MaxOpenConns: 10,
			MaxIdleConns: 5,
		}),
	})

	addr := server.GetAddress()
	if addr != "localhost:19090" {
		t.Errorf("Address mismatch: expected 'localhost:19090', got '%s'", addr)
	}
}
