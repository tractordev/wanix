package task

import (
	"context"
	"io"
	"log"
	"net"
	"strconv"
	"strings"

	"tractor.dev/toolkit-go/engine/cli"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/internal"
	"tractor.dev/wanix/namespace"
)

// contextKey is a value for use with context.WithValue. It's used as
// a pointer so it fits in an interface{} without allocation.
type contextKey struct {
	name string
}

func (k *contextKey) String() string { return "task context value " + k.name }

var (
	TaskContextKey = &contextKey{"task"}
)

func PIDFromContext(ctx context.Context) (string, bool) {
	p, ok := ctx.Value(TaskContextKey).(string)
	return p, ok
}

type Process struct {
	starter func(*Process) error
	ns      *namespace.FS
	id      int
	typ     string
	cmd     string
	env     []string
	exit    string
	dir     string
	fds     map[string]fs.FS
	sys     map[string]fs.FS
}

func (r *Process) Start() error {
	if r.starter != nil {
		return r.starter(r)
	}
	return nil
}

func (r *Process) ID() string {
	return strconv.Itoa(r.id)
}

func (r *Process) Context() context.Context {
	return r.Namespace().Context()
}

func (r *Process) Namespace() *namespace.FS {
	return r.ns
}

func (r *Process) Cmd() string {
	return r.cmd
}

func (r *Process) Env() []string {
	return r.env
}

func (r *Process) Dir() string {
	return r.dir
}

func (r *Process) Bind(srcPath, dstPath string) error {
	return r.ns.Bind(r.ns, srcPath, dstPath, "")
}

func (r *Process) Open(name string) (fs.File, error) {
	return r.OpenContext(context.Background(), name)
}

func (r *Process) Sub(name string) (fs.FS, error) {
	return fs.Sub(fskit.MapFS{
		"ctl": internal.ControlFile(&cli.Command{
			Usage: "ctl",
			Short: "control the resource",
			Run: func(ctx *cli.Context, args []string) {
				if len(args) == 3 && args[0] == "bind" {
					if err := r.Bind(args[1], args[2]); err != nil {
						log.Println(err)
					}
					return
				}
				if len(args) == 1 && args[0] == "start" {
					if err := r.Start(); err != nil {
						log.Println(err)
					}
					return
				}
			},
		}),
		"type": internal.FieldFile(r.typ),
		"cmd": internal.FieldFile(r.cmd, func(in []byte) error {
			if len(in) > 0 {
				r.cmd = strings.TrimSpace(string(in))
			}
			return nil
		}),
		"env": internal.FieldFile(strings.Join(r.env, "\n"), func(in []byte) error {
			if len(in) > 0 {
				r.env = strings.Split(strings.TrimSpace(string(in)), "\n")
			}
			return nil
		}),
		"dir": internal.FieldFile(r.dir, func(in []byte) error {
			if len(in) > 0 {
				r.dir = strings.TrimSpace(string(in))
			}
			return nil
		}),
		"exit": internal.FieldFile(r.exit, func(in []byte) error {
			if len(in) > 0 {
				r.exit = strings.TrimSpace(string(in))
			}
			return nil
		}),
		// "ns":   r.ns,
		"fd":   fskit.MapFS(r.fds),
		".sys": fskit.MapFS(r.sys),
	}, name)
}

func (r *Process) OpenContext(ctx context.Context, name string) (fs.File, error) {
	fsys, err := r.Sub(".")
	if err != nil {
		return nil, err
	}
	return fs.OpenContext(ctx, fsys, name)
}

type ConnFile struct {
	net.Conn
	Name string
}

func newFdFile(conn net.Conn, name string) fs.FS {
	return fskit.OpenFunc(func(ctx context.Context, _ string) (fs.File, error) {
		return &ConnFile{Conn: conn, Name: name}, nil
	})
}

func (s *ConnFile) Stat() (fs.FileInfo, error) {
	return fskit.Entry(s.Name, 0644), nil
}

func (s *ConnFile) WriteAt(p []byte, off int64) (n int, err error) {
	return s.Write(p)
}

func (s *ConnFile) Write(p []byte) (n int, err error) {
	return s.Conn.Write(p)
}

func (s *ConnFile) ReadAt(p []byte, off int64) (n int, err error) {
	if off > 0 {
		return 0, io.EOF
	}
	return s.Read(p)
}

func (s *ConnFile) Read(p []byte) (int, error) {
	return s.Conn.Read(p)
}

func (s *ConnFile) Close() error {
	return nil
}
