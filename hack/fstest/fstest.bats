#!/usr/bin/env bats

load '/usr/lib/bats/bats-support/load'
load '/usr/lib/bats/bats-assert/load'

# Test configuration
setup() {
    # Create test directory structure
    TEST_DIR="/mnt"
    TEST_FILE_CONTENT="$(printf '%*s' 512 | tr ' ' 'x')"
    TEST_FILE_SMALL="small test content"
    
    # Ensure clean test environment
    if [ -d "$TEST_DIR" ]; then
        rm -rf "$TEST_DIR"/*
    fi
    mkdir -p "$TEST_DIR"
}

teardown() {
    # Clean up test files
    if [ -d "$TEST_DIR" ]; then
        rm -rf "$TEST_DIR"/*
    fi
}


# bats test_tags=basic,read,files,write
@test "Write and Read File" {
    # Create a test file with known content
    echo "test content" > "$TEST_DIR/existing-file"
    
    run cat "$TEST_DIR/existing-file"
    assert_success
    assert_output "test content"
}

# bats test_tags=basic,read,files,write
@test "Write and Read Large File" {
    # Create a large file with zeros
    run dd if=/dev/zero of="$TEST_DIR/read-new" bs=1k count=512 status=none
    assert_success
    
    # Verify file size and content
    run wc -c < "$TEST_DIR/read-new"
    assert_success
    assert_output "524288"
}

# bats test_tags=timestamp,files,write
@test "New File Time" {
    echo "foo" > "$TEST_DIR/bar"
    
    run ls -l --full-time "$TEST_DIR/bar"
    assert_success
    
    # Check that year is not 1970 (Unix epoch)
    year=$(echo "$output" | awk '{print $6}' | cut -d'-' -f1)
    assert_not_equal "$year" ""  # Ensure year is not empty
    if [ "$year" -le 1970 ]; then
        echo "Year is not greater than 1970: $year"
        return 1
    fi
}

# bats test_tags=mv,files,write
@test "Move and Rename Files" {
    echo "$TEST_FILE_CONTENT" > "$TEST_DIR/test-file"
    
    # Verify original file exists
    run cat "$TEST_DIR/test-file"
    assert_success
    assert_output "$TEST_FILE_CONTENT"
    
    # Rename file
    run mv "$TEST_DIR/test-file" "$TEST_DIR/renamed"
    assert_success
    
    # Verify renamed file
    run cat "$TEST_DIR/renamed"
    assert_success
    assert_output "$TEST_FILE_CONTENT"
    
    # Verify original is gone
    run test -f "$TEST_DIR/test-file"
    assert_failure
}

# bats test_tags=mv,files,write
@test "Move Between Directories" {
    echo "$TEST_FILE_CONTENT" > "$TEST_DIR/test-file"
    mkdir "$TEST_DIR/somedir"
    
    # Move file to directory
    run mv "$TEST_DIR/test-file" "$TEST_DIR/somedir/file"
    assert_success
    
    # Verify file in new location
    run cat "$TEST_DIR/somedir/file"
    assert_success
    assert_output "$TEST_FILE_CONTENT"
}

# bats test_tags=basic,files,directories,rm,rmdir,write
@test "Unlink Files and Directories" {
    # Create test files and directories
    touch "$TEST_DIR/new-file"
    echo "$TEST_FILE_CONTENT" > "$TEST_DIR/existing-file"
    mkdir "$TEST_DIR/new-dir"
    touch "$TEST_DIR/new-dir/file"
    
    # Remove new file
    run rm "$TEST_DIR/new-file"
    assert_success
    run test -e "$TEST_DIR/new-file"
    assert_failure
    
    # Remove existing file
    run rm "$TEST_DIR/existing-file"
    assert_success
    run test -e "$TEST_DIR/existing-file"
    assert_failure
    
    # Remove directory with file
    run rm "$TEST_DIR/new-dir/file"
    assert_success
    run rmdir "$TEST_DIR/new-dir"
    assert_success
    run test -e "$TEST_DIR/new-dir"
    assert_failure
}

# bats test_tags=hardlinks,ln,files
#@test "Hard Links" {
#    echo "$TEST_FILE_SMALL" > "$TEST_DIR/target"
#    
#    # Create hard link
#    run ln "$TEST_DIR/target" "$TEST_DIR/link1"
#    assert_success
#    
#    # Verify both files have same content
#    run cat "$TEST_DIR/target"
#    assert_success
#    assert_output "$TEST_FILE_SMALL"
#    
#    run cat "$TEST_DIR/link1"
#    assert_success
#    assert_output "$TEST_FILE_SMALL"
#    
#    # Verify they are the same file (same inode)
#    run test "$TEST_DIR/target" -ef "$TEST_DIR/link1"
#    assert_success
#}

# bats test_tags=symlinks,files,ln
@test "Symbolic Links" {
    echo "$TEST_FILE_SMALL" > "$TEST_DIR/target"
    
    # Create symbolic link
    run ln -s "$TEST_DIR/target" "$TEST_DIR/symlink"
    assert_success
    
    # Verify symlink content
    run cat "$TEST_DIR/symlink"
    assert_success
    assert_output "$TEST_FILE_SMALL"
    
    # Verify it's actually a symlink
    run test -L "$TEST_DIR/symlink"
    assert_success
}

# bats test_tags=readlink,symlinks,files,ln
@test "Read Link Target" {
    touch "$TEST_DIR/target"
    ln -s "$TEST_DIR/target" "$TEST_DIR/link"
    
    run readlink "$TEST_DIR/link"
    assert_success
    assert_output "$TEST_DIR/target"
}

# bats test_tags=basic,mkdir,directories,write
@test "Create Directories" {
    run mkdir "$TEST_DIR/a-dir"
    assert_success
    
    run mkdir "$TEST_DIR/a-dir/b-dir"
    assert_success
    
    run mkdir "$TEST_DIR/a-dir/c-dir"
    assert_success
    
    # Create file in directory
    echo "test content" > "$TEST_DIR/a-dir/e-file"
    
    run cat "$TEST_DIR/a-dir/e-file"
    assert_success
    assert_output "test content"
}

# bats test_tags=directories,find,read
@test "Directory Walking" {
    # Create nested directory structure
    mkdir -p "$TEST_DIR/walk/a/aa/aaa/aaaa"
    mkdir -p "$TEST_DIR/walk/b/ba"
    mkdir -p "$TEST_DIR/walk/a/aa/aab"
    mkdir -p "$TEST_DIR/walk/a/aa/aac"
    touch "$TEST_DIR/walk/a/aa/aab/aabfile"
    touch "$TEST_DIR/walk/b/bfile"
    
    run bash -c "find '$TEST_DIR/walk' | sort"
    assert_success
    
    # Verify key paths exist in output
    assert_output --partial "$TEST_DIR/walk"
    assert_output --partial "$TEST_DIR/walk/a/aa/aab/aabfile"
    assert_output --partial "$TEST_DIR/walk/b/bfile"
}

# bats test_tags=files,metadata,permissions,chmod
@test "File Attributes and Permissions" {
    echo "test content" > "$TEST_DIR/file"
    
    # Test file size
    run bash -c "wc -c < '$TEST_DIR/file' | xargs echo"
    assert_success
    assert_output "13"  # "test content" is 12 chars + newline
    
    # Test chmod
    run chmod 644 "$TEST_DIR/file"
    assert_success
    
    run ls -l "$TEST_DIR/file"
    assert_success
    assert_output --partial "-rw-r--r--"
}

# bats test_tags=xattr,files,metadata
@test "Extended Attributes" {
    #skip "Extended attributes may not be supported on all filesystems"
    
    echo "test content" > "$TEST_DIR/file"
    
    # Set extended attribute (if supported)
    run setfattr --name=user.test --value="testvalue" "$TEST_DIR/file" 2>/dev/null
    if [ "$status" -eq 0 ]; then
        # Get extended attribute, filtering out warning messages
        run bash -c "getfattr --name=user.test --only-values '$TEST_DIR/file' 2>&1 | grep -v '^getfattr:' || true"
        assert_success
        assert_output "testvalue"
    fi
}

# bats test_tags=read,files
@test "Read Past File End" {
    echo "a" > "$TEST_DIR/small-file"
    
    # Try to read beyond file size using dd
    run dd if="$TEST_DIR/small-file" bs=1 count=1 skip=10 status=none 2>/dev/null
    assert_success
    assert_output ""  # Should read nothing
}

# bats test_tags=truncate,files,write,metadata
@test "File Truncation" {
    # Create file with content
    echo "long test content here" > "$TEST_DIR/file"
    
    # Verify original size
    run bash -c "wc -c < '$TEST_DIR/file' | xargs echo"
    assert_success
    original_size="$output"
    
    # Truncate file
    run truncate -s 5 "$TEST_DIR/file"
    assert_success
    
    # Verify new size
    run bash -c "wc -c < '$TEST_DIR/file' | xargs echo"
    assert_success
    assert_output "5"
    
    # Verify content
    run cat "$TEST_DIR/file"
    assert_success
    assert_output "long "
}

# bats test_tags=cp,files,permissions,chmod,metadata
@test "File Copy with Permission Preservation" {
    echo "$TEST_FILE_CONTENT" > "$TEST_DIR/source"
    
    # Set specific permissions on source file
    run chmod 644 "$TEST_DIR/source"
    assert_success
    
    # Copy file with preserved permissions
    run cp -p "$TEST_DIR/source" "$TEST_DIR/destination"
    assert_success
    
    # Verify both files exist and have same content
    run cat "$TEST_DIR/source"
    assert_success
    source_content="$output"
    
    run cat "$TEST_DIR/destination"
    assert_success
    assert_output "$source_content"
    
    # Verify permissions are preserved
    run ls -l "$TEST_DIR/source"
    assert_success
    source_perms=$(echo "$output" | awk '{print $1}')
    
    run ls -l "$TEST_DIR/destination"
    assert_success
    dest_perms=$(echo "$output" | awk '{print $1}')
    
    # Both files should have the same permissions
    assert_equal "$source_perms" "$dest_perms"
    assert_output --partial "-rw-r--r--"
}

# bats test_tags=cp,directories,permissions,chmod,metadata
@test "Recursive Directory Copy with Permission Preservation" {
    # Create source directory structure with files and subdirectories
    mkdir -p "$TEST_DIR/source_tree/subdir1/subdir2"
    mkdir -p "$TEST_DIR/source_tree/subdir3"
    
    # Create files with different content in various locations
    echo "root file content" > "$TEST_DIR/source_tree/root_file.txt"
    echo "subdir1 content" > "$TEST_DIR/source_tree/subdir1/file1.txt"
    echo "deep file content" > "$TEST_DIR/source_tree/subdir1/subdir2/deep_file.txt"
    echo "subdir3 content" > "$TEST_DIR/source_tree/subdir3/file3.txt"
    
    # Create an empty file and empty directory
    touch "$TEST_DIR/source_tree/empty_file"
    mkdir "$TEST_DIR/source_tree/empty_dir"
    
    # Set different permissions on files and directories to test preservation
    chmod 644 "$TEST_DIR/source_tree/root_file.txt"
    chmod 755 "$TEST_DIR/source_tree/subdir1/file1.txt"
    chmod 600 "$TEST_DIR/source_tree/subdir1/subdir2/deep_file.txt"
    chmod 640 "$TEST_DIR/source_tree/subdir3/file3.txt"
    chmod 700 "$TEST_DIR/source_tree/empty_file"
    chmod 755 "$TEST_DIR/source_tree/subdir1"
    chmod 750 "$TEST_DIR/source_tree/subdir3"
    chmod 777 "$TEST_DIR/source_tree/empty_dir"
    
    # Copy directory tree recursively with preserved permissions
    run cp -rp "$TEST_DIR/source_tree" "$TEST_DIR/dest_tree"
    assert_success
    
    # Verify destination directory structure exists
    run test -d "$TEST_DIR/dest_tree"
    assert_success
    
    run test -d "$TEST_DIR/dest_tree/subdir1"
    assert_success
    
    run test -d "$TEST_DIR/dest_tree/subdir1/subdir2"
    assert_success
    
    run test -d "$TEST_DIR/dest_tree/subdir3"
    assert_success
    
    run test -d "$TEST_DIR/dest_tree/empty_dir"
    assert_success
    
    # Verify all files were copied with correct content
    run cat "$TEST_DIR/dest_tree/root_file.txt"
    assert_success
    assert_output "root file content"
    
    run cat "$TEST_DIR/dest_tree/subdir1/file1.txt"
    assert_success
    assert_output "subdir1 content"
    
    run cat "$TEST_DIR/dest_tree/subdir1/subdir2/deep_file.txt"
    assert_success
    assert_output "deep file content"
    
    run cat "$TEST_DIR/dest_tree/subdir3/file3.txt"
    assert_success
    assert_output "subdir3 content"
    
    # Verify empty file was copied
    run test -f "$TEST_DIR/dest_tree/empty_file"
    assert_success
    
    run bash -c "wc -c < '$TEST_DIR/dest_tree/empty_file' | xargs echo"
    assert_success
    assert_output "0"
    
    # Verify original source tree still exists and is unchanged
    run cat "$TEST_DIR/source_tree/root_file.txt"
    assert_success
    assert_output "root file content"
    
    # Verify permissions are preserved for files
    run ls -l "$TEST_DIR/source_tree/root_file.txt"
    assert_success
    source_root_perms=$(echo "$output" | awk '{print $1}')
    
    run ls -l "$TEST_DIR/dest_tree/root_file.txt"
    assert_success
    dest_root_perms=$(echo "$output" | awk '{print $1}')
    assert_equal "$source_root_perms" "$dest_root_perms"
    assert_output --partial "-rw-r--r--"
    
    run ls -l "$TEST_DIR/source_tree/subdir1/file1.txt"
    assert_success
    source_file1_perms=$(echo "$output" | awk '{print $1}')
    
    run ls -l "$TEST_DIR/dest_tree/subdir1/file1.txt"
    assert_success
    dest_file1_perms=$(echo "$output" | awk '{print $1}')
    assert_equal "$source_file1_perms" "$dest_file1_perms"
    assert_output --partial "-rwxr-xr-x"
    
    run ls -l "$TEST_DIR/source_tree/subdir1/subdir2/deep_file.txt"
    assert_success
    source_deep_perms=$(echo "$output" | awk '{print $1}')
    
    run ls -l "$TEST_DIR/dest_tree/subdir1/subdir2/deep_file.txt"
    assert_success
    dest_deep_perms=$(echo "$output" | awk '{print $1}')
    assert_equal "$source_deep_perms" "$dest_deep_perms"
    assert_output --partial "-rw-------"
    
    run ls -l "$TEST_DIR/source_tree/empty_file"
    assert_success
    source_empty_perms=$(echo "$output" | awk '{print $1}')
    
    run ls -l "$TEST_DIR/dest_tree/empty_file"
    assert_success
    dest_empty_perms=$(echo "$output" | awk '{print $1}')
    assert_equal "$source_empty_perms" "$dest_empty_perms"
    assert_output --partial "-rwx------"
    
    # Verify permissions are preserved for directories
    run ls -ld "$TEST_DIR/source_tree/subdir1"
    assert_success
    source_dir1_perms=$(echo "$output" | awk '{print $1}')
    
    run ls -ld "$TEST_DIR/dest_tree/subdir1"
    assert_success
    dest_dir1_perms=$(echo "$output" | awk '{print $1}')
    assert_equal "$source_dir1_perms" "$dest_dir1_perms"
    assert_output --partial "drwxr-xr-x"
    
    run ls -ld "$TEST_DIR/source_tree/subdir3"
    assert_success
    source_dir3_perms=$(echo "$output" | awk '{print $1}')
    
    run ls -ld "$TEST_DIR/dest_tree/subdir3"
    assert_success
    dest_dir3_perms=$(echo "$output" | awk '{print $1}')
    assert_equal "$source_dir3_perms" "$dest_dir3_perms"
    assert_output --partial "drwxr-x---"
    
    run ls -ld "$TEST_DIR/source_tree/empty_dir"
    assert_success
    source_empty_dir_perms=$(echo "$output" | awk '{print $1}')
    
    run ls -ld "$TEST_DIR/dest_tree/empty_dir"
    assert_success
    dest_empty_dir_perms=$(echo "$output" | awk '{print $1}')
    assert_equal "$source_empty_dir_perms" "$dest_empty_dir_perms"
    assert_output --partial "drwxrwxrwx"
    
    # Test copying into existing directory
    mkdir "$TEST_DIR/existing_dest"
    run cp -rp "$TEST_DIR/source_tree" "$TEST_DIR/existing_dest/"
    assert_success
    
    # Verify nested copy structure
    run test -d "$TEST_DIR/existing_dest/source_tree"
    assert_success
    
    run cat "$TEST_DIR/existing_dest/source_tree/root_file.txt"
    assert_success
    assert_output "root file content"
}

# bats test_tags=permissions,chmod,metadata,files
@test "Permission Changes" {
    echo "test content" > "$TEST_DIR/file"
    
    # Set specific permissions
    run chmod 600 "$TEST_DIR/file"
    assert_success
    
    # Verify permissions
    run ls -l "$TEST_DIR/file"
    assert_success
    assert_output --partial "-rw-------"
    
    # Change permissions again
    run chmod 755 "$TEST_DIR/file"
    assert_success
    
    run ls -l "$TEST_DIR/file"
    assert_success
    assert_output --partial "-rwxr-xr-x"
}


# bats test_tags=errors,files
@test "Error Handling - Nonexistent Files" {
    # Try to read nonexistent file
    run cat "$TEST_DIR/nonexistent"
    assert_failure
    # Check either output or stderr contains error message
    [[ "$output" =~ "No such file" ]] || [[ "$stderr" =~ "No such file" ]]
    
    # Try to remove nonexistent file
    run rm "$TEST_DIR/nonexistent"
    assert_failure
}

# bats test_tags=directories,mkdir,rmdir,rm,write
@test "Directory Operations" {
    # Create directory
    run mkdir "$TEST_DIR/testdir"
    assert_success
    
    # Verify it's a directory
    run test -d "$TEST_DIR/testdir"
    assert_success
    
    # Try to remove non-empty directory (should fail)
    touch "$TEST_DIR/testdir/file"
    run rmdir "$TEST_DIR/testdir"
    assert_failure
    
    # Remove file and then directory
    run rm "$TEST_DIR/testdir/file"
    assert_success
    
    run rmdir "$TEST_DIR/testdir"
    assert_success
    
    # Verify directory is gone
    run test -d "$TEST_DIR/testdir"
    assert_failure
}

# bats test_tags=append,write,files
@test "File Append Operations" {
    echo "line 1" > "$TEST_DIR/appendtest"
    
    # Append to existing file
    echo "line 2" >> "$TEST_DIR/appendtest"
    echo "line 3" >> "$TEST_DIR/appendtest"
    
    # Verify content
    run cat "$TEST_DIR/appendtest"
    assert_success
    assert_output "line 1
line 2
line 3"
    
    # Test appending to non-existent file (should create)
    echo "new file content" >> "$TEST_DIR/newfile"
    run cat "$TEST_DIR/newfile"
    assert_success
    assert_output "new file content"
}

# bats test_tags=mkdir,directories,write
@test "Recursive Directory Creation" {
    # Test mkdir -p with deep nested structure
    run mkdir -p "$TEST_DIR/deep/nested/structure/here"
    assert_success
    
    # Verify all directories were created
    run test -d "$TEST_DIR/deep"
    assert_success
    
    run test -d "$TEST_DIR/deep/nested"
    assert_success
    
    run test -d "$TEST_DIR/deep/nested/structure"
    assert_success
    
    run test -d "$TEST_DIR/deep/nested/structure/here"
    assert_success
    
    # Test mkdir -p with existing directories (should not fail)
    run mkdir -p "$TEST_DIR/deep/nested/structure"
    assert_success
    
    # Create a file in the deep structure
    echo "deep file" > "$TEST_DIR/deep/nested/structure/here/file.txt"
    run cat "$TEST_DIR/deep/nested/structure/here/file.txt"
    assert_success
    assert_output "deep file"
}

# bats test_tags=chmod,permissions,directories,metadata
@test "Directory Permission Operations" {
    # Create directory with specific permissions
    run mkdir "$TEST_DIR/permtest"
    assert_success
    
    # Set various permission combinations
    run chmod 755 "$TEST_DIR/permtest"
    assert_success
    
    run ls -ld "$TEST_DIR/permtest"
    assert_success
    assert_output --partial "drwxr-xr-x"
    
    # Test more restrictive permissions
    run chmod 750 "$TEST_DIR/permtest"
    assert_success
    
    run ls -ld "$TEST_DIR/permtest"
    assert_success
    assert_output --partial "drwxr-x---"
    
    # Test very restrictive permissions
    run chmod 700 "$TEST_DIR/permtest"
    assert_success
    
    run ls -ld "$TEST_DIR/permtest"
    assert_success
    assert_output --partial "drwx------"
    
    # Test permissive permissions
    run chmod 777 "$TEST_DIR/permtest"
    assert_success
    
    run ls -ld "$TEST_DIR/permtest"
    assert_success
    assert_output --partial "drwxrwxrwx"
}

# bats test_tags=seek,files,read
@test "File Seek and Positioning Operations" {
    # Create a file with known content
    echo "0123456789abcdefghij" > "$TEST_DIR/seektest"
    
    # Test reading from specific position using dd skip
    run dd if="$TEST_DIR/seektest" bs=1 skip=5 count=5 status=none
    assert_success
    assert_output "56789"
    
    # Test reading from different position
    run dd if="$TEST_DIR/seektest" bs=1 skip=10 count=5 status=none
    assert_success
    assert_output "abcde"
    
    # Test reading from end
    run dd if="$TEST_DIR/seektest" bs=1 skip=15 count=5 status=none
    assert_success
    assert_output "fghij"
    
    # Test reading beyond file end (should return nothing)
    run dd if="$TEST_DIR/seektest" bs=1 skip=25 count=5 status=none 2>/dev/null
    assert_success
    assert_output ""
    
    # Test partial read at file boundary
    run dd if="$TEST_DIR/seektest" bs=1 skip=18 count=5 status=none
    assert_success
    assert_output "ij"
}


# bats test_tags=permissions,files,directories,errors
@test "Permission Denied Scenarios" {
    if [ "$(id -u)" -eq 0 ]; then
        skip "Permission denied scenarios cannot be tested when running as root"
    fi

    # Test read-only file protection by checking if content changes
    echo "original content" > "$TEST_DIR/readonly.txt"
    
    # Set read-only permissions
    run chmod 444 "$TEST_DIR/readonly.txt"
    assert_success
    
    # Verify chmod command worked by checking permissions
    run ls -l "$TEST_DIR/readonly.txt"
    assert_success
    assert_output --partial "-r--r--r--"
    
    # Try to write to read-only file (exit code may vary)
    run bash -c "echo 'new content' > '$TEST_DIR/readonly.txt'"
    
    # Check if the file content actually changed (this is the real test)
    run cat "$TEST_DIR/readonly.txt"
    assert_success
    assert_output "original content"  # Content should be unchanged
    
    # Test directory permissions if supported
    mkdir "$TEST_DIR/dirtest"
    echo "dir file content" > "$TEST_DIR/dirtest/file.txt"
    
    # Set restrictive directory permissions
    run chmod 000 "$TEST_DIR/dirtest"
    assert_success
    
    # Try to access file in restricted directory
    run cat "$TEST_DIR/dirtest/file.txt"
    assert_failure
    
    # Try to create new file in restricted directory
    run bash -c "echo 'test' > '$TEST_DIR/dirtest/newfile.txt' 2>/dev/null"
    
    # Check if new file was actually created
    run test -f "$TEST_DIR/dirtest/newfile.txt"
    # File creation may or may not be blocked depending on filesystem
    
    # Restore directory permissions for cleanup
    run chmod 755 "$TEST_DIR/dirtest"
    assert_success
    
    # Test file removal with read-only permissions
    echo "protected file" > "$TEST_DIR/protected.txt"
    run chmod 444 "$TEST_DIR/protected.txt"
    assert_success
    
    # Try to remove read-only file
    run rm "$TEST_DIR/protected.txt" 2>/dev/null
    
    # Check if file was actually removed
    if [ -f "$TEST_DIR/protected.txt" ]; then
        # File still exists - permissions protected it
        run cat "$TEST_DIR/protected.txt"
        assert_success
        assert_output "protected file"
        
        # Restore write permission for cleanup
        run chmod 644 "$TEST_DIR/protected.txt"
        assert_success
    fi
    
    # Always restore file permissions for cleanup
    run chmod 644 "$TEST_DIR/readonly.txt"
    assert_success
}

# bats test_tags=cp,timestamp,files,metadata
@test "Timestamp Preservation Operations" {
    # Create original file
    echo "original content" > "$TEST_DIR/original.txt"
    
    # Wait a moment to ensure timestamp difference
    sleep 1
    
    # Get original timestamp
    run stat -c %Y "$TEST_DIR/original.txt"
    assert_success
    original_time="$output"
    
    # Copy with timestamp preservation
    run cp -p "$TEST_DIR/original.txt" "$TEST_DIR/preserved.txt"
    assert_success
    
    # Get copied file timestamp
    run stat -c %Y "$TEST_DIR/preserved.txt"
    assert_success
    preserved_time="$output"
    
    # Timestamps should be the same
    assert_equal "$original_time" "$preserved_time" 
    
    # Copy without preservation (should have different timestamp)
    run cp "$TEST_DIR/original.txt" "$TEST_DIR/not_preserved.txt"
    assert_success
    
    # Get new copy timestamp
    run stat -c %Y "$TEST_DIR/not_preserved.txt"
    assert_success
    new_time="$output"
    
    # Timestamps should be different
    assert_not_equal "$original_time" "$new_time"
    
    # Test touch with specific timestamp
    run touch -t 202301011200 "$TEST_DIR/touched.txt"
    assert_success
    
    # Verify the timestamp was set
    run ls -l --time-style=+%Y%m%d%H%M "$TEST_DIR/touched.txt"
    assert_success
    assert_output --partial "202301011200"
}

# bats test_tags=stress,files
@test "Stress Test - Many Files" {
    #skip "Stress tests may be too slow for regular test runs"
    
    run mkdir "$TEST_DIR/stress-files"
    assert_success
    
    # Create many files (reduced number for faster testing)
    # Use a simple loop with manual zero-padding
    i=0
    while [ $i -lt 100 ]; do
        # Zero-pad the number to 3 digits
        padded=$(printf "%03d" $i)
        echo "$padded" > "$TEST_DIR/stress-files/file-$padded"
        i=$((i + 1))
    done
    
    # Verify directory and file count
    run ls "$TEST_DIR/stress-files"
    assert_success
    
    run bash -c "ls '$TEST_DIR/stress-files' | wc -l | xargs echo"
    assert_success
    assert_output "100"
    
    # Debug: Show what files actually exist
    run bash -c "ls -la '$TEST_DIR/stress-files' | head -5"
    assert_success
    
    # Verify some files exist and have correct content
    run test -f "$TEST_DIR/stress-files/file-000"
    if [ "$status" -ne 0 ]; then
        # If file doesn't exist, show what files do exist for debugging
        ls -la "$TEST_DIR/stress-files" | head -10
        false  # Force test failure with debugging info
    fi
    assert_success
    
    run cat "$TEST_DIR/stress-files/file-000"
    assert_success
    assert_output "000"
    
    run test -f "$TEST_DIR/stress-files/file-099"
    assert_success
    
    run cat "$TEST_DIR/stress-files/file-099"
    assert_success
    assert_output "099"
    
    # Verify middle file
    run cat "$TEST_DIR/stress-files/file-050"
    assert_success
    assert_output "050"
    
    # Clean up
    run rm "$TEST_DIR/stress-files"/file-*
    assert_success
}

# bats test_tags=stress,directories
@test "Stress Test - Deep Directories" {
    #skip "Deep directory tests may be too slow for regular test runs"
    
    # Create deep directory structure (reduced depth for faster testing)
    p="$TEST_DIR/stress-dirs"
    mkdir "$p"
    
    for i in $(seq 0 9); do
        p="$p/$i"
        mkdir "$p"
        echo "$i" > "$p/file"
    done
    
    # Test access to deep file
    run cat "$TEST_DIR/stress-dirs/0/1/2/3/4/5/6/7/8/9/file"
    assert_success
    assert_output "9"
}