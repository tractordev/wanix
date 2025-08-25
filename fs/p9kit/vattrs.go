package p9kit

import (
	"context"
	"errors"
	"sync"

	"tractor.dev/wanix/fs"
)

// VirtualAttrs holds virtual file attributes that override real filesystem attributes
type VirtualAttrs struct {
	UID *uint32 // Pointer to allow nil for "not set"
	GID *uint32
}

// VirtualAttrStore interface for storing and retrieving virtual attributes
type VirtualAttrStore interface {
	Get(path string) (*VirtualAttrs, error)
	Set(path string, attrs *VirtualAttrs) error
}

// MemoryAttrStore implements VirtualAttrStore using in-memory storage
type MemoryAttrStore struct {
	attrs map[string]*VirtualAttrs
	mu    sync.RWMutex
}

// WithMemAttrStore creates an option to enable in-memory virtual attribute support
func WithMemAttrStore() AttacherOption {
	return &MemoryAttrStore{
		attrs: make(map[string]*VirtualAttrs),
	}
}

func (s *MemoryAttrStore) applyToAttacher(a *attacher) {
	a.vattrs = s
}

func (s *MemoryAttrStore) Get(path string) (*VirtualAttrs, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if attrs, ok := s.attrs[path]; ok {
		return attrs, nil
	}
	return nil, nil
}

func (s *MemoryAttrStore) Set(path string, attrs *VirtualAttrs) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.attrs[path] = attrs
	return nil
}

// XattrAttrStore implements VirtualAttrStore using extended attributes
type XattrAttrStore struct {
	fsys fs.FS
}

// WithXattrAttrStore creates an option to enable extended attribute-based virtual attribute support
func WithXattrAttrStore() AttacherOption {
	return &XattrAttrStore{}
}

func (s *XattrAttrStore) applyToAttacher(a *attacher) {
	s.fsys = a.FS
	a.vattrs = s
}

func (s *XattrAttrStore) Get(path string) (*VirtualAttrs, error) {
	ctx := context.Background()

	var attrs VirtualAttrs
	var hasAttrs bool

	// Get UID from xattr
	if uidData, err := fs.GetXattr(ctx, s.fsys, path, "user.wanix.uid"); err == nil {
		if len(uidData) == 4 { // uint32 is 4 bytes
			uid := uint32(uidData[0]) | uint32(uidData[1])<<8 | uint32(uidData[2])<<16 | uint32(uidData[3])<<24
			attrs.UID = &uid
			hasAttrs = true
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	// Get GID from xattr
	if gidData, err := fs.GetXattr(ctx, s.fsys, path, "user.wanix.gid"); err == nil {
		if len(gidData) == 4 { // uint32 is 4 bytes
			gid := uint32(gidData[0]) | uint32(gidData[1])<<8 | uint32(gidData[2])<<16 | uint32(gidData[3])<<24
			attrs.GID = &gid
			hasAttrs = true
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	if hasAttrs {
		return &attrs, nil
	}
	return nil, nil
}

func (s *XattrAttrStore) Set(path string, attrs *VirtualAttrs) error {
	ctx := context.Background()

	// Set UID if provided
	if attrs.UID != nil {
		uid := *attrs.UID
		uidData := []byte{
			byte(uid),
			byte(uid >> 8),
			byte(uid >> 16),
			byte(uid >> 24),
		}
		if err := fs.SetXattr(ctx, s.fsys, path, "user.wanix.uid", uidData, 0); err != nil {
			return err
		}
	} else {
		// Remove UID xattr if not set
		fs.RemoveXattr(ctx, s.fsys, path, "user.wanix.uid") // ignore error
	}

	// Set GID if provided
	if attrs.GID != nil {
		gid := *attrs.GID
		gidData := []byte{
			byte(gid),
			byte(gid >> 8),
			byte(gid >> 16),
			byte(gid >> 24),
		}
		if err := fs.SetXattr(ctx, s.fsys, path, "user.wanix.gid", gidData, 0); err != nil {
			return err
		}
	} else {
		// Remove GID xattr if not set
		fs.RemoveXattr(ctx, s.fsys, path, "user.wanix.gid") // ignore error
	}

	return nil
}
