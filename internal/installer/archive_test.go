package installer_test

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/gtkpad/devclean/internal/installer"
)

func createTestZip(t *testing.T, dest string, files map[string][]byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatalf("creating dir for zip: %v", err)
	}
	f, err := os.Create(dest)
	if err != nil {
		t.Fatalf("creating test zip file: %v", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("creating zip entry %q: %v", name, err)
		}
		if _, err := w.Write(content); err != nil {
			t.Fatalf("writing zip entry %q: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("closing zip writer: %v", err)
	}
}

func createTestTarGz(t *testing.T, dest string, files map[string][]byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatalf("creating dir for tar.gz: %v", err)
	}
	f, err := os.Create(dest)
	if err != nil {
		t.Fatalf("creating test tar.gz file: %v", err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0o755,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("writing tar header %q: %v", name, err)
		}
		if _, err := tw.Write(content); err != nil {
			t.Fatalf("writing tar entry %q: %v", name, err)
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("closing tar writer: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("closing gzip writer: %v", err)
	}
}

func TestInspectZip_InstallerExe(t *testing.T) {
	tmp := t.TempDir()
	zipPath := filepath.Join(tmp, "setup.zip")

	createTestZip(t, zipPath, map[string][]byte{
		"installer/setup.exe": []byte("MZdummycontent"),
		"readme.txt":          []byte("install instructions"),
	})

	matched, detail, err := installer.InspectZip(zipPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !matched {
		t.Errorf("expected matched=true for zip with .exe, got false")
	}
	if detail == "" {
		t.Errorf("expected non-empty detail, got empty")
	}
}

func TestInspectZip_MacApp(t *testing.T) {
	tmp := t.TempDir()
	zipPath := filepath.Join(tmp, "app.zip")

	createTestZip(t, zipPath, map[string][]byte{
		"MyApp.app/Contents/MacOS/app": []byte("\xfe\xed\xfa\xcfmac-binary"),
		"MyApp.app/Contents/Info.plist": []byte("<plist></plist>"),
	})

	matched, detail, err := installer.InspectZip(zipPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !matched {
		t.Errorf("expected matched=true for zip with .app, got false")
	}
	if detail == "" {
		t.Errorf("expected non-empty detail, got empty")
	}
}

func TestInspectZip_ExecutableMagic(t *testing.T) {
	tmp := t.TempDir()
	zipPath := filepath.Join(tmp, "binary.zip")

	// 2000 bytes with ELF magic bytes
	elfContent := make([]byte, 2048)
	copy(elfContent, []byte{0x7f, 'E', 'L', 'F'})

	createTestZip(t, zipPath, map[string][]byte{
		"bin/server": elfContent,
	})

	matched, _, err := installer.InspectZip(zipPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !matched {
		t.Errorf("expected matched=true for zip with ELF binary, got false")
	}
}

func TestInspectZip_SourceCodeFalsePositive(t *testing.T) {
	tmp := t.TempDir()
	zipPath := filepath.Join(tmp, "repo.zip")

	createTestZip(t, zipPath, map[string][]byte{
		"repo/package.json": []byte(`{"name":"test"}`),
		"repo/bin/tool.exe": []byte("MZdummycontent"),
		"repo/src/index.js": []byte("console.log('hi');"),
	})

	matched, _, err := installer.InspectZip(zipPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if matched {
		t.Errorf("expected matched=false for zip with package.json, got true")
	}

	// Also test go.mod
	zipGoMod := filepath.Join(tmp, "go-repo.zip")
	createTestZip(t, zipGoMod, map[string][]byte{
		"repo/go.mod":      []byte("module test"),
		"repo/scripts/run": []byte("#!/bin/sh\necho hi"),
	})

	matchedGo, _, err := installer.InspectZip(zipGoMod)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if matchedGo {
		t.Errorf("expected matched=false for zip with go.mod, got true")
	}
}

func TestInspectZip_Malformed(t *testing.T) {
	tmp := t.TempDir()
	badZip := filepath.Join(tmp, "corrupt.zip")
	if err := os.WriteFile(badZip, []byte("not a zip file at all"), 0o644); err != nil {
		t.Fatalf("writing corrupt zip: %v", err)
	}

	matched, _, err := installer.InspectZip(badZip)
	if err == nil {
		t.Errorf("expected error for malformed zip, got nil")
	}
	if matched {
		t.Errorf("expected matched=false for malformed zip")
	}
}

func TestInspectZip_Empty(t *testing.T) {
	tmp := t.TempDir()
	emptyZip := filepath.Join(tmp, "empty.zip")
	createTestZip(t, emptyZip, map[string][]byte{})

	matched, _, err := installer.InspectZip(emptyZip)
	if err != nil {
		t.Fatalf("unexpected error on empty zip: %v", err)
	}
	if matched {
		t.Errorf("expected matched=false for empty zip")
	}
}

func TestInspectTarGz_Installer(t *testing.T) {
	tmp := t.TempDir()
	tarPath := filepath.Join(tmp, "package.tar.gz")

	createTestTarGz(t, tarPath, map[string][]byte{
		"pkg/installer.deb": []byte("deb-archive-dummy"),
		"pkg/README":        []byte("readme"),
	})

	matched, detail, err := installer.InspectTarGz(tarPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !matched {
		t.Errorf("expected matched=true for tar.gz with .deb, got false")
	}
	if detail == "" {
		t.Errorf("expected non-empty detail, got empty")
	}
}

func TestInspectTarGz_ExecutableMagic(t *testing.T) {
	tmp := t.TempDir()
	tarPath := filepath.Join(tmp, "binary.tar.gz")

	elfContent := make([]byte, 2048)
	copy(elfContent, []byte{0x7f, 'E', 'L', 'F'})

	createTestTarGz(t, tarPath, map[string][]byte{
		"bin/daemon": elfContent,
	})

	matched, detail, err := installer.InspectTarGz(tarPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !matched {
		t.Errorf("expected matched=true for tar.gz with ELF executable, got false")
	}
	if detail == "" {
		t.Errorf("expected non-empty detail, got empty")
	}
}

func TestInspectTarGz_MacApp(t *testing.T) {
	tmp := t.TempDir()
	tarPath := filepath.Join(tmp, "app.tgz")

	createTestTarGz(t, tarPath, map[string][]byte{
		"Demo.app/Contents/MacOS/demo": []byte("\xfe\xed\xfa\xcfbinary"),
	})

	matched, _, err := installer.InspectTarGz(tarPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !matched {
		t.Errorf("expected matched=true for tar.gz with .app, got false")
	}
}

func TestInspectTarGz_SourceCodeFalsePositive(t *testing.T) {
	tmp := t.TempDir()
	tarPath := filepath.Join(tmp, "repo.tar.gz")

	createTestTarGz(t, tarPath, map[string][]byte{
		"repo/Cargo.toml": []byte("[package]\nname = \"demo\""),
		"repo/build.sh":   []byte("#!/bin/sh\ncargo build"),
	})

	matched, _, err := installer.InspectTarGz(tarPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if matched {
		t.Errorf("expected matched=false for tar.gz with Cargo.toml, got true")
	}
}

func TestInspectTarGz_Malformed(t *testing.T) {
	tmp := t.TempDir()
	badTar := filepath.Join(tmp, "corrupt.tar.gz")
	if err := os.WriteFile(badTar, []byte("not a gzip tar"), 0o644); err != nil {
		t.Fatalf("writing corrupt tar: %v", err)
	}

	matched, _, err := installer.InspectTarGz(badTar)
	if err == nil {
		t.Errorf("expected error for malformed tar.gz, got nil")
	}
	if matched {
		t.Errorf("expected matched=false for malformed tar.gz")
	}
}
