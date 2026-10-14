package analyzer_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/analyzer"
	"github.com/gtkpad/devclean/internal/safety"
)

func TestNode_Methods(t *testing.T) {
	dirNode := &analyzer.Node{
		Path: "/test/path/dir",
		Name: "dir",
		Type: analyzer.ItemDir,
	}
	if !dirNode.IsDir() {
		t.Error("expected IsDir() to be true for ItemDir")
	}
	if dirNode.IsSymlink() {
		t.Error("expected IsSymlink() to be false for ItemDir")
	}
	if got := dirNode.Breadcrumbs(); got != "/test/path/dir/" {
		t.Errorf("expected breadcrumbs '/test/path/dir/', got %q", got)
	}

	symNode := &analyzer.Node{
		Path: "/test/path/link",
		Name: "link",
		Type: analyzer.ItemSymlink,
	}
	if symNode.IsDir() {
		t.Error("expected IsDir() to be false for ItemSymlink")
	}
	if !symNode.IsSymlink() {
		t.Error("expected IsSymlink() to be true for ItemSymlink")
	}

	rootNode := &analyzer.Node{
		Path: "/",
		Name: "/",
		Type: analyzer.ItemDir,
	}
	if got := rootNode.Breadcrumbs(); got != "/" {
		t.Errorf("expected breadcrumbs '/' for root, got %q", got)
	}
}

func TestScanImmediate_BasicDir(t *testing.T) {
	tmpDir := t.TempDir()

	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	file1 := filepath.Join(tmpDir, "file1.txt")
	if err := os.WriteFile(file1, []byte("hello world"), 0o644); err != nil {
		t.Fatalf("write file1 failed: %v", err)
	}

	subFile := filepath.Join(subDir, "file2.txt")
	if err := os.WriteFile(subFile, []byte("nested content"), 0o644); err != nil {
		t.Fatalf("write subFile failed: %v", err)
	}

	bl := safety.NewBlacklist(nil)
	scanner := analyzer.NewScanner(bl, true, 0, 10)

	node, err := scanner.ScanImmediate(tmpDir)
	if err != nil {
		t.Fatalf("ScanImmediate failed: %v", err)
	}

	if node == nil {
		t.Fatal("expected non-nil root node")
	}
	if !node.IsDir() {
		t.Errorf("expected root node to be Dir, got %v", node.Type)
	}
	if len(node.Children) != 2 {
		t.Fatalf("expected 2 children in root, got %d", len(node.Children))
	}

	var foundSubdir, foundFile1 bool
	for _, child := range node.Children {
		if child.Name == "subdir" {
			foundSubdir = true
			if !child.IsDir() {
				t.Errorf("expected subdir to be Dir")
			}
			if child.Parent != node {
				t.Errorf("expected child Parent to point to root node")
			}
		}
		if child.Name == "file1.txt" {
			foundFile1 = true
			if child.IsDir() {
				t.Errorf("expected file1.txt to be File")
			}
			if child.SizeBytes != int64(len("hello world")) {
				t.Errorf("expected size %d, got %d", len("hello world"), child.SizeBytes)
			}
			if !child.Scanned {
				t.Errorf("expected file child to be marked Scanned=true")
			}
			if child.ItemCount != 1 {
				t.Errorf("expected file child ItemCount=1, got %d", child.ItemCount)
			}
		}
	}
	if !foundSubdir || !foundFile1 {
		t.Errorf("missing children: foundSubdir=%v, foundFile1=%v", foundSubdir, foundFile1)
	}
}

func TestScanner_LargeFilesDetection(t *testing.T) {
	tmpDir := t.TempDir()

	largeFilePath := filepath.Join(tmpDir, "large_archive.tar")
	f, err := os.Create(largeFilePath)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	// Create sparse file >= 100 MB (105 MB)
	if err := f.Truncate(105 * 1024 * 1024); err != nil {
		f.Close()
		t.Fatalf("truncate failed: %v", err)
	}
	f.Close()

	smallFilePath := filepath.Join(tmpDir, "small.txt")
	if err := os.WriteFile(smallFilePath, []byte("small file"), 0o644); err != nil {
		t.Fatalf("write small file failed: %v", err)
	}

	scanner := analyzer.NewScanner(nil, true, 0, 10)
	node, err := scanner.ScanImmediate(tmpDir)
	if err != nil {
		t.Fatalf("ScanImmediate failed: %v", err)
	}

	for _, child := range node.Children {
		if child.Name == "large_archive.tar" {
			if !child.IsLargeFile {
				t.Errorf("expected IsLargeFile to be true for %s", child.Name)
			}
		}
		if child.Name == "small.txt" {
			if child.IsLargeFile {
				t.Errorf("expected IsLargeFile to be false for %s", child.Name)
			}
		}
	}
}

func TestScanner_CacheHit(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "sample.txt")
	if err := os.WriteFile(testFile, []byte("cache test"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	scanner := analyzer.NewScanner(nil, true, 0, 10)
	node1, err := scanner.ScanImmediate(tmpDir)
	if err != nil {
		t.Fatalf("first scan failed: %v", err)
	}

	node2, err := scanner.ScanImmediate(tmpDir)
	if err != nil {
		t.Fatalf("second scan failed: %v", err)
	}

	if node1 != node2 {
		t.Errorf("expected cache hit to return exact same pointer (%p != %p)", node1, node2)
	}
}

func TestScanner_ProtectedStatus(t *testing.T) {
	tmpDir := t.TempDir()

	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.Mkdir(gitDir, 0o755); err != nil {
		t.Fatalf("mkdir .git failed: %v", err)
	}

	envFile := filepath.Join(tmpDir, ".env")
	if err := os.WriteFile(envFile, []byte("SECRET=1"), 0o600); err != nil {
		t.Fatalf("write .env failed: %v", err)
	}

	normalDir := filepath.Join(tmpDir, "src")
	if err := os.Mkdir(normalDir, 0o755); err != nil {
		t.Fatalf("mkdir src failed: %v", err)
	}

	bl := safety.NewBlacklist([]string{"*.secret"})
	scanner := analyzer.NewScanner(bl, true, 0, 10)
	node, err := scanner.ScanImmediate(tmpDir)
	if err != nil {
		t.Fatalf("ScanImmediate failed: %v", err)
	}

	for _, child := range node.Children {
		switch child.Name {
		case ".git":
			if !child.IsProtected {
				t.Errorf("expected .git to have IsProtected=true")
			}
		case ".env":
			if !child.IsProtected {
				t.Errorf("expected .env to have IsProtected=true")
			}
		case "src":
			if child.IsProtected {
				t.Errorf("expected src to have IsProtected=false")
			}
		}
	}
}

func TestScanImmediate_FileTarget(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "single.txt")
	if err := os.WriteFile(filePath, []byte("single file content"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	scanner := analyzer.NewScanner(nil, true, 0, 10)
	node, err := scanner.ScanImmediate(filePath)
	if err != nil {
		t.Fatalf("ScanImmediate on file failed: %v", err)
	}
	if node.IsDir() {
		t.Error("expected node to not be dir")
	}
	if node.Type != analyzer.ItemFile {
		t.Errorf("expected ItemFile, got %v", node.Type)
	}
	if node.SizeBytes != int64(len("single file content")) {
		t.Errorf("expected size %d, got %d", len("single file content"), node.SizeBytes)
	}
}

func TestScanImmediate_HiddenFilesFilter(t *testing.T) {
	tmpDir := t.TempDir()
	hiddenFile := filepath.Join(tmpDir, ".hidden")
	if err := os.WriteFile(hiddenFile, []byte("hidden"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	visibleFile := filepath.Join(tmpDir, "visible.txt")
	if err := os.WriteFile(visibleFile, []byte("visible"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	scanner := analyzer.NewScanner(nil, false, 0, 10) // includeHidden = false
	node, err := scanner.ScanImmediate(tmpDir)
	if err != nil {
		t.Fatalf("ScanImmediate failed: %v", err)
	}

	for _, child := range node.Children {
		if child.Name == ".hidden" {
			t.Errorf("expected .hidden to be excluded when includeHidden is false")
		}
	}
}

func TestScanImmediate_NonExistentPath(t *testing.T) {
	scanner := analyzer.NewScanner(nil, true, 0, 10)
	_, err := scanner.ScanImmediate("/path/that/does/not/exist/99999")
	if err == nil {
		t.Fatal("expected error for non-existent path, got nil")
	}
}

func TestScanImmediate_WithSymlinkChild(t *testing.T) {
	tmpDir := t.TempDir()
	targetFile := filepath.Join(tmpDir, "target.txt")
	if err := os.WriteFile(targetFile, []byte("target"), 0o644); err != nil {
		t.Fatalf("write target failed: %v", err)
	}
	symlinkPath := filepath.Join(tmpDir, "symlink.txt")
	if err := os.Symlink(targetFile, symlinkPath); err != nil {
		t.Fatalf("symlink failed: %v", err)
	}

	scanner := analyzer.NewScanner(nil, true, 0, 10)
	node, err := scanner.ScanImmediate(tmpDir)
	if err != nil {
		t.Fatalf("ScanImmediate failed: %v", err)
	}

	var foundSymlink bool
	for _, child := range node.Children {
		if child.Name == "symlink.txt" {
			foundSymlink = true
			if !child.IsSymlink() {
				t.Errorf("expected child to be ItemSymlink, got %v", child.Type)
			}
		}
	}
	if !foundSymlink {
		t.Errorf("expected to find symlink.txt child")
	}
}

func TestScanner_SymlinkNotFollowed(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "sub")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	subFile := filepath.Join(subDir, "file.txt")
	if err := os.WriteFile(subFile, []byte("data"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Create cyclic symlink inside subDir pointing to parent tmpDir: sub/loop -> tmpDir
	loopLink := filepath.Join(subDir, "loop")
	if err := os.Symlink(tmpDir, loopLink); err != nil {
		t.Fatalf("create cyclic symlink failed: %v", err)
	}

	scanner := analyzer.NewScanner(nil, true, 0, 10)
	node, err := scanner.ScanImmediate(tmpDir)
	if err != nil {
		t.Fatalf("ScanImmediate failed: %v", err)
	}

	var targetNode *analyzer.Node
	for _, child := range node.Children {
		if child.Name == "sub" {
			targetNode = child
			break
		}
	}
	if targetNode == nil {
		t.Fatal("expected sub child node")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	bytes, items, err := scanner.CalculateDirSize(ctx, targetNode, nil)
	if err != nil {
		t.Fatalf("CalculateDirSize failed: %v", err)
	}

	if items < 1 {
		t.Errorf("expected at least 1 item, got %d", items)
	}
	if bytes < 4 {
		t.Errorf("expected at least 4 bytes, got %d", bytes)
	}
	if !targetNode.Scanned {
		t.Error("expected targetNode to be marked Scanned=true")
	}
}

func TestScanner_MountBoundary(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("device ID mount boundary isolation only supported on darwin and linux")
	}

	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "sub")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	subFile := filepath.Join(subDir, "file.txt")
	if err := os.WriteFile(subFile, []byte("sub content 12345"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	scanner := analyzer.NewScanner(nil, true, 0, 10)
	// We simulate a node where HasDevID is true, but DeviceID is different from any real filesystem ID
	fakeDevNode := &analyzer.Node{
		Path:     tmpDir,
		Name:     filepath.Base(tmpDir),
		Type:     analyzer.ItemDir,
		HasDevID: true,
		DeviceID: 0xDeadBeefCafe, // Fake device ID
	}

	bytes, items, err := scanner.CalculateDirSize(context.Background(), fakeDevNode, nil)
	if err != nil {
		t.Fatalf("CalculateDirSize failed: %v", err)
	}

	// Subdir has different device ID, so it should be skipped with filepath.SkipDir
	if bytes != 0 {
		t.Errorf("expected 0 bytes because subDir should be skipped across mount boundary, got %d", bytes)
	}
	if items != 0 {
		t.Errorf("expected 0 items because subDir should be skipped, got %d", items)
	}
}

func TestScanner_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	dirNode := &analyzer.Node{
		Path: tmpDir,
		Name: filepath.Base(tmpDir),
		Type: analyzer.ItemDir,
	}

	scanner := analyzer.NewScanner(nil, true, 0, 10)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, _, err := scanner.CalculateDirSize(ctx, dirNode, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled error, got: %v", err)
	}
}


