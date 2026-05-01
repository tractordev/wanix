//go:build js && wasm

package jsfs

import (
	"io"
	"path"
	"unicode/utf8"

	"syscall/js"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
)

type primitiveFile struct {
	name      string
	live      func() js.Value
	parent    js.Value
	key       string
	hasParent bool
	writeBuf  []byte
	readOff   int64
	readBuf   []byte // materialized from last read cycle
}

func newPrimitiveFile(name string, live func() js.Value, parent js.Value, key string, hasParent bool) *primitiveFile {
	return &primitiveFile{
		name:      name,
		live:      live,
		parent:    parent,
		key:       key,
		hasParent: hasParent,
	}
}

func (p *primitiveFile) materializeRead() {
	v := p.live()
	p.readBuf = jsToStringLine(v)
}

func (p *primitiveFile) Stat() (fs.FileInfo, error) {
	p.materializeRead()
	return fskit.Entry(path.Base(p.name), 0644, int64(len(p.readBuf))), nil
}

func (p *primitiveFile) Read(b []byte) (int, error) {
	if p.readBuf == nil {
		p.materializeRead()
	}
	if p.readOff >= int64(len(p.readBuf)) {
		return 0, io.EOF
	}
	n := copy(b, p.readBuf[p.readOff:])
	p.readOff += int64(n)
	return n, nil
}

func (p *primitiveFile) Write(b []byte) (int, error) {
	if !p.hasParent {
		return 0, &fs.PathError{Op: "write", Path: p.name, Err: fs.ErrPermission}
	}
	p.writeBuf = append(p.writeBuf, b...)
	return len(b), nil
}

func (p *primitiveFile) Close() error {
	if len(p.writeBuf) == 0 {
		return nil
	}
	if !p.hasParent {
		return &fs.PathError{Op: "write", Path: p.name, Err: fs.ErrPermission}
	}
	s := string(p.writeBuf)
	if !utf8.ValidString(s) {
		return &fs.PathError{Op: "write", Path: p.name, Err: fs.ErrInvalid}
	}
	trimmed := trimEndGo(s)
	existing := p.live()
	nv, err := coercePrimitive(existing, trimmed)
	if err != nil {
		return &fs.PathError{Op: "write", Path: p.name, Err: err}
	}
	if err := reflectSet(p.parent, p.key, nv); err != nil {
		return &fs.PathError{Op: "write", Path: p.name, Err: err}
	}
	p.writeBuf = nil
	p.readBuf = nil
	p.readOff = 0
	return nil
}

func (p *primitiveFile) Seek(offset int64, whence int) (int64, error) {
	if p.readBuf == nil {
		p.materializeRead()
	}
	var pos int64
	switch whence {
	case io.SeekStart:
		p.readBuf = nil
		pos = offset
	case io.SeekCurrent:
		if p.readBuf == nil {
			p.materializeRead()
		}
		pos = p.readOff + offset
	case io.SeekEnd:
		if p.readBuf == nil {
			p.materializeRead()
		}
		pos = int64(len(p.readBuf)) + offset
	default:
		return 0, &fs.PathError{Op: "seek", Path: p.name, Err: fs.ErrInvalid}
	}
	if pos < 0 {
		return 0, &fs.PathError{Op: "seek", Path: p.name, Err: fs.ErrInvalid}
	}
	p.readOff = pos
	return pos, nil
}

var _ io.ReadWriteSeeker = (*primitiveFile)(nil)
