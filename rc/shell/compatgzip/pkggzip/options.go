// Copyright 2017-2018 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pkggzip

import (
	"compress/gzip"
	"errors"
	"flag"
	"fmt"
	"path/filepath"
	"runtime"
)

var (
	ErrStdoutNoForce = errors.New("ignoring stdout, use -f to compression")
	ErrHelp          = errors.New("help requested")
)

// Options represents the CLI options possible, controlling how
// gzip operates on the given input data.
type Options struct {
	Suffix     string
	Blocksize  int
	Level      int
	Processes  int
	Keep       bool
	Help       bool
	Force      bool
	Quiet      bool
	Stdin      bool
	Stdout     bool
	Test       bool
	Verbose    bool
	Decompress bool
}

// ParseArgs takes CLI args and parses them via a Flagset into fields in
// the Options struct. Returns any errors from parsing and validating options.
func (o *Options) ParseArgs(cmd string, args []string, cmdLine *flag.FlagSet) error {
	var levels [10]bool

	cmdLine.IntVar(&o.Blocksize, "b", 128, "Set compression block size in KiB")
	cmdLine.BoolVar(&o.Decompress, "d", false, "Decompress the compressed input")
	cmdLine.BoolVar(&o.Force, "f", false, "Force overwrite of output file and compress links")
	cmdLine.BoolVar(&o.Help, "h", false, "Display a help screen and quit")
	cmdLine.BoolVar(&o.Keep, "k", false, "Do not delete original file after processing")
	cmdLine.IntVar(&o.Processes, "p", runtime.NumCPU(), "Allow up to n compression threads (ignored; stdlib gzip)")
	cmdLine.BoolVar(&o.Quiet, "q", false, "Print no messages, even on error")
	cmdLine.BoolVar(&o.Stdout, "c", false, "Write all processed output to stdout (won't delete)")
	cmdLine.StringVar(&o.Suffix, "S", ".gz", "Specify suffix for compression")
	cmdLine.BoolVar(&o.Test, "t", false, "Test the integrity of the compressed input")
	cmdLine.BoolVar(&o.Verbose, "v", false, "Produce more verbose output")
	cmdLine.BoolVar(&levels[1], "1", false, "Compression Level 1")
	cmdLine.BoolVar(&levels[2], "2", false, "Compression Level 2")
	cmdLine.BoolVar(&levels[3], "3", false, "Compression Level 3")
	cmdLine.BoolVar(&levels[4], "4", false, "Compression Level 4")
	cmdLine.BoolVar(&levels[5], "5", false, "Compression Level 5")
	cmdLine.BoolVar(&levels[6], "6", false, "Compression Level 6")
	cmdLine.BoolVar(&levels[7], "7", false, "Compression Level 7")
	cmdLine.BoolVar(&levels[8], "8", false, "Compression Level 8")
	cmdLine.BoolVar(&levels[9], "9", false, "Compression Level 9")

	if err := cmdLine.Parse(args); err != nil {
		return err
	}

	var err error
	o.Level, err = parseLevels(levels)
	if err != nil {
		return err
	}

	moreArgs := len(cmdLine.Args()) > 0

	if !moreArgs {
		o.Stdin = true
	}

	return o.modify(cmd, moreArgs)
}

func (o *Options) modify(cmd string, moreArgs bool) error {
	if o.Help {
		return ErrHelp
	}

	if !moreArgs && !o.Force {
		return ErrStdoutNoForce
	}

	if o.Test {
		o.Decompress = true
	}

	switch filepath.Base(cmd) {
	case "gunzip":
		o.Decompress = true
	case "gzcat":
		o.Decompress = true
		o.Stdout = true
	}

	if !moreArgs {
		o.Stdin = true
		o.Stdout = true
	}

	return nil
}

func parseLevels(levels [10]bool) (int, error) {
	var level int

	for i, l := range levels {
		if l && level != 0 {
			return 0, fmt.Errorf("error: multiple compression levels specified")
		} else if l {
			level = i
		}
	}

	if level == 0 {
		return gzip.DefaultCompression, nil
	}

	return level, nil
}
