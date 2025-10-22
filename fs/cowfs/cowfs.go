// Package cowfs implements a copy-on-write filesystem that combines a read-only base
// filesystem with a writable overlay filesystem. All modifications are made to the
// overlay, preserving the base filesystem's contents.
//
// The package provides robust handling of file operations including:
//   - Copy-on-write: Files are only copied to the overlay when modified
//   - Tombstones: Tracking of deleted files from the base layer
//   - Renames: Tracking of renamed files from the base layer with chain collapsing
//   - Directory unions: Merged directory views from both layers with tombstone filtering
//
// Example usage:
//
//	cfs := &cowfs.FS{
//		Base:    baseFS,    // read-only base filesystem
//		Overlay: overlayFS, // writable overlay filesystem
//	}
//
//	// All filesystem operations automatically handle copy-on-write behavior
//	cfs.OpenFile("file.txt", os.O_RDWR, 0644) // Copies from base if needed
//	cfs.Rename("old.txt", "new.txt")          // Tracks rename, tombstones old
//	cfs.Remove("unwanted.txt")                // Tombstones base files
//
// Path Resolution:
// Paths are resolved through rename chains before processing. For example, if a file
// is renamed a->b->c, accessing "a" or "b" will resolve to "c". Tombstones are checked
// only at the terminal path after following all renames.
package cowfs

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
)

// FS implements a copy-on-write filesystem that combines a read-only base
// filesystem with a writable overlay filesystem. All modifications are made to the
// overlay layer while preserving the base layer's contents.
//
// The zero value is not usable; both Base and Overlay must be set.
// Tombstones and Renames are automatically managed by the filesystem operations.
type FS struct {
	// Base is the read-only base filesystem that provides the initial content.
	// Must be set before use.
	Base fs.FS

	// Overlay is the writable filesystem where all modifications are stored.
	// Must be set before use.
	Overlay fs.FS

	// tombstones tracks deleted files from the base layer.
	// Keys are file paths, values are empty structs.
	// Automatically managed; do not modify directly.
	tombstones sync.Map

	// renames tracks renamed files from the base layer.
	// Keys are original base file paths, values are current paths.
	// Automatically managed; do not modify directly.
	renames sync.Map

	// whiteoutDir is the directory where whiteout files are stored.
	// Automatically managed; do not modify directly.
	whiteoutDir string
}

// Reset clears all rename and tombstone tracking in the filesystem.
// It does not remove any files from Base or Overlay; only resets copy-on-write bookkeeping.
func (u *FS) Reset() {
	u.tombstones.Range(func(k, v any) bool {
		u.tombstones.Delete(k)
		return true
	})
	u.renames.Range(func(k, v any) bool {
		u.renames.Delete(k)
		return true
	})
}

// Whiteout enables persistence of tombstones and renames to the overlay filesystem.
// This allows copy-on-write tracking information to survive filesystem remounts or
// application restarts.
//
// The method creates two subdirectories under the provided directory:
//   - deletes/: Contains files tracking tombstoned (deleted) paths from the base layer
//   - renames/: Contains files tracking rename operations from the base layer
//
// Each tracking file is named using the SHA1 hash of the original path:
//   - Delete files contain the tombstoned path as their content
//   - Rename files contain "oldpath newpath" as their content
//
// Calling Whiteout will:
//  1. Create the necessary directory structure in the overlay
//  2. Load any existing tombstones and renames from previous sessions
//  3. Enable automatic persistence of future tombstone and rename operations
//
// Example usage:
//
//	cfs := &cowfs.FS{
//		Base:    baseFS,
//		Overlay: overlayFS,
//	}
//	// Enable persistence to .wh directory in overlay
//	if err := cfs.Whiteout(".wh"); err != nil {
//		return err
//	}
//	// All subsequent Remove and Rename operations will be persisted
//	cfs.Remove("file.txt")  // Creates .wh/deletes/<hash> file
//	cfs.Rename("a", "b")    // Creates .wh/renames/<hash> file
//
// Note: Whiteout should typically be called once during filesystem initialization,
// before performing any operations. The directory path is relative to the overlay
// filesystem root.
func (u *FS) Whiteout(dir string) error {
	u.whiteoutDir = dir
	if err := fs.MkdirAll(u.Overlay, path.Join(dir, "deletes"), 0o755); err != nil {
		return err
	}
	if err := fs.MkdirAll(u.Overlay, path.Join(dir, "renames"), 0o755); err != nil {
		return err
	}
	return fs.WalkDir(u.Overlay, dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasPrefix(p, path.Join(dir, "deletes")) {
			del, err := fs.ReadFile(u.Overlay, p)
			if err != nil {
				return err
			}
			u.tombstones.Store(strings.TrimSpace(string(del)), struct{}{})
			return nil
		}
		if strings.HasPrefix(p, path.Join(dir, "renames")) {
			rename, err := fs.ReadFile(u.Overlay, p)
			if err != nil {
				return err
			}
			parts := strings.Split(strings.TrimSpace(string(rename)), " ")
			u.renames.Store(parts[0], parts[1])
			return nil
		}
		return nil
	})

}

func (u *FS) tombstone(name string) error {
	u.tombstones.Store(name, struct{}{})
	if u.whiteoutDir != "" {
		h := sha1.New()
		h.Write([]byte(name))
		filename := path.Join(u.whiteoutDir, "deletes", fmt.Sprintf("%x", h.Sum(nil)))
		if err := fs.WriteFile(u.Overlay, filename, []byte(name), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func (u *FS) rename(oldname, newname string) error {
	u.renames.Store(oldname, newname)
	if u.whiteoutDir != "" {
		h := sha1.New()
		h.Write([]byte(oldname))
		filename := path.Join(u.whiteoutDir, "renames", fmt.Sprintf("%x", h.Sum(nil)))
		content := []byte(fmt.Sprintf("%s %s", oldname, newname))
		if err := fs.WriteFile(u.Overlay, filename, content, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// resolveTerminal follows rename chains to the terminal path without checking tombstones.
// Returns fs.ErrInvalid for cycles or pathological chains (>1000 hops).
// Use this when you need the final path regardless of tombstone status.
func (u *FS) resolveTerminal(path string) (string, error) {
	const maxHops = 1000 // prevent pathological chains
	seen := map[string]struct{}{}
	cur := path
	hops := 0

	for {
		// Hop limit guard
		hops++
		if hops > maxHops {
			return "", fs.ErrInvalid
		}

		// Cycle guard
		if _, ok := seen[cur]; ok {
			return "", fs.ErrInvalid
		}
		seen[cur] = struct{}{}

		// Follow rename if any
		if v, ok := u.renames.Load(cur); ok {
			// Treat self-maps as invalid
			if v.(string) == cur {
				return "", fs.ErrInvalid
			}
			cur = v.(string)
			continue
		}

		// Terminal: return without checking tombstone
		return cur, nil
	}
}

// resolvePath resolves a path through rename chains and tombstone checks.
// It follows rename mappings to their final destination, then checks if the
// terminal path is tombstoned. Returns fs.ErrNotExist if tombstoned, fs.ErrInvalid
// for cycles or pathological chains (>1000 hops), and the resolved path otherwise.
func (u *FS) resolvePath(path string) (string, error) {
	cur, err := u.resolveTerminal(path)
	if err != nil {
		return "", err
	}

	// Check tombstone at terminal
	if _, dead := u.tombstones.Load(cur); dead {
		return "", fs.ErrNotExist
	}
	return cur, nil
}

// shouldCopy determines if a file needs to be copied from base to overlay.
// Returns true if the file exists in base but not in overlay.
// Returns false if already in overlay or doesn't exist in base.
func (u *FS) shouldCopy(name string) (bool, error) {
	// Already in overlay?
	if _, err := fs.Stat(u.Overlay, name); err == nil {
		return false, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}

	// Exists in base?
	if _, err := fs.Stat(u.Base, name); err == nil {
		return true, nil
	} else if errors.Is(err, fs.ErrNotExist) {
		return false, fs.ErrNotExist
	} else {
		return false, err
	}
}

// copyIfNeeded resolves the path and copies the file from base to overlay if needed.
// Returns the resolved path and any error encountered.
func (u *FS) copyIfNeeded(name string) (string, error) {
	name, err := u.resolvePath(name)
	if err != nil {
		return "", err
	}

	copyNeeded, err := u.shouldCopy(name)
	if err != nil {
		return "", err
	}
	if copyNeeded {
		if err := fs.CopyFS(u.Base, name, u.Overlay, name); err != nil {
			return "", err
		}
	}
	return name, nil
}

// Chtimes changes the access and modification times of the named file.
// If the file exists in the base layer but not in the overlay, it will be
// copied to the overlay before changing the times.
func (u *FS) Chtimes(name string, atime, mtime time.Time) error {
	log.Println("Chtimes", name, atime, mtime)
	name = filepath.Clean(name)
	name, err := u.copyIfNeeded(name)
	if err != nil {
		return err
	}
	return fs.Chtimes(u.Overlay, name, atime, mtime)
}

// Chmod changes the mode of the named file to mode.
// If the file exists in the base layer but not in the overlay, it will be
// copied to the overlay before changing the mode.
func (u *FS) Chmod(name string, mode os.FileMode) error {
	log.Println("Chmod", name, mode)
	name = filepath.Clean(name)
	name, err := u.copyIfNeeded(name)
	if err != nil {
		return err
	}
	return fs.Chmod(u.Overlay, name, mode)
}

// Chown changes the numeric uid and gid of the named file.
// If the file exists in the base layer but not in the overlay, it will be
// copied to the overlay before changing ownership.
func (u *FS) Chown(name string, uid, gid int) error {
	log.Println("Chown", name, uid, gid)
	name = filepath.Clean(name)
	name, err := u.copyIfNeeded(name)
	if err != nil {
		return err
	}
	return fs.Chown(u.Overlay, name, uid, gid)
}

// Rename renames (moves) oldname to newname with full copy-on-write semantics.
// The source path (oldname) is resolved through rename chains before processing.
// The destination path (newname) is NOT resolved - POSIX rename overwrites newname itself.
//
// Behavior:
//   - If oldname exists only in overlay: renamed directly in overlay
//   - If oldname exists only in base: copied to overlay as newname, oldname tombstoned
//   - If oldname exists in both layers: overlay version renamed, base version tombstoned
//   - If newname exists: removed from overlay and/or tombstoned in base before rename
//   - Parent directories are automatically created in overlay if needed
//
// Tracking:
//   - Renames are tracked only for files that originated in the base layer
//   - Rename map entries point from original base paths to current locations
//   - Rename chains are automatically collapsed (a->b->c becomes a->c)
//   - Tombstones are cleared on successful destination creation
//   - Only base origins are tombstoned; overlay-only files are not
func (u *FS) Rename(oldname, newname string) error {
	// 0. Normalize paths
	oldname = filepath.Clean(oldname)
	newname = filepath.Clean(newname)
	log.Println("Rename", oldname, newname)

	// 1. Resolve source path (oldname) through rename chain
	src, err := u.resolvePath(oldname)
	if err != nil {
		return err
	}
	// Note: Do NOT resolve newname through renames - POSIX rename overwrites newname itself

	// 2. Check if source exists in base and overlay
	srcInBase := false
	if _, err := fs.Stat(u.Base, src); err == nil {
		srcInBase = true
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	srcInOverlay := false
	if _, err := fs.Stat(u.Overlay, src); err == nil {
		srcInOverlay = true
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	// 3. Source must exist somewhere
	if !srcInBase && !srcInOverlay {
		return fs.ErrNotExist
	}

	// 4. Handle existing destination: remove if in overlay, tombstone if in base
	statInfo, overlayErr := fs.Stat(u.Overlay, newname)
	if overlayErr == nil {
		// Destination exists in overlay - remove it
		if statInfo.IsDir() {
			// For directories, ensure empty before removing
			if removeErr := fs.Remove(u.Overlay, newname); removeErr != nil {
				return removeErr
			}
		} else {
			if removeErr := fs.Remove(u.Overlay, newname); removeErr != nil {
				return removeErr
			}
		}
	} else if !errors.Is(overlayErr, fs.ErrNotExist) {
		return overlayErr
	}

	_, baseErr := fs.Stat(u.Base, newname)
	if baseErr == nil {
		// Destination exists in base - tombstone it
		if err := u.tombstone(newname); err != nil {
			return err
		}
	} else if !errors.Is(baseErr, fs.ErrNotExist) {
		return baseErr
	}

	// 5. Ensure destination parent directories exist (scaffold in overlay)
	if dir := filepath.Dir(newname); dir != "." {
		if _, statErr := fs.Stat(u.Overlay, dir); errors.Is(statErr, fs.ErrNotExist) {
			if mkdirErr := fs.MkdirAll(u.Overlay, dir, 0o755); mkdirErr != nil {
				return mkdirErr
			}
		} else if statErr != nil {
			return statErr
		}
	}

	// 6. Move the content
	if srcInOverlay {
		// Source is in overlay - rename it directly
		if err := fs.Rename(u.Overlay, src, newname); err != nil {
			return err
		}
	} else {
		// Source only in base - copy up to overlay with new name
		if err := fs.CopyFS(u.Base, src, u.Overlay, newname); err != nil {
			return err
		}
	}

	// 7. Update bookkeeping after successful move

	// Tombstone original base origin(s) - only tombstone if they exist in base
	if srcInBase {
		if err := u.tombstone(src); err != nil {
			return err
		}
	}
	if oldname != src && srcInBase {
		if err := u.tombstone(oldname); err != nil {
			return err
		}
	}

	// Update rename map: redirect any entries pointing to src to point to newname
	u.renames.Range(func(k, v any) bool {
		if v.(string) == src {
			if err := u.rename(k.(string), newname); err != nil {
				log.Println("rename persistence error", err)
				return false
			}
		}
		return true
	})

	// If oldname was a base origin, add/refresh its mapping
	if srcInBase {
		if err := u.rename(oldname, newname); err != nil {
			return err
		}
	}

	// Clear tombstone on the destination (file is now alive there)
	u.tombstones.Delete(newname)

	return nil
}

// Remove removes the named file or directory with copy-on-write semantics.
// The path is resolved through rename chains before processing.
//
// Behavior:
//   - If exists in overlay: removed directly from overlay
//   - If exists only in base: tombstoned (marked as deleted)
//   - If already tombstoned: returns nil (idempotent)
//   - Directories must be empty before removal (returns fs.ErrInvalid if not empty)
//
// Cleanup:
//   - Any rename map entries pointing to the removed file are deleted
//   - Returns fs.ErrNotExist if the file doesn't exist in either layer
func (u *FS) Remove(name string) error {
	// 0. Normalize path
	name = filepath.Clean(name)
	log.Println("Remove", name)

	// 1. Resolve to terminal path (following rename chains)
	target, err := u.resolveTerminal(name)
	if err != nil {
		log.Println("resolveTerminal error", err)
		return err
	}

	// 2. Check if file exists in base (we'll need this info)
	existsInBase := false
	if _, err := fs.Stat(u.Base, target); err == nil {
		existsInBase = true
	} else if !errors.Is(err, fs.ErrNotExist) {
		log.Println("stat base error", err)
		return err
	}

	// 3. Check if file exists in overlay
	existsInOverlay := false
	if _, err := fs.Stat(u.Overlay, target); err == nil {
		existsInOverlay = true
	} else if !errors.Is(err, fs.ErrNotExist) {
		log.Println("stat overlay error", err)
		return err
	}

	// 4. Handle already tombstoned files (idempotent)
	if _, ok := u.tombstones.Load(target); ok {
		// If it's in overlay, still remove it
		if existsInOverlay {
			if err := fs.Remove(u.Overlay, target); err != nil {
				log.Println("remove overlay error", err)
				return err
			}
		}
		return nil
	}

	// 5. If file doesn't exist anywhere, return error
	if !existsInBase && !existsInOverlay {
		if err := u.tombstone(target); err != nil {
			log.Println("tombstone persistence error", err)
		}
		return fs.ErrNotExist
	}

	// 6. Check if it's a directory
	isDir := false
	if existsInOverlay {
		if info, err := fs.Stat(u.Overlay, target); err == nil {
			isDir = info.IsDir()
		}
	} else if existsInBase {
		if info, err := fs.Stat(u.Base, target); err == nil {
			isDir = info.IsDir()
		}
	}

	// 7. If it's a directory, check if it's empty
	if isDir {
		// Check if directory exists in overlay
		var overlayEntries []fs.DirEntry
		if existsInOverlay {
			f, err := u.Overlay.Open(target)
			if err != nil {
				log.Println("open overlay error", err)
				return err
			}
			defer f.Close()
			if dir, ok := f.(fs.ReadDirFile); ok {
				overlayEntries, err = dir.ReadDir(-1)
				if err != nil {
					log.Println("readdir overlay error", err)
					return err
				}
			}
		}

		// Check if directory exists in base
		var baseEntries []fs.DirEntry
		if existsInBase {
			f, err := u.Base.Open(target)
			if err != nil {
				log.Println("open base error", err)
				return err
			}
			defer f.Close()
			if dir, ok := f.(fs.ReadDirFile); ok {
				baseEntries, err = dir.ReadDir(-1)
				if err != nil {
					log.Println("readdir base error", err)
					return err
				}
			}
		}

		// Build a map of all entries, considering tombstones
		entries := make(map[string]struct{})
		for _, e := range baseEntries {
			name := filepath.Join(target, e.Name())
			if _, ok := u.tombstones.Load(name); !ok {
				entries[e.Name()] = struct{}{}
			}
		}
		for _, e := range overlayEntries {
			entries[e.Name()] = struct{}{}
		}

		if len(entries) > 0 {
			log.Println("directory not empty", target)
			return fs.ErrInvalid // Directory not empty
		}
	}

	// 8. Remove from overlay if it exists there
	if existsInOverlay {
		if err := fs.Remove(u.Overlay, target); err != nil {
			log.Println("remove overlay error", err)
			return err
		}
	}

	// 9. tombstone it to record it was deleted
	// log.Println("tombstoning", target)
	if err := u.tombstone(target); err != nil {
		return err
	}

	// 10. Clean up any renames that pointed to this file
	u.renames.Range(func(k, v any) bool {
		if v == target {
			u.renames.Delete(k)
		}
		return true
	})

	return nil
}

// Symlink creates a symbolic link named newname pointing to oldname.
// The symlink is always created in the overlay layer at the exact path specified.
//
// Behavior:
//   - If newname exists in overlay: removed before creating symlink
//   - If newname exists in base: tombstoned before creating symlink
//   - Parent directory must exist in at least one layer
//   - Parent directory is scaffolded in overlay if only in base
//   - The target (oldname) does not need to exist
//   - Tombstone is cleared after successful creation
func (u *FS) Symlink(oldname, newname string) error {
	// 1. Use raw path for new symlink (don't follow renames for create-destination)
	newpath := filepath.Clean(newname)
	log.Println("Symlink", oldname, newname)

	// 2. Check and prepare parent directory
	dir := filepath.Dir(newpath)
	if dir != "." {
		// Check if parent exists in overlay
		parentInOverlay := false
		if _, err := fs.Stat(u.Overlay, dir); err == nil {
			parentInOverlay = true
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}

		// Check if parent exists in base
		parentInBase := false
		if _, err := fs.Stat(u.Base, dir); err == nil {
			parentInBase = true
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}

		// Parent must exist in at least one layer
		if !parentInOverlay && !parentInBase {
			return fs.ErrNotExist
		}

		// If parent only exists in base, create empty directory scaffolding in overlay
		if !parentInOverlay && parentInBase {
			if err := fs.MkdirAll(u.Overlay, dir, 0o755); err != nil {
				return err
			}
		}
	}

	// 3. Handle existing file at target path (remove overlay, tombstone base)
	if _, err := fs.Stat(u.Overlay, newpath); err == nil {
		if err := fs.Remove(u.Overlay, newpath); err != nil {
			return err
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	if _, err := fs.Stat(u.Base, newpath); err == nil {
		// Hide base entry
		if err := u.tombstone(newpath); err != nil {
			return err
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	// 4. Create the symlink in overlay
	// Note: We don't check if oldname exists because symlinks can point to non-existent targets
	if err := fs.Symlink(u.Overlay, oldname, newpath); err != nil {
		return err
	}

	// 5. Clear tombstone after successful creation
	u.tombstones.Delete(newpath)

	return nil
}

// Mkdir creates a new directory with the specified name and permission bits.
// The directory is always created in the overlay layer at the exact path specified.
//
// Behavior:
//   - If directory exists in overlay: returns fs.ErrExist
//   - If directory exists in base: tombstoned then recreated in overlay
//   - Parent directory must exist in at least one layer
//   - Parent directory is scaffolded in overlay if only in base
//   - Tombstone is cleared after successful creation
func (u *FS) Mkdir(name string, perm os.FileMode) error {
	// 1. Use raw path for new directory (don't follow renames for create-destination)
	path := filepath.Clean(name)
	log.Println("Mkdir", name, perm)

	// 2. Check if directory already exists in either layer
	if _, err := fs.Stat(u.Overlay, path); err == nil {
		return fs.ErrExist
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	existsInBase := false
	if _, err := fs.Stat(u.Base, path); err == nil {
		existsInBase = true
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	// 3. Check parent directory existence and create if needed
	dir := filepath.Dir(path)
	if dir != "." {
		// Check if parent exists in overlay
		parentInOverlay := false
		if _, err := fs.Stat(u.Overlay, dir); err == nil {
			parentInOverlay = true
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}

		// Check if parent exists in base
		parentInBase := false
		if _, err := fs.Stat(u.Base, dir); err == nil {
			parentInBase = true
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}

		// Parent must exist in at least one layer
		if !parentInOverlay && !parentInBase {
			return fs.ErrNotExist
		}

		// If parent only exists in base, create empty directory scaffolding in overlay
		if !parentInOverlay && parentInBase {
			if err := fs.MkdirAll(u.Overlay, dir, 0o755); err != nil {
				return err
			}
		}
	}

	// 4. If directory exists in base, tombstone it
	if existsInBase {
		if err := u.tombstone(path); err != nil {
			return err
		}
	}

	// 5. Create directory in overlay
	if err := fs.Mkdir(u.Overlay, path, perm); err != nil {
		return err
	}

	// 6. Clear tombstone after successful creation
	u.tombstones.Delete(path)

	return nil
}

// Create creates or truncates the named file in the overlay.
// If the file exists in the base layer, it will be shadowed by the new file.
// Returns the file ready for reading and writing.
func (u *FS) Create(name string) (fs.File, error) {
	return u.OpenFile(name, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0o666)
}

// Stat returns file information for the named file.
// The path is resolved through rename chains before processing.
// Prefers overlay, falls back to base. Returns fs.ErrNotExist if tombstoned.
func (u *FS) Stat(name string) (os.FileInfo, error) {
	// 0. Normalize path
	name = filepath.Clean(name)
	log.Println("Stat", name)

	// 1. Resolve rename chain
	path, err := u.resolvePath(name)
	if err != nil {
		return nil, err
	}

	// 2. Check for tombstone
	if _, ok := u.tombstones.Load(path); ok {
		return nil, fs.ErrNotExist
	}

	// 3. Try overlay first
	if fi, err := fs.Stat(u.Overlay, path); err == nil {
		return fi, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	// 4. Fallback to base
	fi, err := fs.Stat(u.Base, path)
	if err != nil {
		return nil, err
	}
	return fi, nil
}

// Readlink returns the target path of the specified symbolic link.
// The path is resolved through rename chains before processing.
// Prefers overlay, falls back to base. Returns fs.ErrNotExist if tombstoned.
func (u *FS) Readlink(name string) (string, error) {
	// 0. Normalize path
	name = filepath.Clean(name)

	// 1. Resolve rename chain first
	path, err := u.resolvePath(name)
	if err != nil {
		return "", err
	}

	// 2. Check tombstone
	if _, ok := u.tombstones.Load(path); ok {
		return "", fs.ErrNotExist
	}

	// 3. Try overlay first
	if target, err := fs.Readlink(u.Overlay, path); err == nil {
		return target, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}

	// 4. Fall back to base
	return fs.Readlink(u.Base, path)
}

// Deleted returns paths that should be deleted from the base filesystem.
// This includes all tombstoned files and all rename sources (files that have been moved).
func (u *FS) Deleted() []string {
	finalDeletes := map[string]struct{}{}

	// 1. Add all tombstoned paths
	u.tombstones.Range(func(k, _ any) bool {
		finalDeletes[k.(string)] = struct{}{}
		return true
	})

	// 2. Add all rename sources (original paths that have been moved)
	u.renames.Range(func(k, _ any) bool {
		finalDeletes[k.(string)] = struct{}{}
		return true
	})

	// 3. Convert to sorted slice
	paths := make([]string, 0, len(finalDeletes))
	for p := range finalDeletes {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}

// OpenFile opens the named file with the specified flags and permissions.
// The path is resolved through rename chains before processing.
//
// Write Operations (O_WRONLY, O_RDWR, O_APPEND, O_CREATE, O_TRUNC):
//   - Files only in base are copied to overlay before opening
//   - Parent directories are automatically created/scaffolded in overlay
//   - O_TRUNC creates empty file in overlay (no copy from base)
//   - Tombstones are cleared after successful write/create
//
// Special Flags:
//   - O_EXCL: fails if file exists in overlay or non-tombstoned base
//   - O_CREATE: automatically scaffolds parent directories
//
// Read Operations:
//   - Prefers overlay, falls back to base if not found
//   - Returns fs.ErrNotExist if tombstoned
func (u *FS) OpenFile(name string, flag int, perm os.FileMode) (fs.File, error) {
	// 0. Normalize path
	name = filepath.Clean(name)
	log.Println("OpenFile", name, flag, perm)

	// 1. Resolve rename chain and check tombstone
	path, err := u.resolvePath(name)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	if errors.Is(err, fs.ErrNotExist) {
		path = name // allow creating new files
	}

	// Check if file is tombstoned (unless we're creating a new file)
	if flag&os.O_CREATE == 0 {
		if _, ok := u.tombstones.Load(path); ok {
			return nil, fs.ErrNotExist
		}
	}

	// 2. Check existence in both layers
	existsInOverlay := false
	if _, err := fs.Stat(u.Overlay, path); err == nil {
		existsInOverlay = true
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	existsInBase := false
	if _, err := fs.Stat(u.Base, path); err == nil {
		existsInBase = true
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	// 3. Handle O_EXCL flag - tombstoned base entries don't count as existing
	creating := flag&os.O_CREATE != 0
	exclusive := flag&os.O_EXCL != 0
	if creating && exclusive {
		baseExists := false
		if _, err := fs.Stat(u.Base, path); err == nil {
			if _, dead := u.tombstones.Load(path); !dead {
				baseExists = true
			}
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		if baseExists || existsInOverlay {
			return nil, fs.ErrExist
		}
	}

	// 4. Handle write modes
	writeMode := flag&(os.O_WRONLY|os.O_RDWR|os.O_APPEND|os.O_CREATE|os.O_TRUNC) != 0
	if writeMode {
		// Check parent directory existence
		dir := filepath.Dir(path)
		if dir != "." {
			// Check if parent exists in overlay
			parentInOverlay := false
			if _, err := fs.Stat(u.Overlay, dir); err == nil {
				parentInOverlay = true
			} else if !errors.Is(err, fs.ErrNotExist) {
				return nil, err
			}

			// Check if parent exists in base
			parentInBase := false
			if _, err := fs.Stat(u.Base, dir); err == nil {
				parentInBase = true
			} else if !errors.Is(err, fs.ErrNotExist) {
				return nil, err
			}

			// For write modes, ensure parent exists in overlay
			if !parentInOverlay {
				if parentInBase || creating {
					// Create empty directory scaffolding in overlay
					if err := fs.MkdirAll(u.Overlay, dir, 0o755); err != nil {
						return nil, err
					}
				} else {
					// For non-create write modes, parent must exist
					return nil, fs.ErrNotExist
				}
			}
		}

		// Handle copy-up for write modes
		// For O_TRUNC without O_CREATE on base-only file: will fail naturally (POSIX)
		if existsInBase && !existsInOverlay {
			if flag&os.O_TRUNC != 0 {
				// O_TRUNC: copy empty file to overlay (truncation happens on open)
				if creating {
					// O_CREATE|O_TRUNC: create empty file
					if err := fs.WriteFile(u.Overlay, path, []byte{}, perm); err != nil {
						return nil, err
					}
				}
				// O_TRUNC without O_CREATE: don't copy up, let open fail naturally
			} else {
				// No truncate: copy full file from base
				if err := fs.CopyFS(u.Base, path, u.Overlay, path); err != nil && !errors.Is(err, fs.ErrNotExist) {
					return nil, err
				}
			}
		}

		// Open in overlay
		f, err := fs.OpenFile(u.Overlay, path, flag, perm)
		if err != nil {
			return nil, err
		}

		// Clear tombstone after successful write/create
		u.tombstones.Delete(path)

		return f, nil
	}

	// 5. Read-only mode: prefer overlay, then base
	if f, err := fs.OpenFile(u.Overlay, path, flag, perm); err == nil {
		return f, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	return fs.OpenFile(u.Base, path, flag, perm)
}

// Open returns a file or directory from the copy-on-write filesystem.
// The path is resolved through rename chains before processing.
//
// Behavior:
//   - Files: prefers overlay, falls back to base
//   - Directories in both layers: returns union view with HideFn for tombstoned entries
//   - Directories in one layer: returns that directory (with HideFn if from base)
//   - Tombstoned paths: returns fs.ErrNotExist
//
// Directory Union:
//   - Merges contents from both base and overlay layers
//   - Overlay entries shadow base entries with same name
//   - Tombstoned entries are filtered out via HideFn
//   - Provides unified ReadDir across both layers
func (u *FS) Open(name string) (fs.File, error) {
	// 0. Normalize path
	name = filepath.Clean(name)
	log.Println("Open", name)

	// 1. Resolve rename chain
	path, err := u.resolvePath(name)
	if err != nil {
		return nil, err
	}

	// 2. Tombstone check
	if _, ok := u.tombstones.Load(path); ok {
		return nil, fs.ErrNotExist
	}

	// 3. Check overlay first
	copyNeeded, err := u.shouldCopy(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	// Overlay has it
	if !copyNeeded {
		isDir, err := fs.IsDir(u.Overlay, path)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		if err == nil && !isDir {
			return u.Overlay.Open(path)
		}
		if err == nil && isDir {
			// Directory exists in overlay — check base next
			baseIsDir, err := fs.IsDir(u.Base, path)
			if err != nil || !baseIsDir {
				return u.Overlay.Open(path)
			}

			// Both are dirs → return union file with hide function for tombstoned entries
			bfile, bErr := u.Base.Open(path)
			lfile, lErr := u.Overlay.Open(path)
			if bErr != nil || lErr != nil {
				return nil, fmt.Errorf("BaseErr: %v\nOverlayErr: %v", bErr, lErr)
			}
			return &fskit.OverlayFile{
				Base:    bfile,
				Overlay: lfile,
				HideFn: func(name string) bool {
					// Hide entry if it's tombstoned
					fullPath := filepath.Join(path, name)
					_, hidden := u.tombstones.Load(fullPath)
					return hidden
				},
			}, nil
		}
	}

	// 4. No overlay — check if base is a directory that might need tombstone filtering
	baseIsDir, err := fs.IsDir(u.Base, path)
	if err == nil && baseIsDir {
		// Directory exists only in base — return it with hide function for tombstoned entries
		bfile, err := u.Base.Open(path)
		if err != nil {
			return nil, err
		}
		return &fskit.OverlayFile{
			Base:    bfile,
			Overlay: nil,
			HideFn: func(name string) bool {
				// Hide entry if it's tombstoned
				fullPath := filepath.Join(path, name)
				_, hidden := u.tombstones.Load(fullPath)
				return hidden
			},
		}, nil
	}

	// Fall back to base for files
	return u.Base.Open(path)
}
