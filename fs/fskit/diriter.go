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
	if n <= 0 {
		return rdi.fn()
	}

	// Initialize directory entries on first call
	if rdi.entries == nil {
		entries, err := rdi.fn()
		if err != nil {
			return nil, err
		}
		rdi.entries = entries
		rdi.cursor = 0
	}

	// Check if we've reached the end
	if rdi.cursor >= len(rdi.entries) {
		rdi.entries = nil
		return nil, io.EOF
	}

	// Handle the count parameter
	if n <= 0 {
		// Return all remaining entries
		result := rdi.entries[rdi.cursor:]
		rdi.cursor = len(rdi.entries) // Mark as consumed
		return result, nil
	}

	// Return up to n entries
	end := rdi.cursor + n
	if end > len(rdi.entries) {
		end = len(rdi.entries)
	}

	result := rdi.entries[rdi.cursor:end]
	rdi.cursor = end

	return result, nil
}
