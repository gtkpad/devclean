package installer_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gtkpad/devclean/internal/installer"
)

func TestDetector_DirectExtensions(t *testing.T) {
	tmp := t.TempDir()
	det := installer.NewDetector()

	cases := []struct {
		filename string
		wantType installer.InstallerType
	}{
		{"installer.dmg", installer.TypeDiskImage},
		{"installer.iso", installer.TypeDiskImage},
		{"installer.pkg", installer.TypeInstallerPkg},
		{"installer.deb", installer.TypeLinuxPkg},
		{"installer.rpm", installer.TypeLinuxPkg},
		{"installer.appimage", installer.TypeLinuxPkg},
		{"installer.AppImage", installer.TypeLinuxPkg},
		{"installer.exe", installer.TypeWindowsPkg},
		{"installer.msi", installer.TypeWindowsPkg},
	}

	for _, tc := range cases {
		t.Run(tc.filename, func(t *testing.T) {
			path := filepath.Join(tmp, tc.filename)
			if err := os.WriteFile(path, []byte("dummy installer bytes"), 0o644); err != nil {
				t.Fatalf("creating file %s: %v", tc.filename, err)
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatalf("stat file %s: %v", tc.filename, err)
			}

			cand, matched, err := det.Detect(path, info)
			if err != nil {
				t.Fatalf("unexpected error for %s: %v", tc.filename, err)
			}
			if !matched {
				t.Fatalf("expected matched=true for %s", tc.filename)
			}
			if cand == nil {
				t.Fatalf("expected non-nil candidate for %s", tc.filename)
			}
			if cand.Type != tc.wantType {
				t.Errorf("got type %q, want %q for %s", cand.Type, tc.wantType, tc.filename)
			}
			if cand.Path != path {
				t.Errorf("got path %q, want %q", cand.Path, path)
			}
			if cand.SizeBytes != info.Size() {
				t.Errorf("got size %d, want %d", cand.SizeBytes, info.Size())
			}
			if cand.Detail == "" {
				t.Errorf("expected non-empty detail for %s", tc.filename)
			}
		})
	}
}

func TestDetector_PartialDownloadsIgnored(t *testing.T) {
	tmp := t.TempDir()
	det := installer.NewDetector()

	partials := []string{
		"download.iso.crdownload",
		"setup.exe.part",
		"bigfile.dmg.download",
		"linux.iso.aria2",
	}

	for _, filename := range partials {
		t.Run(filename, func(t *testing.T) {
			path := filepath.Join(tmp, filename)
			if err := os.WriteFile(path, []byte("in-progress download"), 0o644); err != nil {
				t.Fatalf("creating file %s: %v", filename, err)
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatalf("stat file %s: %v", filename, err)
			}

			cand, matched, err := det.Detect(path, info)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if matched {
				t.Errorf("expected matched=false for partial download %s", filename)
			}
			if cand != nil {
				t.Errorf("expected cand=nil for partial download %s", filename)
			}
		})
	}
}

func TestDetector_ArchiveDelegation(t *testing.T) {
	tmp := t.TempDir()
	det := installer.NewDetector()

	// 1. Valid zip with .exe
	zipPath := filepath.Join(tmp, "app.zip")
	createTestZip(t, zipPath, map[string][]byte{
		"bin/run.exe": []byte("MZdummycontent"),
	})
	zipInfo, err := os.Stat(zipPath)
	if err != nil {
		t.Fatalf("stat zip: %v", err)
	}

	candZip, matchedZip, err := det.Detect(zipPath, zipInfo)
	if err != nil {
		t.Fatalf("detect zip error: %v", err)
	}
	if !matchedZip || candZip == nil {
		t.Fatalf("expected zip to match installer")
	}
	if candZip.Type != installer.TypeArchivePkg {
		t.Errorf("got type %q, want %q", candZip.Type, installer.TypeArchivePkg)
	}

	// 2. Valid tar.gz with .deb
	tarPath := filepath.Join(tmp, "package.tar.gz")
	createTestTarGz(t, tarPath, map[string][]byte{
		"installer.deb": []byte("debcontent"),
	})
	tarInfo, err := os.Stat(tarPath)
	if err != nil {
		t.Fatalf("stat tar.gz: %v", err)
	}

	candTar, matchedTar, err := det.Detect(tarPath, tarInfo)
	if err != nil {
		t.Fatalf("detect tar.gz error: %v", err)
	}
	if !matchedTar || candTar == nil {
		t.Fatalf("expected tar.gz to match installer")
	}
	if candTar.Type != installer.TypeArchivePkg {
		t.Errorf("got type %q, want %q", candTar.Type, installer.TypeArchivePkg)
	}

	// Valid .tgz
	tgzPath := filepath.Join(tmp, "package.tgz")
	createTestTarGz(t, tgzPath, map[string][]byte{
		"setup.exe": []byte("MZcontent"),
	})
	tgzInfo, err := os.Stat(tgzPath)
	if err != nil {
		t.Fatalf("stat tgz: %v", err)
	}
	candTgz, matchedTgz, err := det.Detect(tgzPath, tgzInfo)
	if err != nil || !matchedTgz || candTgz == nil {
		t.Fatalf("expected .tgz to match installer: %v", err)
	}

	// 3. Zip with source code (should not match)
	sourceZip := filepath.Join(tmp, "repo.zip")
	createTestZip(t, sourceZip, map[string][]byte{
		"package.json": []byte(`{"name":"test"}`),
		"tool.exe":     []byte("MZcontent"),
	})
	sourceInfo, err := os.Stat(sourceZip)
	if err != nil {
		t.Fatalf("stat source zip: %v", err)
	}

	_, matchedSource, err := det.Detect(sourceZip, sourceInfo)
	if err != nil {
		t.Fatalf("detect source zip error: %v", err)
	}
	if matchedSource {
		t.Errorf("expected source zip to be rejected, got matched=true")
	}

	// 4. Malformed zip
	badZip := filepath.Join(tmp, "bad.zip")
	_ = os.WriteFile(badZip, []byte("bad zip"), 0o644)
	badInfo, _ := os.Stat(badZip)
	_, _, err = det.Detect(badZip, badInfo)
	if err == nil {
		t.Errorf("expected error on malformed zip")
	}

	// 5. Malformed tar.gz
	badTar := filepath.Join(tmp, "bad.tar.gz")
	_ = os.WriteFile(badTar, []byte("bad tar"), 0o644)
	badTarInfo, _ := os.Stat(badTar)
	_, _, err = det.Detect(badTar, badTarInfo)
	if err == nil {
		t.Errorf("expected error on malformed tar.gz")
	}
}

func TestDetector_NonInstallerFiles(t *testing.T) {
	tmp := t.TempDir()
	det := installer.NewDetector()

	nonInstallers := []string{
		"document.pdf",
		"picture.png",
		"notes.txt",
		"archive.tar.bz2", // unsupported format treated as non-installer
		"data.json",
	}

	for _, filename := range nonInstallers {
		t.Run(filename, func(t *testing.T) {
			path := filepath.Join(tmp, filename)
			if err := os.WriteFile(path, []byte("plain non-installer data"), 0o644); err != nil {
				t.Fatalf("creating file %s: %v", filename, err)
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatalf("stat file %s: %v", filename, err)
			}

			cand, matched, err := det.Detect(path, info)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if matched {
				t.Errorf("expected matched=false for %s", filename)
			}
			if cand != nil {
				t.Errorf("expected cand=nil for %s", filename)
			}
		})
	}
}

func TestDetector_DirectoryIgnored(t *testing.T) {
	tmp := t.TempDir()
	det := installer.NewDetector()

	dirPath := filepath.Join(tmp, "some_folder")
	if err := os.Mkdir(dirPath, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	info, err := os.Stat(dirPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	cand, matched, err := det.Detect(dirPath, info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if matched || cand != nil {
		t.Errorf("expected normal directory to be ignored by detector")
	}
}
