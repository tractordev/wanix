package githubfs

import (
	"bytes"
	"encoding/base64"
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

// TODO: Write requests require a commit message. See if there's a nice way
// to expose this to the user instead of a hardcoded message.

// TODO: Write requests can fail if requests are sent in parallel or too close together.
// Automatically stagger write requests to avoid this.

// Given a GitHub repository and access token, this filesystem will use the
// GitHub API to expose a read-write filesystem of the repository contents.
// Its root will contain all branches as directories.
type FS struct {
	owner string
	repo  string
	token string

	branches        map[string]Tree
	branchesExpired bool
}

func New(owner, repoName, accessToken string) *FS {
	return &FS{
		owner:           owner,
		repo:            repoName,
		token:           accessToken,
		branches:        make(map[string]Tree),
		branchesExpired: true,
	}
}

type Tree struct {
	Expired bool `json:"-"`

	Sha       string     `json:"sha"`
	URL       string     `json:"url"`
	Items     []TreeItem `json:"tree"` // TODO: use map[Path]TreeItem instead?
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

func (ti *TreeItem) toFileInfo(branch string) *fileInfo {
	// TODO: mtime?
	mode, _ := strconv.ParseUint(ti.Mode, 8, 32)
	return &fileInfo{
		name:    filepath.Base(ti.Path),
		size:    ti.Size,
		isDir:   ti.Type == "tree",
		mode:    fs.FileMode(mode),
		branch:  branch,
		subpath: ti.Path,
		sha:     ti.Sha,
	}
}

type ErrBadStatus struct {
	status string
}

func (e ErrBadStatus) Error() string {
	return "BadStatus: " + e.status
}

func (g *FS) apiRequest(method, url, acceptHeader string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Accept", acceptHeader)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", g.token))
	req.Header.Add("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	jsutil.Log(method, url, resp.Status)
	if resp.StatusCode == 401 {
		return resp, fs.ErrPermission
	}

	return resp, nil
}

// Every filesystem query is prefixed by a branch name, so `maybeUpdateBranches()`
// must be called for every query before accessing it's Tree. `maybeUpdateTree()`
// is only necessary when accessing Tree contents.

// Both in seconds.
// Optimize for least amount of Requests without visible loss of sync with remote.
const branchesExpiryPeriod = 5
const treeExpiryPeriod = 1

func (g *FS) maybeUpdateBranches() error {
	if !g.branchesExpired {
		return nil
	}

	g.branchesExpired = false
	defer time.AfterFunc(branchesExpiryPeriod*time.Second, func() { g.branchesExpired = true })

	resp, err := g.apiRequest(
		"GET",
		fmt.Sprintf(
			"https://api.github.com/repos/%s/%s/branches",
			g.owner, g.repo,
		),
		"application/vnd.github+json",
		nil,
	)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return ErrBadStatus{status: resp.Status}
	}
	defer resp.Body.Close()

	var branches []struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&branches); err != nil {
		return err
	}

	// TODO: apply diff instead of clearing the whole thing?
	clear(g.branches)
	for _, branch := range branches {
		g.branches[branch.Name] = Tree{Expired: true}
	}
	return nil
}

func (g *FS) maybeUpdateTree(branch string) error {
	existingTree, ok := g.branches[branch]
	if !ok {
		return fs.ErrNotExist
	}

	if !existingTree.Expired {
		return nil
	}

	existingTree.Expired = false
	defer time.AfterFunc(treeExpiryPeriod*time.Second, func() { existingTree.Expired = true })

	resp, err := g.apiRequest(
		"GET",
		fmt.Sprintf(
			"https://api.github.com/repos/%s/%s/git/trees/%s?recursive=1",
			g.owner, g.repo, branch,
		),
		"application/vnd.github+json",
		nil,
	)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return ErrBadStatus{status: resp.Status}
	}
	defer resp.Body.Close()

	var newTree Tree
	if err = json.NewDecoder(resp.Body).Decode(&newTree); err != nil {
		return err
	}

	g.branches[branch] = newTree
	return nil
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
	return g.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
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

	// TODO: handle perm, both mode and permissions.

	// Request file in repo at subpath "name"
	// Read file contents into memory buffer
	// User can read & modify buffer
	// Make a update file (PUT) request

	f := file{gfs: g, flags: flag}
	branch, subpath, hasSubpath := strings.Cut(name, "/")
	justCreated := false

	{
		fi, err := g.Stat(name)
		if err == nil {
			if flag&(os.O_EXCL|os.O_CREATE) == (os.O_EXCL | os.O_CREATE) {
				return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrExist}
			}

			f.fileInfo = *fi.(*fileInfo)

			if fi.IsDir() || !hasSubpath {
				return &f, nil
			}
		}

		if err != nil {
			if errors.Is(err, fs.ErrNotExist) && flag&os.O_CREATE > 0 {
				// Defer creation on remote to avoid request conflicts. (See Sync)
				f.buffer = []byte{}
				f.dirty = true
				f.fileInfo = fileInfo{
					name:    filepath.Base(name),
					mode:    perm,
					modTime: time.Now().UnixMilli(),
					branch:  branch,
					subpath: subpath,
				}

				justCreated = true
			} else {
				return nil, &fs.PathError{Op: "open", Path: name, Err: err.(*fs.PathError).Err}
			}
		}
	}

	if flag&os.O_TRUNC > 0 {
		if !justCreated {
			f.buffer = []byte{}
		}
		f.offset = 0
		return &f, nil
	}

	if !justCreated {
		resp, err := g.apiRequest(
			"GET",
			fmt.Sprintf(
				"https://api.github.com/repos/%s/%s/contents/%s?ref=%s",
				g.owner, g.repo, subpath, branch,
			),
			"application/vnd.github.raw+json",
			nil,
		)
		if err != nil {
			return nil, &fs.PathError{Op: "open", Path: name, Err: err}
		}
		if resp.StatusCode != 200 {
			return nil, &fs.PathError{Op: "open", Path: name, Err: ErrBadStatus{status: resp.Status}}
		}
		defer resp.Body.Close()

		f.buffer, err = io.ReadAll(resp.Body)
		f.fileInfo.size = resp.ContentLength
		if err != nil {
			return nil, &fs.PathError{Op: "open", Path: name, Err: err}
		}
	}

	if flag&os.O_APPEND > 0 {
		f.Seek(0, io.SeekEnd)
	}

	return &f, nil
}

func (g *FS) Remove(name string) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrInvalid}
	}

	fi, err := g.Stat(name)
	if err != nil {
		return &fs.PathError{Op: "remove", Path: name, Err: err.(*fs.PathError).Err}
	}

	if fi.IsDir() {
		// Use RemoveAll instead
		return &fs.PathError{Op: "remove", Path: name, Err: errors.ErrUnsupported}
	}

	fInfo := fi.(*fileInfo)

	resp, err := g.apiRequest(
		"DELETE",
		fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s",
			g.owner, g.repo, fInfo.subpath,
		),
		"application/vnd.github+json",
		bytes.NewBufferString(
			fmt.Sprintf(
				`{"message":"Remove '%s'","branch":"%s","sha":"%s"}`,
				fInfo.subpath, fInfo.branch, fInfo.sha,
			),
		),
	)
	if err != nil {
		return &fs.PathError{Op: "remove", Path: name, Err: err}
	}
	resp.Body.Close()

	if resp.StatusCode != 200 {
		return &fs.PathError{Op: "remove", Path: name, Err: ErrBadStatus{status: resp.Status}}
	}

	// why can't I just update a map value's fields...
	tree := g.branches[fInfo.branch]
	tree.Expired = true
	g.branches[fInfo.branch] = tree

	return nil
}

func (g *FS) RemoveAll(path string) error {
	// TODO
	return g.Remove(path)
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

	branch, subpath, hasSubpath := strings.Cut(name, "/")
	if err := g.maybeUpdateBranches(); err != nil {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: err}
	}
	if !hasSubpath {
		if _, ok := g.branches[branch]; !ok {
			return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist} // TODO: add "BranchNotExist" error
		}
		return &fileInfo{name: name, size: 0, isDir: true, branch: branch}, nil
	}
	if err := g.maybeUpdateTree(branch); err != nil {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: err}
	}

	tree, ok := g.branches[branch]
	if !ok {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist} // TODO: add "BranchNotExist" error
	}
	var item *TreeItem = nil
	for i := 0; i < len(tree.Items); i++ {
		if tree.Items[i].Path == subpath {
			item = &tree.Items[i]
			break
		}
	}

	if item == nil {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
	}

	return item.toFileInfo(branch), nil
}

type file struct {
	gfs *FS

	buffer []byte
	offset int64
	dirty  bool

	flags int
	fileInfo
}

func (f *file) Read(b []byte) (int, error) {
	if f.flags&os.O_WRONLY > 0 {
		return 0, fs.ErrPermission
	}

	if f.offset >= int64(len(f.buffer)) {
		return 0, io.EOF
	}

	var n int
	rest := f.buffer[f.offset:]
	if len(rest) < len(b) {
		n = len(rest)
	} else {
		n = len(b)
	}

	copy(b, rest[:n])
	f.offset += int64(n)
	return n, nil
}

func (f *file) Write(b []byte) (int, error) {
	if f.flags&os.O_RDONLY > 0 {
		return 0, fs.ErrPermission
	}

	writeEnd := f.offset + int64(len(b))

	if writeEnd > int64(cap(f.buffer)) {
		var newCapacity int64
		if cap(f.buffer) == 0 {
			newCapacity = 8
		} else {
			newCapacity = int64(cap(f.buffer)) * 2
		}

		for ; writeEnd > newCapacity; newCapacity *= 2 {
		}

		newBuffer := make([]byte, len(f.buffer), newCapacity)
		copy(newBuffer, f.buffer)
		f.buffer = newBuffer
	}

	copy(f.buffer[f.offset:writeEnd], b)
	if len(f.buffer) < int(writeEnd) {
		f.buffer = f.buffer[:writeEnd]
	}
	f.offset = writeEnd
	f.dirty = true
	return len(b), nil
}

func (f *file) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		f.offset = offset
	case io.SeekCurrent:
		f.offset += offset
	case io.SeekEnd:
		f.offset = int64(len(f.buffer)) + offset
	}
	if f.offset < 0 {
		f.offset = 0
		return 0, fmt.Errorf("%w: resultant offset cannot be negative", fs.ErrInvalid)
	}
	return f.offset, nil
}

func (f *file) Sync() error {
	if !f.dirty {
		return nil
	}

	var encodedContent string
	if len(f.buffer) > 0 {
		encodedContent = base64.StdEncoding.EncodeToString(f.buffer)
	}

	const createBody = `{"message":"Create '%s'","branch":"%s","content":"%s"}`
	const updateBody = `{"message":"Update '%s'","branch":"%s","content":"%s","sha":"%s"}`

	// If f.sha == "" then we must've just created the file locally,
	// so we want to create it on the remote too. Otherwise update the remote file.
	// Deferring creation like this avoids 409 Conflict errors.
	var body *bytes.Buffer
	if f.sha == "" {
		body = bytes.NewBufferString(fmt.Sprintf(createBody, f.subpath, f.branch, encodedContent))
	} else {
		body = bytes.NewBufferString(fmt.Sprintf(updateBody, f.subpath, f.branch, encodedContent, f.sha))
	}

	resp, err := f.gfs.apiRequest(
		"PUT",
		fmt.Sprintf(
			"https://api.github.com/repos/%s/%s/contents/%s",
			f.gfs.owner, f.gfs.repo, f.subpath,
		),
		"application/vnd.github+json",
		body,
	)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return ErrBadStatus{status: resp.Status}
	}
	defer resp.Body.Close()

	var respJson struct {
		sha string
	}
	if err = json.NewDecoder(resp.Body).Decode(&respJson); err != nil {
		return err
	}

	f.size = int64(len(f.buffer))
	f.fileInfo.modTime = time.Now().Local().UnixMilli()
	f.fileInfo.sha = respJson.sha
	f.dirty = false
	return nil
}

func (f *file) Close() error {
	return f.Sync()
}

func (f *file) ReadDir(n int) ([]fs.DirEntry, error) {
	if !f.IsDir() {
		return nil, syscall.ENOTDIR
	}

	if f.name == "." {
		if err := f.gfs.maybeUpdateBranches(); err != nil {
			return nil, &fs.PathError{Op: "readdir", Path: f.name, Err: err}
		}
		var res []fs.DirEntry
		for branch := range f.gfs.branches {
			res = append(res, &fileInfo{name: branch, size: 0, isDir: true})
		}
		return res, nil
	}

	if err := f.gfs.maybeUpdateBranches(); err != nil {
		return nil, &fs.PathError{Op: "readdir", Path: f.name, Err: err}
	}
	if err := f.gfs.maybeUpdateTree(f.branch); err != nil {
		return nil, &fs.PathError{Op: "readdir", Path: f.name, Err: err}
	}

	tree, ok := f.gfs.branches[f.branch]
	if !ok {
		// TODO: "ErrOutdatedFile"?
		// Linux allows reads on open file handles that are outdated, maybe we should do the same?
		// Could embed the TreeItem inside `file`.
		return nil, &fs.PathError{Op: "readdir", Path: f.name, Err: fs.ErrNotExist}
	}

	var res []fs.DirEntry
	for _, item := range tree.Items {
		after, found := strings.CutPrefix(item.Path, f.subpath)
		after = strings.TrimLeft(after, "/")
		// Only get immediate children
		if found && after != "" && !strings.ContainsRune(after, '/') {
			res = append(res, item.toFileInfo(f.branch))
		}
	}

	return res, nil
}

func (f *file) Stat() (fs.FileInfo, error) {
	return &f.fileInfo, nil
}

// Implements the `FileInfo` and `DirEntry` interfaces
type fileInfo struct {
	// Base name
	name    string
	size    int64
	mode    fs.FileMode
	modTime int64
	isDir   bool

	branch  string
	subpath string
	sha     string
}

func (i *fileInfo) Name() string       { return i.name }
func (i *fileInfo) Size() int64        { return i.size }
func (i *fileInfo) Mode() fs.FileMode  { return i.mode }
func (i *fileInfo) ModTime() time.Time { return time.Unix(i.modTime, 0) }
func (i *fileInfo) IsDir() bool        { return i.isDir }
func (i *fileInfo) Sys() any           { return nil }

// These allow it to act as DirEntry as well

func (i *fileInfo) Info() (fs.FileInfo, error) { return i, nil }
func (i *fileInfo) Type() fs.FileMode          { return i.Mode() }
