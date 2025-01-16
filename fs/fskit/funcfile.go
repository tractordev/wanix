package fskit

import (
	"sync"

	"tractor.dev/wanix/fs"
)

type FuncFile struct {
	Node      *Node
	ReadFunc  func(n *Node) error
	CloseFunc func(n *Node) error

	firstRead bool
	closed    bool
	openFile  *nodeFile
	mu        sync.Mutex
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

func (f *FuncFile) Read(b []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return 0, fs.ErrClosed
	}

	if !f.firstRead {
		f.firstRead = true

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
