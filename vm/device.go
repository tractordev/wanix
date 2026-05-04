package vm

import (
	"context"
	"log"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	"tractor.dev/wanix"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
)

type Device struct {
	resources map[string]fs.FS
	aliases   map[string]fs.FS
	nextID    int
	root      *wanix.Task
	mu        sync.Mutex
}

func Drivers(t *wanix.Task) []string {
	b, err := t.Namespace().Bindings("#vm")
	if err != nil {
		log.Println("vm drivers:", err)
		return nil
	}
	result := make([]string, 0, len(b))
	for _, b := range b {
		log.Println("vm driver:", b.Name())
		name := strings.TrimSuffix(b.Name(), filepath.Ext(b.Name()))
		result = append(result, name)
	}
	return result
}

func New(root *wanix.Task) *Device {
	d := &Device{
		resources: make(map[string]fs.FS),
		aliases:   make(map[string]fs.FS),
		nextID:    0,
		root:      root,
	}
	return d
}

func (d *Device) Alloc(kind string) (wanix.Resource, error) {
	if !slices.Contains(Drivers(d.root), kind) {
		return nil, fs.ErrNotExist
	}
	d.nextID++
	rid := strconv.Itoa(d.nextID)
	r := &VM{
		id:     rid,
		kind:   kind,
		device: d,
	}
	d.resources[rid] = r
	return r, nil
}

func (d *Device) Open(name string) (fs.File, error) {
	return d.OpenContext(context.Background(), name)
}

func (d *Device) OpenContext(ctx context.Context, name string) (fs.File, error) {
	fsys := fskit.MapFS{
		"new": fskit.OpenFunc(func(ctx context.Context, name string) (fs.File, error) {
			if name == "." {
				var nodes []fs.DirEntry
				for _, kind := range Drivers(d.root) {
					nodes = append(nodes, fskit.Entry(kind, 0555))
				}
				return fskit.DirFile(fskit.Entry("new", 0555), nodes...), nil
			}
			return &fskit.FuncFile{
				Node: fskit.Entry(name, 0555),
				ReadFunc: func(n *fskit.Node) error {
					r, err := d.Alloc(name)
					if err != nil {
						return err
					}
					fskit.SetData(n, []byte(r.ID()+"\n"))
					return nil
				},
			}, nil
		}),
	}
	return fs.OpenContext(ctx, fskit.UnionFS{fsys, fskit.MapFS(d.resources), fskit.MapFS(d.aliases)}, name)
}
