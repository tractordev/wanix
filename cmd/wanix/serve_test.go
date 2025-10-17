//go:build !js && !wasm

package main

import (
	"bytes"
	"encoding/binary"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type mockConn struct {
	mu       sync.Mutex
	messages [][]byte
}

func (c *mockConn) WriteMessage(messageType int, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	msg := make([]byte, len(data))
	copy(msg, data)
	c.messages = append(c.messages, msg)
	return nil
}

func (c *mockConn) ReadMessage() (int, []byte, error) {
	return 0, nil, nil
}

func (c *mockConn) Close() error {
	return nil
}

func (c *mockConn) LocalAddr() net.Addr {
	return &net.UnixAddr{}
}

func (c *mockConn) RemoteAddr() net.Addr {
	return &net.UnixAddr{}
}

func (c *mockConn) SetDeadline(t time.Time) error {
	return nil
}

func (c *mockConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (c *mockConn) SetWriteDeadline(t time.Time) error {
	return nil
}

func TestQemuAdapterWrite_MultipleMessages(t *testing.T) {
	conn := &mockConn{}
	adapter := &qemuAdapter{Conn: conn}

	// Create two messages and concatenate them.
	msg1 := bytes.Repeat([]byte("a"), 5)
	prefix1 := make([]byte, 4)
	binary.BigEndian.PutUint32(prefix1, uint32(len(msg1)))
	data1 := append(prefix1, msg1...)

	msg2 := bytes.Repeat([]byte("b"), 10)
	prefix2 := make([]byte, 4)
	binary.BigEndian.PutUint32(prefix2, uint32(len(msg2)))
	data2 := append(prefix2, msg2...)

	allData := append(data1, data2...)

	// Write the concatenated messages to the adapter.
	n, err := adapter.Write(allData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Check that the number of bytes written is correct.
	if n != len(allData) {
		t.Errorf("expected %d bytes written, but got %d", len(allData), n)
	}

	// Check that two messages were sent.
	if len(conn.messages) != 2 {
		t.Fatalf("expected 2 messages to be sent, but got %d", len(conn.messages))
	}

	// Check that the buffer is empty.
	if len(adapter.writeBuffer) != 0 {
		t.Errorf("expected buffer to be empty, but it has %d bytes", len(adapter.writeBuffer))
	}

	// Check that the messages are correct.
	if !bytes.Equal(conn.messages[0], msg1) {
		t.Errorf("expected message 1 to be %q, but got %q", msg1, conn.messages[0])
	}
	if !bytes.Equal(conn.messages[1], msg2) {
		t.Errorf("expected message 2 to be %q, but got %q", msg2, conn.messages[1])
	}
}