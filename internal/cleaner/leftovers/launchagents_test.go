package leftovers

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
)

func TestLeftovers_LaunchAgents_StaleClassification(t *testing.T) {
	tempDir := t.TempDir()
	launchAgentsDir := filepath.Join(tempDir, "LaunchAgents")
	if err := os.MkdirAll(launchAgentsDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	missingExe1 := filepath.Join(tempDir, "nonexistent", "binary1")
	missingExe2 := filepath.Join(tempDir, "nonexistent", "binary2")

	// 1. Plist with Program set to missing path
	createTestLaunchAgentXML(t, launchAgentsDir, "com.test.prog.plist", "com.test.prog", missingExe1, nil)

	// 2. Plist with empty Program but ProgramArguments[0] set to missing path
	createTestLaunchAgentBinary(t, launchAgentsDir, "com.test.args.plist", "com.test.args", "", []string{missingExe2, "--daemon"})

	targets, err := scanLaunchAgents(context.Background(), &cleaner.DefaultExecutor{}, []string{launchAgentsDir})
	if err != nil {
		t.Fatalf("scanLaunchAgents failed: %v", err)
	}

	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(targets))
	}

	for _, tgt := range targets {
		if tgt.ReviewReason != domain.ReviewReasonStaleLaunchAgent {
			t.Errorf("target %s reason = %q, want %q", tgt.ID, tgt.ReviewReason, domain.ReviewReasonStaleLaunchAgent)
		}
		if !tgt.ReviewOnly {
			t.Errorf("target %s ReviewOnly = false, want true", tgt.ID)
		}
		if tgt.EligibleBytes != 0 {
			t.Errorf("target %s EligibleBytes = %d, want 0", tgt.ID, tgt.EligibleBytes)
		}
		if tgt.TTLEligible != false {
			t.Errorf("target %s TTLEligible = true, want false", tgt.ID)
		}
		if tgt.Category != "leftovers-launchagent" {
			t.Errorf("target %s Category = %q, want leftovers-launchagent", tgt.ID, tgt.Category)
		}
		if tgt.ProviderMetrics == nil || !strings.Contains(tgt.ProviderMetrics.StatusNote, "missing executable") {
			t.Errorf("target %s StatusNote = %v, want missing executable note", tgt.ID, tgt.ProviderMetrics)
		}
	}
}

func TestLeftovers_LaunchAgents_ActivePreserved(t *testing.T) {
	tempDir := t.TempDir()
	launchAgentsDir := filepath.Join(tempDir, "LaunchAgents")
	if err := os.MkdirAll(launchAgentsDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	// Create real existing executable
	binDir := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("mkdir bin failed: %v", err)
	}
	realExe := filepath.Join(binDir, "active-daemon")
	if err := os.WriteFile(realExe, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write executable failed: %v", err)
	}

	createTestLaunchAgentXML(t, launchAgentsDir, "com.test.active.plist", "com.test.active", realExe, nil)

	targets, err := scanLaunchAgents(context.Background(), &cleaner.DefaultExecutor{}, []string{launchAgentsDir})
	if err != nil {
		t.Fatalf("scanLaunchAgents failed: %v", err)
	}

	if len(targets) != 0 {
		t.Fatalf("expected 0 targets for active executable, got %d: %+v", len(targets), targets)
	}
}

func TestLeftovers_LaunchAgents_InterpretersAmbiguous(t *testing.T) {
	tempDir := t.TempDir()
	launchAgentsDir := filepath.Join(tempDir, "LaunchAgents")
	if err := os.MkdirAll(launchAgentsDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	cases := []struct {
		filename string
		label    string
		prog     string
		args     []string
	}{
		{"sh.plist", "com.test.sh", "/bin/sh", []string{"/bin/sh", "-c", "echo hello"}},
		{"bash.plist", "com.test.bash", "/bin/bash", []string{"/bin/bash", "-c", "echo hello"}},
		{"python.plist", "com.test.python", "/usr/bin/python3", []string{"/usr/bin/python3", "main.py"}},
		{"env.plist", "com.test.env", "/usr/bin/env", []string{"/usr/bin/env", "node", "app.js"}},
		{"osascript.plist", "com.test.osascript", "/usr/bin/osascript", []string{"/usr/bin/osascript", "-e", "display notification"}},
	}

	for _, tc := range cases {
		createTestLaunchAgentXML(t, launchAgentsDir, tc.filename, tc.label, tc.prog, tc.args)
	}

	targets, err := scanLaunchAgents(context.Background(), &cleaner.DefaultExecutor{}, []string{launchAgentsDir})
	if err != nil {
		t.Fatalf("scanLaunchAgents failed: %v", err)
	}

	if len(targets) != len(cases) {
		t.Fatalf("expected %d targets, got %d", len(cases), len(targets))
	}

	for _, tgt := range targets {
		if tgt.ReviewReason != domain.ReviewReasonAmbiguous {
			t.Errorf("target %s reason = %q, want %q", tgt.ID, tgt.ReviewReason, domain.ReviewReasonAmbiguous)
		}
		if !tgt.ReviewOnly || tgt.EligibleBytes != 0 {
			t.Errorf("target %s must be ReviewOnly with 0 EligibleBytes", tgt.ID)
		}
		if tgt.ProviderMetrics == nil || !strings.Contains(tgt.ProviderMetrics.StatusNote, "interpreter or wrapper invocation") {
			t.Errorf("target %s StatusNote = %v, want interpreter note", tgt.ID, tgt.ProviderMetrics)
		}
	}
}

func TestLeftovers_LaunchAgents_RelativePathAmbiguous(t *testing.T) {
	tempDir := t.TempDir()
	launchAgentsDir := filepath.Join(tempDir, "LaunchAgents")
	if err := os.MkdirAll(launchAgentsDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	// Create a real binary on disk for resolved test case
	binDir := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	realBinary := filepath.Join(binDir, "resolved-tool")
	if err := os.WriteFile(realBinary, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write real binary failed: %v", err)
	}

	createTestLaunchAgentXML(t, launchAgentsDir, "rel1.plist", "com.test.missing1", "", []string{"missing-tool-1", "server.js"})
	createTestLaunchAgentXML(t, launchAgentsDir, "rel2.plist", "com.test.missing2", "", []string{"missing-tool-2", "services"})
	createTestLaunchAgentXML(t, launchAgentsDir, "rel3.plist", "com.test.dot", "", []string{"./daemon.sh"})
	// 4. Relative tool that IS resolved on PATH and exists on disk -> should NOT produce target
	createTestLaunchAgentXML(t, launchAgentsDir, "rel4.plist", "com.test.resolved", "", []string{"resolved-tool", "--daemon"})

	mockExec := cleaner.NewMockExecutor()
	mockExec.Errors["lookpath:missing-tool-1"] = os.ErrNotExist
	mockExec.Errors["lookpath:missing-tool-2"] = os.ErrNotExist
	mockExec.Errors["lookpath:./daemon.sh"] = os.ErrNotExist
	mockExec.Outputs["lookpath:resolved-tool"] = []byte(realBinary)

	targets, err := scanLaunchAgents(context.Background(), mockExec, []string{launchAgentsDir})
	if err != nil {
		t.Fatalf("scanLaunchAgents failed: %v", err)
	}

	if len(targets) != 3 {
		t.Fatalf("expected 3 targets, got %d", len(targets))
	}

	for _, tgt := range targets {
		if tgt.ReviewReason != domain.ReviewReasonAmbiguous {
			t.Errorf("target %s reason = %q, want %q", tgt.ID, tgt.ReviewReason, domain.ReviewReasonAmbiguous)
		}
		if tgt.ProviderMetrics == nil || !strings.Contains(tgt.ProviderMetrics.StatusNote, "relative or PATH-based executable") {
			t.Errorf("target %s StatusNote = %v, want relative/PATH note", tgt.ID, tgt.ProviderMetrics)
		}
	}
}

func TestLeftovers_LaunchAgents_CorruptedPlist(t *testing.T) {
	tempDir := t.TempDir()
	launchAgentsDir := filepath.Join(tempDir, "LaunchAgents")
	if err := os.MkdirAll(launchAgentsDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	corruptFile := filepath.Join(launchAgentsDir, "corrupt.plist")
	if err := os.WriteFile(corruptFile, []byte("NOT A VALID PLIST"), 0644); err != nil {
		t.Fatalf("write corrupt plist failed: %v", err)
	}

	targets, err := scanLaunchAgents(context.Background(), &cleaner.DefaultExecutor{}, []string{launchAgentsDir})
	if err != nil {
		t.Fatalf("scanLaunchAgents failed: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("expected 1 target for corrupt plist, got %d", len(targets))
	}

	tgt := targets[0]
	if tgt.ReviewReason != domain.ReviewReasonAmbiguous {
		t.Errorf("corrupt plist reason = %q, want %q", tgt.ReviewReason, domain.ReviewReasonAmbiguous)
	}
	if tgt.SizeBytes != 0 {
		t.Errorf("corrupt plist SizeBytes = %d, want 0", tgt.SizeBytes)
	}
	if tgt.ProviderMetrics == nil || !strings.Contains(tgt.ProviderMetrics.StatusNote, "corrupt or unreadable plist") {
		t.Errorf("corrupt plist StatusNote = %v, want corrupt/unreadable note", tgt.ProviderMetrics)
	}
}

func TestLeftovers_LaunchAgents_EmptyExecutable(t *testing.T) {
	tempDir := t.TempDir()
	launchAgentsDir := filepath.Join(tempDir, "LaunchAgents")
	if err := os.MkdirAll(launchAgentsDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	createTestLaunchAgentXML(t, launchAgentsDir, "empty.plist", "com.test.empty", "", []string{""})

	targets, err := scanLaunchAgents(context.Background(), &cleaner.DefaultExecutor{}, []string{launchAgentsDir})
	if err != nil {
		t.Fatalf("scanLaunchAgents failed: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("expected 1 target for empty executable, got %d", len(targets))
	}

	tgt := targets[0]
	if tgt.ReviewReason != domain.ReviewReasonAmbiguous {
		t.Errorf("empty executable reason = %q, want %q", tgt.ReviewReason, domain.ReviewReasonAmbiguous)
	}
	if tgt.ProviderMetrics == nil || !strings.Contains(tgt.ProviderMetrics.StatusNote, "empty or missing Program/ProgramArguments") {
		t.Errorf("empty executable StatusNote = %v, want empty/missing note", tgt.ProviderMetrics)
	}
}

func TestLeftovers_LaunchAgents_PermissionDegradation(t *testing.T) {
	tempDir := t.TempDir()
	unreadableDir := filepath.Join(tempDir, "UnreadableLaunchAgents")
	if err := os.MkdirAll(unreadableDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	// Make directory unreadable
	if err := os.Chmod(unreadableDir, 0000); err != nil {
		t.Fatalf("chmod 0000 failed: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(unreadableDir, 0755)
	})

	targets, err := scanLaunchAgents(context.Background(), &cleaner.DefaultExecutor{}, []string{unreadableDir})
	if err != nil {
		t.Fatalf("scanLaunchAgents returned error instead of graceful degradation: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("expected 1 degraded target, got %d: %+v", len(targets), targets)
	}

	tgt := targets[0]
	if tgt.ReviewReason != domain.ReviewReasonSystemProtected {
		t.Errorf("degraded target reason = %q, want %q", tgt.ReviewReason, domain.ReviewReasonSystemProtected)
	}
	if tgt.ProviderMetrics == nil || !tgt.ProviderMetrics.Partial {
		t.Errorf("degraded target Partial = false, want true")
	}
	if tgt.ProviderMetrics == nil || !strings.Contains(tgt.ProviderMetrics.StatusNote, "permission denied") {
		t.Errorf("degraded target StatusNote = %v, want permission denied note", tgt.ProviderMetrics)
	}
}

func TestLeftovers_LaunchAgents_Symlinks(t *testing.T) {
	tempDir := t.TempDir()
	launchAgentsDir := filepath.Join(tempDir, "LaunchAgents")
	if err := os.MkdirAll(launchAgentsDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	realPlist := createTestLaunchAgentXML(t, tempDir, "real.plist", "com.test.real", "/missing/bin", nil)
	symlinkPlist := filepath.Join(launchAgentsDir, "symlink.plist")
	if err := os.Symlink(realPlist, symlinkPlist); err != nil {
		t.Fatalf("symlink failed: %v", err)
	}

	targets, err := scanLaunchAgents(context.Background(), &cleaner.DefaultExecutor{}, []string{launchAgentsDir})
	if err != nil {
		t.Fatalf("scanLaunchAgents failed: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("expected 1 target for symlink, got %d", len(targets))
	}

	tgt := targets[0]
	if tgt.ReviewReason != domain.ReviewReasonAmbiguous {
		t.Errorf("symlink target reason = %q, want %q", tgt.ReviewReason, domain.ReviewReasonAmbiguous)
	}
	if tgt.ProviderMetrics == nil || !strings.Contains(tgt.ProviderMetrics.StatusNote, "symlink") {
		t.Errorf("symlink target StatusNote = %v, want symlink note", tgt.ProviderMetrics)
	}
}

func TestLeftovers_LaunchAgents_SystemLibraryExcluded(t *testing.T) {
	tempDir := t.TempDir()
	sysDir := filepath.Join(tempDir, "System", "Library", "LaunchAgents")
	if err := os.MkdirAll(sysDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	createTestLaunchAgentXML(t, sysDir, "com.apple.fake.plist", "com.apple.fake", "/missing/bin", nil)

	targets, err := scanLaunchAgents(context.Background(), &cleaner.DefaultExecutor{}, []string{sysDir})
	if err != nil {
		t.Fatalf("scanLaunchAgents failed: %v", err)
	}

	if len(targets) != 0 {
		t.Fatalf("expected 0 targets because /System/Library is strictly excluded, got %d: %+v", len(targets), targets)
	}
}

func TestLeftovers_LaunchAgents_InterpreterWithScriptTarget(t *testing.T) {
	tempDir := t.TempDir()
	launchAgentsDir := filepath.Join(tempDir, "LaunchAgents")
	if err := os.MkdirAll(launchAgentsDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	// 1. Valid script file that exists on disk
	scriptsDir := filepath.Join(tempDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatalf("mkdir scripts failed: %v", err)
	}
	realScript := filepath.Join(scriptsDir, "daemon.sh")
	if err := os.WriteFile(realScript, []byte("#!/bin/sh\necho ok\n"), 0755); err != nil {
		t.Fatalf("write script failed: %v", err)
	}

	// 2. Valid bundle that exists on disk
	appsDir := filepath.Join(tempDir, "Applications")
	realApp := createTestAppBundle(t, appsDir, "RealApp", "com.test.realapp", "RealApp")

	// Create plists
	// Case A: /bin/sh pointing to existing script -> active (0 targets)
	createTestLaunchAgentXML(t, launchAgentsDir, "sh_valid.plist", "com.test.shvalid", "/bin/sh", []string{"/bin/sh", realScript})
	// Case B: /usr/bin/open with flags pointing to existing app -> active (0 targets)
	createTestLaunchAgentXML(t, launchAgentsDir, "open_valid.plist", "com.test.openvalid", "/usr/bin/open", []string{"/usr/bin/open", "--wait-apps", realApp})
	// Case C: /bin/sh pointing to missing script -> stale (1 target)
	missingScript := filepath.Join(scriptsDir, "missing.sh")
	createTestLaunchAgentXML(t, launchAgentsDir, "sh_missing.plist", "com.test.shmissing", "/bin/sh", []string{"/bin/sh", missingScript})

	targets, err := scanLaunchAgents(context.Background(), &cleaner.DefaultExecutor{}, []string{launchAgentsDir})
	if err != nil {
		t.Fatalf("scanLaunchAgents failed: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("expected exactly 1 stale target, got %d: %+v", len(targets), targets)
	}

	tgt := targets[0]
	if tgt.ReviewReason != domain.ReviewReasonStaleLaunchAgent {
		t.Errorf("expected ReviewReasonStaleLaunchAgent, got %s", tgt.ReviewReason)
	}
	if tgt.ProviderMetrics == nil || !strings.Contains(tgt.ProviderMetrics.StatusNote, "missing script target") {
		t.Errorf("expected StatusNote to contain 'missing script target', got %v", tgt.ProviderMetrics)
	}
}

func TestLeftovers_LaunchAgents_EmptyStubIgnored(t *testing.T) {
	tempDir := t.TempDir()
	launchAgentsDir := filepath.Join(tempDir, "LaunchAgents")
	if err := os.MkdirAll(launchAgentsDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	// Create empty plist stub (<dict/>) like com.google.keystone.agent.plist
	emptyStub := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict/>
</plist>
`
	stubPath := filepath.Join(launchAgentsDir, "com.google.keystone.agent.plist")
	if err := os.WriteFile(stubPath, []byte(emptyStub), 0644); err != nil {
		t.Fatalf("write stub failed: %v", err)
	}

	targets, err := scanLaunchAgents(context.Background(), &cleaner.DefaultExecutor{}, []string{launchAgentsDir})
	if err != nil {
		t.Fatalf("scanLaunchAgents failed: %v", err)
	}

	if len(targets) != 0 {
		t.Errorf("expected 0 targets for empty migrated stub, got %d: %+v", len(targets), targets)
	}
}
