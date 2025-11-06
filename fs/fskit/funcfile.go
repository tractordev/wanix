package fskit

import (
	"io"
	"sync"

	"tractor.dev/wanix/fs"
)

type FuncFile struct {
	Node      *Node
	ReadFunc  func(n *Node) error
	CloseFunc func(n *Node) error

	hasRead  bool
	closed   bool
	openFile *nodeFile
	mu       sync.Mutex
}

func (f *FuncFile) Close() error {
	f.mu.Lock()

	if f.closed {
		f.mu.Unlock()
		return fs.ErrClosed
	}

	f.closed = true
	openFile := f.openFile
	f.mu.Unlock()

	// Close the underlying openFile to sync data back to node
	if openFile != nil {
		err := openFile.Close()
		if err != nil {
			return err
		}
	}

	// Call CloseFunc after closing openFile
	if f.CloseFunc != nil && openFile != nil {
		return f.CloseFunc(openFile.inode)
	}

	return nil
}

func (f *FuncFile) Stat() (fs.FileInfo, error) {
	return f.Node, nil
}

// no-op
func (f *FuncFile) Seek(offset int64, whence int) (int64, error) {
	return 0, nil
}

func (f *FuncFile) ReadAt(b []byte, off int64) (int, error) {
	f.mu.Lock()

	if f.closed {
		f.mu.Unlock()
		return 0, fs.ErrClosed
	}

	// if our first read has an offset, we need to return EOF
	// because it could actually be a second read on an open fid
	// tracked at a higher level, and we need to only trigger the
	// readfunc once for a fid.
	if !f.hasRead && off > 0 {
		f.mu.Unlock()
		return 0, io.EOF
	}

	if !f.hasRead && off == 0 {
		f.mu.Unlock()
		return f.Read(b)
	}

	f.mu.Unlock()
	return f.openFile.ReadAt(b, off)
}

func (f *FuncFile) Read(b []byte) (int, error) {
	f.mu.Lock()

	if f.closed {
		f.mu.Unlock()
		return 0, fs.ErrClosed
	}

	// Check if we need to initialize
	needsInit := !f.hasRead
	if needsInit {
		f.hasRead = true
		node := f.Node
		f.mu.Unlock()

		if f.ReadFunc != nil {
			err := f.ReadFunc(node)
			if err != nil {
				return 0, err
			}
		}

		// Call openFile without holding f.mu to avoid deadlock
		openFile := node.openFile()

		f.mu.Lock()
		// Check if file was closed while we were creating openFile
		if f.closed {
			f.mu.Unlock()
			return 0, fs.ErrClosed
		}
		f.openFile = openFile
		f.mu.Unlock()

		// Use the openFile we just created
		return openFile.Read(b)
	}

	// Not first read - wait for openFile to be initialized
	// (in case another goroutine is currently initializing)
	for f.openFile == nil && !f.closed {
		f.mu.Unlock()
		f.mu.Lock()
	}

	if f.closed {
		f.mu.Unlock()
		return 0, fs.ErrClosed
	}

	openFile := f.openFile
	f.mu.Unlock()

	// Read without holding f.mu to avoid deadlock
	return openFile.Read(b)
}

func (f *FuncFile) Write(b []byte) (int, error) {
	f.mu.Lock()

	if f.closed {
		f.mu.Unlock()
		return 0, fs.ErrClosed
	}

	// Check if we need to initialize
	needsInit := f.openFile == nil
	if needsInit {
		node := f.Node
		f.mu.Unlock()

		// Call openFile without holding f.mu to avoid deadlock
		openFile := node.openFile()

		f.mu.Lock()
		// Check if file was closed while we were creating openFile
		if f.closed {
			f.mu.Unlock()
			return 0, fs.ErrClosed
		}
		// Only set if still nil (another goroutine might have set it)
		if f.openFile == nil {
			f.openFile = openFile
			f.mu.Unlock()
			// Use the openFile we just created
			return openFile.Write(b)
		}
		// Another goroutine set it, use that one
		openFile = f.openFile
		f.mu.Unlock()
		return openFile.Write(b)
	}

	openFile := f.openFile
	f.mu.Unlock()

	// Write without holding f.mu to avoid deadlock
	return openFile.Write(b)
}
