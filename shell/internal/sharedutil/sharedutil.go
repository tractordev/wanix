package sharedutil

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Unix absolute path. Returns cwd if path is empty
func AbsPath(path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	wd, _ := os.Getwd()
	return filepath.Join(wd, path)
}

// Convert a Unix path to an io/fs path (See `io/fs.ValidPath()`)
// Use `absPath()` instead if passing result to OS functions
func UnixToFsPath(path string) string {
	if !filepath.IsAbs(path) {
		// Join calls Clean internally
		wd, _ := os.Getwd()
		path = filepath.Join(strings.TrimLeft(wd, "/"), path)
	} else {
		path = filepath.Clean(strings.TrimLeft(path, "/"))
	}
	return path
}

func CheckErr(w io.Writer, err error) (hadError bool) {
	if err != nil {
		io.WriteString(w, fmt.Sprintln(err))
		return true
	}
	return false
}
