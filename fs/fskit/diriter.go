package fskit

import (
	"io"

	"tractor.dev/wanix/fs"
)

type DirIter struct {
	fn      func() ([]fs.DirEntry, error)
	entries []fs.DirEntry
	cursor  int
}

func NewDirIter(fn func() ([]fs.DirEntry, error)) *DirIter {
	return &DirIter{fn: fn}
}

func (rdi *DirIter) ReadDir(n int) ([]fs.DirEntry, error) {
	// Initialize directory entries on first call
	if rdi.entries == nil {
		entries, err := rdi.fn()
		if err != nil {
			return nil, err
		}
		rdi.entries = entries
		rdi.cursor = 0
	}

	// Handle the count parameter
	if n <= 0 {
		// Return all remaining entries
		// Per io/fs spec: when n <= 0, return all entries and nil (not EOF)
		result := rdi.entries[rdi.cursor:]
		rdi.cursor = len(rdi.entries) // Mark as consumed
		return result, nil
	}

	// For n > 0, check if we've reached the end
	if rdi.cursor >= len(rdi.entries) {
		rdi.entries = nil
		return nil, io.EOF
	}

	// Return up to n entries
	end := rdi.cursor + n
	if end > len(rdi.entries) {
		end = len(rdi.entries)
	}

	result := rdi.entries[rdi.cursor:end]
	rdi.cursor = end

	// If we've read to the end with n > 0, return EOF on next call
	if rdi.cursor >= len(rdi.entries) {
		// Don't return EOF yet, just mark for next time
	}

	return result, nil
}
