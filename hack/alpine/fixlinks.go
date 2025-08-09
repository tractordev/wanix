package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("Usage: %s <root-dir>", os.Args[0])
	}
	root := filepath.Clean(os.Args[1])

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink == 0 {
			return nil
		}

		target, err := os.Readlink(path)
		if err != nil {
			return err
		}

		if !filepath.IsAbs(target) {
			return nil // already relative
		}

		// Map absolute target inside rootfs
		mapped := filepath.Join(root, target)

		rel, err := filepath.Rel(filepath.Dir(path), mapped)
		if err != nil {
			return err
		}

		// If the target is in the same directory, prefix with "./"
		if filepath.Dir(path) == filepath.Dir(mapped) && !filepath.IsAbs(rel) && !startsWithDot(rel) {
			rel = "./" + rel
		}

		if err := os.Remove(path); err != nil {
			return err
		}
		if err := os.Symlink(rel, path); err != nil {
			return err
		}

		fmt.Printf("Converted: %s -> %s\n", path, rel)
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
}

func startsWithDot(s string) bool {
	return len(s) > 0 && s[0] == '.'
}
