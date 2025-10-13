//go:build fstest

package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"os"
	"os/signal"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/httpfs"
	"tractor.dev/wanix/hack/fstest"
	"tractor.dev/wanix/hack/slogger"
)

func main() {
	var v bool
	var vv bool
	var cacher bool
	flag.BoolVar(&cacher, "cacher", false, "enable caching")
	flag.BoolVar(&v, "v", false, "verbose: enable debug logging")
	flag.BoolVar(&vv, "vv", false, "very verbose: enable debug logging and p9debug")
	flag.Parse()

	// Set log level based on verbosity
	logLevel := slog.LevelInfo
	if v || vv {
		logLevel = slog.LevelDebug
	}
	slogger.Use(logLevel)

	var fsys fs.FS
	hfs := httpfs.New("https://r2fs.proteco.workers.dev/")
	if cacher {
		fsys = httpfs.NewCacher(hfs)
	} else {
		fsys = hfs
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		<-c
		cancel()
	}()

	// Enable p9debug if -vv is set
	p9debug := vv
	if err := fstest.RunFor(ctx, fsys, flag.Arg(0), p9debug); err != nil {
		log.Fatal(err)
	}

}
