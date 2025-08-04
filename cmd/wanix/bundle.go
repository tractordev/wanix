package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/wanix/external/linux"
	v86 "tractor.dev/wanix/external/v86"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/runtime/assets"
	"tractor.dev/wanix/shell"
)

func bundleCmd() *cli.Command {
	cmd := &cli.Command{
		Usage: "bundle",
		Short: "bundle commands",
	}
	cmd.AddCommand(bundleInitCmd())
	cmd.AddCommand(bundlePackCmd())
	cmd.AddCommand(bundleUnpackCmd())
	cmd.AddCommand(bundleInspectCmd())
	return cmd
}

func bundleInitCmd() *cli.Command {
	var (
		embedRuntime bool
		embedV86     bool
		embedLinux   bool
		embedShell   bool
	)
	cmd := &cli.Command{
		Usage: "init <dir>",
		Short: "Initialize a new unpacked bundle",
		Args:  cli.MinArgs(1),
		Run: func(ctx *cli.Context, args []string) {
			dir := args[0]

			// Check if dir exists and is not empty
			if entries, err := os.ReadDir(dir); err == nil && len(entries) > 0 {
				log.Fatal("directory exists and is not empty")
			}

			// Create dir if it doesn't exist
			if err := os.MkdirAll(dir, 0755); err != nil {
				log.Fatal(err)
			}

			// Create empty initrc file
			initrc := filepath.Join(dir, "initrc")
			if err := os.WriteFile(initrc, []byte{}, 0755); err != nil {
				log.Fatal(err)
			}

			if embedRuntime {
				wasmFs, err := assets.WasmFS(false)
				if err != nil {
					log.Fatal(err)
				}
				if err := copyFile(wasmFs, "wanix.wasm", filepath.Join(dir, "wanix.wasm")); err != nil {
					log.Fatal(err)
				}
			}

			if embedShell {
				imagedir := filepath.Join(dir, "vm", "image")
				if err := os.MkdirAll(imagedir, 0755); err != nil {
					log.Fatal(err)
				}
				if err := copyFile(shell.Dir, "shell.tgz", filepath.Join(imagedir, "shell.tgz")); err != nil {
					log.Fatal(err)
				}
				embedV86 = true
				embedLinux = true
			}

			if embedV86 {
				v86dir := filepath.Join(dir, "vm", "v86")
				if err := os.MkdirAll(v86dir, 0755); err != nil {
					log.Fatal(err)
				}

				files := []string{
					"v86.wasm",
					"libv86.js",
					"seabios.bin",
					"vgabios.bin",
				}
				for _, f := range files {
					if err := copyFile(v86.Dir, f, filepath.Join(v86dir, f)); err != nil {
						log.Fatal(err)
					}
				}
			}

			if embedLinux {
				linuxdir := filepath.Join(dir, "vm", "linux")
				if err := os.MkdirAll(linuxdir, 0755); err != nil {
					log.Fatal(err)
				}
				if err := copyFile(linux.Dir, "bzImage", filepath.Join(linuxdir, "bzImage")); err != nil {
					log.Fatal(err)
				}
			}

		},
	}
	cmd.Flags().BoolVar(&embedRuntime, "runtime", false, "embed Wasm runtime")
	cmd.Flags().BoolVar(&embedV86, "v86", false, "embed v86 hypervisor")
	cmd.Flags().BoolVar(&embedLinux, "linux", false, "embed Linux kernel")
	cmd.Flags().BoolVar(&embedShell, "shell", false, "embed Wanix shell")
	return cmd
}

func copyFile(src fs.FS, srcName, dstName string) error {
	data, err := fs.ReadFile(src, srcName)
	if err != nil {
		return err
	}
	return os.WriteFile(dstName, data, 0644)
}

func bundlePackCmd() *cli.Command {
	cmd := &cli.Command{
		Usage: "pack <bundle-dir> [bundle-file]",
		Short: "Pack a bundle into a single file",
		Args:  cli.MinArgs(1),
		Run: func(ctx *cli.Context, args []string) {
			log.SetFlags(log.Ltime | log.Lmicroseconds | log.Lshortfile)

			bundleDir := args[0]
			bundleFile := ""

			if len(args) < 2 {
				if base := filepath.Base(bundleDir); base != "." && base != "/" {
					bundleFile = base + ".tgz"
				} else {
					log.Fatal("bundle directory not valid")
				}
			} else {
				bundleFile = args[1]
			}

			// Create output file
			f, err := os.Create(bundleFile)
			if err != nil {
				log.Fatal(err)
			}
			defer f.Close()

			// Create gzip writer
			gw := gzip.NewWriter(f)
			defer gw.Close()

			// Create tar writer
			tw := tar.NewWriter(gw)
			defer tw.Close()

			// Walk the bundle directory and add files to tar
			err = filepath.Walk(bundleDir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				// Get relative path from bundle dir
				relPath, err := filepath.Rel(bundleDir, path)
				if err != nil {
					return err
				}

				// Skip the root dir itself
				if relPath == "." {
					return nil
				}

				// Create tar header
				header, err := tar.FileInfoHeader(info, "")
				if err != nil {
					return err
				}
				header.Name = relPath

				// Write header
				if err := tw.WriteHeader(header); err != nil {
					return err
				}

				// If not a directory, write file content
				if !info.IsDir() {
					file, err := os.Open(path)
					if err != nil {
						return err
					}
					defer file.Close()

					if _, err := io.Copy(tw, file); err != nil {
						return err
					}
				}

				return nil
			})

			if err != nil {
				log.Fatal(err)
			}
		},
	}
	return cmd
}

func bundleUnpackCmd() *cli.Command {
	cmd := &cli.Command{
		Usage: "unpack <bundle-file> [bundle-dir]",
		Short: "Unpack a bundle into a directory",
		Args:  cli.MinArgs(1),
		Run: func(ctx *cli.Context, args []string) {
			bundleFile := args[0]
			if !strings.HasSuffix(bundleFile, ".tgz") {
				log.Fatal("bundle file must end with .tgz")
			}

			bundleDir := bundleFile
			if len(args) > 1 {
				bundleDir = args[1]
			} else {
				bundleDir = strings.TrimSuffix(bundleFile, ".tgz")
			}

			// Check if bundle dir exists
			if fi, err := os.Stat(bundleDir); err == nil {
				if !fi.IsDir() {
					log.Fatal("bundle dir is a file")
				}
				// Check if empty
				f, err := os.Open(bundleDir)
				if err != nil {
					log.Fatal(err)
				}
				defer f.Close()

				if _, err := f.Readdirnames(1); err != io.EOF {
					log.Fatal("bundle dir exists and is not empty")
				}
			} else {
				if err := os.MkdirAll(bundleDir, 0755); err != nil {
					log.Fatal(err)
				}
			}

			// Open and read the gzipped tar file
			f, err := os.Open(bundleFile)
			if err != nil {
				log.Fatal(err)
			}
			defer f.Close()

			gzr, err := gzip.NewReader(f)
			if err != nil {
				log.Fatal(err)
			}
			defer gzr.Close()

			tr := tar.NewReader(gzr)
			for {
				header, err := tr.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					log.Fatal(err)
				}

				target := filepath.Join(bundleDir, header.Name)

				switch header.Typeflag {
				case tar.TypeDir:
					if err := os.MkdirAll(target, 0755); err != nil {
						log.Fatal(err)
					}
				case tar.TypeReg:
					dir := filepath.Dir(target)
					if err := os.MkdirAll(dir, 0755); err != nil {
						log.Fatal(err)
					}

					f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
					if err != nil {
						log.Fatal(err)
					}
					defer f.Close()

					if _, err := io.Copy(f, tr); err != nil {
						log.Fatal(err)
					}
				}
			}
		},
	}
	return cmd
}

func bundleInspectCmd() *cli.Command {
	cmd := &cli.Command{
		Usage: "inspect <bundle>",
		Short: "Show information about a bundle",
		Args:  cli.MinArgs(1),
		Run: func(ctx *cli.Context, args []string) {
			bundlePath := args[0]

			// Check if bundle is a directory or file
			fi, err := os.Stat(bundlePath)
			if err != nil {
				log.Fatal(err)
			}

			if fi.IsDir() {
				// Walk directory and print tree
				err = filepath.Walk(bundlePath, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}
					rel, err := filepath.Rel(bundlePath, path)
					if err != nil {
						return err
					}
					if rel == "." {
						return nil
					}

					depth := len(strings.Split(rel, string(os.PathSeparator))) - 1
					prefix := strings.Repeat("  ", depth) + "├─ "
					if info.IsDir() {
						fmt.Printf("%s%s/\n", prefix, filepath.Base(path))
					} else {
						fmt.Printf("%s%s\n", prefix, filepath.Base(path))
					}
					return nil
				})
				if err != nil {
					log.Fatal(err)
				}
			} else {
				// Open and read tar.gz file
				f, err := os.Open(bundlePath)
				if err != nil {
					log.Fatal(err)
				}
				defer f.Close()

				gzr, err := gzip.NewReader(f)
				if err != nil {
					log.Fatal(err)
				}
				defer gzr.Close()

				tr := tar.NewReader(gzr)

				// Track directories we've seen to avoid duplicates
				dirs := make(map[string]bool)

				// First pass to collect all paths
				var paths []string
				for {
					header, err := tr.Next()
					if err == io.EOF {
						break
					}
					if err != nil {
						log.Fatal(err)
					}

					// Add path with trailing slash if it's a directory
					if header.Typeflag == tar.TypeDir {
						dirName := header.Name
						if !strings.HasSuffix(dirName, "/") {
							dirName = dirName + "/"
						}
						if !dirs[dirName] {
							paths = append(paths, dirName)
							dirs[dirName] = true
						}
					} else {
						paths = append(paths, header.Name)
					}

					// Add all parent directories
					dir := filepath.Dir(header.Name)
					for dir != "." {
						dirWithSlash := dir + "/"
						if !dirs[dirWithSlash] {
							paths = append(paths, dirWithSlash)
							dirs[dirWithSlash] = true
						}
						dir = filepath.Dir(dir)
					}
				}

				// Sort paths for consistent output
				sort.Strings(paths)

				// Print tree
				for _, path := range paths {
					depth := len(strings.Split(strings.TrimSuffix(path, "/"), "/")) - 1
					prefix := strings.Repeat("  ", depth) + "├─ "
					base := filepath.Base(path)
					if strings.HasSuffix(path, "/") {
						fmt.Printf("%s%s/\n", prefix, strings.TrimSuffix(base, "/"))
					} else {
						fmt.Printf("%s%s\n", prefix, base)
					}
				}
			}
		},
	}
	return cmd
}
