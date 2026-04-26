// Copyright 2023 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package compatgzip

import (
	"path/filepath"
	"strings"

	pkggzip "tractor.dev/wanix/rc/shell/compatgzip/pkggzip"
)

func getOutputPath(f *pkggzip.File) string {
	if f.Options.Stdout || f.Options.Test {
		return f.Path
	} else if f.Options.Decompress {
		return strings.TrimSuffix(f.Path, f.Options.Suffix)
	}
	return f.Path + f.Options.Suffix
}

func resolveOutputPath(g *Gzip, f *pkggzip.File) string {
	outputPath := getOutputPath(f)
	if filepath.IsAbs(outputPath) || g.WorkingDir == "" {
		return outputPath
	}
	return filepath.Join(g.WorkingDir, outputPath)
}
