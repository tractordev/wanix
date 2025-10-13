package httpfs

import (
	"bytes"
	"fmt"
	"io"
	"iter"
	"log"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/pstat"
)

// TODO:
// - xattrs
// - revisit ownership

type wrappedFS interface {
	fs.FS
	unwrap() *FS
}

type ApplyPatchFS interface {
	ApplyPatch(name string, tarBuf bytes.Buffer) error
}

func ApplyPatch(fsys fs.FS, name string, tarBuf bytes.Buffer) error {
	if fsys, ok := fsys.(ApplyPatchFS); ok {
		return fsys.ApplyPatch(name, tarBuf)
	}
	return fs.ErrNotSupported
}

// FS implements an HTTP-backed filesystem following the design specification
type FS struct {
	baseURL string
	client  *http.Client
	log     *slog.Logger
}

func (fsys *FS) unwrap() *FS {
	return fsys
}

// New creates a new HTTP filesystem with the given base URL
func New(baseURL string) *FS {
	return &FS{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		client:  &http.Client{},
		log:     slog.Default(), // for now
	}
}

// NewWithClient creates a new HTTP filesystem with a custom HTTP client
// func NewWithClient(baseURL string, client *http.Client) *FS {
// 	return &FS{
// 		baseURL:    strings.TrimSuffix(baseURL, "/"),
// 		client:     client,
// 		nodeCache:  make(map[string]*cacheEntry),
// 		dirTTL:     8 * time.Second,
// 		refreshing: make(map[string]bool),
// 	}
// }

// normalizeHTTPPath ensures proper HTTP path formatting
func (fsys *FS) normalizeHTTPPath(name string) string {
	// Clean the path and ensure it starts with /
	name = filepath.Clean("/" + name)
	// Convert backslashes to forward slashes for HTTP
	name = strings.ReplaceAll(name, "\\", "/")
	return name
}

// buildURL constructs the full HTTP URL for a path
func (fsys *FS) buildURL(path string) string {
	return fsys.baseURL + fsys.normalizeHTTPPath(path)
}

// doRequest logs and executes an HTTP request
func (fsys *FS) doRequest(req *http.Request) (*http.Response, error) {
	fsys.log.Debug(req.Method, "url", req.URL)
	return fsys.client.Do(req)
}

// parseDirectory parses the directory listing content into fileNode entries
func parseDirectory(fsys wrappedFS, basepath string, content []byte) ([]*Node, error) {
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")

	var entries []*Node
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}

		name := parts[0]
		modeStr := parts[1]
		mode := parseMode(modeStr)
		isDir := mode&fs.ModeDir != 0

		// Construct full path for the entry
		entryPath := basepath
		if !strings.HasSuffix(entryPath, "/") {
			entryPath += "/"
		}
		entryPath += name

		entry := &Node{
			fs:    fsys,
			name:  name,
			path:  entryPath,
			mode:  mode,
			isDir: isDir,
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// parseMode converts a Unix mode string to fs.FileMode
func parseMode(modeStr string) fs.FileMode {
	if modeStr == "" {
		return fs.FileMode(0644)
	}
	unixMode, err := strconv.ParseUint(modeStr, 10, 32)
	if err != nil {
		return fs.FileMode(0644)
	}
	return pstat.UnixModeToFileMode(uint32(unixMode))
}

// formatMode converts a Go fs.FileMode to Unix mode string
func formatMode(mode fs.FileMode) string {
	unixMode := pstat.FileModeToUnixMode(mode)
	return strconv.FormatUint(uint64(unixMode), 10)
}

// parseModTime converts a timestamp string to time.Time
func parseModTime(timeStr string) time.Time {
	if timeStr == "" {
		return time.Time{}
	}
	timestamp, err := strconv.ParseInt(timeStr, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(timestamp, 0)
}

func parseSizeFromContentRange(contentRange string) int64 {
	if contentRange == "" {
		return 0
	}
	parts := strings.Split(contentRange, "/")
	if len(parts) != 2 {
		return 0
	}
	s, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0
	}
	return s
}

// parseOwnership extracts uid and gid from ownership string
func parseOwnership(ownerStr string) (uid, gid int) {
	if ownerStr == "" {
		return 0, 0
	}
	parts := strings.Split(ownerStr, ":")
	if len(parts) != 2 {
		return 0, 0
	}
	uid, _ = strconv.Atoi(parts[0])
	gid, _ = strconv.Atoi(parts[1])
	return uid, gid
}

// parseContentDisposition extracts filename from Content-Disposition header
func parseContentDisposition(disposition string) (string, error) {
	if disposition == "" {
		return "", fmt.Errorf("empty Content-Disposition header")
	}

	_, params, err := mime.ParseMediaType(disposition)
	if err != nil {
		return "", fmt.Errorf("failed to parse Content-Disposition: %w", err)
	}

	filename := params["filename"]
	if filename == "" {
		return "", fmt.Errorf("no filename in Content-Disposition")
	}

	return filename, nil
}

// parseNodesMultipart parses a multipart response containing directory listings
func parseNodesMultipart(fsys wrappedFS, body io.Reader, boundary string) iter.Seq2[*Node, error] {
	return func(yield func(*Node, error) bool) {
		reader := multipart.NewReader(body, boundary)

		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				if !yield(nil, fmt.Errorf("failed to read multipart part: %w", err)) {
					return
				}
			}

			// Extract path from Content-Location header,
			// but also parse Content-Disposition for backward compatibility
			var path string
			disposition := part.Header.Get("Content-Disposition")
			if disposition == "" {
				path = part.Header.Get("Content-Location")
				if path == "" {
					log.Printf("httpfs: missing Content-Location header")
					part.Close()
					continue
				}
			} else {
				path, err = parseContentDisposition(disposition)
				if err != nil {
					log.Printf("httpfs: failed to parse Content-Disposition: %v", err)
					part.Close()
					continue
				}
			}

			// Read part content
			content, err := io.ReadAll(part)
			part.Close()
			if err != nil {
				log.Printf("httpfs: failed to read part content for %s: %v", path, err)
				continue
			}

			fileNode, err := ParseNode(fsys, path, http.Header(part.Header), content)
			if err != nil {
				log.Printf("httpfs: failed to parse file node for %s: %v", path, err)
				continue
			}

			if !yield(fileNode, nil) {
				return
			}
		}

	}
}
