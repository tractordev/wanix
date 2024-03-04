// httpfs is a read-only filesystem built on top of HTTP
package httpfs

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"tractor.dev/toolkit-go/engine/fs/watchfs"
)

// FileServer wraps http.FileServer with extra endpoints for metadata.
// Requests to dir paths with `?readdir` will a return JSON array of dir entries.
// Requests to paths with `?stat` will return a JSON object of file info.
// Requests to paths with `?watch` will stream file change events.
func FileServer(fsys fs.FS) http.Handler {
	var wfs watchfs.WatchFS
	if w, ok := fsys.(watchfs.WatchFS); ok {
		wfs = w
	} else {
		wfs = watchfs.New(fsys)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimSuffix(r.URL.Path, "/")
		if name == "" {
			name = "."
		}

		fi, err := fs.Stat(fsys, name)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if r.URL.RawQuery == "watch" {
			watch, err := wfs.Watch(name, &watchfs.Config{
				Recursive: true, // just always do recursively
			})
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			// this long-polling impl won't catch every possible
			// event but is good enough for noticing general changes
			e := <-watch.Iter()
			watch.Close()
			eventErr := ""
			if e.Err != nil {
				eventErr = e.Err.Error()
			}
			b, err := json.Marshal(map[string]any{
				"type":    uint(e.Type),
				"path":    e.Path,
				"oldpath": e.OldPath,
				"err":     eventErr,
				"isDir":   e.IsDir(),
				"mode":    uint(e.Mode()),
				"size":    e.Size(),
				"name":    e.Name(),
				"modTime": e.ModTime().Unix(),
			})
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if _, err := w.Write(b); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			return
		}

		if r.URL.RawQuery == "stat" {
			b, err := json.Marshal(map[string]any{
				"isDir":   fi.IsDir(),
				"mode":    uint(fi.Mode()),
				"size":    fi.Size(),
				"name":    fi.Name(),
				"modTime": fi.ModTime().Unix(),
			})
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("content-type", "application/json")
			w.Write(b)
			return
		}

		if fi.IsDir() && r.URL.RawQuery == "readdir" {
			de, err := fs.ReadDir(fsys, name)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					http.Error(w, err.Error(), http.StatusNotFound)
					return
				}
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			var dir []string
			for _, e := range de {
				if e.IsDir() {
					dir = append(dir, e.Name()+"/")
				} else {
					dir = append(dir, e.Name())
				}
			}
			b, err := json.Marshal(dir)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("content-type", "application/json")
			w.Write(b)
			return
		}

		http.FileServer(http.FS(fsys)).ServeHTTP(w, r)
	})
}

type FS struct {
	baseURL string
}

func New(baseURL string) *FS {
	return &FS{baseURL}
}

func (fsys *FS) Watch(name string, cfg *watchfs.Config) (*watchfs.Watch, error) {
	if cfg == nil {
		cfg = &watchfs.Config{}
	}
	watch, inbox, closer := watchfs.NewWatch(name, *cfg)
	if cfg.Handler != nil {
		go func() {
			for e := range watch.Iter() {
				cfg.Handler(e)
			}
		}()
	}
	go func() {
		defer close(inbox)
		for {
			url := filepath.Join(fsys.baseURL, name)
			resp, err := http.DefaultClient.Get(url + "?watch")
			if err != nil {
				return
			}
			if resp.StatusCode == 404 {
				return
			}
			b, err := io.ReadAll(resp.Body)
			if err != nil {
				return
			}
			resp.Body.Close()
			event := make(map[string]any)
			if err := json.Unmarshal(b, &event); err != nil {
				continue
			}
			inbox <- inflateEvent(event)
			select {
			case inbox <- inflateEvent(event):
				continue
			case _, ok := <-closer:
				if !ok {
					return
				}
			default:
			}
		}
	}()
	return watch, nil
}

func inflateEvent(e map[string]any) watchfs.Event {
	fsInfo := &info{
		name:    e["name"].(string),
		size:    int64(e["size"].(float64)),
		mode:    uint(e["mode"].(float64)),
		modTime: int64(e["modTime"].(float64)),
		isDir:   e["isDir"].(bool),
	}
	return watchfs.Event{
		Path:     e["path"].(string),
		OldPath:  e["oldpath"].(string),
		Type:     watchfs.EventType(uint(e["type"].(float64))),
		Err:      errors.New(e["err"].(string)),
		FileInfo: fsInfo,
	}
}

func (fsys *FS) OpenFile(name string, flag int, perm fs.FileMode) (fs.File, error) {
	return fsys.Open(name)
}

func (fsys *FS) Open(name string) (fs.File, error) {
	url := filepath.Join(fsys.baseURL, name)
	resp, err := http.DefaultClient.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 404 {
		return nil, fs.ErrNotExist
	}
	return &file{
		ReadCloser: resp.Body,
		Name:       name,
		FS:         fsys,
	}, nil
}

func (fsys *FS) stat(name string) (*info, error) {
	url := filepath.Join(fsys.baseURL, name)
	resp, err := http.DefaultClient.Get(url + "?stat")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 404 {
		return nil, fs.ErrNotExist
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	m := map[string]any{}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return &info{
		name:    m["name"].(string),
		size:    int64(m["size"].(float64)),
		mode:    uint(m["mode"].(float64)),
		modTime: int64(m["modTime"].(float64)),
		isDir:   m["isDir"].(bool),
	}, nil
}

func (fsys *FS) Stat(name string) (fs.FileInfo, error) {
	return fsys.stat(name)
}

func (fsys *FS) ReadDir(name string) ([]fs.DirEntry, error) {
	url := filepath.Join(fsys.baseURL, name)
	resp, err := http.DefaultClient.Get(url + "?readdir")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 404 {
		return nil, fs.ErrNotExist
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	dir := []any{}
	if err := json.Unmarshal(b, &dir); err != nil {
		return nil, err
	}

	var out []fs.DirEntry
	for _, sub := range dir {
		info, err := fsys.stat(filepath.Join(name, sub.(string)))
		if err != nil {
			return nil, err
		}
		out = append(out, info)
	}
	return out, nil
}

type file struct {
	io.ReadCloser
	Name string
	FS   *FS
}

func (f *file) Stat() (fs.FileInfo, error) {
	return f.FS.Stat(f.Name)
}

func (f *file) ReadDir(n int) ([]fs.DirEntry, error) {
	return f.FS.ReadDir(f.Name)
}

type info struct {
	name    string
	size    int64
	mode    uint
	modTime int64
	isDir   bool
}

func (i *info) Name() string       { return i.name }
func (i *info) Size() int64        { return i.size }
func (i *info) Mode() fs.FileMode  { return fs.FileMode(i.mode) }
func (i *info) ModTime() time.Time { return time.Unix(i.modTime, 0) }
func (i *info) IsDir() bool        { return i.isDir }
func (i *info) Sys() any           { return nil }

// these allow it to act as DirInfo as well
func (i *info) Info() (fs.FileInfo, error) {
	return i, nil
}
func (i *info) Type() fs.FileMode {
	return i.Mode()
}
