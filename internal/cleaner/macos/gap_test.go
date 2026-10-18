package macos

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gtkpad/devclean/internal/domain"
)

func TestMacOSCleaner_Constructors(t *testing.T) {
	c1 := New()
	if c1.Name() != "macos" {
		t.Errorf("New().Name() = %q, want 'macos'", c1.Name())
	}

	mockExec := newMockProcessExecutor()
	c2 := NewWithExecutor(mockExec)
	if c2.Name() != "macos" {
		t.Errorf("NewWithExecutor().Name() = %q, want 'macos'", c2.Name())
	}
}

func TestMacOSCleaner_ResolveTargetBundleID(t *testing.T) {
	tests := []struct {
		name     string
		target   domain.Target
		expected string
	}{
		{
			name:     "spotify cache category",
			target:   domain.Target{Category: "macos-spotify-cache", ID: "some-id", Path: "/path/to/spotify"},
			expected: "com.spotify.client",
		},
		{
			name:     "code sign clones target ID",
			target:   domain.Target{ID: "macos-code-sign-clones", Path: "/path/to/clones"},
			expected: "com.google.Chrome.code_sign_clone",
		},
		{
			name:     "safari cache ID prefix",
			target:   domain.Target{ID: "macos-cache-com.apple.Safari", Path: "/path/to/safari"},
			expected: "com.apple.Safari",
		},
		{
			name:     "xcode log ID prefix",
			target:   domain.Target{ID: "macos-log-com.apple.dt.Xcode", Path: "/path/to/xcode"},
			expected: "com.apple.dt.Xcode",
		},
		{
			name:     "daemon siri ID",
			target:   domain.Target{ID: "macos-daemon-siri", Path: "/path/to/siri"},
			expected: "com.apple.siri.context",
		},
		{
			name:     "daemon unknown ID",
			target:   domain.Target{ID: "macos-daemon-unknown", Path: "/path/to/unknown"},
			expected: "unknown",
		},
		{
			name:     "custom fallback to base path",
			target:   domain.Target{ID: "custom", Path: "/Library/Caches/sample-app"},
			expected: "sample-app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveTargetBundleID(tt.target)
			if got != tt.expected {
				t.Errorf("resolveTargetBundleID(%+v) = %q, want %q", tt.target, got, tt.expected)
			}
		})
	}
}

func TestMacOSCleaner_Clean_EdgeCases(t *testing.T) {
	tempDir := t.TempDir()
	mockHome := filepath.Join(tempDir, "mock_home")
	mockExec := newMockProcessExecutor()
	c := NewWithDirAndExecutor(mockHome, mockExec)

	// 1. Context cancellation check
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := c.Clean(ctx, []domain.Target{{Path: filepath.Join(mockHome, "sample")}})
	if err == nil {
		t.Fatalf("expected error on canceled context, got nil")
	}

	// 2. Non-existent target is safely ignored without error
	err = c.Clean(context.Background(), []domain.Target{{Path: filepath.Join(mockHome, "nonexistent")}})
	if err != nil {
		t.Fatalf("unexpected error on missing target: %v", err)
	}

	// 3. ReviewOnly target is skipped without mutation
	reviewTargetDir := filepath.Join(mockHome, "review_only")
	_ = os.MkdirAll(reviewTargetDir, 0755)
	reviewFile := filepath.Join(reviewTargetDir, "sentinel.txt")
	_ = os.WriteFile(reviewFile, []byte("INVIOLABLE"), 0644)

	err = c.Clean(context.Background(), []domain.Target{
		{
			Path:       reviewTargetDir,
			ReviewOnly: true,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error on ReviewOnly target: %v", err)
	}
	if _, err := os.Stat(reviewFile); err != nil {
		t.Fatalf("ReviewOnly target was mutated: %v", err)
	}

	// 4. Symlink target is skipped (Symlink Guard)
	realDir := filepath.Join(mockHome, "real_dir")
	_ = os.MkdirAll(realDir, 0755)
	realFile := filepath.Join(realDir, "data.txt")
	_ = os.WriteFile(realFile, []byte("protected_content"), 0644)

	symlinkTarget := filepath.Join(mockHome, "symlink_dir")
	_ = os.Symlink(realDir, symlinkTarget)

	err = c.Clean(context.Background(), []domain.Target{{Path: symlinkTarget}})
	if err != nil {
		t.Fatalf("unexpected error on symlink target: %v", err)
	}
	if _, err := os.Lstat(symlinkTarget); os.IsNotExist(err) {
		t.Fatalf("symlink target should not be physically removed")
	}
	if _, err := os.Stat(realFile); err != nil {
		t.Fatalf("real file behind symlink was mutated: %v", err)
	}

	// 5. System blacklisted or whitelisted path is skipped
	blacklistedTarget := filepath.Join(mockHome, ".ssh")
	_ = os.MkdirAll(blacklistedTarget, 0700)
	keyFile := filepath.Join(blacklistedTarget, "id_rsa")
	_ = os.WriteFile(keyFile, []byte("secret_key"), 0600)

	err = c.Clean(context.Background(), []domain.Target{{Path: blacklistedTarget}})
	if err != nil {
		t.Fatalf("unexpected error on blacklisted target: %v", err)
	}
	if _, err := os.Stat(keyFile); err != nil {
		t.Fatalf("blacklisted file was removed: %v", err)
	}

	// 6. Normal target successfully cleaned
	normalCleanDir := filepath.Join(mockHome, "normal_clean")
	_ = os.MkdirAll(normalCleanDir, 0755)
	dataFile := filepath.Join(normalCleanDir, "data.bin")
	_ = os.WriteFile(dataFile, []byte("clean_me"), 0644)

	err = c.Clean(context.Background(), []domain.Target{{Path: normalCleanDir}})
	if err != nil {
		t.Fatalf("unexpected error on normal target clean: %v", err)
	}
	if _, err := os.Stat(normalCleanDir); !os.IsNotExist(err) {
		t.Fatalf("expected normal target to be deleted, but still exists")
	}

	// 7. Trash target error propagation in Clean
	dummyTrashFile := filepath.Join(tempDir, "file_as_trash_target")
	_ = os.WriteFile(dummyTrashFile, []byte("not_a_dir"), 0644)
	err = c.Clean(context.Background(), []domain.Target{{Category: "macos-trash", Path: dummyTrashFile}})
	if err == nil {
		t.Fatalf("expected error from Clean with invalid trash target, got nil")
	}
}

func TestMacOSCleaner_CleanTrash_Errors(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Non-existent trash directory returns nil cleanly
	err := cleanTrash(context.Background(), filepath.Join(tempDir, "nonexistent_trash"))
	if err != nil {
		t.Errorf("cleanTrash on nonexistent dir = %v, want nil", err)
	}

	// 2. Trash directory pointing to a regular file triggers ReadDir error
	regularFile := filepath.Join(tempDir, "file_as_trash")
	if err := os.WriteFile(regularFile, []byte("not_a_dir"), 0644); err != nil {
		t.Fatalf("failed to create dummy file: %v", err)
	}
	err = cleanTrash(context.Background(), regularFile)
	if err == nil {
		t.Errorf("cleanTrash on regular file should return error, got nil")
	}

	// 3. Clean trash with canceled context
	trashDir := filepath.Join(tempDir, "canceled_trash")
	_ = os.MkdirAll(trashDir, 0755)
	_ = os.WriteFile(filepath.Join(trashDir, "item1.txt"), []byte("data"), 0644)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = cleanTrash(ctx, trashDir)
	if err == nil {
		t.Errorf("cleanTrash with canceled context should return error, got nil")
	}

	// 4. Trash directory with read-only permissions preventing unlink of child
	roTrashDir := filepath.Join(tempDir, "ro_trash")
	_ = os.MkdirAll(roTrashDir, 0755)
	childItem := filepath.Join(roTrashDir, "locked_child")
	_ = os.WriteFile(childItem, []byte("cannot_delete"), 0644)
	// Remove write permission from parent roTrashDir
	_ = os.Chmod(roTrashDir, 0555)
	defer os.Chmod(roTrashDir, 0755) // cleanup

	err = cleanTrash(context.Background(), roTrashDir)
	if err == nil {
		t.Logf("Note: running as root may bypass directory write permission")
	} else {
		t.Logf("cleanTrash returned expected removal error: %v", err)
	}
}

func TestMacOSCleaner_ScanDirectoryWithTCCFallback(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Normal directory
	normalDir := filepath.Join(tempDir, "normal")
	_ = os.MkdirAll(normalDir, 0755)
	file1 := filepath.Join(normalDir, "file1.txt")
	_ = os.WriteFile(file1, []byte("hello world"), 0644) // 11 bytes

	size, partial, note := scanDirectoryWithTCCFallback(context.Background(), normalDir)
	if size != 11 {
		t.Errorf("scanDirectoryWithTCCFallback size = %d, want 11", size)
	}
	if partial {
		t.Errorf("scanDirectoryWithTCCFallback partial = true, want false")
	}
	if note != "" {
		t.Errorf("scanDirectoryWithTCCFallback note = %q, want empty", note)
	}

	// 2. Canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, _ = scanDirectoryWithTCCFallback(ctx, normalDir)

	// 3. Inaccessible directory (simulating permission denied)
	restrictedDir := filepath.Join(tempDir, "restricted")
	_ = os.MkdirAll(restrictedDir, 0755)
	subDir := filepath.Join(restrictedDir, "noperm")
	_ = os.MkdirAll(subDir, 0000)
	defer os.Chmod(subDir, 0755) // cleanup

	_, part, n := scanDirectoryWithTCCFallback(context.Background(), restrictedDir)
	_ = part
	_ = n
}

func TestMacOSCleaner_Whitelist_IsSafeSymlink(t *testing.T) {
	tempDir := t.TempDir()
	parent := filepath.Join(tempDir, "parent")
	_ = os.MkdirAll(parent, 0755)

	child := filepath.Join(parent, "child")
	_ = os.MkdirAll(child, 0755)

	outside := filepath.Join(tempDir, "outside")
	_ = os.MkdirAll(outside, 0755)

	goodLink := filepath.Join(parent, "good")
	_ = os.Symlink(child, goodLink)

	badLink := filepath.Join(parent, "bad")
	_ = os.Symlink(outside, badLink)

	if !isSafeSymlink(parent, goodLink) {
		t.Errorf("isSafeSymlink(parent, goodLink) = false, want true")
	}
	if isSafeSymlink(parent, badLink) {
		t.Errorf("isSafeSymlink(parent, badLink) = true, want false")
	}
}
