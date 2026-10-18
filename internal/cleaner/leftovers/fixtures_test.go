package leftovers

import (
	"os"
	"path/filepath"
	"testing"

	"howett.net/plist"
)

// createTestAppBundle creates a mock macOS application bundle hierarchy with Contents/Info.plist.
func createTestAppBundle(t *testing.T, appsDir, bundleName, bundleID, appName string) string {
	t.Helper()
	appPath := filepath.Join(appsDir, bundleName+".app")
	contentsDir := filepath.Join(appPath, "Contents")
	if err := os.MkdirAll(contentsDir, 0755); err != nil {
		t.Fatalf("failed to create app bundle dir: %v", err)
	}

	info := AppBundleInfoPlist{
		CFBundleIdentifier: bundleID,
		CFBundleName:       appName,
		CFBundleExecutable: bundleName,
	}
	data, err := plist.Marshal(info, plist.XMLFormat)
	if err != nil {
		t.Fatalf("failed to marshal Info.plist: %v", err)
	}
	if err := os.WriteFile(filepath.Join(contentsDir, "Info.plist"), data, 0644); err != nil {
		t.Fatalf("failed to write Info.plist: %v", err)
	}
	return appPath
}

// createTestLaunchAgentXML serializes and writes an XML format LaunchAgent plist.
func createTestLaunchAgentXML(t *testing.T, dir, filename, label, program string, args []string) string {
	t.Helper()
	p := LaunchAgentPlist{
		Label:            label,
		Program:          program,
		ProgramArguments: args,
	}
	data, err := plist.Marshal(p, plist.XMLFormat)
	if err != nil {
		t.Fatalf("failed to marshal XML plist: %v", err)
	}
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write XML plist: %v", err)
	}
	return path
}

// createTestLaunchAgentBinary serializes and writes a binary format (bplist00) LaunchAgent plist.
func createTestLaunchAgentBinary(t *testing.T, dir, filename, label, program string, args []string) string {
	t.Helper()
	p := LaunchAgentPlist{
		Label:            label,
		Program:          program,
		ProgramArguments: args,
	}
	data, err := plist.Marshal(p, plist.BinaryFormat)
	if err != nil {
		t.Fatalf("failed to marshal binary plist: %v", err)
	}
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write binary plist: %v", err)
	}
	return path
}

// createTestDirWithSentinels creates a directory with sentinel files to verify non-deletion.
func createTestDirWithSentinels(t *testing.T, parentDir, name string) (dirPath string, sentinels []string) {
	t.Helper()
	dirPath = filepath.Join(parentDir, name)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	sentinel1 := filepath.Join(dirPath, "sentinel.txt")
	if err := os.WriteFile(sentinel1, []byte("sentinel data"), 0644); err != nil {
		t.Fatalf("failed to write sentinel: %v", err)
	}
	sentinels = append(sentinels, sentinel1)

	subDir := filepath.Join(dirPath, "data")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}
	sentinel2 := filepath.Join(subDir, "important.db")
	if err := os.WriteFile(sentinel2, []byte("database state"), 0644); err != nil {
		t.Fatalf("failed to write sentinel: %v", err)
	}
	sentinels = append(sentinels, sentinel2)

	return dirPath, sentinels
}
