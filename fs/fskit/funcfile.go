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
	defer f.mu.Unlock()

	if f.closed {
		return fs.ErrClosed
	}

	f.closed = true

	if f.CloseFunc != nil && f.openFile != nil {
		return f.CloseFunc(f.openFile.Node)
	}

	return nil
}

func (f *FuncFile) Stat() (fs.FileInfo, error) {
	return f.Node, nil
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

	return f.openFile.ReadAt(b, off)
}

func (f *FuncFile) Read(b []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return 0, fs.ErrClosed
	}

	if !f.hasRead {
		f.hasRead = true

		if f.ReadFunc != nil {
			err := f.ReadFunc(f.Node)
			if err != nil {
				return 0, err
			}
		}

		f.openFile = f.Node.file()
	}

	return f.openFile.Read(b)
}

func (f *FuncFile) Write(b []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return 0, fs.ErrClosed
	}

	if f.openFile == nil {
		f.openFile = f.Node.file()
	}

	return f.openFile.Write(b)
}
