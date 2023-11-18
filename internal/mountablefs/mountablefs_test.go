package mountablefs

import (
	"io/fs"
	"testing"

	"tractor.dev/toolkit-go/engine/fs/fstest"
	"tractor.dev/toolkit-go/engine/fs/memfs"
)

func TestMountUnmount(t *testing.T) {
	host := memfs.New()
	mnt := memfs.New()

	fstest.WriteFS(t, host, map[string]string{
		"all":             "host",
		"mount/host-data": "host",
	})

	fstest.WriteFS(t, mnt, map[string]string{
		"all2":         "mounted",
		"rickroll.mpv": "mounted",
	})

	fsys := New(host)
	if err := fsys.Mount(mnt, "mount"); err != nil {
		t.Fatal(err)
	}

	fstest.CheckFS(t, fsys, map[string]string{
		"all":                "host",
		"mount/all2":         "mounted",
		"mount/rickroll.mpv": "mounted",
	})

	if _, err := fsys.Open("mount/host-data"); err == nil {
		t.Fatalf("Open: expected file %s to be masked by mount: got nil, expected %v", "mount/host-data", fs.ErrNotExist)
	}

	if err := fsys.Unmount("all"); err == nil {
		t.Fatal("Unmount: expected error when attempting to Unmount a non-mountpoint, got nil")
	}

	if err := fsys.Unmount("mount"); err != nil {
		t.Fatal(err)
	}

	fstest.CheckFS(t, fsys, map[string]string{
		"all":             "host",
		"mount/host-data": "host",
	})

	if _, err := fsys.Open("mount/rickroll.mpv"); err == nil {
		t.Fatalf("Open: unexpected file %s: expected error %v", "mount/rickroll.mpv", fs.ErrNotExist)
	}
}

func TestRemove(t *testing.T) {
	host := memfs.New()
	mnt := memfs.New()

	fstest.WriteFS(t, host, map[string]string{
		"A/B":      "host",
		"C/D/blah": "host",
	})

	fstest.WriteFS(t, mnt, map[string]string{
		"E/F": "mounted",
		"G/H": "mounted",
	})

	fsys := New(host)
	if err := fsys.Mount(mnt, "C/D"); err != nil {
		t.Fatal(err)
	}

	fstest.CheckFS(t, fsys, map[string]string{
		"A/B":     "host",
		"C/D/E/F": "mounted",
		"C/D/G/H": "mounted",
	})

	if err := fsys.Remove("A/B"); err != nil {
		t.Fatal(err)
	}

	if err := fsys.Remove("C/D/E/F"); err != nil {
		t.Fatal(err)
	}

	if err := fsys.RemoveAll("C/D/G"); err != nil {
		t.Fatal(err)
	}

	fstest.CheckFS(t, fsys, map[string]string{
		// dirs are empty strings
		"A/":     "",
		"C/D/E/": "",
	})

	if err := fsys.Remove("A/B"); err == nil {
		t.Fatalf("Remove: expected attempt to Remove a non-existant file to fail")
	}

	if err := fsys.Remove("C/D/G"); err == nil {
		t.Fatalf("Remove: expected attempt to Remove a non-existant file to fail")
	}

	if err := fsys.Remove("C/D"); err == nil {
		t.Fatalf("Remove: expected attempt to Remove a mount-point file to fail")
	}

	if err := fsys.RemoveAll("C/D"); err == nil {
		t.Fatalf("RemoveAll: expected attempt to RemoveAll a mount-point file to fail")
	}

	if err := fsys.RemoveAll("/"); err == nil {
		t.Fatalf("RemoveAll: expected attempt to RemoveAll a path containing a mount-point file to fail")
	}

	fstest.CheckFS(t, fsys, map[string]string{
		// dirs are empty strings
		"C/D/E/": "",
	})

	if err := fsys.Unmount("C/D"); err != nil {
		t.Fatal(err)
	}
}

func TestRename(t *testing.T) {
	host := memfs.New()
	mnt := memfs.New()

	fstest.WriteFS(t, host, map[string]string{
		"all":             "host",
		"mount/host-data": "host",
	})

	fstest.WriteFS(t, mnt, map[string]string{
		"all2":         "mounted",
		"rickroll.mpv": "mounted",
	})

	fsys := New(host)
	if err := fsys.Mount(mnt, "mount"); err != nil {
		t.Fatal(err)
	}

	if err := fsys.Rename("all", "none"); err != nil {
		t.Fatal(err)
	}

	if err := fsys.Rename("mount/all2", "mount/none2"); err != nil {
		t.Fatal(err)
	}

	if err := fsys.Rename("mount/rickroll.mpv", "rickroll.mpv"); err == nil {
		t.Fatalf("Rename: expected error when attempting to rename across filesystems")
	}

    if err := fsys.Rename("mount", "not-a-mount"); err == nil {
        t.Fatalf("Rename: expected error when attempting to rename a mountpoint")
    }

	fstest.CheckFS(t, fsys, map[string]string{
		"none":               "host",
		"mount/none2":        "mounted",
		"mount/rickroll.mpv": "mounted",
	})

	if err := fsys.Unmount("mount"); err != nil {
		t.Fatal(err)
	}
}

func TestMkdir(t *testing.T) {
	host := memfs.New()
	mnt := memfs.New()

	fstest.WriteFS(t, host, map[string]string{
		"all":             "host",
		"mount/host-data": "host",
	})

	fstest.WriteFS(t, mnt, map[string]string{
		"all2":         "mounted",
		"rickroll.mpv": "mounted",
	})

	fsys := New(host)
	if err := fsys.Mount(mnt, "mount"); err != nil {
		t.Fatal(err)
	}

	if err := fsys.Mkdir("all/new-host-dir", 0755); err != nil {
		t.Fatal(err)
	}

	if err := fsys.Mkdir("mount/secret_tunnel", 0755); err != nil {
		t.Fatal(err)
	}

	// TODO: memfs has incorrect behavior for creating and checking parents, so until
	// it's fixed I'm leaving these commented. They're really testing the underlying
	// filesystem implementation anyway.
	// if err := fsys.Mkdir("mount/rickroll.mpv/nope", 0755); err == nil {
	//     t.Fatalf("Mkdir: expected error when attempting to make a directory under a file")
	// }

	// if err := fsys.Mkdir("mount/hello/goodbye", 0755); err == nil {
	//     t.Fatalf("Mkdir: expected error when attempting to make a directory with missing parents")
	// }

	if err := fsys.MkdirAll("mount/secret_tunnel/super_secret/deadend", 0755); err != nil {
		t.Fatal(err)
	}

	fstest.CheckFS(t, fsys, map[string]string{
		// dirs are empty strings
		"all/new-host-dir/":  "",
		"mount/all2":         "mounted",
		"mount/rickroll.mpv": "mounted",
		"mount/secret_tunnel/super_secret/deadend/": "",
	})

	if err := fsys.Unmount("mount"); err != nil {
		t.Fatal(err)
	}
}
