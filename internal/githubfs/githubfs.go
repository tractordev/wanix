package githubfs

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"tractor.dev/toolkit-go/engine/fs"
	"tractor.dev/wanix/internal/jsutil"
)

// TODO: Some file operations require a commit message. See if there's a nice way
// to expose this to the user instead of a hardcoded message.

// Given a GitHub repository and access token, this filesystem will use the
// GitHub API to expose a read-write filesystem of the repository contents.
// If not given a branch, its root will contain all branches as directories.
type FS struct {
	owner  string
	repo   string
	branch string

	token string

	tree        Tree
	treeExpired bool
}

func New(owner, repoName, branch, accessToken string) *FS {
	return &FS{owner: owner,
		repo:        repoName,
		branch:      branch,
		token:       accessToken,
		treeExpired: true,
	}
}

type Tree struct {
	Sha       string     `json:"sha"`
	URL       string     `json:"url"`
	Tree      []TreeItem `json:"tree"`
	Truncated bool       `json:"truncated"`
}
type TreeItem struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Type string `json:"type"`
	Size int64  `json:"size"`
	Sha  string `json:"sha"`
	URL  string `json:"url"`
}

func (ti *TreeItem) toFileInfo(isDirEntry bool) *fileInfo {
	// TODO: mtime?
	mode, _ := strconv.ParseUint(ti.Mode, 8, 32)
	return &fileInfo{
		name:       ti.Path,
		size:       ti.Size,
		isDir:      ti.Type == "tree",
		mode:       fs.FileMode(mode),
		isDirEntry: isDirEntry,
	}
}

func (g *FS) maybeUpdateTree(branch string) error {
	if !g.treeExpired {
		return nil
	}

	g.treeExpired = false
	defer time.AfterFunc(time.Second, func() { g.treeExpired = true })

	req, err := http.NewRequest(
		"GET",
		fmt.Sprintf(
			"https://api.github.com/repos/%s/%s/git/trees/%s?recursive=1",
			g.owner, g.repo, branch,
		),
		nil,
	)
	if err != nil {
		return err
	}
	req.Header.Add("Accept", "application/vnd.github+json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", g.token))
	req.Header.Add("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	jsutil.Log("GET tree", branch, resp.Status)
	if resp.StatusCode != 200 {
		return fmt.Errorf("BadStatus: %d", resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(&g.tree)
}

func (g *FS) Chmod(name string, mode fs.FileMode) error {
	return errors.ErrUnsupported
}

func (g *FS) Chown(name string, uid, gid int) error {
	return errors.ErrUnsupported
}

func (g *FS) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return errors.ErrUnsupported
}

func (g *FS) Create(name string) (fs.File, error) {
	panic("TODO")
}

func (g *FS) Mkdir(name string, perm fs.FileMode) error {
	panic("TODO")
}

func (g *FS) MkdirAll(path string, perm fs.FileMode) error {
	panic("TODO")
}

func (g *FS) Open(name string) (fs.File, error) {
	return g.OpenFile(name, os.O_RDONLY, 0)
}

func (g *FS) OpenFile(name string, flag int, perm fs.FileMode) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}

	// TODO: handle flags. Depending on flags we can avoid some API requests.
	// TODO: handle perm, both mode and permissions.

	// Only readonly opens implemented for now (OpenFile(name, O_RDWR, perm))

	// Request file in repo at subpath "name"
	// Decode file contents into memory buffer
	// User can read & modify buffer
	// Re-encode buffer to Base64 and make a update file (PUT) request

	fi, err := g.Stat(name)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err.(*fs.PathError).Err}
	}

	if fi.IsDir() {
		return &file{gfs: g, ReadCloser: NopReadCloser{}, FileInfo: fi}, nil
	}

	req, err := http.NewRequest(
		"GET",
		fmt.Sprintf(
			"https://api.github.com/repos/%s/%s/contents/%s",
			g.owner, g.repo, name,
		),
		nil,
	)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Accept", "application/vnd.github.raw+json") // TODO: base64 encoding
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", g.token))
	req.Header.Add("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	jsutil.Log("GET contents", name, resp.Status)
	return &file{gfs: g, ReadCloser: resp.Body, FileInfo: fi}, nil
}

func (g *FS) ReadDir(name string) ([]fs.DirEntry, error) {
	err := g.maybeUpdateTree(g.branch)
	if err != nil {
		return nil, err
	}

	var prefix string
	if name != "." {
		prefix = name
	}

	var res []fs.DirEntry
	for _, file := range g.tree.Tree {
		after, found := strings.CutPrefix(file.Path, prefix)
		after = strings.TrimLeft(after, "/")
		// Only get immediate children
		if found && after != "" && !strings.ContainsRune(after, '/') {
			jsutil.Log(after, file.Path)
			res = append(res, file.toFileInfo(true))
		}
	}

	return res, nil
}

func (g *FS) Remove(name string) error {
	panic("TODO")
}

func (g *FS) RemoveAll(path string) error {
	panic("TODO")
}

func (g *FS) Rename(oldname, newname string) error {
	panic("TODO")
}

func (g *FS) Stat(name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrInvalid}
	}

	if name == "." {
		return &fileInfo{name: name, size: 0, isDir: true}, nil
	}

	err := g.maybeUpdateTree(g.branch)
	if err != nil {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: err}
	}

	// TODO: use map[Path]TreeItem instead?
	var file *TreeItem = nil
	for i := range g.tree.Tree {
		if g.tree.Tree[i].Path == name {
			file = &g.tree.Tree[i]
		}
	}

	if file == nil {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
	}

	return file.toFileInfo(false), nil
}

type file struct {
	gfs *FS
	io.ReadCloser
	fs.FileInfo
}

func (f *file) ReadDir(n int) ([]fs.DirEntry, error) {
	if !f.IsDir() {
		return nil, syscall.ENOTDIR
	}
	return f.gfs.ReadDir(f.Name())
}

func (f *file) Stat() (fs.FileInfo, error) {
	return f.FileInfo, nil
}

// Implements the `FileInfo` and `DirEntry` interfaces
type fileInfo struct {
	name       string
	size       int64
	mode       fs.FileMode
	modTime    int64
	isDir      bool
	isDirEntry bool
}

func (i *fileInfo) Name() string {
	if i.isDirEntry {
		return filepath.Base(i.name)
	} else {
		return i.name
	}
}
func (i *fileInfo) Size() int64        { return i.size }
func (i *fileInfo) Mode() fs.FileMode  { return i.mode }
func (i *fileInfo) ModTime() time.Time { return time.Unix(i.modTime, 0) }
func (i *fileInfo) IsDir() bool        { return i.isDir }
func (i *fileInfo) Sys() any           { return nil }

// These allow it to act as DirEntry as well

func (i *fileInfo) Info() (fs.FileInfo, error) {
	fi := *i
	fi.isDirEntry = false
	return &fi, nil
}
func (i *fileInfo) Type() fs.FileMode {
	return i.Mode()
}

type NopReadCloser struct{}

func (NopReadCloser) Read(b []byte) (int, error) { return 0, nil }
func (NopReadCloser) Close() error               { return nil }
