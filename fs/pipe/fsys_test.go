package pipe

import (
	"fmt"
	"os"
	"testing"
	"time"

	"tractor.dev/wanix/fs"
)

// Shells open with O_APPEND for ">>" and O_CREATE|O_TRUNC for ">". A pipe
// end should open for writing either way, without seeking or reading.
func TestOpenFileFlags(t *testing.T) {
	cases := []struct {
		name string
		flag int
	}{
		{"append", os.O_WRONLY | os.O_APPEND},
		{"create-truncate", os.O_WRONLY | os.O_CREATE | os.O_TRUNC},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			done := make(chan error, 1)
			go func() { done <- writeThroughOpenFile(c.flag) }()
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("timed out: an open or write on the pipe blocked")
			}
		})
	}
}

func writeThroughOpenFile(flag int) error {
	fsys, _, other := NewFS(true)
	f, err := fs.OpenFile(fsys, DataFile, flag, 0644)
	if err != nil {
		return fmt.Errorf("OpenFile: %w", err)
	}
	tf, ok := f.(interface{ Truncate(int64) error })
	if !ok {
		return fmt.Errorf("pipe end has no Truncate")
	}
	if err := tf.Truncate(0); err != nil {
		return fmt.Errorf("Truncate: %w", err)
	}
	if err := fs.Chtimes(fsys, DataFile, time.Now(), time.Now()); err != nil {
		return fmt.Errorf("Chtimes: %w", err)
	}
	msg := "hello\n"
	if _, err := fs.Write(f, []byte(msg)); err != nil {
		return fmt.Errorf("Write: %w", err)
	}
	buf := make([]byte, 100)
	n, err := other.Read(buf)
	if err != nil {
		return fmt.Errorf("Read: %w", err)
	}
	if string(buf[:n]) != msg {
		return fmt.Errorf("read %q, want %q", buf[:n], msg)
	}
	return nil
}
