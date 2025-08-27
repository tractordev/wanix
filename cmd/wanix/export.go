//go:build !js && !wasm

package main

import (
	"log"
	"os"
	"path/filepath"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/wanix/runtime/assets"
)

func exportCmd() *cli.Command {
	cmd := &cli.Command{
		Usage: "export <dir>",
		Short: "(deprecated)",
		Args:  cli.ExactArgs(1),
		Run: func(ctx *cli.Context, args []string) {
			log.SetFlags(log.Ltime | log.Lmicroseconds | log.Lshortfile)

			dir := args[0]

			// Check if dir exists
			if fi, err := os.Stat(dir); err == nil {
				if !fi.IsDir() {
					log.Fatalf("%s exists but is not a directory", dir)
				}
				// Check if directory is empty
				entries, err := os.ReadDir(dir)
				if err != nil {
					log.Fatal(err)
				}
				if len(entries) > 0 {
					log.Fatalf("Directory %s is not empty", dir)
				}
			} else if os.IsNotExist(err) {
				// Create directory if it doesn't exist
				if err := os.MkdirAll(dir, 0755); err != nil {
					log.Fatal(err)
				}
			} else {
				log.Fatal(err)
			}

			// TODO: a flag to prefer debug
			wasmFsys, err := assets.WasmFS(false)
			if err != nil {
				log.Fatal(err)
			}

			// Copy files to directory
			fatal(copyFile(wasmFsys, "wanix.wasm", filepath.Join(dir, "wanix.wasm")))
			for _, f := range []string{
				"wanix.min.js",
				"wanix.js",
				"wanix-sw.js",
				"wanix.css",
				"favicon.ico",
				"index.html",
			} {
				if err := copyFile(assets.Dir, f, filepath.Join(dir, f)); err != nil {
					log.Fatal(err)
				}
			}

		},
	}
	return cmd
}
