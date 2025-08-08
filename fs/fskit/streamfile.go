package fskit

import (
	"io"

	"tractor.dev/wanix/fs"
)

type StreamFile struct {
	io.Reader
	io.Writer
	io.Closer
	node *Node
}

func NewStreamFile(r io.Reader, w io.Writer, c io.Closer, attrs ...any) *StreamFile {
	return &StreamFile{
		Reader: r,
		Writer: w,
		Closer: c,
		node:   RawNode(attrs...),
	}
}

func (f *StreamFile) Identity() fs.ID {
	return fs.Identity(f.Reader)
}

func (f *StreamFile) Stat() (fs.FileInfo, error) {
	return f.node, nil
}

func (f *StreamFile) Close() error {
	if f.Closer != nil {
		return f.Closer.Close()
	}
	return nil
}

func (f *StreamFile) Read(p []byte) (int, error) {
	return f.Reader.Read(p)
}

func (f *StreamFile) Write(p []byte) (int, error) {
	return f.Writer.Write(p)
}
