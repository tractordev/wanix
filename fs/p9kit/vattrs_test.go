package p9kit

import (
	"testing"

	"github.com/hugelgupf/p9/p9"
	"tractor.dev/wanix/fs/memfs"
)

func TestMemAttrStore_CreatorOwnership(t *testing.T) {
	root, err := Attacher(memfs.New(), WithMemAttrStore()).Attach()
	if err != nil {
		t.Fatal(err)
	}

	owner := func(name string) (p9.UID, p9.GID) {
		t.Helper()
		_, f, err := root.Walk([]string{name})
		if err != nil {
			t.Fatalf("walk %s: %v", name, err)
		}
		_, _, attr, err := f.GetAttr(p9.AttrMaskAll)
		if err != nil {
			t.Fatalf("getattr %s: %v", name, err)
		}
		return attr.UID, attr.GID
	}

	if _, err := root.Mkdir("data", 0700, 70, 70); err != nil {
		t.Fatal(err)
	}
	if u, g := owner("data"); u != 70 || g != 70 {
		t.Fatalf("mkdir owner = %d:%d, want 70:70", u, g)
	}

	_, data, err := root.Walk([]string{"data"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := data.Mkdir("sub", 0700, 70, 70); err != nil {
		t.Fatal(err)
	}
	f, _, _, err := data.Create("file", p9.WriteOnly, 0600, 70, p9.NoGID)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if u, g := owner("data/file"); u != 70 || g != 0 {
		t.Fatalf("create owner = %d:%d, want 70:0", u, g)
	}

	// Rename carries ownership, including for descendants.
	if err := root.RenameAt("data", root, "moved"); err != nil {
		t.Fatal(err)
	}
	if u, _ := owner("moved/sub"); u != 70 {
		t.Fatalf("renamed descendant uid = %d, want 70", u)
	}

	// Unlink drops ownership so a new path with the same name doesn't inherit it.
	_, moved, err := root.Walk([]string{"moved"})
	if err != nil {
		t.Fatal(err)
	}
	if err := moved.UnlinkAt("file", 0); err != nil {
		t.Fatal(err)
	}
	f, _, _, err = moved.Create("file", p9.WriteOnly, 0600, p9.NoUID, p9.NoGID)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if u, _ := owner("moved/file"); u != 0 {
		t.Fatalf("recreated file uid = %d, want 0", u)
	}
}
