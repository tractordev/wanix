# CacheFS

CacheFS is a filesystem implementation that provides read-through caching with write-back functionality. It combines a local filesystem (used as cache) with a remote filesystem (source of truth) to provide fast access to frequently used files while maintaining data consistency.

## Features

- **Read-through caching**: Files are served from local cache when available and valid, otherwise fetched from remote
- **Write-back caching**: Write operations are applied to local filesystem immediately and to remote filesystem asynchronously
- **Configurable TTL**: Cache entries expire after a configurable time-to-live duration
- **Error logging**: Async write errors are logged to a configurable logger
- **Full filesystem interface**: Supports all standard filesystem operations including metadata operations

## Architecture

```
┌─────────────────┐    ┌─────────────────┐
│   Application   │    │   Application   │
└─────────┬───────┘    └─────────┬───────┘
          │ Read                 │ Write
          ▼                      ▼
┌─────────────────────────────────────────┐
│              CacheFS                    │
├─────────────────┬───────────────────────┤
│  Read-through   │    Write-back         │
│     Cache       │      Cache            │
└─────────┬───────┴───────────┬───────────┘
          │                   │
          ▼                   ▼
┌─────────────────┐    ┌─────────────────┐
│  Local FS       │    │   Remote FS     │
│   (Cache)       │    │ (Source of      │
│                 │    │  Truth)         │
└─────────────────┘    └─────────────────┘
```

## Usage

```go
package main

import (
    "log"
    "time"
    
    "tractor.dev/wanix/fs/cachefs"
    "tractor.dev/wanix/fs/fskit"
    "tractor.dev/wanix/fs/httpfs"
)

func main() {
    // Create local filesystem for caching
    local := fskit.MemFS{}
    
    // Create remote filesystem (e.g., HTTP-based)
    remote := httpfs.New("https://example.com/api/files")
    
    // Create cache filesystem with 1-hour TTL
    logger := log.Default()
    cfs := cachefs.New(local, remote, time.Hour, logger)
    defer cfs.Close()
    
    // Use like any other filesystem
    file, err := cfs.Open("document.txt")
    if err != nil {
        log.Fatal(err)
    }
    defer file.Close()
    
    // Read operations use cache when possible
    content, err := io.ReadAll(file)
    if err != nil {
        log.Fatal(err)
    }
    
    // Write operations go to both local and remote
    err = fs.WriteFile(cfs, "newfile.txt", []byte("content"), 0644)
    if err != nil {
        log.Fatal(err)
    }
    
    // Wait for all async operations to complete before exiting
    cfs.Wait()
}
```

## Supported Operations

CacheFS implements the following filesystem interfaces:

### Core Operations
- `Open(name)` / `OpenContext(ctx, name)` - Read-through cached file access
- `Create(name)` / `CreateContext(ctx, name)` - Create files with write-back
- `Stat(name)` / `StatContext(ctx, name)` - File metadata with caching

### Directory Operations
- `Mkdir(name, perm)` - Create directories
- `Remove(name)` - Remove files and directories

### Metadata Operations
- `Chmod(name, mode)` - Change file permissions
- `Chown(name, uid, gid)` - Change file ownership
- `Chtimes(name, atime, mtime)` - Change file timestamps

### Advanced Operations
- `Rename(oldname, newname)` - Move/rename files
- `Symlink(oldname, newname)` - Create symbolic links
- `Readlink(name)` - Read symbolic link targets

## Caching Behavior

### Read Operations
1. Check if file is cached locally and cache entry is valid (not expired)
2. If cached and valid, serve from local filesystem
3. If not cached or expired, fetch from remote filesystem
4. Store fetched content in local filesystem for future reads
5. Update cache metadata with expiration time

### Write Operations
1. Apply write operation to local filesystem immediately (synchronous)
2. Invalidate cache entry for the modified path
3. Queue async write operation to remote filesystem
4. Log any errors from remote write operations

### Cache Invalidation
Cache entries are invalidated in the following scenarios:
- File is modified through CacheFS (write, chmod, etc.)
- Cache entry expires based on TTL
- Manual invalidation via cache methods

## Error Handling

- Local filesystem errors are returned immediately to the caller
- Remote filesystem errors during async operations are logged but don't affect the operation's success
- A dedicated error logging goroutine processes async write errors
- Errors are logged with context about the failed operation

## Testing

The package includes comprehensive tests covering:
- Read-through caching behavior
- Write-back functionality
- Directory operations
- Metadata operations
- Cache invalidation
- Error handling with async operations
- Cache TTL expiration

Run tests with:
```bash
go test ./fs/cachefs -v
```

## Thread Safety

CacheFS is thread-safe:
- The underlying readthru.Cache uses mutexes for metadata protection
- Async write operations are properly synchronized with WaitGroups
- Error logging uses buffered channels to prevent blocking

## Dependencies

- `tractor.dev/wanix/fs` - Core filesystem interfaces
- `tractor.dev/wanix/internal/readthru` - Read-through caching logic
- `tractor.dev/wanix/fs/fskit` - Testing utilities (MemFS)
