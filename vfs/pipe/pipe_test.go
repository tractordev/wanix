package pipe

import (
	"context"
	"io"
	"testing"

	"tractor.dev/wanix/fs"
)

func TestNew(t *testing.T) {
	t.Run("blocking pipes", func(t *testing.T) {
		p1, p2 := New(true)
		if p1 == nil || p2 == nil {
			t.Fatal("New returned nil ports")
		}

		// Verify the pipes are connected properly
		msg := []byte("hello")
		n, err := p1.Write(msg)
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}
		if n != len(msg) {
			t.Errorf("Write returned %d, want %d", n, len(msg))
		}

		buf := make([]byte, 100)
		n, err = p2.Read(buf)
		if err != nil {
			t.Fatalf("Read failed: %v", err)
		}
		if n != len(msg) {
			t.Errorf("Read returned %d, want %d", n, len(msg))
		}
		if string(buf[:n]) != string(msg) {
			t.Errorf("Read %q, want %q", buf[:n], msg)
		}
	})

	t.Run("non-blocking pipes", func(t *testing.T) {
		p1, p2 := New(false)
		if p1 == nil || p2 == nil {
			t.Fatal("New returned nil ports")
		}

		// Test bidirectional communication
		msg1 := []byte("ping")
		msg2 := []byte("pong")

		// p1 -> p2
		if _, err := p1.Write(msg1); err != nil {
			t.Fatalf("p1.Write failed: %v", err)
		}
		buf := make([]byte, 100)
		n, err := p2.Read(buf)
		if err != nil {
			t.Fatalf("p2.Read failed: %v", err)
		}
		if string(buf[:n]) != string(msg1) {
			t.Errorf("p2 read %q, want %q", buf[:n], msg1)
		}

		// p2 -> p1
		if _, err := p2.Write(msg2); err != nil {
			t.Fatalf("p2.Write failed: %v", err)
		}
		n, err = p1.Read(buf)
		if err != nil {
			t.Fatalf("p1.Read failed: %v", err)
		}
		if string(buf[:n]) != string(msg2) {
			t.Errorf("p1 read %q, want %q", buf[:n], msg2)
		}
	})
}

func TestPort(t *testing.T) {
	t.Run("close", func(t *testing.T) {
		p1, p2 := New(false)

		if err := p1.Close(); err != nil {
			t.Fatalf("Close failed: %v", err)
		}

		// Write to closed port should fail
		_, err := p1.Write([]byte("test"))
		if err != io.ErrClosedPipe {
			t.Errorf("Write after close got error %v, want %v", err, io.ErrClosedPipe)
		}

		// Read from closed port should return EOF after buffer is empty
		buf := make([]byte, 10)
		_, err = p1.Read(buf)
		if err != io.EOF {
			t.Errorf("Read after close got error %v, want %v", err, io.EOF)
		}

		// Other end should also be closed since closing one end closes the entire pipe
		_, err = p2.Write([]byte("test"))
		if err != io.ErrClosedPipe {
			t.Errorf("Write to other end after close got error %v, want %v", err, io.ErrClosedPipe)
		}
	})

	t.Run("concurrent access", func(t *testing.T) {
		p1, p2 := New(true)
		done := make(chan struct{})

		go func() {
			msg := []byte("test")
			for i := 0; i < 100; i++ {
				if _, err := p1.Write(msg); err != nil {
					t.Errorf("concurrent write failed: %v", err)
				}
			}
			done <- struct{}{}
		}()

		go func() {
			buf := make([]byte, 4)
			for i := 0; i < 100; i++ {
				if _, err := p2.Read(buf); err != nil {
					t.Errorf("concurrent read failed: %v", err)
				}
			}
			done <- struct{}{}
		}()

		<-done
		<-done
	})
}

func TestMultipleReaders(t *testing.T) {
	t.Run("multiple readers share data", func(t *testing.T) {
		p1, p2 := New(false)

		// Write some data
		data := []byte("hello world")
		_, err := p1.Write(data)
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}

		// Create multiple readers by creating multiple ports that read from the same buffer
		// This simulates what would happen if someone tried to create multiple readers
		// from the same pipe end

		// First reader gets all the data
		buf1 := make([]byte, 100)
		n1, err := p2.Read(buf1)
		if err != nil {
			t.Fatalf("First read failed: %v", err)
		}
		if string(buf1[:n1]) != string(data) {
			t.Errorf("First reader got %q, want %q", buf1[:n1], data)
		}

		// Second reader gets nothing (EOF) because the data was already consumed
		buf2 := make([]byte, 100)
		n2, err := p2.Read(buf2)
		if err != io.EOF {
			t.Errorf("Second read got error %v, want EOF", err)
		}
		if n2 != 0 {
			t.Errorf("Second read got %d bytes, want 0", n2)
		}
	})

	t.Run("multiple ports from same buffer", func(t *testing.T) {
		// This test shows what happens if someone tries to create multiple ports
		// that read from the same buffer (which shouldn't be possible with the current API)
		buffer := NewBuffer(false)

		port1 := &Port{reader: buffer, writer: buffer}
		port2 := &Port{reader: buffer, writer: buffer}

		// Write data
		data := []byte("test data")
		_, err := port1.Write(data)
		if err != nil {
			t.Fatalf("Write failed: %v", err)
		}

		// First reader gets the data
		buf1 := make([]byte, 100)
		n1, err := port1.Read(buf1)
		if err != nil {
			t.Fatalf("First read failed: %v", err)
		}
		if string(buf1[:n1]) != string(data) {
			t.Errorf("First reader got %q, want %q", buf1[:n1], data)
		}

		// Second reader gets nothing because data was consumed
		buf2 := make([]byte, 100)
		n2, err := port2.Read(buf2)
		if err != io.EOF {
			t.Errorf("Second read got error %v, want EOF", err)
		}
		if n2 != 0 {
			t.Errorf("Second read got %d bytes, want 0", n2)
		}
	})
}

func TestPortFileCloseNoOp(t *testing.T) {
	fsys, pf1, pf2 := NewFS(false)

	// Open writer and reader ends as files
	wf, err := fs.OpenContext(context.Background(), fsys, "data")
	if err != nil {
		t.Fatalf("open writer: %v", err)
	}
	rf, err := fs.OpenContext(context.Background(), fsys, "data1")
	if err != nil {
		wf.Close()
		t.Fatalf("open reader: %v", err)
	}

	// Write some data, then close writer file handle
	msg := []byte("persist after close")
	if _, err := wf.(interface{ Write([]byte) (int, error) }).Write(msg); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := wf.Close(); err != nil {
		t.Fatalf("writer close: %v", err)
	}

	// Ensure underlying port still open and data can be read from other side
	buf := make([]byte, len(msg))
	n, err := rf.Read(buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(buf[:n]) != string(msg) {
		t.Fatalf("got %q, want %q", buf[:n], msg)
	}

	// Now explicitly close the underlying pipe via PortFile and ensure further writes fail
	if err := pf1.Port.Close(); err != nil {
		t.Fatalf("port close: %v", err)
	}
	if _, err := pf2.Write([]byte("x")); err == nil {
		t.Fatalf("expected write error after pipe close")
	}

	_ = rf.Close()
}
