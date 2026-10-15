package ether

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/progrium/go-netstack/vnet"
	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
	"tractor.dev/wanix/misc"
)

// Service is a Plan 9 style ether(3) filesystem backed by a vnet.VirtualNetwork.
type Service struct {
	mu        sync.RWMutex
	resources map[string]fs.FS
	nextID    int
	closed    bool

	vn  *vnet.VirtualNetwork
	cfg *vnet.Configuration
}

// New returns a Service whose conversations are AcceptQemu sessions on vn.
// cfg should be the configuration vn was created with; it is used for
// addr, stats and frame size limits.
func New(vn *vnet.VirtualNetwork, cfg *vnet.Configuration) *Service {
	return &Service{
		resources: make(map[string]fs.FS),
		vn:        vn,
		cfg:       cfg,
	}
}

func (s *Service) Open(name string) (fs.File, error) {
	return s.OpenContext(context.Background(), name)
}

func (s *Service) OpenContext(ctx context.Context, name string) (fs.File, error) {
	if name == "clone" {
		c, err := s.Alloc()
		if err != nil {
			return nil, &fs.PathError{Op: "open", Path: name, Err: err}
		}
		// the conversation was allocated holding one reference for this handle
		return c.wrap(newCtlFile(c)), nil
	}
	return fs.OpenContext(ctx, s.rootFS(), name)
}

func (s *Service) Stat(name string) (fs.FileInfo, error) {
	return s.StatContext(context.Background(), name)
}

func (s *Service) StatContext(ctx context.Context, name string) (fs.FileInfo, error) {
	if name == "clone" {
		return cloneNode(), nil
	}
	return fs.StatContext(ctx, s.rootFS(), name)
}

func (s *Service) Route(ctx context.Context, name string) (fs.FS, string, error) {
	// clone must be opened by Service so it can allocate
	if name == "clone" {
		return s, name, nil
	}
	return s.rootFS().Route(ctx, name)
}

func cloneNode() *fskit.Node {
	return fskit.Entry("clone", 0666)
}

func (s *Service) rootFS() fskit.UnionFS {
	s.mu.RLock()
	defer s.mu.RUnlock()
	resources := make(fskit.MapFS, len(s.resources))
	for k, v := range s.resources {
		resources[k] = v
	}
	return fskit.UnionFS{
		fskit.MapFS{
			"clone": cloneNode(),
			"addr":  misc.FieldFile(func() (string, error) { return s.addr(), nil }),
			"stats": misc.FieldFile(func() (string, error) { return s.stats(), nil }),
		},
		resources,
	}
}

func (s *Service) addr() string {
	return strings.ToLower(strings.NewReplacer(":", "", "-", "").Replace(s.cfg.GatewayMacAddress))
}

func (s *Service) stats() string {
	s.mu.RLock()
	n := len(s.resources)
	s.mu.RUnlock()
	var b strings.Builder
	fmt.Fprintf(&b, "mtu: %d\n", s.cfg.MTU)
	fmt.Fprintf(&b, "subnet: %s\n", s.cfg.Subnet)
	fmt.Fprintf(&b, "gateway: %s\n", s.cfg.GatewayIP)
	fmt.Fprintf(&b, "gatewaymac: %s\n", s.cfg.GatewayMacAddress)
	fmt.Fprintf(&b, "conversations: %d\n", n)
	return b.String()
}

// Get returns the conversation with the given id.
func (s *Service) Get(id string) (*Conv, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res, ok := s.resources[id]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return res.(*Conv), nil
}

// Alloc creates a new conversation and starts its AcceptQemu session.
// The returned conversation holds one reference that the caller must release.
func (s *Service) Alloc() (*Conv, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, fs.ErrClosed
	}
	id := strconv.Itoa(s.nextID)
	s.nextID++
	c := newConv(id, s)
	s.resources[id] = c
	c.start()
	return c, nil
}

// Close ends all sessions. Conversations remain until their handles are closed.
func (s *Service) Close() error {
	s.mu.Lock()
	s.closed = true
	var convs []*Conv
	for _, r := range s.resources {
		convs = append(convs, r.(*Conv))
	}
	s.mu.Unlock()
	for _, c := range convs {
		c.end(fs.ErrClosed)
	}
	return nil
}

func (s *Service) remove(id string) {
	s.mu.Lock()
	delete(s.resources, id)
	s.mu.Unlock()
}

func (s *Service) maxFrame() int {
	mtu := s.cfg.MTU
	if mtu <= 0 {
		mtu = 1500
	}
	return mtu + ethHeaderLen
}
