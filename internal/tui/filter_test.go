package tui

import (
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/domain"
)

func testFlatTargets() []flatTarget {
	return []flatTarget{
		{target: domain.Target{ID: "1", Path: "/home/user/project-a/node_modules", SizeBytes: 500, Risk: domain.RiskLow, LastAccessed: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}, ecosystem: "node", groupIdx: 0},
		{target: domain.Target{ID: "2", Path: "/home/user/project-b/target", SizeBytes: 1000, Risk: domain.RiskMedium, LastAccessed: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)}, ecosystem: "rust", groupIdx: 1},
		{target: domain.Target{ID: "3", Path: "/home/user/project-c/.next", SizeBytes: 200, Risk: domain.RiskLow, LastAccessed: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)}, ecosystem: "node", groupIdx: 0},
		{target: domain.Target{ID: "4", Path: "/tmp/cargo/registry", SizeBytes: 2000, Risk: domain.RiskHigh, LastAccessed: time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)}, ecosystem: "rust", groupIdx: 1},
		{target: domain.Target{ID: "5", Path: "/home/user/go/pkg/mod", SizeBytes: 800, Risk: domain.RiskMedium, LastAccessed: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)}, ecosystem: "go", groupIdx: 2},
	}
}

func TestRecomputeVisible_RiskFilter(t *testing.T) {
	targets := testFlatTargets()

	tests := []struct {
		name     string
		risk     riskFilter
		wantLen  int
		wantRisk domain.RiskLevel
	}{
		{"all returns everything", riskAll, 5, ""},
		{"low returns only low", riskFilterLow, 2, domain.RiskLow},
		{"medium returns only medium", riskFilterMedium, 2, domain.RiskMedium},
		{"high returns only high", riskFilterHigh, 1, domain.RiskHigh},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := filterState{sort: sortSize, risk: tt.risk}
			result := recomputeVisible(targets, fs, 0)
			if len(result) != tt.wantLen {
				t.Errorf("got %d indices, want %d", len(result), tt.wantLen)
			}
			if tt.wantRisk != "" {
				for _, idx := range result {
					if targets[idx].target.Risk != tt.wantRisk {
						t.Errorf("index %d has risk %s, want %s", idx, targets[idx].target.Risk, tt.wantRisk)
					}
				}
			}
		})
	}
}

func TestRecomputeVisible_FuzzyFilter(t *testing.T) {
	targets := testFlatTargets()

	tests := []struct {
		name      string
		fuzzyText string
		wantMin   int
	}{
		{"empty returns all", "", 5},
		{"node matches node_modules and .next paths", "node", 1},
		{"cargo matches cargo registry", "cargo", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := filterState{sort: sortSize, fuzzyText: tt.fuzzyText}
			result := recomputeVisible(targets, fs, 0)
			if len(result) < tt.wantMin {
				t.Errorf("got %d indices, want at least %d", len(result), tt.wantMin)
			}
		})
	}
}

func TestRecomputeVisible_SortModes(t *testing.T) {
	targets := testFlatTargets()

	t.Run("sortSize descending", func(t *testing.T) {
		fs := filterState{sort: sortSize}
		result := recomputeVisible(targets, fs, 0)
		for i := 1; i < len(result); i++ {
			prev := targets[result[i-1]].target.SizeBytes
			curr := targets[result[i]].target.SizeBytes
			if prev < curr {
				t.Errorf("not sorted by size desc: index %d (%d) < index %d (%d)", i-1, prev, i, curr)
			}
		}
	})

	t.Run("sortAge ascending (oldest first)", func(t *testing.T) {
		fs := filterState{sort: sortAge}
		result := recomputeVisible(targets, fs, 0)
		for i := 1; i < len(result); i++ {
			prev := targets[result[i-1]].target.LastAccessed
			curr := targets[result[i]].target.LastAccessed
			if prev.After(curr) {
				t.Errorf("not sorted by age asc: index %d (%v) after index %d (%v)", i-1, prev, i, curr)
			}
		}
	})

	t.Run("sortName alphabetical", func(t *testing.T) {
		fs := filterState{sort: sortName}
		result := recomputeVisible(targets, fs, 0)
		for i := 1; i < len(result); i++ {
			prev := targets[result[i-1]].target.Path
			curr := targets[result[i]].target.Path
			if prev > curr {
				t.Errorf("not sorted by name: %s > %s", prev, curr)
			}
		}
	})
}

func TestRecomputeVisible_CombinedFilters(t *testing.T) {
	targets := testFlatTargets()

	// low risk + fuzzy "project" → should filter by both
	fs := filterState{sort: sortSize, risk: riskFilterLow, fuzzyText: "project"}
	result := recomputeVisible(targets, fs, 0)

	for _, idx := range result {
		if targets[idx].target.Risk != domain.RiskLow {
			t.Errorf("index %d has risk %s, want low", idx, targets[idx].target.Risk)
		}
	}
}

func TestTargetSource_Interface(t *testing.T) {
	targets := testFlatTargets()
	src := targetSource(targets)

	if src.Len() != len(targets) {
		t.Errorf("Len() = %d, want %d", src.Len(), len(targets))
	}

	for i, ft := range targets {
		if src.String(i) != ft.target.Path {
			t.Errorf("String(%d) = %q, want %q", i, src.String(i), ft.target.Path)
		}
	}
}

func TestFilterState_IsActive(t *testing.T) {
	tests := []struct {
		name string
		fs   filterState
		want bool
	}{
		{"default is not active", filterState{sort: sortSize, risk: riskAll}, false},
		{"fuzzy text makes active", filterState{fuzzyText: "foo"}, true},
		{"non-default sort makes active", filterState{sort: sortAge}, true},
		{"risk filter makes active", filterState{risk: riskFilterHigh}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.fs.isActive(); got != tt.want {
				t.Errorf("isActive() = %v, want %v", got, tt.want)
			}
		})
	}
}
