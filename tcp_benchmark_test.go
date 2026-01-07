package main

import (
"testing"
"time"
)

func BenchmarkTCPPing(b *testing.B) {
server := NewTCPServer(&TCPServerConfig{
Address: "localhost:9091",
Runtime: &DBRuntime{},
})
server.Start()
defer server.Stop()

client := NewTCPClient(&TCPClientConfig{
Address: "localhost:9091",
Timeout: 5 * time.Second,
})
client.Connect()
defer client.Disconnect()

b.ResetTimer()
for i := 0; i < b.N; i++ {
client.Ping()
}
}

func BenchmarkTCPMessageEncode(b *testing.B) {
msg := &TCPMessage{
Type:  MessageTypePing,
ID:    "test-123",
Query: "SELECT * FROM users WHERE id = ?",
Args:  []interface{}{1, "test", 3.14},
}

b.ResetTimer()
for i := 0; i < b.N; i++ {
EncodeTCPMessage(msg)
}
}

func BenchmarkTCPMessageDecode(b *testing.B) {
msg := &TCPMessage{
Type:  MessageTypeQuery,
ID:    "test-123",
Query: "SELECT * FROM users",
}
encoded, _ := EncodeTCPMessage(msg)

b.ResetTimer()
for i := 0; i < b.N; i++ {
DecodeTCPMessage(encoded)
}
}
