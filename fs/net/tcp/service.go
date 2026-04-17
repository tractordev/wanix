package tcp

import (
	"context"
	"strconv"
	"sync"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
)

type Service struct {
	mu        sync.RWMutex
	resources map[string]fs.FS
	nextID    int

	AllocHook func(s *Service, rid string) error
}

func New() *Service {
	return &Service{
		resources: make(map[string]fs.FS),
		nextID:    0,
	}
}

func (s *Service) Open(name string) (fs.File, error) {
	return s.OpenContext(context.Background(), name)
}

func (s *Service) OpenContext(ctx context.Context, name string) (fs.File, error) {
	fsys, rname, err := s.ResolveFS(ctx, name)
	if err != nil {
		return nil, err
	}
	return fs.OpenContext(ctx, fsys, rname)
}

func (s *Service) Stat(name string) (fs.FileInfo, error) {
	return s.StatContext(context.Background(), name)
}

func (s *Service) StatContext(ctx context.Context, name string) (fs.FileInfo, error) {
	fsys, rname, err := s.ResolveFS(ctx, name)
	if err != nil {
		return nil, err
	}
	return fs.StatContext(ctx, fsys, rname)
}

func (s *Service) ResolveFS(ctx context.Context, name string) (fs.FS, string, error) {
	root := fskit.MapFS{
		"new": fskit.OpenFunc(func(ctx context.Context, name string) (fs.File, error) {
			if name != "." {
				return nil, fs.ErrNotExist
			}
			return &fskit.FuncFile{
				Node: fskit.Entry("new", 0555),
				ReadFunc: func(n *fskit.Node) error {
					rid, err := s.Alloc()
					if err != nil {
						return err
					}
					if s.AllocHook != nil {
						if err := s.AllocHook(s, rid); err != nil {
							return err
						}
					}
					fskit.SetData(n, []byte(rid+"\n"))
					return nil
				},
			}, nil
		}),
	}
	return fs.Resolve(fskit.UnionFS{root, fskit.MapFS(s.resources)}, ctx, name)
}

func (s *Service) Get(rid string) (*Conn, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res, ok := s.resources[rid]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return res.(*Conn), nil
}

func (s *Service) Alloc() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextID++
	rid := strconv.Itoa(s.nextID)
	s.resources[rid] = newConn(rid, s)
	return rid, nil
}

func (s *Service) remove(rid string) {
	s.mu.Lock()
	res, ok := s.resources[rid]
	if ok {
		delete(s.resources, rid)
	}
	s.mu.Unlock()

	if ok {
		res.(*Conn).shutdown()
	}
}
