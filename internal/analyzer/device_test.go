package analyzer

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestGetDeviceID(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.txt")
	if err := os.WriteFile(tmpFile, []byte("data"), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	fi, err := os.Stat(tmpFile)
	if err != nil {
		t.Fatalf("failed to stat test file: %v", err)
	}

	devID, hasDev := getDeviceID(fi)
	switch runtime.GOOS {
	case "darwin", "linux":
		if !hasDev {
			t.Errorf("expected hasDev to be true on %s", runtime.GOOS)
		}
		if devID == 0 {
			t.Errorf("expected devID > 0 on %s, got %d", runtime.GOOS, devID)
		}
	default:
		if hasDev {
			t.Errorf("expected hasDev to be false on %s", runtime.GOOS)
		}
	}

	// Nil fi check
	if dev, ok := getDeviceID(nil); ok || dev != 0 {
		t.Errorf("expected (0, false) for nil fi, got (%d, %v)", dev, ok)
	}
}
