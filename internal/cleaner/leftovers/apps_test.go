package leftovers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gtkpad/devclean/internal/cleaner"
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

func TestLeftovers_AppIndexing_PackageReceipts(t *testing.T) {
	tempDir := t.TempDir()
	receiptsDir := filepath.Join(tempDir, "receipts")
	if err := os.MkdirAll(receiptsDir, 0755); err != nil {
		t.Fatalf("failed to create receiptsDir: %v", err)
	}

	// Receipt 1: XML plist for Claude Desktop
	createTestReceiptXML(t, receiptsDir, "com.anthropic.claudefordesktop.plist", "com.anthropic.claudefordesktop", "/Applications", "Claude.pkg")

	// Receipt 2: Binary plist for DotNet pkg
	createTestReceiptBinary(t, receiptsDir, "com.microsoft.dotnet.pkg.plist", "com.microsoft.dotnet", "/usr/local/share/dotnet", "dotnet.pkg")

	// Receipt 3: Non-plist entry or directory (should be ignored gracefully)
	if err := os.MkdirAll(filepath.Join(receiptsDir, "ignored.dir"), 0755); err != nil {
		t.Fatalf("failed to create ignored dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(receiptsDir, "ignored.txt"), []byte("not a plist"), 0644); err != nil {
		t.Fatalf("failed to write ignored txt: %v", err)
	}

	idx := newAppIndex()
	idx.scanPackageReceipts(context.Background(), receiptsDir)

	if !idx.BundleIDs["com.anthropic.claudefordesktop"] {
		t.Errorf("expected bundle ID com.anthropic.claudefordesktop to be indexed")
	}
	if !idx.BundleIDs["com.microsoft.dotnet.pkg"] { // stripped from filename
		t.Errorf("expected bundle ID com.microsoft.dotnet.pkg (from filename) to be indexed")
	}
	if !idx.BundleIDs["com.microsoft.dotnet"] { // from PackageIdentifier
		t.Errorf("expected bundle ID com.microsoft.dotnet (from PackageIdentifier) to be indexed")
	}
	if !idx.AppNames["claude"] {
		t.Errorf("expected app name claude (from Claude.pkg) to be indexed")
	}
	if !idx.AppNames["dotnet"] {
		t.Errorf("expected app name dotnet (from dotnet.pkg) to be indexed")
	}

	if !idx.Matches("com.anthropic.claudefordesktop") {
		t.Errorf("expected idx.Matches(\"com.anthropic.claudefordesktop\") to be true")
	}
	if !idx.Matches("claude") {
		t.Errorf("expected idx.Matches(\"claude\") to be true")
	}
	if !idx.Matches("dotnet") {
		t.Errorf("expected idx.Matches(\"dotnet\") to be true")
	}
}

func TestLeftovers_AppIndexing_NameNormalization(t *testing.T) {
	idx := newAppIndex()
	idx.addAppName("goland")
	idx.addAppName("adspower global")
	idx.addAppName("telegram")
	idx.addAppName("teams")
	idx.addAppName("node")

	testCases := []struct {
		candidate string
		wantMatch bool
	}{
		// Version suffix trimming (GoLand2024.1 -> goland, teams2 -> teams, node-v20.10.0 -> node)
		{"GoLand2024.1", true},
		{"goland2024.2.1", true},
		{"teams2", true},
		{"node-v20.10.0", true},

		// Underscore and hyphen normalization (adspower_global -> adspower global)
		{"adspower_global", true},
		{"adspower-global", true},
		{"adspower global", true},

		// Prefix token matching for dotless candidates (Telegram Desktop -> telegram)
		{"Telegram Desktop", true},
		{"telegram helper", true},

		// Unrelated candidates
		{"UnknownApp", false},
		{"random_tool_name", false},
	}

	for _, tc := range testCases {
		got := idx.Matches(tc.candidate)
		if got != tc.wantMatch {
			t.Errorf("idx.Matches(%q) = %v, want %v", tc.candidate, got, tc.wantMatch)
		}
	}
}

func TestLeftovers_SpotlightResolver_Memoization(t *testing.T) {
	mockExec := cleaner.NewMockExecutor()
	bundleID := "com.vendor.product"
	query := fmt.Sprintf("kMDItemCFBundleIdentifier == %q", bundleID)
	cmdKey := fmt.Sprintf("mdfind -onlyin / %s", query)
	mockExec.Outputs[cmdKey] = []byte("/Applications/Product.app\n")

	resolver := NewDefaultSpotlightResolver(mockExec)

	// Call 1: should invoke mock executor
	matched, err := resolver.ResolveBundleID(context.Background(), bundleID)
	if err != nil {
		t.Fatalf("unexpected error resolving bundle ID: %v", err)
	}
	if !matched {
		t.Errorf("expected matched to be true")
	}
	if len(mockExec.Calls) != 1 {
		t.Fatalf("expected 1 call to mock executor, got %d", len(mockExec.Calls))
	}

	// Call 2: should return cached result without invoking mock executor again
	matched2, err := resolver.ResolveBundleID(context.Background(), bundleID)
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	if !matched2 {
		t.Errorf("expected matched2 to be true")
	}
	if len(mockExec.Calls) != 1 {
		t.Errorf("expected call count to remain 1 due to memoization, got %d", len(mockExec.Calls))
	}

	// Test via AppIndex
	idx := newAppIndex()
	idx.SetSpotlightResolver(resolver)

	// Calling Matches with dot-containing bundle ID should trigger spotlight
	if !idx.Matches("com.vendor.product") {
		t.Errorf("idx.Matches(\"com.vendor.product\") = false, want true")
	}

	// Calling Matches with dotless candidate MUST NOT invoke Spotlight
	callsBefore := len(mockExec.Calls)
	if idx.Matches("dotlesscandidate") {
		t.Errorf("idx.Matches(\"dotlesscandidate\") = true, want false")
	}
	callsAfter := len(mockExec.Calls)
	if callsBefore != callsAfter {
		t.Errorf("Spotlight was invoked for dotless candidate! calls before: %d, after: %d", callsBefore, callsAfter)
	}
}
