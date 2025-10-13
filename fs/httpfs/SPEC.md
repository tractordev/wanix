# HTTP Filesystem Protocol Specification

## Overview

The HTTP Filesystem Protocol provides a RESTful interface for performing POSIX-like filesystem operations over HTTP. It enables hierarchical file and directory manipulation using standard HTTP methods with filesystem-specific metadata encoded in HTTP headers.

## Protocol Design

### Core Principles

- **RESTful Interface**: HTTP methods map directly to filesystem operations
- **Metadata in Headers**: File attributes encoded as HTTP headers
- **Path-based URLs**: Filesystem paths map directly to URL paths
- **Directory Listings**: Directory contents encoded as plain text
- **Atomic Operations**: Individual operations are atomic

### URL Structure

Filesystem paths map directly to HTTP URLs:
- Base URL: `https://example.com/fs`
- File path: `/path/to/file.txt` → `https://example.com/fs/path/to/file.txt`
- Directory path: `/path/to/dir` → `https://example.com/fs/path/to/dir`

**Note**: Directory paths MAY end with `/` as a convenience to automatically set `Content-Type: application/x-directory`.

## HTTP Methods

### GET - Read File/Directory

Retrieves file content or directory listing.

**Request:**
```http
GET /path/to/file.txt HTTP/1.1
```

**Response (File):**
```http
HTTP/1.1 200 OK
Content-Type: application/octet-stream
Content-Length: 1024
Content-Mode: 33188
Content-Modified: 1641024000
Content-Ownership: 1000:1000

[file content]
```

**Response (Directory):**
```http
HTTP/1.1 200 OK
Content-Type: application/x-directory
Content-Length: 45
Content-Mode: 16877
Content-Modified: 1641024000
Content-Ownership: 1000:1000

file.txt 33188
subdir 16877
```

### HEAD - Get Metadata Only

Retrieves file/directory metadata without content.

**Request:**
```http
HEAD /path/to/file.txt HTTP/1.1
```

**Response:**
```http
HTTP/1.1 200 OK
Content-Type: application/octet-stream
Content-Length: 1024
Content-Mode: 33188
Content-Modified: 1641024000
Content-Ownership: 1000:1000
```

### PUT - Create/Replace File/Directory

Creates or completely replaces a file or directory. For new files/directories, default metadata is applied if not provided. For existing files/directories, metadata behavior depends on the Change-Timestamp header and server implementation.

**Request (File):**
```http
PUT /path/to/file.txt HTTP/1.1
Content-Type: application/octet-stream
Content-Length: 12
Content-Mode: 33188
Content-Modified: 1641024000
Content-Ownership: 1000:1000

Hello World!
```

**Request (Directory):**
```http
PUT /path/to/dir HTTP/1.1
Content-Type: application/x-directory
Content-Length: 0
Content-Mode: 16877
Content-Modified: 1641024000
Content-Ownership: 1000:1000

```

**Alternative (using trailing slash convenience):**
```http
PUT /path/to/dir/ HTTP/1.1
Content-Length: 0
Content-Mode: 16877
Content-Modified: 1641024000
Content-Ownership: 1000:1000

```

**Response:**
```http
HTTP/1.1 200 OK

OK
```

### PATCH - Update Metadata Only

Updates file/directory metadata without changing content.

**Request:**
```http
PATCH /path/to/file.txt HTTP/1.1
Content-Mode: 33261
```

**Response:**
```http
HTTP/1.1 200 OK

OK
```

### DELETE - Remove File/Directory

Removes a file or directory.

**Request:**
```http
DELETE /path/to/file.txt HTTP/1.1
```

**Response:**
```http
HTTP/1.1 200 OK

OK
```

### MOVE - Move/Rename File/Directory

Moves or renames a file or directory to a new location. For directories, the operation is recursive.

**Request:**
```http
MOVE /path/to/source.txt HTTP/1.1
Destination: /path/to/destination.txt
Overwrite: T
```

**Response:**
```http
HTTP/1.1 200 OK

OK
```

**Behavior:**
- Source file/directory is moved to the destination path
- For directories, all contents are recursively moved
- Parent directory listings are updated for both source and destination
- Source is deleted after successful move
- If destination exists and `Overwrite: F`, returns `412 Precondition Failed`

### COPY - Copy File/Directory

Copies a file or directory to a new location. For directories, the operation is recursive.

**Request:**
```http
COPY /path/to/source.txt HTTP/1.1
Destination: /path/to/copy.txt
Overwrite: T
```

**Response:**
```http
HTTP/1.1 200 OK

OK
```

**Behavior:**
- Source file/directory is copied to the destination path
- For directories, all contents are recursively copied
- Parent directory listing is updated for the destination
- Source remains unchanged
- If destination exists and `Overwrite: F`, returns `412 Precondition Failed`


## Protocol Extensions

### Multipart Directory Streaming

The HTTP Filesystem Protocol supports enhanced directory operations through special URL path suffixes that return multipart responses containing detailed metadata for multiple files and directories.

#### Directory Metadata Streaming

**Request:**
```http
GET /path/to/directory/: HTTP/1.1
```

**Response:**
```http
HTTP/1.1 200 OK
Content-Type: multipart/mixed; boundary=----boundary123

------boundary123
Content-Disposition: attachment; filename="/path/to/directory"
Content-Type: application/x-directory
Content-Mode: 16877
Content-Modified: 1641024000
Content-Ownership: 1000:1000
Content-Length: 30

file.txt 33188
subdir 16877

------boundary123
Content-Disposition: attachment; filename="/path/to/directory/file.txt"
Content-Type: application/octet-stream
Content-Mode: 33188
Content-Modified: 1641024000
Content-Ownership: 1000:1000
Content-Range: bytes 0-0/1024

------boundary123
Content-Disposition: attachment; filename="/path/to/directory/subdir"
Content-Type: application/x-directory
Content-Mode: 16877
Content-Modified: 1641024000
Content-Ownership: 1000:1000
Content-Length: 14

file2.txt 33188

------boundary123--
```

**Behavior:**
- The `:` suffix triggers multipart metadata streaming for a directory
- Returns full metadata for the directory itself and all direct children
- Each part contains complete headers for one file/directory
- Content-Disposition header contains the full path
- **Directory parts include their body** (directory listing content) with Content-Length
- **File parts have no body** but include size via Content-Range header
- Content-Range uses `bytes 0-0/SIZE` format to convey file size without content
- Enables efficient bulk metadata retrieval without transferring file contents

#### Recursive Directory Tree Streaming

**Request:**
```http
GET /path/to/directory/... HTTP/1.1
```

**Response:**
```http
HTTP/1.1 200 OK
Content-Type: multipart/mixed; boundary=----boundary456

------boundary456
Content-Disposition: attachment; filename="/path/to/directory"
Content-Type: application/x-directory
Content-Mode: 16877
Content-Modified: 1641024000
Content-Length: 30

file.txt 33188
subdir 16877

------boundary456
Content-Disposition: attachment; filename="/path/to/directory/subdir"
Content-Type: application/x-directory
Content-Mode: 16877
Content-Modified: 1641024000
Content-Length: 14

nested.txt 33188

------boundary456
Content-Disposition: attachment; filename="/path/to/directory/subdir/nested"
Content-Type: application/x-directory
Content-Mode: 16877
Content-Modified: 1641024000
Content-Length: 0



------boundary456--
```

**Behavior:**
- The `...` suffix triggers recursive multipart metadata streaming
- Returns metadata for the directory and all descendant **directories only** (files not included)
- **All directory parts include their body** (directory listing content) with Content-Length
- Directory listings contain both files and subdirectories, but only directories appear as separate parts
- Enables efficient bulk discovery of entire directory trees
- Useful for synchronization and caching scenarios where you need the complete directory structure

### Caching and Performance

Useful to cache "nodes" of metadata for files and directories plus 
directory contents. Avoid caching file contents. Use the bulk multipart
operations to fill and update a node cache. Highly recommend a refresh-ahead
caching strategy.

## Filesystem Metadata Headers

### Content-Mode
- **Purpose**: Unix file mode (permissions + type)
- **Format**: Decimal string representation of Unix mode
- **Required**: No (defaults applied)
- **Examples**:
  - `33188` - Regular file with 0644 permissions
  - `16877` - Directory with 0755 permissions
  - `33261` - Executable file with 0755 permissions

### Content-Modified
- **Purpose**: Last modification timestamp
- **Format**: Unix timestamp (seconds since epoch) as decimal string
- **Required**: No (current time used if omitted)
- **Example**: `1641024000`

### Content-Ownership
- **Purpose**: File owner and group
- **Format**: `uid:gid` format
- **Required**: No (defaults to `0:0`)
- **Example**: `1000:1000`

### Content-Type
- **Purpose**: MIME type indicator
- **Values**:
  - `application/x-directory` - Directory
  - `application/x-symlink` - Symbolic link
  - `application/octet-stream` - Binary file (default)
  - Other standard MIME types as appropriate
- **Required**: No (auto-detected from path and content)

### Content-Length
- **Purpose**: Size of content in bytes
- **Format**: Decimal string
- **Required**: Yes for PUT requests
- **Behavior**: Standard HTTP header

### Content-Range
- **Purpose**: Conveys file size in multipart responses without transferring content
- **Format**: `bytes 0-0/SIZE` where SIZE is the file size in bytes
- **Required**: Used in multipart directory streaming responses for file entries
- **Example**: `Content-Range: bytes 0-0/1024` (indicates 1024 byte file)
- **Behavior**: In multipart directory streaming (`:` and `...` suffixes), file entries use Content-Range instead of Content-Length to provide size metadata without including file content in the response body

### Change-Timestamp
- **Purpose**: Operation ordering timestamp to prevent lost updates
- **Format**: Microseconds since Unix epoch as decimal string
- **Required**: No (but recommended for conflict resolution)
- **Example**: `1641024000123456`
- **Behavior**: Used for compare-and-swap operations to ensure changes are applied in correct order

### Destination
- **Purpose**: Target path for MOVE and COPY operations
- **Format**: Absolute path starting with `/`
- **Required**: Yes for MOVE and COPY requests
- **Example**: `/path/to/destination.txt`
- **Behavior**: Specifies where the source should be moved or copied to

### Overwrite
- **Purpose**: Controls whether MOVE and COPY operations can overwrite existing files
- **Format**: `T` (true) or `F` (false)
- **Required**: No (defaults to `T`)
- **Example**: `F`
- **Behavior**: When `F`, operations fail with `412 Precondition Failed` if destination exists

## Directory Listing Format

Directory contents are encoded as plain text with the format:

```
filename mode
dirname mode
```

**Characteristics:**
- One entry per line
- Space-separated name and mode
- Lexicographically sorted
- Unix mode in decimal format
- Terminated with newline

**Example:**
```
.hidden 33188
README.md 33188
bin 16877
src 16877
```

## File vs Directory Detection

1. **Primary**: Content-Type header (`application/x-directory`)
2. **Secondary**: Mode value (directory flag in Unix mode)
3. **Convenience**: Path ending with `/` automatically sets directory content-type

## Symbolic Links

Symbolic links are supported through special handling:

### Creating Symlinks
```http
PUT /path/to/symlink HTTP/1.1
Content-Type: application/x-symlink
Content-Mode: 120777
Content-Length: 11

/target/path
```

### Reading Symlinks
```http
GET /path/to/symlink HTTP/1.1

# Response:
HTTP/1.1 200 OK
Content-Type: application/x-symlink
Content-Mode: 120777
Content-Length: 11

/target/path
```

### Symlink Characteristics
- **Content**: The symlink target path
- **Content-Type**: Must be `application/x-symlink`
- **Content-Mode**: Unix mode with symlink flag (typically `120777` for 0777 permissions + symlink flag)
- **Behavior**: Content body contains the target path as plain text

## Error Responses

### HTTP Status Codes

- `200 OK` - Operation successful
- `404 Not Found` - File/directory does not exist
- `405 Method Not Allowed` - HTTP method not supported
- `412 Precondition Failed` - Conditional request failed

### Error Bodies

Error responses include plain text descriptions:

```http
HTTP/1.1 404 Not Found

Object Not Found
```

```http
HTTP/1.1 405 Method Not Allowed
Allow: GET, HEAD, PUT, PATCH, DELETE, MOVE, COPY

Method Not Allowed
```

## Operation Semantics

### File Operations

**Reading Files:**
- GET retrieves content with metadata in headers
- HEAD retrieves only metadata
- Returns 404 if file doesn't exist

**Writing Files:**
- PUT creates/replaces entire file
- For new files: Default metadata is applied if not provided (mode 0644, ownership 0:0, current timestamp)
- Content-Length header required

**Modifying Files:**
- PATCH updates metadata only, content unchanged
- Only provided headers are updated
- Existing metadata preserved if not specified

### Directory Operations

**Reading Directories:**
- GET returns directory listing as plain text
- Content-Type is `application/x-directory`
- Entries sorted lexicographically

**Creating Directories:**
- PUT with `Content-Type: application/x-directory` header
- Empty or minimal content body
- Alternatively, PUT with path ending in `/` automatically sets directory content-type

**Directory Maintenance:**
- Server automatically maintains parent directory listings
- Adding/removing files updates parent directory
- PATCH with Content-Mode updates entry in parent listing

## Protocol Extensions

### Conditional Requests
Standard HTTP conditional headers supported:
- `If-Match` / `If-None-Match`
- `If-Modified-Since` / `If-Unmodified-Since`

### Range Requests
Standard HTTP range requests supported for partial file reads:
- `Range: bytes=0-1023`

## Security Considerations

### Authentication & Authorization
- Protocol is transport-agnostic regarding authentication
- Implementations should use standard HTTP authentication
- Access control is implementation-specific

### Path Security
- Implementations should validate paths to prevent directory traversal
- Relative path components (`.`, `..`) require careful handling
- Path injection attacks should be prevented

## Implementation Guidelines

### Server Requirements
- MUST support GET, HEAD, PUT, PATCH, DELETE, MOVE, COPY methods
- MUST detect directories via `Content-Type: application/x-directory`
- MUST support symlinks via `Content-Type: application/x-symlink`
- SHOULD treat paths ending with `/` as convenience for setting directory content-type
- MUST maintain directory listings automatically
- MUST support Destination header for MOVE and COPY operations
- SHOULD support Overwrite header for MOVE and COPY operations
- SHOULD support Change-Timestamp header for operation ordering
- SHOULD support conditional requests
- SHOULD validate metadata format
- SHOULD support multipart directory streaming with `:` suffix
- SHOULD support recursive multipart streaming with `...` suffix
- MAY implement caching with stale-while-revalidate pattern
- MAY send Cache-Control-TTL headers for cache guidance

### Client Requirements
- MUST set `Content-Type: application/x-directory` for directory operations
- MUST set `Content-Type: application/x-symlink` for symlink operations
- MAY use trailing `/` on directory paths as convenience
- MUST send required headers on PUT operations
- MUST send Destination header for MOVE and COPY operations
- MAY send Overwrite header for MOVE and COPY operations
- SHOULD send Change-Timestamp header for conflict resolution
- MUST parse directory listing format correctly
- SHOULD handle standard HTTP error responses
- SHOULD support conditional requests
- MAY use `:` suffix for multipart directory metadata streaming
- MAY use `...` suffix for recursive multipart tree streaming
- SHOULD parse multipart responses when using streaming extensions
- MAY implement caching with respect to Cache-Control-TTL headers

### Interoperability
- Metadata header names are case-insensitive (per HTTP)
- Directory listing format is strict (space-separated, sorted)
- Unix mode values are decimal integers
- Timestamps are Unix epoch seconds

## Example Workflows

### Creating a File
```http
PUT /documents/readme.txt HTTP/1.1
Content-Type: text/plain
Content-Length: 13
Content-Mode: 33188
Content-Ownership: 1000:1000

Hello, World!
```

### Reading a Directory
```http
GET /documents HTTP/1.1

# Response:
readme.txt 33188
scripts 16877
```

### Changing File Permissions
```http
PATCH /documents/script.sh HTTP/1.1
Content-Mode: 33261
```

### Checking File Existence
```http
HEAD /documents/config.json HTTP/1.1

# 200 = exists, 404 = doesn't exist
```

### Bulk Directory Metadata Retrieval
```http
GET /documents/: HTTP/1.1

# Returns multipart response with metadata for all files in /documents/
```

### Recursive Directory Tree Discovery
```http
GET /projects/... HTTP/1.1

# Returns multipart response with metadata for entire /projects/ tree
```

## Future Considerations

### Planned Extensions
- Extended attributes (xattrs) support
- File locking mechanisms
- Bulk operations
- Directory watching/notifications

### Current Limitations
- No atomic multi-file operations
- Access time not currently supported
- No built-in versioning or conflict resolution
