# R2 Filesystem Storage Specification

This document describes how a POSIX-like filesystem is stored in Cloudflare R2 object storage. This specification focuses on the storage format and data structures, not the HTTP API interface.

## Overview

The R2 filesystem stores files, directories, and their metadata as objects in an R2 bucket. Each filesystem path corresponds to a single R2 object key, with metadata stored in R2's custom metadata fields and directory listings stored as object content.

## Object Keys

- **Path Mapping**: Filesystem paths map directly to R2 object keys
  - Root directory: `/` (stored as key `/`)
  - Files and directories: `/path/to/file` (stored as key `/path/to/file`)
  - No trailing slashes in keys (except root)

## Metadata Storage

All filesystem metadata is stored in R2's `customMetadata` field as key-value pairs:

### Core Metadata Fields

| Field | Description | Format | Example |
|-------|-------------|---------|---------|
| `Content-Mode` | File mode/permissions | Integer string (decimal) | `"33188"` (0644 file), `"16877"` (0755 dir) |
| `Content-Modified` | Last modified time | Seconds since Unix epoch | `"1696291200"` |
| `Content-Ownership` | Owner and group | `"user:group"` format | `"1000:1000"` |
| `Content-Type` | File type indicator | MIME type string | `"application/x-directory"` |
| `Change-Timestamp` | Change ordering | Microseconds since Unix epoch | `"1696291200000000"` |

### File Mode Values

File modes are stored as decimal integers representing the full mode value:

- **Regular Files**: `33188` (0100644) - 0100000 (file flag) + 0644 (permissions)
- **Directories**: `16877` (0040755) - 0040000 (directory flag) + 0755 (permissions)  
- **Symlinks**: `41471` (0120777) - 0120000 (symlink flag) + 0777 (permissions)

### Content-Type Values

- **Regular Files**: Detected MIME type or `"application/octet-stream"`
- **Directories**: `"application/x-directory"`
- **Symlinks**: `"application/x-symlink"`

### Default Values

When objects are created without explicit metadata:

- `Content-Mode`: `"33188"` (files), `"16877"` (directories)
- `Content-Modified`: Current timestamp
- `Content-Ownership`: `"0:0"`
- `Content-Type`: Auto-detected or defaults above

## File Storage

### Regular Files

- **Object Key**: The file path (e.g., `/home/user/document.txt`)
- **Object Body**: The file content as binary data
- **Metadata**: Standard filesystem metadata in `customMetadata`

### Symlinks

- **Object Key**: The symlink path (e.g., `/home/user/link`)
- **Object Body**: The symlink target path as UTF-8 text
- **Metadata**: 
  - `Content-Mode`: Symlink mode (e.g., `"41471"`)
  - `Content-Type`: `"application/x-symlink"`
  - Other standard metadata

## Directory Storage

### Directory Objects

- **Object Key**: The directory path (e.g., `/home/user`)
- **Object Body**: Directory listing in text format (see below)
- **Metadata**:
  - `Content-Mode`: Directory mode (e.g., `"16877"`)
  - `Content-Type`: `"application/x-directory"`
  - Other standard metadata

### Directory Listing Format

Directory contents are stored as newline-delimited text in the directory object's body:

```
filename1 mode1
filename2 mode2
filename3 mode3
```

**Format Rules**:
- Each line: `"<name> <mode>\n"`
- Names are sorted lexicographically
- Modes are decimal integers matching the child's `Content-Mode`
- Empty directories have empty content (or single newline)

**Example**:
```
.bashrc 33188
.profile 33188
Documents 16877
bin 16877
```

### Root Directory

The root directory (`/`) is stored as a special case:
- **Object Key**: `/`
- **Content**: Directory listing of root-level entries
- **Metadata**: Standard directory metadata

## Atomic Operations

### Compare-and-Swap

All modifications use R2's conditional operations to ensure atomicity:

1. **Read**: Get current object with ETag
2. **Modify**: Apply changes to metadata/content
3. **Write**: Put with `If-Match: <etag>` header
4. **Retry**: On 412 Precondition Failed, retry from step 1

### Directory Consistency

When filesystem structure changes, multiple objects are updated atomically:

1. **File Creation**: Update parent directory listing + create file object
2. **File Deletion**: Delete file object + update parent directory listing  
3. **Move/Rename**: Update source parent + destination parent + move object
4. **Directory Operations**: Recursively handle all children

## Change Ordering

### Change Timestamps

The `Change-Timestamp` field provides ordering for concurrent modifications:

- **Format**: Microseconds since Unix epoch as decimal string
- **Usage**: Higher timestamps take precedence during conflicts
- **Generation**: Set by client to current time in microseconds

### Conflict Resolution

When multiple clients modify the same object:

1. Compare `Change-Timestamp` values
2. Apply change with higher timestamp
3. Preserve metadata from higher timestamp
4. Use compare-and-swap to prevent lost updates

## Storage Efficiency

### Object Count

- Each file/directory = 1 R2 object
- Directory listings stored as object content (not separate objects)
- No additional metadata objects required

### Size Considerations

- Small files: Minimum R2 object overhead
- Large files: Stored directly as single objects
- Directory listings: Minimal text overhead
- Metadata: Stored in R2's built-in metadata fields

## Implementation Notes

### Path Handling

- Paths are normalized (no trailing slashes except root)
- Case-sensitive path matching
- UTF-8 encoding for all paths and content

### Concurrency

- Multiple readers: Fully concurrent
- Multiple writers: Serialized via compare-and-swap
- Directory updates: Atomic via conditional puts

### Error Handling

- Missing parent directories: Operations fail
- Concurrent modifications: Automatic retry with exponential backoff
- Invalid metadata: Operations fail with validation errors

## Limitations

### R2 Constraints

- Object key length: 1024 bytes maximum
- Metadata size: 2KB per object maximum  
- No native directory operations (simulated via listings)

### Filesystem Features

- **Supported**: Files, directories, symlinks, permissions, ownership, timestamps
- **Not Supported**: Hard links, special files (devices, FIFOs), extended attributes
- **Partial Support**: Atomic operations (via compare-and-swap)

## Example Storage Layout

For filesystem structure:
```
/
├── etc/
│   ├── passwd
│   └── hosts
└── home/
    └── user/
        ├── .bashrc -> /etc/skel/.bashrc
        └── documents/
            └── readme.txt
```

R2 objects created:
```
Key: "/"
Content-Type: "application/x-directory"
Content-Mode: "16877"
Body: "etc 16877\nhome 16877\n"

Key: "/etc"  
Content-Type: "application/x-directory"
Content-Mode: "16877"
Body: "hosts 33188\npasswd 33188\n"

Key: "/etc/passwd"
Content-Type: "text/plain"
Content-Mode: "33188"
Body: <passwd file content>

Key: "/etc/hosts"
Content-Type: "text/plain"  
Content-Mode: "33188"
Body: <hosts file content>

Key: "/home"
Content-Type: "application/x-directory"
Content-Mode: "16877"
Body: "user 16877\n"

Key: "/home/user"
Content-Type: "application/x-directory"
Content-Mode: "16877"  
Body: ".bashrc 41471\ndocuments 16877\n"

Key: "/home/user/.bashrc"
Content-Type: "application/x-symlink"
Content-Mode: "41471"
Body: "/etc/skel/.bashrc"

Key: "/home/user/documents"
Content-Type: "application/x-directory"
Content-Mode: "16877"
Body: "readme.txt 33188\n"

Key: "/home/user/documents/readme.txt"
Content-Type: "text/plain"
Content-Mode: "33188"  
Body: <readme content>
```
