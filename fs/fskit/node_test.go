package fskit

import (
	"bytes"
	"io"
	"sync"
	"testing"
	"time"

	"tractor.dev/wanix/fs"
)

// metadataReader is a test reader that accesses node metadata
type metadataReader struct {
	node *Node
	data []byte
}

func (mr *metadataReader) Read(b []byte) (int, error) {
	// Access node metadata - this would deadlock if reader was called while holding f.mu
	_ = mr.node.Name()
	_ = mr.node.Size()
	nr := copy(b, mr.data)
	return nr, io.EOF
}

func TestNodeCreation(t *testing.T) {
	n := Entry("test.txt", 0644, []byte("hello world"))

	if n.Name() != "test.txt" {
		t.Errorf("expected name 'test.txt', got %q", n.Name())
	}

	if n.Mode() != 0644 {
		t.Errorf("expected mode 0644, got %o", n.Mode())
	}

	if n.Size() != 11 {
		t.Errorf("expected size 11, got %d", n.Size())
	}
}

func TestNodeDataCopy(t *testing.T) {
	original := []byte("hello")
	n := Entry("test.txt", 0644, original)

	// Get data twice
	data1 := n.Data()
	data2 := n.Data()

	// Verify they're equal
	if !bytes.Equal(data1, data2) {
		t.Error("Data() should return equal data")
	}

	// Modify first copy
	data1[0] = 'X'

	// Verify second copy is unchanged
	data3 := n.Data()
	if data3[0] != 'h' {
		t.Error("Data() should return independent copies")
	}

	// Verify original is unchanged
	if original[0] != 'h' {
		t.Error("Node should not modify original slice")
	}
}

func TestNodeFileReadWrite(t *testing.T) {
	n := Entry("test.txt", 0644, []byte("hello"))

	// Open file
	f, err := n.Open(".")
	if err != nil {
		t.Fatalf("failed to open: %v", err)
	}
	defer f.Close()

	// Read data
	buf := make([]byte, 5)
	nr, err := f.Read(buf)
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	if nr != 5 {
		t.Errorf("expected to read 5 bytes, got %d", nr)
	}
	if string(buf) != "hello" {
		t.Errorf("expected 'hello', got %q", string(buf))
	}
}

func TestNodeFileWrite(t *testing.T) {
	n := Entry("test.txt", 0644, []byte{})

	f, err := n.Open(".")
	if err != nil {
		t.Fatalf("failed to open: %v", err)
	}

	// Write data
	wf, ok := f.(interface{ Write([]byte) (int, error) })
	if !ok {
		t.Fatal("file does not implement Write")
	}

	nw, err := wf.Write([]byte("hello world"))
	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	if nw != 11 {
		t.Errorf("expected to write 11 bytes, got %d", nw)
	}

	// Close to sync
	if err := f.Close(); err != nil {
		t.Fatalf("failed to close: %v", err)
	}

	// Verify data was written back
	data := n.Data()
	if string(data) != "hello world" {
		t.Errorf("expected 'hello world', got %q", string(data))
	}
}

func TestNodeFileSeek(t *testing.T) {
	n := Entry("test.txt", 0644, []byte("hello world"))

	f, err := n.Open(".")
	if err != nil {
		t.Fatalf("failed to open: %v", err)
	}
	defer f.Close()

	sf, ok := f.(interface {
		Seek(int64, int) (int64, error)
	})
	if !ok {
		t.Fatal("file does not implement Seek")
	}

	// Seek to position 6
	pos, err := sf.Seek(6, 0)
	if err != nil {
		t.Fatalf("failed to seek: %v", err)
	}
	if pos != 6 {
		t.Errorf("expected position 6, got %d", pos)
	}

	// Read from new position
	buf := make([]byte, 5)
	nr, err := f.Read(buf)
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	if string(buf[:nr]) != "world" {
		t.Errorf("expected 'world', got %q", string(buf[:nr]))
	}
}

func TestNodeConcurrentReads(t *testing.T) {
	data := bytes.Repeat([]byte("test"), 1000)
	n := Entry("test.txt", 0644, data)

	var wg sync.WaitGroup
	errors := make(chan error, 10)

	// Launch 10 concurrent readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			f, err := n.Open(".")
			if err != nil {
				errors <- err
				return
			}
			defer f.Close()

			buf, err := io.ReadAll(f)
			if err != nil {
				errors <- err
				return
			}

			if !bytes.Equal(buf, data) {
				errors <- io.ErrUnexpectedEOF
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent read error: %v", err)
	}
}

func TestNodeConcurrentWrites(t *testing.T) {
	n := Entry("test.txt", 0644, []byte{})

	var wg sync.WaitGroup
	errors := make(chan error, 10)

	// Launch 10 concurrent writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			f, err := n.Open(".")
			if err != nil {
				errors <- err
				return
			}

			wf, ok := f.(interface{ Write([]byte) (int, error) })
			if !ok {
				errors <- io.ErrShortWrite
				f.Close()
				return
			}

			// Each writer writes a unique pattern
			data := bytes.Repeat([]byte{byte('A' + id)}, 100)
			_, err = wf.Write(data)
			if err != nil {
				errors <- err
			}

			f.Close()
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent write error: %v", err)
	}

	// Verify final data exists and has reasonable length
	finalData := n.Data()
	if len(finalData) == 0 {
		t.Error("expected data to be written")
	}
}

func TestNodeMultipleHandlesIndependent(t *testing.T) {
	n := Entry("test.txt", 0644, []byte("hello"))

	// Open two file handles
	f1, err := n.Open(".")
	if err != nil {
		t.Fatalf("failed to open f1: %v", err)
	}
	defer f1.Close()

	f2, err := n.Open(".")
	if err != nil {
		t.Fatalf("failed to open f2: %v", err)
	}
	defer f2.Close()

	// Read from f1
	buf1 := make([]byte, 2)
	n1, _ := f1.Read(buf1)
	if string(buf1[:n1]) != "he" {
		t.Errorf("f1 expected 'he', got %q", string(buf1[:n1]))
	}

	// Read from f2 - should start from beginning
	buf2 := make([]byte, 2)
	n2, _ := f2.Read(buf2)
	if string(buf2[:n2]) != "he" {
		t.Errorf("f2 expected 'he', got %q", string(buf2[:n2]))
	}

	// Write to f1
	w1, ok := f1.(interface{ Write([]byte) (int, error) })
	if ok {
		w1.Write([]byte("XXX"))
	}

	// Read from f2 - should still see original data
	buf3 := make([]byte, 3)
	n3, _ := f2.Read(buf3)
	if string(buf3[:n3]) != "llo" {
		t.Errorf("f2 expected 'llo', got %q", string(buf3[:n3]))
	}
}

func TestNodeWriteAtPosition(t *testing.T) {
	n := Entry("test.txt", 0644, []byte("hello world"))

	f, err := n.Open(".")
	if err != nil {
		t.Fatalf("failed to open: %v", err)
	}
	defer f.Close()

	wf, ok := f.(interface {
		Seek(int64, int) (int64, error)
		Write([]byte) (int, error)
	})
	if !ok {
		t.Fatal("file does not implement Seek/Write")
	}

	// Seek to position 6
	wf.Seek(6, 0)

	// Overwrite "world" with "tests"
	wf.Write([]byte("tests"))
	f.Close()

	// Verify
	data := n.Data()
	if string(data) != "hello tests" {
		t.Errorf("expected 'hello tests', got %q", string(data))
	}
}

func TestNodeDataIsolation(t *testing.T) {
	// Test that modifications to one file don't affect other open files
	n := Entry("test.txt", 0644, []byte("original"))

	f1, _ := n.Open(".")
	f2, _ := n.Open(".")

	// Write with f1
	w1 := f1.(interface{ Write([]byte) (int, error) })
	w1.Write([]byte("modified"))

	// Read with f2 - should still see "original"
	buf := make([]byte, 100)
	n2, _ := f2.Read(buf)
	if string(buf[:n2]) != "original" {
		t.Errorf("expected 'original', got %q", string(buf[:n2]))
	}

	f1.Close()
	f2.Close()

	// After f1 closes, node should have "modified"
	if string(n.Data()) != "modified" {
		t.Errorf("expected 'modified', got %q", string(n.Data()))
	}
}

func TestNodeConcurrentGetters(t *testing.T) {
	n := Entry("test.txt", 0644, []byte("hello"))

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Call getters concurrently
			_ = n.Name()
			_ = n.Mode()
			_ = n.Size()
			_ = n.ModTime()
			_ = n.IsDir()
			_ = n.Data()
		}()
	}

	wg.Wait()
}

func TestNodeConcurrentSetters(t *testing.T) {
	n := Entry("test.txt", 0644, []byte("hello"))

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Call setters concurrently
			SetMode(n, fs.FileMode(0600+id))
			SetSize(n, int64(id))
			SetModTime(n, time.Now())
		}(i)
	}

	wg.Wait()

	// Just verify we didn't crash
	_ = n.Mode()
	_ = n.Size()
}

func TestNodeReadAt(t *testing.T) {
	n := Entry("test.txt", 0644, []byte("hello world"))

	f, err := n.Open(".")
	if err != nil {
		t.Fatalf("failed to open: %v", err)
	}
	defer f.Close()

	raf, ok := f.(interface {
		ReadAt([]byte, int64) (int, error)
	})
	if !ok {
		t.Fatal("file does not implement ReadAt")
	}

	// Read at offset 6
	buf := make([]byte, 5)
	nr, err := raf.ReadAt(buf, 6)
	if err != nil && err != io.EOF {
		t.Fatalf("failed to read at: %v", err)
	}
	if string(buf[:nr]) != "world" {
		t.Errorf("expected 'world', got %q", string(buf[:nr]))
	}
}

func TestNodeWriteAt(t *testing.T) {
	n := Entry("test.txt", 0644, []byte("hello world"))

	f, err := n.Open(".")
	if err != nil {
		t.Fatalf("failed to open: %v", err)
	}

	waf, ok := f.(interface {
		WriteAt([]byte, int64) (int, error)
	})
	if !ok {
		t.Fatal("file does not implement WriteAt")
	}

	// Write at offset 6
	_, err = waf.WriteAt([]byte("tests"), 6)
	if err != nil {
		t.Fatalf("failed to write at: %v", err)
	}

	f.Close()

	// Verify
	data := n.Data()
	if string(data) != "hello tests" {
		t.Errorf("expected 'hello tests', got %q", string(data))
	}
}

func TestNodeClosedFileOperations(t *testing.T) {
	n := Entry("test.txt", 0644, []byte("hello"))

	f, err := n.Open(".")
	if err != nil {
		t.Fatalf("failed to open: %v", err)
	}

	// Close the file
	f.Close()

	// Try to read - should fail
	buf := make([]byte, 5)
	_, err = f.Read(buf)
	if err != fs.ErrClosed {
		t.Errorf("expected ErrClosed, got %v", err)
	}

	// Try to write - should fail
	if wf, ok := f.(interface{ Write([]byte) (int, error) }); ok {
		_, err = wf.Write([]byte("test"))
		if err != fs.ErrClosed {
			t.Errorf("expected ErrClosed on write, got %v", err)
		}
	}
}

func TestNodeRaceConditionInodeFieldAccess(t *testing.T) {
	// This test verifies that accessing inode fields through accessor methods
	// is thread-safe. Run with: go test -race
	n := Entry("test.txt", 0644, []byte("hello world"))

	var wg sync.WaitGroup

	// Goroutine 1: Constantly renames the node
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			SetName(n, "file"+string(rune('0'+i%10))+".txt")
		}
	}()

	// Goroutine 2: Constantly opens and closes files (accessing inode.path via accessor)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			f, err := n.Open(".")
			if err != nil {
				continue
			}
			buf := make([]byte, 5)
			f.Read(buf) // This accesses inode.Path() and inode.Log() through accessors
			f.Close()   // This also accesses inode.Path() and inode.Log() through accessors
		}
	}()

	wg.Wait()
}

func TestNodeNoDeadlockWithCustomReaderWriter(t *testing.T) {
	// This test verifies that custom readers/writers can be called
	// without causing deadlocks, even if they try to access the node

	n := Entry("test.txt", 0644, []byte{})
	reader := &metadataReader{node: n, data: []byte("custom reader data")}

	// Set the custom reader
	n.mu.Lock()
	n.reader = reader
	n.mu.Unlock()

	// Open and read - this would deadlock in the old implementation
	f, err := n.Open(".")
	if err != nil {
		t.Fatalf("failed to open: %v", err)
	}

	buf := make([]byte, 100)
	nr, err := f.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatalf("failed to read: %v", err)
	}

	if string(buf[:nr]) != "custom reader data" {
		t.Errorf("expected 'custom reader data', got %q", string(buf[:nr]))
	}

	f.Close()
}

func TestNodeModTimeTracking(t *testing.T) {
	// Test that modTime is tracked in the file and applied on close
	n := Entry("test.txt", 0644, []byte("hello"))

	// Record original modTime
	originalModTime := n.ModTime()

	// Wait a bit to ensure time difference
	time.Sleep(10 * time.Millisecond)

	// Open and write
	f, err := n.Open(".")
	if err != nil {
		t.Fatalf("failed to open: %v", err)
	}

	// Seek to end
	sf := f.(interface {
		Seek(int64, int) (int64, error)
	})
	sf.Seek(0, 2) // SEEK_END

	wf := f.(interface{ Write([]byte) (int, error) })
	wf.Write([]byte(" world"))

	// ModTime should NOT change until close
	if n.ModTime() != originalModTime {
		t.Error("modTime should not change until file is closed")
	}

	// Close the file
	f.Close()

	// Now modTime should be updated
	newModTime := n.ModTime()
	if !newModTime.After(originalModTime) {
		t.Errorf("modTime should be updated after close: original=%v, new=%v", originalModTime, newModTime)
	}

	// Verify data was written
	if string(n.Data()) != "hello world" {
		t.Errorf("expected 'hello world', got %q", string(n.Data()))
	}
}
