package fskit

import (
	"io"
	"sync"
	"testing"
	"time"

	"tractor.dev/wanix/fs"
)

func TestFuncFileBasicRead(t *testing.T) {
	node := Entry("test.txt", 0644, []byte("initial data"))

	readFuncCalled := false
	ff := &FuncFile{
		Node: node,
		ReadFunc: func(n *Node) error {
			readFuncCalled = true
			// Modify the node data
			SetData(n, []byte("data from ReadFunc"))
			return nil
		},
	}

	// First read should call ReadFunc
	buf := make([]byte, 100)
	n, err := ff.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatalf("failed to read: %v", err)
	}

	if !readFuncCalled {
		t.Error("ReadFunc was not called")
	}

	if string(buf[:n]) != "data from ReadFunc" {
		t.Errorf("expected 'data from ReadFunc', got %q", string(buf[:n]))
	}

	// Second read should not call ReadFunc again
	readFuncCalled = false
	_, err = ff.Read(buf)
	if readFuncCalled {
		t.Error("ReadFunc should not be called on second read")
	}
}

func TestFuncFileReadWithoutReadFunc(t *testing.T) {
	node := Entry("test.txt", 0644, []byte("test data"))

	ff := &FuncFile{
		Node: node,
		// No ReadFunc
	}

	buf := make([]byte, 100)
	n, err := ff.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatalf("failed to read: %v", err)
	}

	if string(buf[:n]) != "test data" {
		t.Errorf("expected 'test data', got %q", string(buf[:n]))
	}
}

func TestFuncFileWrite(t *testing.T) {
	node := Entry("test.txt", 0644, []byte{})

	ff := &FuncFile{
		Node: node,
	}

	// Write data
	n, err := ff.Write([]byte("written data"))
	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	if n != 12 {
		t.Errorf("expected to write 12 bytes, got %d", n)
	}

	// Close to sync
	ff.Close()

	// Verify data was written to node
	if string(node.Data()) != "written data" {
		t.Errorf("expected 'written data', got %q", string(node.Data()))
	}
}

func TestFuncFileCloseFunc(t *testing.T) {
	node := Entry("test.txt", 0644, []byte("data"))

	closeFuncCalled := false
	var closedNode *Node

	ff := &FuncFile{
		Node: node,
		CloseFunc: func(n *Node) error {
			closeFuncCalled = true
			closedNode = n
			return nil
		},
	}

	// Read to initialize openFile
	buf := make([]byte, 10)
	ff.Read(buf)

	// Close should call CloseFunc
	err := ff.Close()
	if err != nil {
		t.Fatalf("failed to close: %v", err)
	}

	if !closeFuncCalled {
		t.Error("CloseFunc was not called")
	}

	if closedNode != node {
		t.Error("CloseFunc received wrong node")
	}
}

func TestFuncFileReadAt(t *testing.T) {
	node := Entry("test.txt", 0644, []byte("hello world"))

	ff := &FuncFile{
		Node: node,
	}

	// ReadAt with offset 0 should trigger Read
	buf := make([]byte, 5)
	n, err := ff.ReadAt(buf, 0)
	if err != nil && err != io.EOF {
		t.Fatalf("failed to read at: %v", err)
	}

	if string(buf[:n]) != "hello" {
		t.Errorf("expected 'hello', got %q", string(buf[:n]))
	}

	// ReadAt with non-zero offset on first read should return EOF
	ff2 := &FuncFile{
		Node: Entry("test2.txt", 0644, []byte("data")),
	}

	_, err = ff2.ReadAt(buf, 5)
	if err != io.EOF {
		t.Errorf("expected EOF, got %v", err)
	}
}

func TestFuncFileStat(t *testing.T) {
	node := Entry("test.txt", 0644, []byte("data"))

	ff := &FuncFile{
		Node: node,
	}

	info, err := ff.Stat()
	if err != nil {
		t.Fatalf("failed to stat: %v", err)
	}

	if info.Name() != "test.txt" {
		t.Errorf("expected name 'test.txt', got %q", info.Name())
	}
}

func TestFuncFileClosedOperations(t *testing.T) {
	node := Entry("test.txt", 0644, []byte("data"))

	ff := &FuncFile{
		Node: node,
	}

	// Close the file
	ff.Close()

	// Try to read - should fail
	buf := make([]byte, 10)
	_, err := ff.Read(buf)
	if err != fs.ErrClosed {
		t.Errorf("expected ErrClosed, got %v", err)
	}

	// Try to write - should fail
	_, err = ff.Write([]byte("test"))
	if err != fs.ErrClosed {
		t.Errorf("expected ErrClosed on write, got %v", err)
	}

	// Try to read at - should fail
	_, err = ff.ReadAt(buf, 0)
	if err != fs.ErrClosed {
		t.Errorf("expected ErrClosed on readat, got %v", err)
	}

	// Try to close again - should fail
	err = ff.Close()
	if err != fs.ErrClosed {
		t.Errorf("expected ErrClosed on second close, got %v", err)
	}
}

func TestFuncFileConcurrentReads(t *testing.T) {
	node := Entry("test.txt", 0644, []byte("test data for concurrent reads"))

	ff := &FuncFile{
		Node: node,
		ReadFunc: func(n *Node) error {
			// Simulate some work
			time.Sleep(10 * time.Millisecond)
			return nil
		},
	}

	var wg sync.WaitGroup
	errors := make(chan error, 10)

	// Launch 10 concurrent readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			buf := make([]byte, 100)
			_, err := ff.Read(buf)
			if err != nil && err != io.EOF {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent read error: %v", err)
	}
}

func TestFuncFileConcurrentWrites(t *testing.T) {
	node := Entry("test.txt", 0644, []byte{})

	ff := &FuncFile{
		Node: node,
	}

	var wg sync.WaitGroup
	errors := make(chan error, 10)

	// Launch 10 concurrent writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			_, err := ff.Write([]byte("data"))
			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent write error: %v", err)
	}
}

func TestFuncFileNoDeadlockWithNodeAccess(t *testing.T) {
	// This test verifies that FuncFile operations don't deadlock
	// when the ReadFunc or CloseFunc access node properties

	node := Entry("test.txt", 0644, []byte("initial"))

	ff := &FuncFile{
		Node: node,
		ReadFunc: func(n *Node) error {
			// Access node properties - would deadlock in old implementation
			_ = n.Name()
			_ = n.Size()
			_ = n.Mode()
			SetData(n, []byte("modified by ReadFunc"))
			return nil
		},
		CloseFunc: func(n *Node) error {
			// Access node properties during close
			_ = n.Name()
			SetModTime(n, time.Now())
			return nil
		},
	}

	// Read should not deadlock
	buf := make([]byte, 100)
	n, err := ff.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatalf("failed to read: %v", err)
	}

	if string(buf[:n]) != "modified by ReadFunc" {
		t.Errorf("expected 'modified by ReadFunc', got %q", string(buf[:n]))
	}

	// Close should not deadlock
	err = ff.Close()
	if err != nil {
		t.Fatalf("failed to close: %v", err)
	}
}

func TestFuncFileReadFuncError(t *testing.T) {
	node := Entry("test.txt", 0644, []byte("data"))

	ff := &FuncFile{
		Node: node,
		ReadFunc: func(n *Node) error {
			return io.ErrUnexpectedEOF
		},
	}

	buf := make([]byte, 10)
	_, err := ff.Read(buf)
	if err != io.ErrUnexpectedEOF {
		t.Errorf("expected ErrUnexpectedEOF, got %v", err)
	}
}

func TestFuncFileMultipleReads(t *testing.T) {
	node := Entry("test.txt", 0644, []byte("hello world"))

	ff := &FuncFile{
		Node: node,
	}

	// First read
	buf := make([]byte, 5)
	n, err := ff.Read(buf)
	if err != nil {
		t.Fatalf("first read failed: %v", err)
	}
	if string(buf[:n]) != "hello" {
		t.Errorf("first read: expected 'hello', got %q", string(buf[:n]))
	}

	// Second read should continue from where we left off
	n, err = ff.Read(buf)
	if err != nil {
		t.Fatalf("second read failed: %v", err)
	}
	if string(buf[:n]) != " worl" {
		t.Errorf("second read: expected ' worl', got %q", string(buf[:n]))
	}

	// Third read
	buf2 := make([]byte, 1)
	n, err = ff.Read(buf2)
	if err != nil && err != io.EOF {
		t.Fatalf("third read failed: %v", err)
	}
	if string(buf2[:n]) != "d" {
		t.Errorf("third read: expected 'd', got %q", string(buf2[:n]))
	}
}

func TestFuncFileConcurrentReadWrite(t *testing.T) {
	// Test concurrent reads and writes don't deadlock
	node := Entry("test.txt", 0644, []byte("initial"))

	ff := &FuncFile{
		Node: node,
	}

	var wg sync.WaitGroup

	// Reader
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			buf := make([]byte, 100)
			ff.Read(buf)
			time.Sleep(1 * time.Millisecond)
		}
	}()

	// Writer
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			ff.Write([]byte("data"))
			time.Sleep(1 * time.Millisecond)
		}
	}()

	wg.Wait()
}

func TestFuncFileCloseDuringRead(t *testing.T) {
	// Test that closing while read is in progress doesn't cause issues
	node := Entry("test.txt", 0644, []byte("data"))

	ff := &FuncFile{
		Node: node,
		ReadFunc: func(n *Node) error {
			// Simulate slow read function
			time.Sleep(50 * time.Millisecond)
			SetData(n, []byte("slow data"))
			return nil
		},
	}

	// Start reading in a goroutine
	go func() {
		buf := make([]byte, 100)
		ff.Read(buf)
	}()

	// Give it a moment to start
	time.Sleep(10 * time.Millisecond)

	// Try to close - should not panic or deadlock
	ff.Close()
}
