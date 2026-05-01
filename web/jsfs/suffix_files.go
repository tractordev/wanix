//go:build js && wasm

package jsfs

import (
	"io"
	"path"
	"strings"
	"syscall"
	"syscall/js"
	"unicode/utf8"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
)

// refFile assigns a value by global path (see spec :ref).
type refFile struct {
	name      string
	parent    js.Value
	key       string
	hasParent bool
	buf       []byte
}

func newRefFile(name string, loc resolved) *refFile {
	return &refFile{
		name:      name,
		parent:    loc.parent,
		key:       loc.key,
		hasParent: loc.hasParent,
	}
}

func (r *refFile) Stat() (fs.FileInfo, error) {
	return fskit.Entry(path.Base(r.name), 0200, 0), nil
}

func (r *refFile) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: r.name, Err: syscall.EINVAL}
}

func (r *refFile) Write(b []byte) (int, error) {
	if !r.hasParent {
		return 0, &fs.PathError{Op: "write", Path: r.name, Err: fs.ErrPermission}
	}
	r.buf = append(r.buf, b...)
	return len(b), nil
}

func (r *refFile) Close() error {
	if len(r.buf) == 0 {
		return nil
	}
	if !r.hasParent {
		return &fs.PathError{Op: "write", Path: r.name, Err: fs.ErrPermission}
	}
	body := string(r.buf)
	if !utf8.ValidString(body) {
		return &fs.PathError{Op: "write", Path: r.name, Err: fs.ErrInvalid}
	}
	body = strings.TrimSpace(trimEndGo(body))
	if body == "" {
		if err := reflectSet(r.parent, r.key, js.Null()); err != nil {
			return &fs.PathError{Op: "write", Path: r.name, Err: err}
		}
		r.buf = nil
		return nil
	}
	if !strings.HasPrefix(body, "@") {
		return &fs.PathError{Op: "write", Path: r.name, Err: syscall.EINVAL}
	}
	rv, err := resolveGlobalPath(body)
	if err != nil {
		return &fs.PathError{Op: "write", Path: r.name, Err: err}
	}
	if err := reflectSet(r.parent, r.key, rv); err != nil {
		return &fs.PathError{Op: "write", Path: r.name, Err: err}
	}
	r.buf = nil
	return nil
}

func (r *refFile) Seek(int64, int) (int64, error) {
	return 0, &fs.PathError{Op: "seek", Path: r.name, Err: fs.ErrInvalid}
}

type jsonValueFile struct {
	name      string
	live      func() js.Value
	parent    js.Value
	key       string
	hasParent bool
	writeBuf  []byte
	readOff   int64
	readBuf   []byte
}

func newJSONValueFile(name string, loc resolved, live func() js.Value) *jsonValueFile {
	return &jsonValueFile{
		name:      name,
		live:      live,
		parent:    loc.parent,
		key:       loc.key,
		hasParent: loc.hasParent,
	}
}

func (j *jsonValueFile) materialize() error {
	v := j.live()
	b, err := jsonStringifyLine(v)
	if err != nil {
		return err
	}
	j.readBuf = b
	return nil
}

func (j *jsonValueFile) Stat() (fs.FileInfo, error) {
	if err := j.materialize(); err != nil {
		return nil, &fs.PathError{Op: "stat", Path: j.name, Err: err}
	}
	return fskit.Entry(path.Base(j.name), 0644, int64(len(j.readBuf))), nil
}

func (j *jsonValueFile) Read(b []byte) (int, error) {
	if j.readBuf == nil {
		if err := j.materialize(); err != nil {
			return 0, &fs.PathError{Op: "read", Path: j.name, Err: err}
		}
	}
	if j.readOff >= int64(len(j.readBuf)) {
		return 0, io.EOF
	}
	n := copy(b, j.readBuf[j.readOff:])
	j.readOff += int64(n)
	return n, nil
}

func (j *jsonValueFile) Write(b []byte) (int, error) {
	if !j.hasParent {
		return 0, &fs.PathError{Op: "write", Path: j.name, Err: fs.ErrPermission}
	}
	j.writeBuf = append(j.writeBuf, b...)
	return len(b), nil
}

func (j *jsonValueFile) Close() error {
	if len(j.writeBuf) == 0 {
		return nil
	}
	if !j.hasParent {
		return &fs.PathError{Op: "write", Path: j.name, Err: fs.ErrPermission}
	}
	s := string(j.writeBuf)
	if !utf8.ValidString(s) {
		return &fs.PathError{Op: "write", Path: j.name, Err: fs.ErrInvalid}
	}
	s = trimEndGo(s)
	v, err := jsonParseValue(s)
	if err != nil {
		return &fs.PathError{Op: "write", Path: j.name, Err: syscall.EINVAL}
	}
	if err := reflectSet(j.parent, j.key, v); err != nil {
		return &fs.PathError{Op: "write", Path: j.name, Err: err}
	}
	j.writeBuf = nil
	j.readBuf = nil
	j.readOff = 0
	return nil
}

func (j *jsonValueFile) Seek(offset int64, whence int) (int64, error) {
	if j.readBuf == nil {
		if err := j.materialize(); err != nil {
			return 0, &fs.PathError{Op: "seek", Path: j.name, Err: err}
		}
	}
	var base int64
	switch whence {
	case io.SeekStart:
		base = 0
	case io.SeekCurrent:
		base = j.readOff
	case io.SeekEnd:
		base = int64(len(j.readBuf))
	default:
		return 0, &fs.PathError{Op: "seek", Path: j.name, Err: fs.ErrInvalid}
	}
	j.readOff = base + offset
	if j.readOff < 0 {
		return 0, &fs.PathError{Op: "seek", Path: j.name, Err: fs.ErrInvalid}
	}
	return j.readOff, nil
}

func jsonParseValue(s string) (v js.Value, err error) {
	defer func() {
		if recover() != nil {
			err = fs.ErrInvalid
		}
	}()
	if strings.TrimSpace(s) == "" {
		return js.Undefined(), fs.ErrInvalid
	}
	v = js.Global().Get("JSON").Call("parse", js.ValueOf(s))
	return v, nil
}

type typeFile struct {
	name    string
	val     js.Value
	readOff int64
}

func newTypeFile(name string, v js.Value) *typeFile {
	return &typeFile{name: name, val: v}
}

func (t *typeFile) typeLine() []byte {
	raw := js.Global().Get("Object").Get("prototype").Get("toString").Call("call", t.val).String()
	const p = "[object "
	if strings.HasPrefix(raw, p) && strings.HasSuffix(raw, "]") {
		raw = raw[len(p) : len(raw)-1]
	}
	return append([]byte(raw), '\n')
}

func (t *typeFile) Stat() (fs.FileInfo, error) {
	line := t.typeLine()
	return fskit.Entry(path.Base(t.name), 0444, int64(len(line))), nil
}

func (t *typeFile) Read(b []byte) (int, error) {
	line := t.typeLine()
	if t.readOff >= int64(len(line)) {
		return 0, io.EOF
	}
	n := copy(b, line[t.readOff:])
	t.readOff += int64(n)
	return n, nil
}

func (t *typeFile) Write([]byte) (int, error) {
	return 0, &fs.PathError{Op: "write", Path: t.name, Err: syscall.EINVAL}
}

func (t *typeFile) Close() error { return nil }

func (t *typeFile) Seek(offset int64, whence int) (int64, error) {
	line := t.typeLine()
	var base int64
	switch whence {
	case io.SeekStart:
		base = 0
	case io.SeekCurrent:
		base = t.readOff
	case io.SeekEnd:
		base = int64(len(line))
	default:
		return 0, &fs.PathError{Op: "seek", Path: t.name, Err: fs.ErrInvalid}
	}
	t.readOff = base + offset
	if t.readOff < 0 {
		return 0, &fs.PathError{Op: "seek", Path: t.name, Err: fs.ErrInvalid}
	}
	return t.readOff, nil
}
