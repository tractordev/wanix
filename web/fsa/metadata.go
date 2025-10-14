//go:build js && wasm

package fsa

import (
	"errors"
	"log"
	"sync"
	"time"

	"github.com/fxamacker/cbor/v2"
	"tractor.dev/wanix/fs"
)

// FileMetadata represents stored metadata for a file
type FileMetadata struct {
	Mode  fs.FileMode `cbor:"mode"`
	Mtime time.Time   `cbor:"mtime"`
	Atime time.Time   `cbor:"atime"`
}

// MetadataStore manages file metadata globally
type MetadataStore struct {
	data         sync.Map     // path -> FileMetadata
	opfsRoot     *FS          // Reference to OPFS root for persistence
	dirty        bool         // Needs flush to disk
	writePending bool         // Write is scheduled
	mu           sync.RWMutex // Protects dirty and writePending flags
}

var globalMetadata *MetadataStore
var metadataOnce sync.Once

// GetMetadataStore returns the global metadata store singleton
func GetMetadataStore() *MetadataStore {
	metadataOnce.Do(func() {
		globalMetadata = &MetadataStore{}
	})
	return globalMetadata
}

// Initialize sets up the metadata store with OPFS root and loads existing data
func (ms *MetadataStore) Initialize(opfsRoot *FS) error {
	ms.opfsRoot = opfsRoot
	return ms.loadFromDisk()
}

// GetMetadata retrieves metadata for a path
func (ms *MetadataStore) GetMetadata(path string) (FileMetadata, bool) {
	if val, ok := ms.data.Load(path); ok {
		return val.(FileMetadata), true
	}
	return FileMetadata{}, false
}

// SetMetadata stores metadata for a path and schedules async write
func (ms *MetadataStore) SetMetadata(path string, metadata FileMetadata) {
	ms.data.Store(path, metadata)
	ms.scheduleWrite()
}

// SetMode updates only the mode for a path
func (ms *MetadataStore) SetMode(path string, mode fs.FileMode) {
	metadata, exists := ms.GetMetadata(path)
	if !exists {
		metadata = FileMetadata{
			Mode:  mode,
			Mtime: time.Now(),
			Atime: time.Now(),
		}
	} else {
		metadata.Mode = mode
	}
	ms.SetMetadata(path, metadata)
}

// SetTimes updates mtime and atime for a path
func (ms *MetadataStore) SetTimes(path string, atime, mtime time.Time) {
	metadata, exists := ms.GetMetadata(path)
	if !exists {
		metadata = FileMetadata{
			Mode:  DefaultFileMode, // Will be corrected when file is accessed
			Mtime: mtime,
			Atime: atime,
		}
	} else {
		metadata.Mtime = mtime
		metadata.Atime = atime
	}
	ms.SetMetadata(path, metadata)
}

// DeleteMetadata removes metadata for a path (used when files are deleted)
func (ms *MetadataStore) DeleteMetadata(path string) {
	ms.data.Delete(path)
	ms.scheduleWrite()
}

// scheduleWrite marks the store as dirty and schedules an async write
func (ms *MetadataStore) scheduleWrite() {
	ms.mu.Lock()
	ms.dirty = true
	if !ms.writePending {
		ms.writePending = true
		ms.mu.Unlock()

		// Debounce writes to avoid excessive disk I/O (similar to current implementation)
		time.AfterFunc(100*time.Millisecond, func() {
			ms.flushToDisk()
		})
	} else {
		ms.mu.Unlock()
	}
}

// flushToDisk writes the metadata to the #stat file
func (ms *MetadataStore) flushToDisk() {
	ms.mu.Lock()
	if !ms.dirty {
		ms.writePending = false
		ms.mu.Unlock()
		return
	}
	ms.dirty = false
	ms.writePending = false
	ms.mu.Unlock()

	// Collect all metadata
	metadata := make(map[string]FileMetadata)
	ms.data.Range(func(key, value any) bool {
		metadata[key.(string)] = value.(FileMetadata)
		return true
	})

	// Write to disk outside of critical section
	go func() {
		b, err := cbor.Marshal(metadata)
		if err != nil {
			log.Println("fsa: metadata: marshal:", err)
			return
		}

		if err := fs.WriteFile(ms.opfsRoot, "#stat", b, 0755); err != nil {
			log.Println("fsa: metadata: write:", err)
		}
	}()
}

// loadFromDisk loads metadata from the #stat file
func (ms *MetadataStore) loadFromDisk() error {
	if ms.opfsRoot == nil {
		return nil // Not initialized yet
	}

	b, err := fs.ReadFile(ms.opfsRoot, "#stat")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil // No existing metadata file
		}
		return err
	}

	var metadata map[string]FileMetadata
	if err := cbor.Unmarshal(b, &metadata); err != nil {
		log.Println("fsa: metadata: unmarshal:", err)
		log.Println("fsa: metadata: resetting")
		return nil
	}

	// Load into sync.Map
	for path, meta := range metadata {
		ms.data.Store(path, meta)
	}

	return nil
}
