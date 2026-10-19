# Phase 07: ReviewOnly & Coordinator Policy - Pattern Map

**Mapped:** 2026-10-15  
**Files analyzed:** 12  
**Analogs found:** 12 / 12  

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/domain/types.go` | model | transform | `internal/domain/types.go` | exact |
| `internal/domain/target_test.go` | test | transform | `internal/domain/types_test.go` | exact |
| `internal/deleter/deleter.go` | service | batch | `internal/deleter/deleter.go` | exact |
| `internal/deleter/policy_test.go` | test | batch | `internal/deleter/deleter_test.go` | exact |
| `cmd/clean.go` | controller | batch | `cmd/clean.go` | exact |
| `cmd/format.go` | utility | transform | `cmd/format.go` | exact |
| `internal/ui/report.go` | component | transform | `internal/ui/report.go` | exact |
| `internal/tui/view.go` | component | event-driven | `internal/tui/view.go` | exact |
| `internal/tui/model.go` | controller | event-driven | `internal/tui/model.go` | exact |
| `internal/tui/review_test.go` | test | event-driven | `internal/tui/model_test.go` | exact |
| `internal/mcp/server.go` | controller | request-response | `internal/mcp/server.go` | exact |
| `internal/mcp/review_test.go` | test | request-response | `internal/mcp/server_test.go` | exact |

---

## Pattern Assignments

### `internal/domain/types.go` (model, transform)

**Analog:** `internal/domain/types.go`

**Role & Data Flow:** Central domain entity declarations, field extensions, and helper receiver methods for immutability and zero-allocation serialization.

**Imports pattern** (`internal/domain/types.go:1-7`):
```go
package domain

import (
	"context"
	"strings"
	"time"
)
```

**Core struct definition & Constants pattern** (`internal/domain/types.go:9-41`):
```go
type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

type Target struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Path          string        `json:"path"`
	SizeBytes     int64         `json:"size_bytes"`
	EligibleBytes int64         `json:"eligible_bytes"`
	TTLEligible   bool          `json:"ttl_eligible"`
	Risk          RiskLevel     `json:"risk"`
	LastAccessed  time.Time     `json:"last_accessed"`
	Category      string        `json:"category"`
	AI            *AIAssessment `json:"ai_assessment,omitempty"`
	SmartScore    float64       `json:"smart_score,omitempty"`
}

// IsVirtual returns true if the target represents a virtual resource (e.g. Docker images,
// volumes, build cache) that cannot be manipulated via standard filesystem operations.
func (t Target) IsVirtual() bool {
	return strings.HasPrefix(t.Path, "docker://") || strings.Contains(t.Path, "://")
}
```

**Adaptation for Phase 7:**
- Add semantic review reason constants (`ReviewReasonSharedStore`, `ReviewReasonActiveProcess`, `ReviewReasonOrphanLeftover`, `ReviewReasonSystemProtected`, `ReviewReasonSessionSensitive`).
- Add flat fields `ReviewOnly bool` (`json:"review_only"`) and `ReviewReason string` (`json:"review_reason,omitempty"`).
- Add helper method `func (t Target) IsReviewOnly() bool { return t.ReviewOnly }`.
- Maintain structural rule: when `ReviewOnly == true`, `EligibleBytes` must evaluate to `0` and `TTLEligible` to `false`.

---

### `internal/domain/target_test.go` (test, transform)

**Analog:** `internal/domain/types_test.go`

**Role & Data Flow:** Unit tests verifying domain model invariants, JSON round-trip marshaling, backward compatibility, and helper method logic.

**Imports pattern** (`internal/domain/types_test.go:1-7`):
```go
package domain

import (
	"encoding/json"
	"testing"
	"time"
)
```

**JSON serialization & Helper test pattern** (`internal/domain/types_test.go:119-181`):
```go
func TestTarget_IsVirtual(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"docker://sha256:abc", true},
		{"docker://volume/my-vol", true},
		{"custom://artifact", true},
		{"/home/user/project/node_modules", false},
		{"/Users/user/.cache/go-build", false},
		{"", false},
	}

	for _, tc := range tests {
		target := Target{Path: tc.path}
		if got := target.IsVirtual(); got != tc.want {
			t.Errorf("Target{Path: %q}.IsVirtual() = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestTarget_JSONSerialization(t *testing.T) {
	target := Target{
		ID:            "test-ttl-1",
		Name:          "node_modules",
		Path:          "/home/user/project/node_modules",
		SizeBytes:     1024000,
		EligibleBytes: 512000,
		TTLEligible:   true,
		Risk:          RiskMedium,
		LastAccessed:  time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
		Category:      "node",
	}

	data, err := json.Marshal(target)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
...
```

**Backward compatibility verification pattern** (`internal/domain/types_test.go:183-211`):
```go
func TestTarget_BackwardCompatibility(t *testing.T) {
	// Old JSON payload without eligible_bytes or ttl_eligible
	oldJSON := `{
		"id": "legacy-1",
		"name": "cargo-target",
		"path": "/home/user/rust/target",
		"size_bytes": 2048000,
		"risk": "low",
		"category": "rust"
	}`

	var decoded Target
	if err := json.Unmarshal([]byte(oldJSON), &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.EligibleBytes != 0 {
		t.Errorf("decoded.EligibleBytes = %d, want 0", decoded.EligibleBytes)
	}
	if decoded.TTLEligible != false {
		t.Errorf("decoded.TTLEligible = %v, want false", decoded.TTLEligible)
	}
}
```

**Adaptation for Phase 7:**
- Verify `Target.IsReviewOnly()` returns expected boolean.
- Assert JSON serialization contains `review_only: true` and `review_reason: "shared_store"` when present, and omits `review_reason` when empty.
- Verify backward compatibility: unmarshaling payloads without `review_only` defaults `ReviewOnly` to `false` and `ReviewReason` to `""`.
- Assert invariant: when `ReviewOnly == true`, `EligibleBytes == 0` and `TTLEligible == false`.

---

### `internal/deleter/deleter.go` (service, batch)

**Analog:** `internal/deleter/deleter.go`

**Role & Data Flow:** Central deletion engine and single authoritative gate orchestrating the 6-stage lifecycle, filtering policies, and encapsulation of side-effects under `!opts.DryRun`.

**Imports pattern** (`internal/deleter/deleter.go:3-18`):
```go
package deleter

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/history"
	"github.com/gtkpad/devclean/internal/hooks"
	"github.com/gtkpad/devclean/internal/safety"
	"github.com/gtkpad/devclean/internal/scheduler"
	"github.com/gtkpad/devclean/internal/trash"
)
```

**Result model & Stage 1 safety gate pattern** (`internal/deleter/deleter.go:46-53, 132-143`):
```go
type Result struct {
	TotalFreed   int64
	TotalCount   int
	CleanedPaths []string
	Ecosystems   []string
	Errors       []error
	Duration     time.Duration
}

// Stage 1: Safety Validation & Blacklist filtering
var safeTargets []domain.Target
for _, t := range rawTargets {
	if c.blacklist != nil && c.blacklist.IsBlacklisted(t.Path) {
		continue
	}
	safeTargets = append(safeTargets, t)
}
if len(safeTargets) == 0 {
	continue
}
```

**DryRun side-effect encapsulation pattern** (`internal/deleter/deleter.go:343-363`):
```go
	// Stage 6.1: Silent Trash Auto-Expiration & Quota Management (D-09, D-10)
	if !opts.DryRun && c.trashMgr != nil {
		_ = c.trashMgr.EnforceRetention(ctx)

		quotaBefore := c.trashMgr.QuotaStatus()
		_ = c.trashMgr.EnforceMaxSize(ctx)
		quotaAfter := c.trashMgr.QuotaStatus()

		if quotaBefore.Exceeded || quotaAfter.Exceeded {
			reportQuota := quotaBefore
			if !reportQuota.Exceeded {
				reportQuota = quotaAfter
			}
			if opts.OnWarning != nil {
				opts.OnWarning(fmt.Sprintf(
					"Aviso: Lixeira atingiu %s (limite configurado: %s). Lotes mais antigos foram expurgados.",
					humanSize(reportQuota.CurrentBytes), humanSize(reportQuota.MaxBytes),
				))
			}
		}
	}
```

**Adaptation for Phase 7:**
- Add `SkippedReviewOnly []domain.Target` to `Result` struct (`D-06`).
- In Stage 1: evaluate canonical symlinks against blacklist; check `t.ReviewOnly`. If `t.ReviewOnly`, append to `res.SkippedReviewOnly`, fire `opts.OnWarning(fmt.Sprintf("Item de revisão retido [%s]: %s (%s)", ...))`, and skip from `safeTargets` (`D-05`).
- If all targets in batch are `ReviewOnly` or blacklisted, finish gracefully with `TotalCount: 0, TotalFreed: 0, err == nil` (`D-07`).
- Ensure all Stage 3-6 mutations remain strictly guarded by `if !opts.DryRun` (`D-13`, `D-14`).

---

### `internal/deleter/policy_test.go` (test, batch)

**Analog:** `internal/deleter/deleter_test.go`

**Role & Data Flow:** Integration and policy tests asserting single-gate safety invariants, `ReviewOnly` rejection, 100% review batch completion, symlink protection, and canary file assertions under `DryRun`.

**Imports & Test Mocks pattern** (`internal/deleter/deleter_test.go:1-45, 79-96`):
```go
package deleter_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/deleter"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/history"
	"github.com/gtkpad/devclean/internal/hooks"
	"github.com/gtkpad/devclean/internal/safety"
	"github.com/gtkpad/devclean/internal/scheduler"
	"github.com/gtkpad/devclean/internal/trash"
)

func createPhysicalTarget(t *testing.T, baseDir, relPath string, size int64) domain.Target {
	t.Helper()
	targetPath := filepath.Join(baseDir, relPath)
	if err := os.MkdirAll(targetPath, 0o755); err != nil {
		t.Fatalf("creating target dir %s: %v", targetPath, err)
	}
	contentFile := filepath.Join(targetPath, "file.bin")
	data := make([]byte, size)
	if err := os.WriteFile(contentFile, data, 0o644); err != nil {
		t.Fatalf("writing target file %s: %v", contentFile, err)
	}
	return domain.Target{
		ID:        relPath,
		Name:      filepath.Base(relPath),
		Path:      targetPath,
		SizeBytes: size,
	}
}
```

**Canary verification & DryRun invariant pattern** (`internal/deleter/deleter_test.go:484-548`):
```go
// Running Coordinator with DryRun: true:
opts := deleter.Options{DryRun: true}
res, err := coord.Delete(context.Background(), targetsByEco, opts)
if err != nil {
	t.Fatalf("unexpected error: %v", err)
}
// Assert canary file still exists on disk unchanged
if _, err := os.Stat(canaryFile); os.IsNotExist(err) {
	t.Errorf("canary file was mutated or deleted during dry-run!")
}
```

**Adaptation for Phase 7:**
- `TestCoordinator_ReviewOnly_RejectionInStage1`: create target with `ReviewOnly: true`, verify it is rejected into `res.SkippedReviewOnly` with warning emitted.
- `TestCoordinator_ReviewOnly_100PercentBatch_GracefulExit`: run with only `ReviewOnly` items, assert `res.TotalCount == 0`, `res.TotalFreed == 0`, and `err == nil`.
- `TestCoordinator_Blacklist_SymlinkResolution`: symlink pointing to `.git` or protected path is rejected in Stage 1.
- `TestCoordinator_DryRun_ZeroMutation_Canary`: verify canary file, trash manifest, scheduler last-run, and history JSONL remain 100% untouched during DryRun (`D-15`).

---

### `cmd/clean.go` (controller, batch)

**Analog:** `cmd/clean.go`

**Role & Data Flow:** CLI command handler that parses user flags (`--auto`, `--force`, `--dry-run`, `--trash`), coordinates inventory candidates, delegates exclusively to `Coordinator.Delete`, and reports output.

**Imports pattern** (`cmd/clean.go:3-22`):
```go
package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gtkpad/devclean/internal/deleter"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/engine"
	"github.com/gtkpad/devclean/internal/fs"
	"github.com/gtkpad/devclean/internal/hooks"
	"github.com/gtkpad/devclean/internal/intelligence"
	"github.com/gtkpad/devclean/internal/safety"
	"github.com/gtkpad/devclean/internal/trash"
	"github.com/gtkpad/devclean/internal/ui"
	"github.com/spf13/cobra"
)
```

**Coordinator execution & Reporting pattern** (`cmd/clean.go:216-246, 295-316`):
```go
			opts := deleter.Options{
				DryRun:   dryRun,
				UseTrash: useTrashOpt,
				Mode:     mode,
				Auto:     auto,
				OnProgress: func(ev deleter.ProgressEvent) {
					if quiet || format == "json" {
						return
					}
					switch ev.Action {
					case deleter.ActionWouldDelete:
						fmt.Fprintf(cmd.OutOrStdout(), "Would delete: %s (%s)\n", ev.Target.Path, humanSize(ev.Target.SizeBytes))
					case deleter.ActionMovingTrash:
						fmt.Fprintf(cmd.OutOrStdout(), "Moving to trash: %s\n", ev.Target.Path)
					case deleter.ActionDeletingDirect, deleter.ActionVirtualDirect:
						fmt.Fprintf(cmd.OutOrStdout(), "Deleting: %s\n", ev.Target.Path)
					}
				},
				OnWarning: func(msg string) {
					if !quiet {
						fmt.Fprintln(os.Stderr, "Warning:", msg)
					}
				},
			}

			res, err := coord.Delete(ctx, targetsByEco, opts)
			if err != nil {
				return fmt.Errorf("deletion failed: %w", err)
			}
...
			reportData := ui.ReportData{
				Title:          title,
				TotalFreed:     res.TotalFreed,
				TotalCount:     res.TotalCount,
				TrashCount:     trashCount,
				PermanentCount: permanentCount,
				Duration:       res.Duration,
				DiskBefore:     diskBeforeFree,
				DiskAfter:      diskAfterFree,
				Ecosystems:     ecoMetrics,
			}

			if format == "json" {
				return ui.FormatReportJSON(cmd.OutOrStdout(), reportData)
			}

			ui.RenderReport(cmd.OutOrStdout(), reportData)
```

**Adaptation for Phase 7:**
- Remove external trash maintenance calls (`trashMgr.EnforceRetention`, `trashMgr.EnforceMaxSize`) at lines 184-195 (`D-13`).
- Pass `res.SkippedReviewOnly` count and bytes to `ui.ReportData` (`D-12`).
- Do not fail if `totalCandidates > 0` but `res.TotalCount == 0` due to 100% `ReviewOnly` targets (`D-07`).

---

### `cmd/format.go` (utility, transform)

**Analog:** `cmd/format.go`

**Role & Data Flow:** CLI presentation formatter converting scan results into aligned ASCII tables or structured JSON with column padding and indicator badges.

**Imports pattern** (`cmd/format.go:1-11`):
```go
package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/format"
)
```

**Table formatting & column layout pattern** (`cmd/format.go:48-72`):
```go
		for _, t := range r.Targets {
			path := t.Path
			if len(path) > 50 {
				path = "..." + path[len(path)-47:]
			}

			lastAccessed := t.LastAccessed.Format("2006-01-02")

			switch {
			case smart:
				fmt.Fprintf(w, "%-12s %-50s %10s %8s %12s %6.2f\n",
					r.Ecosystem, path, humanSize(t.SizeBytes), string(t.Risk), lastAccessed, t.SmartScore)
			case verbose:
				extra := fmt.Sprintf("[%s] %s", t.ID, t.Name)
				if t.EligibleBytes < t.SizeBytes {
					extra += fmt.Sprintf(" (eligible: %s, ttl: %t)", humanSize(t.EligibleBytes), t.TTLEligible)
				}
				fmt.Fprintf(w, "%-12s %-50s %10s %8s %12s  %s\n",
					r.Ecosystem, path, humanSize(t.SizeBytes), string(t.Risk), lastAccessed, extra)
			default:
				fmt.Fprintf(w, "%-12s %-50s %10s %8s %12s\n",
					r.Ecosystem, path, humanSize(t.SizeBytes), string(t.Risk), lastAccessed)
			}
		}
```

**Adaptation for Phase 7:**
- In `formatTable`, prefix `path = "⊙ " + path` when `t.ReviewOnly` is true (`D-09`).
- In `verbose` mode, append `[ReviewOnly: <reason>] (eligible: 0 B)` when `t.ReviewOnly` is true (`D-09`).
- In `formatJSON`, `domain.Target` fields `review_only` and `review_reason` serialize automatically.

---

### `internal/ui/report.go` (component, transform)

**Analog:** `internal/ui/report.go`

**Role & Data Flow:** Renders Lip Gloss card summaries and structured JSON reports for post-cleaning status, displaying recovered bytes, duration, and notices.

**Imports pattern** (`internal/ui/report.go:1-16`):
```go
package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"golang.org/x/term"
)
```

**Card styling & report layout pattern** (`internal/ui/report.go:26-36, 108-118`):
```go
type ReportData struct {
	Title          string            `json:"title"`
	TotalFreed     int64             `json:"total_freed"`
	TotalCount     int               `json:"total_count"`
	TrashCount     int               `json:"trash_count"`
	PermanentCount int               `json:"permanent_count"`
	Duration       time.Duration     `json:"duration"`
	DiskBefore     *uint64           `json:"disk_before,omitempty"`
	DiskAfter      *uint64           `json:"disk_after,omitempty"`
	Ecosystems     []EcosystemMetric `json:"ecosystems,omitempty"`
}

	var sb strings.Builder
	title := data.Title
	if title == "" {
		title = "DevClean Summary"
	}
	sb.WriteString(headerStyle.Render("✨ " + title) + "\n\n")

	sb.WriteString(fmt.Sprintf("%s %s\n", labelStyle.Render("Espaço Recuperado:"), valueStyle.Render(formatBytes(data.TotalFreed))))
	sb.WriteString(fmt.Sprintf("%s %s\n", labelStyle.Render("Duração Total:    "), valueStyle.Render(fmt.Sprintf("%d ms", data.Duration.Milliseconds()))))
	sb.WriteString(fmt.Sprintf("%s %s\n", labelStyle.Render("Alvos Processados:"), valueStyle.Render(fmt.Sprintf("%d (Lixeira: %d, Permanente: %d)", data.TotalCount, data.TrashCount, data.PermanentCount))))
```

**Adaptation for Phase 7:**
- Add `SkippedReviewCount int` and `SkippedReviewBytes int64` to `ReportData` and `CleanReportJSON`.
- When `SkippedReviewCount > 0`, render informative notice banner in card:
  `sb.WriteString(fmt.Sprintf("\n%s\n", reviewNoticeStyle.Render(fmt.Sprintf("⊙ %d itens mantidos para revisão (%s) — execute 'devclean scan' para detalhes", data.SkippedReviewCount, formatBytes(data.SkippedReviewBytes)))))` (`D-12`).

---

### `internal/tui/view.go` (component, event-driven)

**Analog:** `internal/tui/view.go`

**Role & Data Flow:** View layer of the Bubbletea TUI, responsible for line-by-line rendering of targets, indicators, and status badges.

**Imports pattern** (`internal/tui/view.go:1-14`):
```go
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/gtkpad/devclean/internal/domain"
)
```

**Target line rendering pattern** (`internal/tui/view.go:157-190`):
```go
func renderTargetLine(m model, idx int, ft flatTarget) string {
	// Cursor indicator
	cursor := "  "
	if idx == m.cursor {
		cursor = "> "
	}

	// Checkbox
	checkbox := "[ ]"
	if m.selected[ft.target.ID] {
		checkbox = "[x]"
	}

	// Risk badge
	badge := riskBadge(ft.target.Risk)

	// Path (truncated if needed)
	maxPathLen := 50
	if m.width > 90 {
		maxPathLen = m.width - 40
	}
	path := ft.target.Path
	if len(path) > maxPathLen {
		path = "..." + path[len(path)-maxPathLen+3:]
	}

	// Size
	size := humanSize(ft.target.SizeBytes)

	// Smart icon
	icon := m.targetIcon(ft.target)

	return fmt.Sprintf("%s%s %s%s  %10s  %s", cursor, checkbox, icon, path, size, badge)
}
```

**Adaptation for Phase 7:**
- Define `reviewIndicatorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)`.
- If `ft.target.ReviewOnly`, render `checkbox = reviewIndicatorStyle.Render("[⊙]")` instead of `[ ]` or `[x]` (`D-10`).

---

### `internal/tui/model.go` (controller, event-driven)

**Analog:** `internal/tui/model.go`

**Role & Data Flow:** Bubbletea reactive event loop managing cursor movements, hotkey selection updates (`Space`, `a`, `m`, `l`), and state transitions.

**Imports pattern** (`internal/tui/model.go:1-18`):
```go
package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/deleter"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/safety"
)
```

**Selection update handlers pattern** (`internal/tui/model.go:248-305`):
```go
	case msg.Type == tea.KeyRunes && string(msg.Runes) == "m":
		threshold := m.cfg.TUI.MassSelectThresholdMB * 1024 * 1024
		for _, idx := range m.visibleTargets {
			if m.flatTargets[idx].target.SizeBytes >= threshold {
				m.selected[m.flatTargets[idx].target.ID] = true
			}
		}

	case msg.Type == tea.KeySpace:
		if m.cursor < len(m.visibleTargets) {
			id := m.flatTargets[m.visibleTargets[m.cursor]].target.ID
			m.selected[id] = !m.selected[id]
			if !m.selected[id] {
				delete(m.selected, id)
			}
		}

	case msg.Type == tea.KeyRunes && string(msg.Runes) == "a":
		for _, idx := range m.visibleTargets {
			m.selected[m.flatTargets[idx].target.ID] = true
		}

	case msg.Type == tea.KeyRunes && string(msg.Runes) == "l":
		m.selected = make(map[string]bool)
		for _, idx := range m.visibleTargets {
			if m.flatTargets[idx].target.Risk == domain.RiskLow {
				m.selected[m.flatTargets[idx].target.ID] = true
			}
		}
```

**Adaptation for Phase 7:**
- On `KeySpace`: check `if t.ReviewOnly`, ignore toggle and do not mutate `m.selected` (`D-10`).
- On `"a"`, `"m"`, and `"l"`: guard loops with `if !t.ReviewOnly`, preventing bulk selection of review targets (`D-10`).

---

### `internal/tui/review_test.go` (test, event-driven)

**Analog:** `internal/tui/model_test.go` and `internal/tui/view_test.go`

**Role & Data Flow:** TUI unit tests simulating key events on `model` and inspecting string output from `renderTargetLine` / `viewSelecting`.

**Imports & Setup pattern** (`internal/tui/model_test.go:1-38`):
```go
package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/deleter"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/safety"
)

func keyMsg(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}
```

**Model update testing pattern** (`internal/tui/model_test.go:81-90`):
```go
func TestUpdate_Selecting_CursorDown(t *testing.T) {
	m := testModel()

	updated, _ := m.Update(keyMsg('j'))
	um := updated.(model)

	if um.cursor != 1 {
		t.Errorf("cursor = %d, want 1", um.cursor)
	}
}
```

**Adaptation for Phase 7:**
- `TestTUI_ReviewOnly_RenderLockedIndicator`: construct flatTarget with `ReviewOnly: true`, verify rendered line contains `[⊙]`.
- `TestTUI_ReviewOnly_SpaceToggle_Ignored`: position cursor on ReviewOnly target, press Space, assert `m.selected[id]` remains false.
- `TestTUI_ReviewOnly_MassSelect_Excluded`: trigger `'a'`, `'m'`, `'l'`, assert non-review targets are selected while `ReviewOnly` target remains unselected.

---

### `internal/mcp/server.go` (controller, request-response)

**Analog:** `internal/mcp/server.go`

**Role & Data Flow:** MCP tool server implementing `scan_environment`, `propose_deletion`, and `execute_approved_deletion` for LLM integrations.

**Imports pattern** (`internal/mcp/server.go:1-18`):
```go
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/deleter"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/engine"
	"github.com/gtkpad/devclean/internal/history"
	"github.com/gtkpad/devclean/internal/safety"
	"github.com/gtkpad/devclean/internal/trash"
	"github.com/mark3labs/mcp-go/mcp"
)
```

**`handlePropose` pattern** (`internal/mcp/server.go:218-239`):
```go
	type proposalEntry struct {
		ID       string `json:"id"`
		Path     string `json:"path"`
		Size     int64  `json:"size_bytes"`
		Status   string `json:"status"` // "approved", "blocked", "not_found"
		Reason   string `json:"reason,omitempty"`
	}

	var entries []proposalEntry
	for _, id := range ids {
		t, found := targetMap[id]
		if !found {
			entries = append(entries, proposalEntry{ID: id, Status: "not_found", Reason: "target not found in current scan"})
			continue
		}
		if s.bl.IsBlacklisted(t.Path) {
			entries = append(entries, proposalEntry{ID: id, Path: t.Path, Size: t.SizeBytes, Status: "blocked", Reason: "path is protected by safety blacklist"})
			continue
		}
		entries = append(entries, proposalEntry{ID: id, Path: t.Path, Size: t.SizeBytes, Status: "approved"})
	}
```

**Adaptation for Phase 7:**
- In `proposalEntry`: add `EligibleBytes int64` (`json:"eligible_bytes"`) and `ReviewReason string` (`json:"review_reason,omitempty"`).
- In `handlePropose`: if `t.ReviewOnly`, set `Status: "review_only"`, `EligibleBytes: 0`, `ReviewReason: t.ReviewReason`, and explanatory `Reason` (`D-11`).
- In `handleScan`: `Target` serializes `review_only` and `review_reason` directly (`D-11`).
- In `handleExecute`: pass directly to `Coordinator.Delete()`, where Stage 1 rejects `ReviewOnly` targets even if somehow requested (`D-08`).

---

### `internal/mcp/review_test.go` (test, request-response)

**Analog:** `internal/mcp/server_test.go`

**Role & Data Flow:** MCP tool integration tests verifying JSON request-response contracts for `propose_deletion` and `scan_environment`.

**Imports & Fixture pattern** (`internal/mcp/server_test.go:1-18, 41-61`):
```go
package mcp

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/deleter"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/engine"
	"github.com/gtkpad/devclean/internal/history"
	"github.com/gtkpad/devclean/internal/platform"
	"github.com/gtkpad/devclean/internal/safety"
	"github.com/mark3labs/mcp-go/mcp"
)

func testServer() (*Server, *fakeCleaner) {
...
```

**`handlePropose` assertion pattern** (`internal/mcp/server_test.go:151-189`):
```go
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"target_ids": `["safe-1", "blocked-1", "missing-1"]`,
	}

	result, err := srv.handlePropose(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := extractText(t, result)
	var entries []struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(text), &entries); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
```

**Adaptation for Phase 7:**
- Test `propose_deletion` with a `ReviewOnly` target: assert `status: "review_only"`, `eligible_bytes: 0`, and `review_reason: "shared_store"`.
- Test `scan_environment`: assert returned targets include `review_only: true` and `review_reason` attributes.

---

## Shared Patterns

### 1. Central Gate Policy Interception
**Source:** `internal/deleter/deleter.go:132-143`
**Apply to:** `internal/deleter/deleter.go`, `cmd/clean.go`, `internal/mcp/server.go`
```go
// Stage 1: Central safety & policy gate in Coordinator
var safeTargets []domain.Target
for _, t := range rawTargets {
	// Canonical symlink + blacklist check
	if c.blacklist != nil && (c.blacklist.IsBlacklisted(t.Path) || isSymlinkToBlacklist(t.Path, c.blacklist)) {
		continue
	}
	// Policy check: ReviewOnly rejection
	if t.ReviewOnly {
		res.SkippedReviewOnly = append(res.SkippedReviewOnly, t)
		if opts.OnWarning != nil {
			opts.OnWarning(fmt.Sprintf("Item de revisão retido [%s]: %s (%s)", t.ReviewReason, t.Path, humanSize(t.SizeBytes)))
		}
		continue
	}
	safeTargets = append(safeTargets, t)
}
```

### 2. DryRun Zero-Mutation Invariant
**Source:** `internal/deleter/deleter.go:156-160, 343-348`
**Apply to:** `internal/deleter/deleter.go`, `cmd/clean.go`
```go
// Invariant: all side-effecting operations encapsulated under !opts.DryRun
if !opts.DryRun {
	// 1. Hooks execution
	// 2. Trash moving or direct deletion
	// 3. History JSONL logging
	// 4. Scheduler last-run metrics
	// 5. Trash auto-expiration and quota management
}
```

### 3. Canonical Symlink Resolution Guard
**Source:** `internal/safety/symlink.go:7-13`, `internal/safety/blacklist.go:62-76`
**Apply to:** `internal/safety/blacklist.go`, `internal/deleter/deleter.go`
```go
func isSymlinkToBlacklist(path string, bl *safety.Blacklist) bool {
	if bl == nil {
		return false
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	return resolved != path && bl.IsBlacklisted(resolved)
}
```

### 4. Non-Fatal Structured Warnings via Callbacks
**Source:** `internal/deleter/deleter.go:43, 356-361`, `cmd/clean.go:234-238`
**Apply to:** `internal/deleter/deleter.go`, `cmd/clean.go`
```go
if opts.OnWarning != nil {
	opts.OnWarning(fmt.Sprintf("Aviso: %s", message))
}
```

### 5. `⊙` Marker Visual Formatting
**Source:** `cmd/format.go:48-72`, `internal/tui/view.go:157-190`, `internal/ui/report.go:113-118`
**Apply to:** `cmd/format.go`, `internal/tui/view.go`, `internal/ui/report.go`
```go
// Visual indicator pattern for review targets
// CLI: prefix "⊙ " to path
// TUI: render "[⊙]" in place of checkbox
// Summary Card: "⊙ N itens mantidos para revisão (X GB)"
```

---

## No Analog Found

None. All 12 files to be created or modified have direct analogs in the existing repository.

---

## Metadata

**Analog search scope:**
- `internal/domain/`
- `internal/deleter/`
- `cmd/`
- `internal/ui/`
- `internal/tui/`
- `internal/mcp/`
- `internal/safety/`

**Files scanned:** 84 tracked source files  
**Pattern extraction date:** 2026-10-15  
