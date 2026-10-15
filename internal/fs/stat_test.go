package fs_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gtkpad/devclean/internal/fs"
)

func TestDirExists(t *testing.T) {
	// 1. Caminho vazio
	if fs.DirExists("") {
		t.Errorf("DirExists(\"\") should return false, got true")
	}

	// 2. Diretório existente
	tempDir := t.TempDir()
	if !fs.DirExists(tempDir) {
		t.Errorf("DirExists(%q) should return true, got false", tempDir)
	}

	// 3. Arquivo regular existente
	regularFile := filepath.Join(tempDir, "testfile.txt")
	if err := os.WriteFile(regularFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	if fs.DirExists(regularFile) {
		t.Errorf("DirExists(%q) on regular file should return false, got true", regularFile)
	}

	// 4. Caminho inexistente
	nonExistent := filepath.Join(tempDir, "non_existent_folder")
	if fs.DirExists(nonExistent) {
		t.Errorf("DirExists(%q) should return false, got true", nonExistent)
	}

	// 5. Symlink quebrado
	brokenSymlink := filepath.Join(tempDir, "broken_link")
	if err := os.Symlink(filepath.Join(tempDir, "target_does_not_exist"), brokenSymlink); err != nil {
		t.Fatalf("failed to create broken symlink: %v", err)
	}
	if fs.DirExists(brokenSymlink) {
		t.Errorf("DirExists(%q) on broken symlink should return false, got true", brokenSymlink)
	}

	// 6. Symlink válido para diretório (deve retornar true pois os.Stat segue symlinks)
	targetDir := filepath.Join(tempDir, "real_target_dir")
	if err := os.Mkdir(targetDir, 0755); err != nil {
		t.Fatalf("failed to create target dir: %v", err)
	}
	validSymlink := filepath.Join(tempDir, "valid_symlink")
	if err := os.Symlink(targetDir, validSymlink); err != nil {
		t.Fatalf("failed to create valid symlink: %v", err)
	}
	if !fs.DirExists(validSymlink) {
		t.Errorf("DirExists(%q) on valid symlink to dir should return true, got false", validSymlink)
	}
}
