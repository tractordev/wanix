package wasmutil

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"

	"tractor.dev/wanix/fs"
)

func DetectType(fsys fs.FS, path string) (string, error) {
	f, err := fsys.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	fseeker, ok := f.(io.Seeker)
	if !ok {
		return "", errors.New("file is not seekable")
	}

	// Skip WASM header (8 bytes: magic + version)
	fseeker.Seek(8, 0)

	// Read sections until we find imports (section ID 2)
	for {
		var sectionID byte
		if err := binary.Read(f, binary.LittleEndian, &sectionID); err != nil {
			return "", err
		}

		size := readVarUint(f)

		if sectionID == 2 { // Import section
			buf := make([]byte, size)
			f.Read(buf)

			if bytes.Contains(buf, []byte("wasi_snapshot_preview1")) {
				return "wasi", nil
			}
			if bytes.Contains(buf, []byte("gojs")) || bytes.Contains(buf, []byte("\x03env")) {
				return "gojs", nil
			}
			return "", errors.New("unknown WASM type")
		}

		fseeker.Seek(int64(size), io.SeekCurrent)
	}
}

func readVarUint(r io.Reader) uint64 {
	var v uint64
	var s uint
	b := []byte{0}
	for {
		r.Read(b)
		v |= uint64(b[0]&0x7f) << s
		if b[0]&0x80 == 0 {
			break
		}
		s += 7
	}
	return v
}
