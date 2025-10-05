//go:build !wasm

package localfs

import (
	"os"
	"path/filepath"
	"testing"

	"tractor.dev/wanix/fs"
	"tractor.dev/wanix/fs/pstat"
)

func TestUidGidOverride(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "localfs_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test regular localfs (no override)
	t.Run("NoOverride", func(t *testing.T) {
		fsys, err := New(tempDir)
		if err != nil {
			t.Fatalf("Failed to create localfs: %v", err)
		}

		fi, err := fsys.Stat("test.txt")
		if err != nil {
			t.Fatalf("Failed to stat file: %v", err)
		}

		// Get original uid/gid
		origStat := pstat.SysToStat(fi.Sys())
		if origStat == nil {
			t.Fatal("pstat.SysToStat() returned nil, skipping uid/gid check")
		}

		// Test with virtual uid/gid enabled
		t.Run("WithVirtualUidGid", func(t *testing.T) {
			virtualFS, err := NewWithVirtualUidGid(tempDir)
			if err != nil {
				t.Fatalf("Failed to create virtual localfs: %v", err)
			}

			fi, err := virtualFS.Stat("test.txt")
			if err != nil {
				t.Fatalf("Failed to stat file with virtual uid/gid: %v", err)
			}

			statInfo := pstat.SysToStat(fi.Sys())
			if statInfo.Uid != 0 {
				t.Errorf("Expected uid 0, got %d", statInfo.Uid)
			}
			if statInfo.Gid != 0 {
				t.Errorf("Expected gid 0, got %d", statInfo.Gid)
			}

			// Test chown operation
			err = virtualFS.Chown("test.txt", 1000, 1000)
			if err != nil {
				t.Fatalf("Chown failed: %v", err)
			}

			// Verify chown was stored in memory
			fi, err = virtualFS.Stat("test.txt")
			if err != nil {
				t.Fatalf("Failed to stat file after chown: %v", err)
			}

			statInfo = pstat.SysToStat(fi.Sys())
			if statInfo.Uid != 1000 {
				t.Errorf("Expected uid 1000 after chown, got %d", statInfo.Uid)
			}
			if statInfo.Gid != 1000 {
				t.Errorf("Expected gid 1000 after chown, got %d", statInfo.Gid)
			}

			// Verify the actual filesystem wasn't modified
			actualFi, err := os.Stat(testFile)
			if err != nil {
				t.Fatalf("Failed to stat actual file: %v", err)
			}

			actualStat := pstat.SysToStat(actualFi.Sys())
			if actualStat.Uid != origStat.Uid {
				t.Errorf("Actual file uid changed from %d to %d", origStat.Uid, actualStat.Uid)
			}
			if actualStat.Gid != origStat.Gid {
				t.Errorf("Actual file gid changed from %d to %d", origStat.Gid, actualStat.Gid)
			}

		})

	})
}

func TestChownFS(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "localfs_chown_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test that localfs implements ChownFS interface
	fsys, err := NewWithVirtualUidGid(tempDir)
	if err != nil {
		t.Fatalf("Failed to create localfs: %v", err)
	}

	// Verify it implements the ChownFS interface
	var _ fs.ChownFS = fsys // This will cause a compile error if FS doesn't implement ChownFS

	// Test using the fs.Chown helper function
	err = fs.Chown(fsys, "test.txt", 2000, 2000)
	if err != nil {
		t.Fatalf("fs.Chown failed: %v", err)
	}

	// Verify the change was applied
	fi, err := fsys.Stat("test.txt")
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	sys := fi.Sys()
	if sys == nil {
		t.Skip("Sys() returned nil, skipping verification")
	}

	if statInfo, ok := sys.(*pstat.Stat); ok {
		if statInfo.Uid != 2000 {
			t.Errorf("Expected uid 2000, got %d", statInfo.Uid)
		}
		if statInfo.Gid != 2000 {
			t.Errorf("Expected gid 2000, got %d", statInfo.Gid)
		}
	} else {
		t.Fatalf("Sys() did not return *stat.Stat_t, got %T", sys)
	}
}
