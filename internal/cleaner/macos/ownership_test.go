package macos

import (
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"
)

type mockFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
	sys     any
}

func (m mockFileInfo) Name() string       { return m.name }
func (m mockFileInfo) Size() int64        { return m.size }
func (m mockFileInfo) Mode() os.FileMode  { return m.mode }
func (m mockFileInfo) ModTime() time.Time { return m.modTime }
func (m mockFileInfo) IsDir() bool        { return m.isDir }
func (m mockFileInfo) Sys() any           { return m.sys }

func TestOwnership_CurrentUser(t *testing.T) {
	tmpFile, err := os.CreateTemp(t.TempDir(), "devclean-ownership-*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer tmpFile.Close()

	info, err := tmpFile.Stat()
	if err != nil {
		t.Fatalf("failed to stat temp file: %v", err)
	}

	if !isOwnedByCurrentUser(info) {
		t.Errorf("expected isOwnedByCurrentUser to return true for current user temp file, got false")
	}
}

func TestOwnership_MismatchedUID(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("skipping Darwin-specific Stat_t test on non-Darwin")
	}

	currentUID := uint32(os.Getuid())
	mismatchedUID := currentUID + 9999

	mockInfo := mockFileInfo{
		name: "foreign_file",
		sys:  &syscall.Stat_t{Uid: mismatchedUID},
	}

	if isOwnedByCurrentUser(mockInfo) {
		t.Errorf("expected isOwnedByCurrentUser to return false for mismatched UID %d, got true", mismatchedUID)
	}
}

func TestSymlinkGuard_ExternalReference(t *testing.T) {
	tempDir := t.TempDir()

	cacheRoot := filepath.Join(tempDir, "caches")
	externalDir := filepath.Join(tempDir, "external")
	if err := os.MkdirAll(cacheRoot, 0755); err != nil {
		t.Fatalf("failed to create cache root: %v", err)
	}
	if err := os.MkdirAll(externalDir, 0755); err != nil {
		t.Fatalf("failed to create external dir: %v", err)
	}

	// 1. External file target
	externalFile := filepath.Join(externalDir, "secret.txt")
	if err := os.WriteFile(externalFile, []byte("secret"), 0644); err != nil {
		t.Fatalf("failed to write external file: %v", err)
	}

	// Symlink in cacheRoot pointing outside to externalFile
	escapingSymlink := filepath.Join(cacheRoot, "escaping_link")
	if err := os.Symlink(externalFile, escapingSymlink); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	if isSafeSymlink(cacheRoot, escapingSymlink) {
		t.Errorf("expected isSafeSymlink to reject escaping symlink pointing to %s, got true", externalFile)
	}

	// 2. Internal file target (safe)
	internalFile := filepath.Join(cacheRoot, "internal.txt")
	if err := os.WriteFile(internalFile, []byte("cached data"), 0644); err != nil {
		t.Fatalf("failed to write internal file: %v", err)
	}

	internalSymlink := filepath.Join(cacheRoot, "internal_link")
	if err := os.Symlink(internalFile, internalSymlink); err != nil {
		t.Fatalf("failed to create internal symlink: %v", err)
	}

	if !isSafeSymlink(cacheRoot, internalSymlink) {
		t.Errorf("expected isSafeSymlink to accept internal symlink pointing to %s, got false", internalFile)
	}

	// 3. Symlink pointing to system root or blacklisted location
	systemSymlink := filepath.Join(cacheRoot, "system_link")
	_ = os.Symlink("/System", systemSymlink)
	if isSafeSymlink(cacheRoot, systemSymlink) {
		t.Errorf("expected isSafeSymlink to reject symlink pointing to /System, got true")
	}

	// 4. Broken symlink
	brokenSymlink := filepath.Join(cacheRoot, "broken_link")
	_ = os.Symlink(filepath.Join(cacheRoot, "nonexistent"), brokenSymlink)
	if isSafeSymlink(cacheRoot, brokenSymlink) {
		t.Errorf("expected isSafeSymlink to reject broken symlink, got true")
	}
}
