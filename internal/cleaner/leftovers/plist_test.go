package leftovers

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLeftovers_LaunchAgents_XMLAndBinary(t *testing.T) {
	tempDir := t.TempDir()

	label := "com.example.service"
	prog := "/usr/local/bin/my-service"
	args := []string{"/usr/local/bin/my-service", "--verbose", "--port=8080"}

	xmlPath := createTestLaunchAgentXML(t, tempDir, "com.example.service.xml.plist", label, prog, args)
	binPath := createTestLaunchAgentBinary(t, tempDir, "com.example.service.bin.plist", label, prog, args)

	xmlParsed, err := parseLaunchAgentFile(xmlPath)
	if err != nil {
		t.Fatalf("failed to parse XML plist: %v", err)
	}

	binParsed, err := parseLaunchAgentFile(binPath)
	if err != nil {
		t.Fatalf("failed to parse binary plist: %v", err)
	}

	if xmlParsed.Label != label || binParsed.Label != label {
		t.Errorf("Label mismatch: xml=%q, bin=%q, want=%q", xmlParsed.Label, binParsed.Label, label)
	}
	if xmlParsed.Program != prog || binParsed.Program != prog {
		t.Errorf("Program mismatch: xml=%q, bin=%q, want=%q", xmlParsed.Program, binParsed.Program, prog)
	}
	if !reflect.DeepEqual(xmlParsed.ProgramArguments, args) || !reflect.DeepEqual(binParsed.ProgramArguments, args) {
		t.Errorf("ProgramArguments mismatch: xml=%v, bin=%v, want=%v", xmlParsed.ProgramArguments, binParsed.ProgramArguments, args)
	}

	// Test AppBundle Info.plist parsing
	bundleDir := createTestAppBundle(t, tempDir, "SampleApp", "com.example.sample", "Sample App")
	infoPath := filepath.Join(bundleDir, "Contents", "Info.plist")
	info, err := parseAppBundleInfoFile(infoPath)
	if err != nil {
		t.Fatalf("failed to parse AppBundle Info.plist: %v", err)
	}
	if info.CFBundleIdentifier != "com.example.sample" {
		t.Errorf("CFBundleIdentifier = %q, want com.example.sample", info.CFBundleIdentifier)
	}
	if info.CFBundleName != "Sample App" {
		t.Errorf("CFBundleName = %q, want Sample App", info.CFBundleName)
	}
	if info.CFBundleExecutable != "SampleApp" {
		t.Errorf("CFBundleExecutable = %q, want SampleApp", info.CFBundleExecutable)
	}
}

func TestLeftovers_Plist_CorruptHandling(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Truncated plist
	truncPath := filepath.Join(tempDir, "truncated.plist")
	if err := os.WriteFile(truncPath, []byte("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<plist><dict><key>"), 0644); err != nil {
		t.Fatalf("failed to write truncated plist: %v", err)
	}

	p, err := parseLaunchAgentFile(truncPath)
	if err == nil {
		t.Errorf("expected error for truncated plist, got nil (parsed: %+v)", p)
	}

	info, err := parseAppBundleInfoFile(truncPath)
	if err == nil {
		t.Errorf("expected error for truncated info plist, got nil (parsed: %+v)", info)
	}

	// 2. Random garbage bytes
	garbagePath := filepath.Join(tempDir, "garbage.plist")
	if err := os.WriteFile(garbagePath, []byte{0xde, 0xad, 0xbe, 0xef, 0x01, 0x02, 0x03, 0x04}, 0644); err != nil {
		t.Fatalf("failed to write garbage plist: %v", err)
	}

	p, err = parseLaunchAgentFile(garbagePath)
	if err == nil {
		t.Errorf("expected error for garbage plist, got nil (parsed: %+v)", p)
	}

	// 3. Nonexistent file
	nonExistent := filepath.Join(tempDir, "does-not-exist.plist")
	p, err = parseLaunchAgentFile(nonExistent)
	if err == nil {
		t.Errorf("expected error for nonexistent plist, got nil")
	}
}
