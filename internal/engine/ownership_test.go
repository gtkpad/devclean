package engine

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/domain"
)

func TestOwnership_GetEcosystemTier(t *testing.T) {
	generic := []string{"macos", "system", "browsers", "leftovers", "MacOS", "SYSTEM"}
	for _, eco := range generic {
		if got := getEcosystemTier(eco); got != TierGeneric {
			t.Errorf("getEcosystemTier(%q) = %v, want TierGeneric", eco, got)
		}
	}

	dedicated := []string{"node", "rust", "python", "bun", "go", "dotnet", "java", "docker"}
	for _, eco := range dedicated {
		if got := getEcosystemTier(eco); got != TierDedicated {
			t.Errorf("getEcosystemTier(%q) = %v, want TierDedicated", eco, got)
		}
	}
}

func TestOwnership_IsDescendant(t *testing.T) {
	cases := []struct {
		parent string
		child  string
		want   bool
	}{
		{"/a/b", "/a/b/c", true},
		{"/a/b", "/a/b/c/d", true},
		{"/a/b", "/a/b", false},
		{"/a/b", "/a/bc", false},
		{"/a/b", "/x/y", false},
		{"/a/b", "/a", false},
		{"", "/a/b", false},
		{"/a/b", "", false},
	}

	for _, tc := range cases {
		got := isDescendant(tc.parent, tc.child)
		if got != tc.want {
			t.Errorf("isDescendant(%q, %q) = %v, want %v", tc.parent, tc.child, got, tc.want)
		}
	}
}

func TestOwnership_DedicatedWinsOverGeneric(t *testing.T) {
	tmp := t.TempDir()
	realPath := filepath.Join(tmp, "shared_cache")
	if err := os.MkdirAll(realPath, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	results := []domain.ScanResult{
		{
			Ecosystem: "macos", // TierGeneric
			Targets: []domain.Target{
				{
					ID:            "macos-target",
					Name:          "Generic macOS cache",
					Path:          realPath,
					SizeBytes:     50 * 1024 * 1024,
					EligibleBytes: 50 * 1024 * 1024,
					TTLEligible:   true,
					Risk:          domain.RiskLow,
					LastAccessed:  time.Now(),
				},
			},
		},
		{
			Ecosystem: "node", // TierDedicated
			Targets: []domain.Target{
				{
					ID:            "node-target",
					Name:          "Node npm cache",
					Path:          realPath,
					SizeBytes:     50 * 1024 * 1024,
					EligibleBytes: 50 * 1024 * 1024,
					TTLEligible:   true,
					Risk:          domain.RiskLow,
					LastAccessed:  time.Now(),
				},
			},
		},
	}

	resolved := ResolveCanonicalOwnership(results)

	var totalTargets int
	var survivingTarget *domain.Target
	var survivingEco string

	for _, r := range resolved {
		for i := range r.Targets {
			totalTargets++
			survivingTarget = &r.Targets[i]
			survivingEco = r.Ecosystem
		}
	}

	if totalTargets != 1 {
		t.Fatalf("expected 1 surviving target, got %d", totalTargets)
	}
	if survivingEco != "node" {
		t.Errorf("expected surviving ecosystem to be 'node', got %q", survivingEco)
	}
	if survivingTarget.ID != "node-target" {
		t.Errorf("expected surviving target ID 'node-target', got %q", survivingTarget.ID)
	}
	if survivingTarget.SizeBytes != 50*1024*1024 {
		t.Errorf("expected SizeBytes 50MB, got %d", survivingTarget.SizeBytes)
	}
}

func TestOwnership_DeductChildBytesFromParent(t *testing.T) {
	tmp := t.TempDir()
	parentDir := filepath.Join(tmp, "parent")
	childDir := filepath.Join(parentDir, "sub")
	if err := os.MkdirAll(childDir, 0755); err != nil {
		t.Fatalf("failed to create dirs: %v", err)
	}

	results := []domain.ScanResult{
		{
			Ecosystem: "macos", // Parent reported by generic cleaner
			Targets: []domain.Target{
				{
					ID:            "parent-target",
					Name:          "Parent generic directory",
					Path:          parentDir,
					SizeBytes:     100 * 1024 * 1024,
					EligibleBytes: 100 * 1024 * 1024,
					TTLEligible:   true,
				},
			},
		},
		{
			Ecosystem: "node", // Child reported by dedicated cleaner
			Targets: []domain.Target{
				{
					ID:            "child-target",
					Name:          "Child node modules",
					Path:          childDir,
					SizeBytes:     40 * 1024 * 1024,
					EligibleBytes: 40 * 1024 * 1024,
					TTLEligible:   true,
				},
			},
		},
	}

	resolved := ResolveCanonicalOwnership(results)

	var parentTarget, childTarget *domain.Target
	for _, r := range resolved {
		for i := range r.Targets {
			if r.Targets[i].ID == "parent-target" {
				parentTarget = &r.Targets[i]
			}
			if r.Targets[i].ID == "child-target" {
				childTarget = &r.Targets[i]
			}
		}
	}

	if parentTarget == nil || childTarget == nil {
		t.Fatalf("missing parent or child target: parent=%v, child=%v", parentTarget, childTarget)
	}

	const expectedParent = 60 * 1024 * 1024
	const expectedChild = 40 * 1024 * 1024

	if parentTarget.SizeBytes != expectedParent {
		t.Errorf("parent SizeBytes = %d, want %d", parentTarget.SizeBytes, expectedParent)
	}
	if parentTarget.EligibleBytes != expectedParent {
		t.Errorf("parent EligibleBytes = %d, want %d", parentTarget.EligibleBytes, expectedParent)
	}

	if childTarget.SizeBytes != expectedChild {
		t.Errorf("child SizeBytes = %d, want %d", childTarget.SizeBytes, expectedChild)
	}
	if childTarget.EligibleBytes != expectedChild {
		t.Errorf("child EligibleBytes = %d, want %d", childTarget.EligibleBytes, expectedChild)
	}

	total := parentTarget.SizeBytes + childTarget.SizeBytes
	if total != 100*1024*1024 {
		t.Errorf("total bytes = %d, want %d (zero double counting)", total, 100*1024*1024)
	}
}

func TestOwnership_ClampAgainstNegative(t *testing.T) {
	tmp := t.TempDir()
	parentDir := filepath.Join(tmp, "parent")
	childDir := filepath.Join(parentDir, "sub")
	if err := os.MkdirAll(childDir, 0755); err != nil {
		t.Fatalf("failed to create dirs: %v", err)
	}

	// Child estimate (50MB) is greater than parent estimate (30MB)
	results := []domain.ScanResult{
		{
			Ecosystem: "macos",
			Targets: []domain.Target{
				{
					ID:            "parent-underflow",
					Path:          parentDir,
					SizeBytes:     30 * 1024 * 1024,
					EligibleBytes: 30 * 1024 * 1024,
				},
			},
		},
		{
			Ecosystem: "node",
			Targets: []domain.Target{
				{
					ID:            "child-larger",
					Path:          childDir,
					SizeBytes:     50 * 1024 * 1024,
					EligibleBytes: 50 * 1024 * 1024,
				},
			},
		},
	}

	resolved := ResolveCanonicalOwnership(results)

	var parentTarget *domain.Target
	for _, r := range resolved {
		for i := range r.Targets {
			if r.Targets[i].ID == "parent-underflow" {
				parentTarget = &r.Targets[i]
			}
		}
	}

	if parentTarget == nil {
		t.Fatal("parent target not found")
	}

	if parentTarget.SizeBytes != 0 {
		t.Errorf("parent SizeBytes should be clamped to 0, got %d", parentTarget.SizeBytes)
	}
	if parentTarget.EligibleBytes != 0 {
		t.Errorf("parent EligibleBytes should be clamped to 0, got %d", parentTarget.EligibleBytes)
	}
}

func TestOwnership_SymlinkDeduplication(t *testing.T) {
	tmp := t.TempDir()
	realDir := filepath.Join(tmp, "real_cache")
	symlinkDir := filepath.Join(tmp, "symlink_cache")

	if err := os.MkdirAll(realDir, 0755); err != nil {
		t.Fatalf("failed to create real dir: %v", err)
	}
	if err := os.Symlink(realDir, symlinkDir); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	canonReal, err := filepath.EvalSymlinks(realDir)
	if err != nil {
		canonReal = realDir
	}

	results := []domain.ScanResult{
		{
			Ecosystem: "macos",
			Targets: []domain.Target{
				{
					ID:        "symlink-target",
					Path:      symlinkDir,
					SizeBytes: 1024 * 1024,
				},
			},
		},
		{
			Ecosystem: "node",
			Targets: []domain.Target{
				{
					ID:        "real-target",
					Path:      realDir,
					SizeBytes: 1024 * 1024,
				},
			},
		},
	}

	resolved := ResolveCanonicalOwnership(results)

	var totalTargets int
	var survivingTarget *domain.Target
	for _, r := range resolved {
		for i := range r.Targets {
			totalTargets++
			survivingTarget = &r.Targets[i]
		}
	}

	if totalTargets != 1 {
		t.Fatalf("expected 1 surviving deduplicated target, got %d", totalTargets)
	}
	if survivingTarget.Path != canonReal {
		t.Errorf("surviving target Path = %q, want canonical %q", survivingTarget.Path, canonReal)
	}
}

func TestOwnership_CircularSymlinkHandling(t *testing.T) {
	tmp := t.TempDir()
	loopPath := filepath.Join(tmp, "circular_loop")
	// Create circular symlink loop -> loop
	if err := os.Symlink(loopPath, loopPath); err != nil {
		t.Fatalf("failed to create circular symlink: %v", err)
	}

	results := []domain.ScanResult{
		{
			Ecosystem: "node",
			Targets: []domain.Target{
				{
					ID:        "circular-target",
					Path:      loopPath,
					SizeBytes: 1024,
				},
			},
		},
	}

	// Should not hang, crash, or panic
	resolved := ResolveCanonicalOwnership(results)

	if len(resolved) != 1 || len(resolved[0].Targets) != 1 {
		t.Fatalf("expected 1 result and 1 target, got %v", resolved)
	}

	target := resolved[0].Targets[0]
	if target.Path != filepath.Clean(loopPath) {
		t.Errorf("expected path %q, got %q", filepath.Clean(loopPath), target.Path)
	}
}

func TestOwnership_VirtualTargetsImmune(t *testing.T) {
	results := []domain.ScanResult{
		{
			Ecosystem: "docker",
			Targets: []domain.Target{
				{
					ID:            "docker-img",
					Name:          "Docker image",
					Path:          "docker://image/sha256:abcdef",
					SizeBytes:     500 * 1024 * 1024,
					EligibleBytes: 500 * 1024 * 1024,
					TTLEligible:   true,
				},
			},
		},
		{
			Ecosystem: "pnpm",
			Targets: []domain.Target{
				{
					ID:            "pnpm-store",
					Name:          "pnpm store prune",
					Path:          "pnpm://store/prune",
					SizeBytes:     200 * 1024 * 1024,
					EligibleBytes: 200 * 1024 * 1024,
					TTLEligible:   true,
				},
			},
		},
	}

	resolved := ResolveCanonicalOwnership(results)

	if len(resolved) != 2 {
		t.Fatalf("expected 2 scan results, got %d", len(resolved))
	}

	if resolved[0].Targets[0].Path != "docker://image/sha256:abcdef" {
		t.Errorf("docker path altered: %q", resolved[0].Targets[0].Path)
	}
	if resolved[0].Targets[0].SizeBytes != 500*1024*1024 {
		t.Errorf("docker SizeBytes altered: %d", resolved[0].Targets[0].SizeBytes)
	}

	if resolved[1].Targets[0].Path != "pnpm://store/prune" {
		t.Errorf("pnpm path altered: %q", resolved[1].Targets[0].Path)
	}
	if resolved[1].Targets[0].SizeBytes != 200*1024*1024 {
		t.Errorf("pnpm SizeBytes altered: %d", resolved[1].Targets[0].SizeBytes)
	}
}

func TestOwnership_MultiLevelNesting(t *testing.T) {
	tmp := t.TempDir()
	rootDir := filepath.Join(tmp, "root")
	subDir := filepath.Join(rootDir, "sub")
	nestedDir := filepath.Join(subDir, "nested")

	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("failed to create nested dirs: %v", err)
	}

	results := []domain.ScanResult{
		{
			Ecosystem: "macos", // Cleaner A (Generic)
			Targets: []domain.Target{
				{
					ID:            "target-root",
					Path:          rootDir,
					SizeBytes:     100 * 1024 * 1024,
					EligibleBytes: 100 * 1024 * 1024,
				},
			},
		},
		{
			Ecosystem: "node", // Cleaner B (Dedicated)
			Targets: []domain.Target{
				{
					ID:            "target-sub",
					Path:          subDir,
					SizeBytes:     40 * 1024 * 1024,
					EligibleBytes: 40 * 1024 * 1024,
				},
			},
		},
		{
			Ecosystem: "rust", // Cleaner C (Dedicated)
			Targets: []domain.Target{
				{
					ID:            "target-nested",
					Path:          nestedDir,
					SizeBytes:     10 * 1024 * 1024,
					EligibleBytes: 10 * 1024 * 1024,
				},
			},
		},
	}

	resolved := ResolveCanonicalOwnership(results)

	var rootT, subT, nestedT *domain.Target
	for _, r := range resolved {
		for i := range r.Targets {
			switch r.Targets[i].ID {
			case "target-root":
				rootT = &r.Targets[i]
			case "target-sub":
				subT = &r.Targets[i]
			case "target-nested":
				nestedT = &r.Targets[i]
			}
		}
	}

	if rootT == nil || subT == nil || nestedT == nil {
		t.Fatalf("missing one of the targets: root=%v, sub=%v, nested=%v", rootT, subT, nestedT)
	}

	const expectedRoot = 60 * 1024 * 1024
	const expectedSub = 30 * 1024 * 1024
	const expectedNested = 10 * 1024 * 1024

	if rootT.SizeBytes != expectedRoot {
		t.Errorf("root SizeBytes = %d, want %d", rootT.SizeBytes, expectedRoot)
	}
	if rootT.EligibleBytes != expectedRoot {
		t.Errorf("root EligibleBytes = %d, want %d", rootT.EligibleBytes, expectedRoot)
	}

	if subT.SizeBytes != expectedSub {
		t.Errorf("sub SizeBytes = %d, want %d", subT.SizeBytes, expectedSub)
	}
	if subT.EligibleBytes != expectedSub {
		t.Errorf("sub EligibleBytes = %d, want %d", subT.EligibleBytes, expectedSub)
	}

	if nestedT.SizeBytes != expectedNested {
		t.Errorf("nested SizeBytes = %d, want %d", nestedT.SizeBytes, expectedNested)
	}
	if nestedT.EligibleBytes != expectedNested {
		t.Errorf("nested EligibleBytes = %d, want %d", nestedT.EligibleBytes, expectedNested)
	}

	total := rootT.SizeBytes + subT.SizeBytes + nestedT.SizeBytes
	if total != 100*1024*1024 {
		t.Errorf("total size = %d, want %d", total, 100*1024*1024)
	}
}
