package httpfs

import (
	"archive/tar"
	"bytes"
	"context"
	"log/slog"
	"sync"
	"time"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/cowfs"
	"tractor.dev/wanix/fs/memfs"
	"tractor.dev/wanix/fs/tarfs"
)

type watcherState struct {
	ch       chan fs.Event
	name     string
	timer    *time.Timer
	interval int // 0=30s, 1=1m, 2=2m, 3=4m
	mu       sync.Mutex
	cancel   chan struct{}
}

type Batcher struct {
	cow      *cowfs.FS
	buf      *memfs.FS
	mu       sync.Mutex
	t        *time.Timer
	tm       sync.Mutex
	watchers sync.Map // map[chan fs.Event]*watcherState
	log      *slog.Logger
	fs.FS
}

func NewBatcher(fsys *FS) *Batcher {
	cfs := NewCacher(fsys)
	buf := memfs.New()
	return &Batcher{
		log: slog.Default(),
		buf: buf,
		cow: &cowfs.FS{
			Base:    cfs,
			Overlay: buf,
		},
		FS: cfs,
	}
}

func (b *Batcher) changed() {
	b.tm.Lock()
	defer b.tm.Unlock()
	if b.t != nil {
		b.t.Stop()
	}
	b.t = time.AfterFunc(5*time.Second, func() {
		b.sendBatch()
	})
}

func (b *Batcher) sendBatch() {
	d, err := b.Snapshot()
	if err != nil {
		b.log.Debug("Snapshot", "err", err)
		return
	}
	if err := ApplyPatch(b.FS, ".", d); err != nil {
		b.log.Debug("ApplyPatch", "err", err)
		return
	}
	// warm the cache
	_, err = fs.ReadDir(b.FS, ".")
	if err != nil {
		b.log.Debug("ReadDir", "err", err)
		return
	}
	b.broadcast(fs.Event{
		Path: ".",
		Op:   "batch",
	})
}

func (b *Batcher) Snapshot() (bytes.Buffer, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	defer tw.Close()

	if err := tarfs.Archive(b.buf, tw); err != nil {
		return bytes.Buffer{}, err
	}

	for _, name := range b.cow.Deleted() {
		header := &tar.Header{
			Name: name,
			Mode: 0,
			Size: 0,
		}
		if header.PAXRecords == nil {
			header.PAXRecords = make(map[string]string)
		}
		header.PAXRecords["delete"] = ""
		if err := tw.WriteHeader(header); err != nil {
			return bytes.Buffer{}, err
		}
	}

	b.buf.Clear()
	b.cow.Reset()

	return buf, nil

}

func getWatchDuration(interval int) time.Duration {
	switch interval {
	case 0:
		return 30 * time.Second
	case 1:
		return 1 * time.Minute
	case 2:
		return 2 * time.Minute
	default:
		return 4 * time.Minute
	}
}

func (ws *watcherState) startTimer() {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	if ws.timer != nil {
		ws.timer.Stop()
	}

	duration := getWatchDuration(ws.interval)
	ws.timer = time.AfterFunc(duration, func() {
		// Check if cancelled
		select {
		case <-ws.cancel:
			return
		default:
		}

		// Send empty event to this watcher
		select {
		case ws.ch <- fs.Event{}:
		default:
			// Channel full, skip
		}

		// Advance interval: 30s -> 1m -> 2m -> 4m (stays at 4m)
		ws.mu.Lock()
		if ws.interval < 3 {
			ws.interval++
		}
		ws.mu.Unlock()

		// Schedule next timer
		ws.startTimer()
	})
}

func (ws *watcherState) resetTimer() {
	ws.mu.Lock()
	ws.interval = 0
	ws.mu.Unlock()
	ws.startTimer()
}

func (ws *watcherState) stopTimer() {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	if ws.timer != nil {
		ws.timer.Stop()
		ws.timer = nil
	}
	close(ws.cancel)
}

func (b *Batcher) broadcast(event fs.Event) {
	b.watchers.Range(func(key, value interface{}) bool {
		ws := value.(*watcherState)
		// Reset the timer for each watcher when broadcasting
		ws.resetTimer()
		ws.ch <- event
		return true
	})
}

func (b *Batcher) Watch(ctx context.Context, name string, exclude ...string) (<-chan fs.Event, error) {
	b.log.Debug("Watch", "name", name)
	ch := make(chan fs.Event, 128)

	ws := &watcherState{
		ch:     ch,
		name:   name,
		cancel: make(chan struct{}),
	}

	b.watchers.Store(ch, ws)

	// Start this watcher's timer
	ws.startTimer()

	go func() {
		<-ctx.Done()
		ws.stopTimer()
		b.watchers.Delete(ch)
		close(ch)
	}()

	return ch, nil
}

// Create creates or truncates the named file
func (b *Batcher) Create(name string) (fs.File, error) {
	return b.CreateContext(context.Background(), name)
}

// CreateContext is a helper for creating files with content and mode
func (b *Batcher) CreateContext(ctx context.Context, name string) (fs.File, error) {
	b.log.Debug("Create", "name", name)
	defer b.changed()
	b.mu.Lock()
	defer b.mu.Unlock()
	// if b.tee {
	// 	f, err := b.FS.CreateContext(ctx, name, nil, 0644)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	f.Close()
	// }
	return b.cow.Create(name)
}

func (b *Batcher) Symlink(oldname, newname string) error {
	return b.SymlinkContext(context.Background(), oldname, newname)
}

func (b *Batcher) SymlinkContext(ctx context.Context, oldname, newname string) error {
	b.log.Debug("Symlink", "oldname", oldname, "newname", newname)
	defer b.changed()
	b.mu.Lock()
	defer b.mu.Unlock()
	// if b.tee {
	// 	if err := b.FS.SymlinkContext(ctx, oldname, newname); err != nil {
	// 		return err
	// 	}
	// }
	return b.cow.Symlink(oldname, newname)
}

func (b *Batcher) Rename(oldname, newname string) error {
	return b.RenameContext(context.Background(), oldname, newname)
}

func (b *Batcher) RenameContext(ctx context.Context, oldname, newname string) error {
	b.log.Debug("Rename", "oldname", oldname, "newname", newname)
	defer b.changed()
	b.mu.Lock()
	defer b.mu.Unlock()
	// if b.tee {
	// 	b.FS.RenameContext(ctx, oldname, newname)
	// }
	return b.cow.Rename(oldname, newname)
}

// Mkdir creates a directory
func (b *Batcher) Mkdir(name string, perm fs.FileMode) error {
	return b.MkdirContext(context.Background(), name, perm)
}

// MkdirContext creates a directory with context
func (b *Batcher) MkdirContext(ctx context.Context, name string, perm fs.FileMode) error {
	b.log.Debug("Mkdir", "name", name, "perm", perm)
	defer b.changed()
	b.mu.Lock()
	defer b.mu.Unlock()
	// if b.tee {
	// 	if err := b.FS.MkdirContext(ctx, name, perm); err != nil {
	// 		return err
	// 	}
	// }
	return b.cow.Mkdir(name, perm)
}

// Remove removes a file or empty directory
func (b *Batcher) Remove(name string) error {
	return b.RemoveContext(context.Background(), name)
}

// RemoveContext removes a file or directory with context
func (b *Batcher) RemoveContext(ctx context.Context, name string) error {
	b.log.Debug("Remove", "name", name)
	defer b.changed()
	b.mu.Lock()
	defer b.mu.Unlock()
	// if b.tee {
	// 	if err := b.FS.RemoveContext(ctx, name); err != nil {
	// 		return err
	// 	}
	// }
	return b.cow.Remove(name)
}

// Chmod changes the mode of the named file
func (b *Batcher) Chmod(name string, mode fs.FileMode) error {
	return b.ChmodContext(context.Background(), name, mode)
}

// ChmodContext changes file mode with context
func (b *Batcher) ChmodContext(ctx context.Context, name string, mode fs.FileMode) error {
	b.log.Debug("Chmod", "name", name, "mode", mode)
	defer b.changed()
	b.mu.Lock()
	defer b.mu.Unlock()
	// if b.tee {
	// 	if err := b.FS.ChmodContext(ctx, name, mode); err != nil {
	// 		return err
	// 	}
	// }
	return b.cow.Chmod(name, mode)
}

// Chown changes the numeric uid and gid of the named file
func (b *Batcher) Chown(name string, uid, gid int) error {
	return b.ChownContext(context.Background(), name, uid, gid)
}

// ChownContect changes ownership with context
func (b *Batcher) ChownContext(ctx context.Context, name string, uid, gid int) error {
	b.log.Debug("Chown", "name", name, "uid", uid, "gid", gid)
	defer b.changed()
	b.mu.Lock()
	defer b.mu.Unlock()
	// if b.tee {
	// 	if err := b.FS.ChownContect(ctx, name, uid, gid); err != nil {
	// 		return err
	// 	}
	// }
	return b.cow.Chown(name, uid, gid)
}

// Chtimes changes the access and modification times of the named file
func (b *Batcher) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return b.ChtimesContext(context.Background(), name, atime, mtime)
}

// ChtimesContext changes times with context
func (b *Batcher) ChtimesContext(ctx context.Context, name string, atime time.Time, mtime time.Time) error {
	b.log.Debug("Chtimes", "name", name, "atime", atime, "mtime", mtime)
	defer b.changed()
	b.mu.Lock()
	defer b.mu.Unlock()
	// if b.tee {
	// 	if err := b.FS.ChtimesContext(ctx, name, atime, mtime); err != nil {
	// 		return err
	// 	}
	// }
	return b.cow.Chtimes(name, atime, mtime)
}
