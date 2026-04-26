// Copyright 2013-2017 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package compatls implements the ls core utility.
package compatls

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/u-root/u-root/pkg/core"
	"github.com/u-root/u-root/pkg/uroot/unixflag"
	"tractor.dev/wanix/rc/shell/compatls/lsfmt"
)

type command struct {
	core.Base
}

func New() core.Command {
	c := &command{}
	c.Init()
	return c
}

type flags struct {
	all       bool
	human     bool
	directory bool
	long      bool
	quoted    bool
	recurse   bool
	classify  bool
	size      bool
	final     bool
}

type file struct {
	path string
	osfi os.FileInfo
	lsfi lsfmt.FileInfo
	err  error
}

func (c *command) printFile(stringer lsfmt.Stringer, f file, flags flags) {
	if f.err != nil {
		fmt.Fprintln(c.Stdout, f.err)
		return
	}
	if flags.all || !strings.HasPrefix(f.lsfi.Name, ".") {
		if (runtime.GOOS == "plan9" || runtime.GOOS == "windows") && !flags.final {
			f.lsfi.Name = f.path
		}
		if flags.classify {
			f.lsfi.Name = f.lsfi.Name + indicator(f.lsfi)
		}
		fmt.Fprintln(c.Stdout, stringer.FileString(f.lsfi))
	}
}

func (c *command) listName(stringer lsfmt.Stringer, d string, prefix bool, f flags) {
	var files []file
	resolvedPath := c.ResolvePath(d)

	filepath.Walk(resolvedPath, func(path string, osfi os.FileInfo, err error) error {
		file := file{path: path, osfi: osfi}
		if osfi != nil && !errors.Is(err, os.ErrNotExist) {
			file.lsfi = lsfmt.FromOSFileInfo(path, osfi)
			if err != nil && path == resolvedPath {
				file.err = err
			}
		} else {
			file.err = err
		}
		files = append(files, file)

		if err != nil {
			return filepath.SkipDir
		}
		if !f.recurse && path == resolvedPath && f.directory {
			return filepath.SkipDir
		}
		if path != resolvedPath && file.lsfi.Mode.IsDir() && !f.recurse {
			return filepath.SkipDir
		}
		return nil
	})

	if f.size {
		sort.SliceStable(files, func(i, j int) bool {
			return files[i].lsfi.Size > files[j].lsfi.Size
		})
	}

	for _, file := range files {
		if file.err != nil {
			c.printFile(stringer, file, f)
			continue
		}
		if f.recurse {
			file.lsfi.Name = file.path
		} else if file.path == resolvedPath {
			if f.directory {
				fmt.Fprintln(c.Stdout, stringer.FileString(file.lsfi))
				continue
			}
			if file.osfi.IsDir() {
				file.lsfi.Name = "."
				if prefix {
					if f.quoted {
						fmt.Fprintf(c.Stdout, "%q:\n", d)
					} else {
						fmt.Fprintf(c.Stdout, "%v:\n", d)
					}
				}
			}
		}
		c.printFile(stringer, file, f)
	}
}

func indicator(fi lsfmt.FileInfo) string {
	if fi.Mode.IsRegular() && fi.Mode&0o111 != 0 {
		return "*"
	}
	if fi.Mode&os.ModeDir != 0 {
		return "/"
	}
	if fi.Mode&os.ModeSymlink != 0 {
		return "@"
	}
	if fi.Mode&os.ModeSocket != 0 {
		return "="
	}
	if fi.Mode&os.ModeNamedPipe != 0 {
		return "|"
	}
	return ""
}

func (c *command) list(names []string, f flags) error {
	if len(names) == 0 {
		names = []string{"."}
	}
	tw := &tabwriter.Writer{}
	tw.Init(c.Stdout, 0, 0, 1, ' ', 0)
	stdout := c.Stdout
	c.Stdout = tw
	defer func() {
		tw.Flush()
		c.Stdout = stdout
	}()

	var s lsfmt.Stringer = lsfmt.NameStringer{}
	if f.quoted {
		s = lsfmt.QuotedStringer{}
	}
	if f.long {
		s = lsfmt.LongStringer{Human: f.human, Name: s}
	}
	prefix := len(names) > 1
	for _, d := range names {
		c.listName(s, d, prefix, f)
		tw.Flush()
	}
	return nil
}

func (c *command) Run(args ...string) error {
	return c.RunContext(context.Background(), args...)
}

func (c *command) RunContext(ctx context.Context, args ...string) error {
	var f flags

	fs := flag.NewFlagSet("ls", flag.ContinueOnError)
	fs.SetOutput(c.Stderr)
	fs.BoolVar(&f.all, "a", false, "show hidden files")
	fs.BoolVar(&f.human, "h", false, "human readable sizes")
	fs.BoolVar(&f.directory, "d", false, "list directories but not their contents")
	fs.BoolVar(&f.long, "l", false, "long form")
	fs.BoolVar(&f.quoted, "Q", false, "quoted")
	fs.BoolVar(&f.recurse, "R", false, "equivalent to findutil's find")
	fs.BoolVar(&f.classify, "F", false, "append indicator (, one of */=>@|) to entries")
	fs.BoolVar(&f.size, "S", false, "sort by size")
	if runtime.GOOS == "plan9" || runtime.GOOS == "windows" {
		fs.BoolVar(&f.final, "p", false, "Print only the final path element of each file name")
	}
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: ls [OPTIONS] [DIRS]...\n\n")
		fmt.Fprintf(fs.Output(), "Options:\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(unixflag.ArgsToGoArgs(args)); err != nil {
		return err
	}
	return c.list(fs.Args(), f)
}
