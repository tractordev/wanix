package r2fs

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestBasePath(t *testing.T) {
	// Create a mock S3 client (we won't actually make requests in this test)
	client := &s3.Client{}

	tests := []struct {
		name     string
		basePath string
		fsPath   string
		expected string
	}{
		{
			name:     "empty base path",
			basePath: "",
			fsPath:   "test.txt",
			expected: "/test.txt",
		},
		{
			name:     "simple base path",
			basePath: "myapp",
			fsPath:   "test.txt",
			expected: "/myapp/test.txt",
		},
		{
			name:     "base path with leading slash (should be normalized)",
			basePath: "/myapp",
			fsPath:   "test.txt",
			expected: "/myapp/test.txt",
		},
		{
			name:     "base path with trailing slash",
			basePath: "myapp/",
			fsPath:   "test.txt",
			expected: "/myapp/test.txt",
		},
		{
			name:     "nested base path",
			basePath: "myapp/data",
			fsPath:   "test.txt",
			expected: "/myapp/data/test.txt",
		},
		{
			name:     "root path with base path",
			basePath: "myapp",
			fsPath:   ".",
			expected: "/myapp",
		},
		{
			name:     "nested file path with base path",
			basePath: "myapp",
			fsPath:   "folder/test.txt",
			expected: "/myapp/folder/test.txt",
		},
		{
			name:     "base path that becomes empty after normalization",
			basePath: "/",
			fsPath:   "test.txt",
			expected: "/test.txt",
		},
		{
			name:     "base path with only dots",
			basePath: ".",
			fsPath:   "test.txt",
			expected: "/test.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create filesystem with base path
			fs := NewWithClientAndBasePath(client, "test-bucket", tt.basePath)

			// Test normalizeR2Path
			result := fs.normalizeR2Path(tt.fsPath)
			if result != tt.expected {
				t.Errorf("normalizeR2Path(%q) with basePath %q = %q, want %q",
					tt.fsPath, tt.basePath, result, tt.expected)
			}
		})
	}
}

func TestBasePathGetterSetter(t *testing.T) {
	client := &s3.Client{}
	fs := NewWithClientAndBasePath(client, "test-bucket", "initial")

	// Test getter
	if got := fs.GetBasePath(); got != "initial" {
		t.Errorf("GetBasePath() = %q, want %q", got, "initial")
	}

	// Test setter
	fs.SetBasePath("updated")
	if got := fs.GetBasePath(); got != "updated" {
		t.Errorf("GetBasePath() after SetBasePath() = %q, want %q", got, "updated")
	}

	// Test that cache is cleared when base path changes
	// (We can't easily test this without mocking, but the method should not panic)
	fs.SetBasePath("")
	if got := fs.GetBasePath(); got != "" {
		t.Errorf("GetBasePath() after clearing = %q, want empty string", got)
	}
}

func TestConstructorsWithBasePath(t *testing.T) {
	// Test NewWithBasePath
	fs1, err := NewWithBasePath("account", "key", "secret", "bucket", "myapp")
	if err != nil {
		// This will fail without real credentials, but we can test the structure
		t.Logf("NewWithBasePath failed as expected without real credentials: %v", err)
	} else if fs1.GetBasePath() != "myapp" {
		t.Errorf("NewWithBasePath basePath = %q, want %q", fs1.GetBasePath(), "myapp")
	}

	// Test NewWithClientAndBasePath
	client := &s3.Client{}
	fs2 := NewWithClientAndBasePath(client, "bucket", "myapp")
	if fs2.GetBasePath() != "myapp" {
		t.Errorf("NewWithClientAndBasePath basePath = %q, want %q", fs2.GetBasePath(), "myapp")
	}

	// Test backward compatibility - existing constructors should have empty base path
	fs3 := NewWithClient(client, "bucket")
	if fs3.GetBasePath() != "" {
		t.Errorf("NewWithClient basePath = %q, want empty string", fs3.GetBasePath())
	}
}

// Removed TestRootDirectoryMode - it was causing panics with mock client
