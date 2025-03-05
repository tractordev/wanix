package cap

import (
	"fmt"
	"strings"
	"testing"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/fskit"
)

func TestDevice(t *testing.T) {
	dev := New(nil)
	dev.Register("hellofs", func(_ *Resource) (Mounter, error) {
		return func(_ []string) (fs.FS, error) {
			return fskit.MapFS{"hellofile": fskit.RawNode([]byte("hello, world\n"))}, nil
		}, nil
	})

	// check for new/hellofs
	e, err := fs.ReadDir(dev, "new")
	if err != nil {
		t.Fatal(err)
	}
	if len(e) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(e))
	}
	if e[0].Name() != "hellofs" {
		t.Fatalf("expected hellofs, got %s", e[0].Name())
	}

	// read new/hellofs to allocate a resource id
	b, err := fs.ReadFile(dev, "new/hellofs")
	if err != nil {
		t.Fatal(err)
	}
	rid := strings.TrimSpace(string(b))
	if rid != "1" {
		t.Fatalf("expected 1, got %s", rid)
	}

	// check for resource type
	b, err = fs.ReadFile(dev, fmt.Sprintf("%s/type", rid))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "hellofs\n" {
		t.Fatalf("expected hellofs, got %s", string(b))
	}

	// check for file in resource mount
	b, err = fs.ReadFile(dev, fmt.Sprintf("%s/mount/hellofile", rid))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "hello, world\n" {
		t.Fatalf("expected hello, world, got %s", string(b))
	}
}
