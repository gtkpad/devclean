package leftovers

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/domain"
)

func TestLeftovers_FullLifecycle_Hermetic(t *testing.T) {
	tempDir := t.TempDir()

	// Setup directories
	appsDir := filepath.Join(tempDir, "Applications")
	launchAgentsDir := filepath.Join(tempDir, "Library", "LaunchAgents")
	libraryDir := filepath.Join(tempDir, "Library")
	appSupportDir := filepath.Join(libraryDir, "Application Support")
	cachesDir := filepath.Join(libraryDir, "Caches")
	containersDir := filepath.Join(libraryDir, "Containers")

	for _, d := range []string{appsDir, launchAgentsDir, appSupportDir, cachesDir, containersDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("failed to create test dir %s: %v", d, err)
		}
	}

	// 1. Setup Active application bundle
	createTestAppBundle(t, appsDir, "ActiveApp", "com.corp.activeapp", "ActiveApp")

	// 2. Setup Active LaunchAgent whose binary exists
	activeBinPath := filepath.Join(tempDir, "bin", "active_daemon")
	if err := os.MkdirAll(filepath.Dir(activeBinPath), 0755); err != nil {
		t.Fatalf("failed to create bin dir: %v", err)
	}
	if err := os.WriteFile(activeBinPath, []byte("echo active"), 0755); err != nil {
		t.Fatalf("failed to write active binary: %v", err)
	}
	createTestLaunchAgentXML(t, launchAgentsDir, "com.corp.active.plist", "com.corp.active", activeBinPath, nil)

	// 3. Setup Stale LaunchAgents (1 XML, 1 Binary) pointing to missing binaries
	createTestLaunchAgentXML(t, launchAgentsDir, "com.corp.stale.xml.plist", "com.corp.stale.xml", "/nonexistent/binary/xml", nil)
	createTestLaunchAgentBinary(t, launchAgentsDir, "com.corp.stale.bin.plist", "com.corp.stale.bin", "/nonexistent/binary/bin", nil)

	// 4. Setup Interpreter LaunchAgent (/bin/sh wrapper)
	createTestLaunchAgentXML(t, launchAgentsDir, "com.corp.script.plist", "com.corp.script", "/bin/sh", []string{"/bin/sh", "-c", "echo test"})

	// 5. Setup Application Support for ActiveApp and DeadApp
	activeAppSupport := filepath.Join(appSupportDir, "ActiveApp")
	if err := os.MkdirAll(activeAppSupport, 0755); err != nil {
		t.Fatalf("failed to create active app support: %v", err)
	}
	if err := os.WriteFile(filepath.Join(activeAppSupport, "state.db"), []byte("active app support state"), 0644); err != nil {
		t.Fatalf("failed to write active app support file: %v", err)
	}

	deadAppSupport := filepath.Join(appSupportDir, "DeadApp")
	if err := os.MkdirAll(deadAppSupport, 0755); err != nil {
		t.Fatalf("failed to create dead app support: %v", err)
	}
	if err := os.WriteFile(filepath.Join(deadAppSupport, "orphaned.data"), []byte("dead app support orphaned data 12345"), 0644); err != nil {
		t.Fatalf("failed to write dead app support file: %v", err)
	}

	// 6. Setup Caches and Containers for DeadApp
	deadAppCache := filepath.Join(cachesDir, "com.corp.deadapp")
	if err := os.MkdirAll(deadAppCache, 0755); err != nil {
		t.Fatalf("failed to create dead app cache: %v", err)
	}
	if err := os.WriteFile(filepath.Join(deadAppCache, "cache.db"), []byte("dead app cache payload"), 0644); err != nil {
		t.Fatalf("failed to write dead app cache file: %v", err)
	}

	deadAppContainer := filepath.Join(containersDir, "com.corp.deadapp")
	if err := os.MkdirAll(deadAppContainer, 0755); err != nil {
		t.Fatalf("failed to create dead app container: %v", err)
	}
	if err := os.WriteFile(filepath.Join(deadAppContainer, "container.plist"), []byte("container data"), 0644); err != nil {
		t.Fatalf("failed to write dead app container file: %v", err)
	}

	// Instantiate cleaner with hermetic roots
	c := NewWithConfig(nil, tempDir, "darwin", []string{appsDir}, []string{launchAgentsDir}, libraryDir)

	if c.Name() != "leftovers" {
		t.Fatalf("c.Name() = %q, want 'leftovers'", c.Name())
	}
	if !c.IsInstalled() {
		t.Fatalf("c.IsInstalled() = false, want true")
	}

	// Scan with rules enabled
	targets, err := c.Scan(context.Background(), domain.EcosystemRules{Enabled: true})
	if err != nil {
		t.Fatalf("c.Scan() unexpected error: %v", err)
	}

	// Verify all returned targets satisfy inviolable requirements
	for _, target := range targets {
		if !target.ReviewOnly {
			t.Errorf("target %s has ReviewOnly == false; MUST always be true", target.ID)
		}
		if target.EligibleBytes != 0 {
			t.Errorf("target %s has EligibleBytes == %d; MUST always be 0", target.ID, target.EligibleBytes)
		}
		if target.TTLEligible {
			t.Errorf("target %s has TTLEligible == true; MUST always be false", target.ID)
		}
	}

	// Classify surfaced targets
	var staleLaunchAgents []domain.Target
	var ambiguousLaunchAgents []domain.Target
	var appLeftovers []domain.Target

	for _, t := range targets {
		switch t.Category {
		case "leftovers-launchagent":
			if t.ReviewReason == domain.ReviewReasonStaleLaunchAgent {
				staleLaunchAgents = append(staleLaunchAgents, t)
			} else if t.ReviewReason == domain.ReviewReasonAmbiguous {
				ambiguousLaunchAgents = append(ambiguousLaunchAgents, t)
			}
		case "leftovers-app":
			if t.ReviewReason == domain.ReviewReasonAppLeftover {
				appLeftovers = append(appLeftovers, t)
			}
		}
	}

	// 1. Stale LaunchAgents: exactly 2 (XML and binary)
	if len(staleLaunchAgents) != 2 {
		t.Fatalf("staleLaunchAgents count = %d, want 2", len(staleLaunchAgents))
	}

	// 2. Ambiguous LaunchAgent: exactly 1 (/bin/sh wrapper)
	if len(ambiguousLaunchAgents) != 1 {
		t.Fatalf("ambiguousLaunchAgents count = %d, want 1", len(ambiguousLaunchAgents))
	}

	// 3. Active LaunchAgent must NOT be surfaced
	for _, trg := range targets {
		if trg.ID == "launchagent-com.corp.active.plist" {
			t.Errorf("active LaunchAgent was surfaced unexpectedly: %v", trg)
		}
	}

	// 4. App Leftovers: exactly 1 consolidated target for deadapp
	if len(appLeftovers) != 1 {
		t.Fatalf("appLeftovers count = %d, want 1; got %v", len(appLeftovers), appLeftovers)
	}
	leftover := appLeftovers[0]
	if leftover.ID != "app-leftover-deadapp" {
		t.Errorf("app leftover ID = %q, want 'app-leftover-deadapp'", leftover.ID)
	}
	if leftover.Name != "App Leftover: deadapp" {
		t.Errorf("app leftover Name = %q, want 'App Leftover: deadapp'", leftover.Name)
	}
	if leftover.ProviderMetrics == nil || leftover.ProviderMetrics.TotalCount != 3 {
		t.Errorf("app leftover TotalCount = %v, want 3 residual locations", leftover.ProviderMetrics)
	}
	if leftover.SizeBytes <= 0 {
		t.Errorf("app leftover SizeBytes = %d, want > 0", leftover.SizeBytes)
	}

	// 5. ActiveApp must NOT be surfaced as leftover
	for _, trg := range targets {
		if trg.ID == "app-leftover-activeapp" {
			t.Errorf("active app was surfaced as leftover unexpectedly: %v", trg)
		}
	}

	// Verify Scan with rules.Enabled == false returns nil, nil
	disabledTargets, err := c.Scan(context.Background(), domain.EcosystemRules{Enabled: false})
	if err != nil || disabledTargets != nil {
		t.Errorf("Scan with Enabled: false returned (%v, %v), want (nil, nil)", disabledTargets, err)
	}
}

func TestLeftovers_CleanRefusesDeletion(t *testing.T) {
	tempDir := t.TempDir()

	appsDir := filepath.Join(tempDir, "Applications")
	launchAgentsDir := filepath.Join(tempDir, "Library", "LaunchAgents")
	libraryDir := filepath.Join(tempDir, "Library")
	appSupportDir := filepath.Join(libraryDir, "Application Support")

	for _, d := range []string{appsDir, launchAgentsDir, appSupportDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", d, err)
		}
	}

	// Stale LaunchAgent plist
	plistFile := createTestLaunchAgentXML(t, launchAgentsDir, "com.sentinel.plist", "com.sentinel", "/missing/bin", nil)

	// App leftover directory with sentinels
	leftoverDir, sentinels := createTestDirWithSentinels(t, appSupportDir, "DeadApp")

	// Collect before states (hashes, sizes, modtimes)
	type sentinelState struct {
		path    string
		hash    [32]byte
		size    int64
		modTime time.Time
	}

	allPaths := append([]string{plistFile}, sentinels...)
	var beforeStates []sentinelState

	for _, p := range allPaths {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatalf("stat failed for %s: %v", p, err)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read failed for %s: %v", p, err)
		}
		beforeStates = append(beforeStates, sentinelState{
			path:    p,
			hash:    sha256.Sum256(data),
			size:    info.Size(),
			modTime: info.ModTime(),
		})
	}

	c := NewWithConfig(nil, tempDir, "darwin", []string{appsDir}, []string{launchAgentsDir}, libraryDir)
	targets, err := c.Scan(context.Background(), domain.EcosystemRules{Enabled: true})
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	if len(targets) == 0 {
		t.Fatalf("expected at least 1 target from scan, got 0")
	}

	// Directly invoke Clean()
	err = c.Clean(context.Background(), targets)
	if err != nil {
		t.Fatalf("Clean() returned error: %v", err)
	}

	// Inviolable refusal check: 100% of sentinel files and plists MUST still exist
	for _, b := range beforeStates {
		info, err := os.Stat(b.path)
		if err != nil {
			t.Fatalf("CRITICAL SECURITY VIOLATION (LEFT-04): file %s missing after Clean()! err: %v", b.path, err)
		}
		data, err := os.ReadFile(b.path)
		if err != nil {
			t.Fatalf("failed to read file %s after Clean(): %v", b.path, err)
		}
		currHash := sha256.Sum256(data)
		if currHash != b.hash {
			t.Fatalf("CRITICAL SECURITY VIOLATION (LEFT-04): content of %s mutated after Clean()!", b.path)
		}
		if info.Size() != b.size {
			t.Fatalf("CRITICAL SECURITY VIOLATION (LEFT-04): size of %s mutated after Clean()!", b.path)
		}
		if !info.ModTime().Equal(b.modTime) {
			t.Fatalf("CRITICAL SECURITY VIOLATION (LEFT-04): modtime of %s changed after Clean()!", b.path)
		}
	}

	// Verify leftover directory still exists
	if _, err := os.Stat(leftoverDir); err != nil {
		t.Fatalf("leftover directory was removed by Clean(): %v", err)
	}
}

func TestLeftovers_DryRun(t *testing.T) {
	tempDir := t.TempDir()

	appsDir := filepath.Join(tempDir, "Applications")
	launchAgentsDir := filepath.Join(tempDir, "Library", "LaunchAgents")
	libraryDir := filepath.Join(tempDir, "Library")
	appSupportDir := filepath.Join(libraryDir, "Application Support")

	for _, d := range []string{appsDir, launchAgentsDir, appSupportDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", d, err)
		}
	}

	plistFile := createTestLaunchAgentXML(t, launchAgentsDir, "com.dryrun.plist", "com.dryrun", "/missing/bin", nil)
	leftoverDir, sentinels := createTestDirWithSentinels(t, appSupportDir, "DryRunApp")

	allFiles := append([]string{plistFile, leftoverDir}, sentinels...)
	modTimesBefore := make(map[string]time.Time)
	for _, f := range allFiles {
		st, err := os.Stat(f)
		if err != nil {
			t.Fatalf("stat failed: %v", err)
		}
		modTimesBefore[f] = st.ModTime()
	}

	c := NewWithConfig(nil, tempDir, "darwin", []string{appsDir}, []string{launchAgentsDir}, libraryDir)

	targets, err := c.Scan(context.Background(), domain.EcosystemRules{Enabled: true})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(targets))
	}

	// Simulate dry-run (no actions taken, or Clean called)
	// Even calling Clean must be a no-op
	if err := c.Clean(context.Background(), targets); err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	// Assert zero mutations
	for _, f := range allFiles {
		st, err := os.Stat(f)
		if err != nil {
			t.Fatalf("file %s missing during dry run: %v", f, err)
		}
		if !st.ModTime().Equal(modTimesBefore[f]) {
			t.Errorf("file %s modified during dry run: before=%v, after=%v", f, modTimesBefore[f], st.ModTime())
		}
	}
}
