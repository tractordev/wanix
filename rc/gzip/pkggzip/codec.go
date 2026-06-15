// Copyright 2017-2024 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Portable gzip compress/decompress for gojs/wasm: uses compress/gzip only
// (u-root's default build uses github.com/klauspost/pgzip).
package pkggzip

import (
	"compress/gzip"
	"io"
)

// Decompress expands gzip-compressed input from r to w.
// blocksize and processes are ignored (kept for API compatibility with u-root).
func decompress(r io.Reader, w io.Writer, _, _ int) error {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, zr); err != nil {
		_ = zr.Close()
		return err
	}
	return zr.Close()
}

// Compress deflates input from r to w.
// blocksize and processes are ignored (kept for API compatibility with u-root).
func compress(r io.Reader, w io.Writer, level int, _, _ int) error {
	zw, err := gzip.NewWriterLevel(w, level)
	if err != nil {
		return err
	}
	if _, err := io.Copy(zw, r); err != nil {
		_ = zw.Close()
		return err
	}
	return zw.Close()
}
