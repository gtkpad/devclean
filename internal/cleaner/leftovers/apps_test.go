package leftovers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestLeftovers_AppIndexing_Discovery(t *testing.T) {
	tempDir := t.TempDir()
	appsDir := filepath.Join(tempDir, "Applications")
	if err := os.MkdirAll(appsDir, 0755); err != nil {
		t.Fatalf("failed to create appsDir: %v", err)
	}

	createTestAppBundle(t, appsDir, "Slack", "com.tinyspeck.slackmacgap", "Slack")
	createTestAppBundle(t, appsDir, "Spotify", "com.spotify.client", "Spotify")

	utilDir := filepath.Join(appsDir, "Utilities")
	createTestAppBundle(t, utilDir, "Terminal", "com.apple.Terminal", "Terminal")

	index, err := scanInstalledApps(context.Background(), []string{appsDir})
	if err != nil {
		t.Fatalf("scanInstalledApps failed: %v", err)
	}

	expectedBundleIDs := []string{
		"com.tinyspeck.slackmacgap",
		"com.spotify.client",
		"com.apple.terminal",
	}
	for _, id := range expectedBundleIDs {
		if !index.BundleIDs[id] {
			t.Errorf("expected bundle ID %q in index.BundleIDs, got %v", id, index.BundleIDs)
		}
	}

	expectedAppNames := []string{
		"slack",
		"spotify",
		"terminal",
	}
	for _, name := range expectedAppNames {
		if !index.AppNames[name] {
			t.Errorf("expected app name %q in index.AppNames, got %v", name, index.AppNames)
		}
	}
}

func TestLeftovers_AppIndexing_DepthPruning(t *testing.T) {
	tempDir := t.TempDir()
	appsDir := filepath.Join(tempDir, "Applications")
	if err := os.MkdirAll(appsDir, 0755); err != nil {
		t.Fatalf("failed to create appsDir: %v", err)
	}

	// 1. Deep directory structure beyond depth 3: A/B/C/D/E/TooDeep.app
	deepDir := filepath.Join(appsDir, "A", "B", "C", "D", "E")
	createTestAppBundle(t, deepDir, "TooDeep", "com.example.toodeep", "TooDeep")

	// 2. Normal app with 100 fake files and a nested .app inside Contents/Frameworks/
	normalAppDir := createTestAppBundle(t, appsDir, "PrunedApp", "com.example.prunedapp", "PrunedApp")
	frameworksDir := filepath.Join(normalAppDir, "Contents", "Frameworks")
	if err := os.MkdirAll(frameworksDir, 0755); err != nil {
		t.Fatalf("failed to create frameworksDir: %v", err)
	}
	for i := 0; i < 100; i++ {
		fakeFile := filepath.Join(frameworksDir, fmt.Sprintf("libfake%d.dylib", i))
		if err := os.WriteFile(fakeFile, []byte("fake binary"), 0644); err != nil {
			t.Fatalf("failed to write fake file: %v", err)
		}
	}
	// Also place a nested .app bundle inside Frameworks - should NEVER be discovered due to pruning
	createTestAppBundle(t, frameworksDir, "InternalHelper", "com.example.internalhelper", "InternalHelper")

	index, err := scanInstalledApps(context.Background(), []string{appsDir})
	if err != nil {
		t.Fatalf("scanInstalledApps failed: %v", err)
	}

	// PrunedApp should be indexed
	if !index.AppNames["prunedapp"] {
		t.Errorf("expected prunedapp to be indexed")
	}

	// InternalHelper inside Contents/Frameworks must NOT be indexed because traversal pruned at PrunedApp.app
	if index.AppNames["internalhelper"] || index.BundleIDs["com.example.internalhelper"] {
		t.Errorf("Contents/Frameworks was traversed! InternalHelper was unexpectedly indexed")
	}

	// TooDeep.app is at depth > 3 and should NOT be indexed
	if index.AppNames["toodeep"] || index.BundleIDs["com.example.toodeep"] {
		t.Errorf("TooDeep.app at depth > 3 was unexpectedly indexed")
	}
}

func TestLeftovers_AppIndexing_Matching(t *testing.T) {
	index := newAppIndex()
	index.addBundleID("com.tinyspeck.slackmacgap")
	index.addAppName("Slack")
	index.addBundleID("com.spotify.client")
	index.addAppName("Spotify")

	testCases := []struct {
		candidate string
		wantMatch bool
	}{
		// Exact app name (case-insensitive)
		{"Slack", true},
		{"slack", true},
		{"SLACK", true},
		{"Spotify", true},
		{"spotify", true},

		// Exact bundle ID
		{"com.tinyspeck.slackmacgap", true},
		{"com.spotify.client", true},
		{"COM.TINYSPECK.SLACKMACGAP", true},

		// Helper process prefix / extension matching
		{"com.tinyspeck.slackmacgap.helper", true},
		{"com.tinyspeck.slackmacgap.helper.renderer", true},
		{"com.spotify.client.helper", true},

		// Last dot segment matches app name
		{"org.something.slack", true},
		{"net.vendor.spotify", true},

		// Non-existent applications
		{"NonExistentApp", false},
		{"com.unknown.orphan", false},
		{"random-folder", false},
	}

	for _, tc := range testCases {
		got := index.Matches(tc.candidate)
		if got != tc.wantMatch {
			t.Errorf("index.Matches(%q) = %v; want %v", tc.candidate, got, tc.wantMatch)
		}
	}
}

func TestLeftovers_AppIndexing_HomebrewPackages(t *testing.T) {
	tempDir := t.TempDir()
	caskroom := filepath.Join(tempDir, "Caskroom")
	cellar := filepath.Join(tempDir, "Cellar")

	if err := os.MkdirAll(filepath.Join(caskroom, "dotnet-sdk@8"), 0755); err != nil {
		t.Fatalf("failed to create mock caskroom dotnet-sdk@8: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(caskroom, "copilot-cli"), 0755); err != nil {
		t.Fatalf("failed to create mock caskroom copilot-cli: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(cellar, "k9s"), 0755); err != nil {
		t.Fatalf("failed to create mock cellar k9s: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(cellar, "rtk"), 0755); err != nil {
		t.Fatalf("failed to create mock cellar rtk: %v", err)
	}

	idx := newAppIndex()
	idx.scanHomebrewPackages(context.Background(), []string{caskroom, cellar})

	expectedNames := []string{
		"dotnet-sdk@8",
		"dotnet-sdk",
		"copilot-cli",
		"copilot cli",
		"k9s",
		"rtk",
	}
	for _, name := range expectedNames {
		if !idx.AppNames[name] {
			t.Errorf("expected idx.AppNames to contain %q, got: %v", name, idx.AppNames)
		}
	}

	if !idx.Matches("dotnet-sdk") {
		t.Errorf("expected idx.Matches(\"dotnet-sdk\") to be true")
	}
	if !idx.Matches("copilot cli") {
		t.Errorf("expected idx.Matches(\"copilot cli\") to be true")
	}
	if !idx.Matches("k9s") {
		t.Errorf("expected idx.Matches(\"k9s\") to be true")
	}
}
