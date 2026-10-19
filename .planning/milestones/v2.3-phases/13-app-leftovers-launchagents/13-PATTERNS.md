# Phase 13: App Leftovers & LaunchAgents - Pattern Map

**Mapped:** 2026-10-18  
**Files analyzed:** 17 (10 new, 7 modified)  
**Analogs found:** 17 / 17 (15 exact, 2 role-match, 0 no analog)  

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|:---:|
| `internal/cleaner/leftovers/cleaner.go` | service / cleaner | batch / file-I/O | `internal/cleaner/macos/macos.go` | exact |
| `internal/cleaner/leftovers/plist.go` | utility / parser | file-I/O / transform | `internal/cleaner/browsers/safari.go` | role-match |
| `internal/cleaner/leftovers/launchagents.go` | service / scanner | file-I/O / batch | `internal/cleaner/macos/daemons.go` | exact |
| `internal/cleaner/leftovers/apps.go` | service / scanner | file-I/O / batch | `internal/cleaner/macos/appsupport.go` | role-match |
| `internal/cleaner/leftovers/leftovers.go` | service / scanner | file-I/O / batch | `internal/cleaner/macos/appsupport.go` | exact |
| `internal/cleaner/leftovers/exclusions.go` | middleware / filter | transform | `internal/cleaner/macos/whitelist.go` | exact |
| `internal/cleaner/leftovers/ownership_darwin.go` | utility / guard | file-I/O | `internal/cleaner/macos/ownership_darwin.go` | exact |
| `internal/cleaner/leftovers/ownership_other.go` | utility / guard | file-I/O | `internal/cleaner/macos/ownership_other.go` | exact |
| `internal/cleaner/leftovers/fixtures_test.go` | test / fixture | file-I/O | `internal/cleaner/browsers/fixtures_test.go` | exact |
| `internal/cleaner/leftovers/cleaner_test.go` | test | batch / file-I/O | `internal/cleaner/macos/macos_test.go` | exact |
| `internal/domain/types.go` | model | config / transform | `internal/domain/types.go` | exact |
| `internal/domain/target_test.go` | test | batch | `internal/domain/target_test.go` | exact |
| `internal/config/config.go` | config | transform | `internal/config/config.go` | exact |
| `internal/config/defaults.go` | config | transform | `internal/config/defaults.go` | exact |
| `internal/engine/register.go` | service / registry | transform | `internal/engine/register.go` | exact |
| `internal/engine/engine.go` | service / coordinator | request-response | `internal/engine/engine.go` | exact |
| `internal/engine/register_test.go` | test | batch | `internal/engine/register_test.go` | exact |

---

## Pattern Assignments

### `internal/cleaner/leftovers/cleaner.go` (service / cleaner, batch / file-I/O)

**Analog:** `internal/cleaner/macos/macos.go` (lines 16-112)

**Imports pattern:**
```go
package leftovers

import (
	"context"
	"os"
	"path/filepath"
	"runtime"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
)
```

**Constructor & Dependency Injection pattern** (`macos.go` lines 16-45):
```go
type leftoversCleaner struct {
	exec             cleaner.CommandExecutor
	homeDir          string
	goos             string
	appRoots         []string
	launchAgentsDirs []string
	libraryDir       string
}

func New() domain.Cleaner {
	return NewWithExecutor(&cleaner.DefaultExecutor{})
}

func NewWithExecutor(exec cleaner.CommandExecutor) domain.Cleaner {
	home, _ := os.UserHomeDir()
	return NewWithConfig(exec, home, runtime.GOOS, nil, nil, "")
}

func NewWithConfig(
	exec cleaner.CommandExecutor,
	homeDir, goos string,
	appRoots []string,
	launchAgentsDirs []string,
	libraryDir string,
) domain.Cleaner {
	if appRoots == nil {
		appRoots = []string{"/Applications", "/System/Applications", filepath.Join(homeDir, "Applications")}
	}
	if launchAgentsDirs == nil {
		launchAgentsDirs = []string{
			filepath.Join(homeDir, "Library", "LaunchAgents"),
			"/Library/LaunchAgents",
		}
	}
	if libraryDir == "" {
		libraryDir = filepath.Join(homeDir, "Library")
	}
	return &leftoversCleaner{
		exec:             exec,
		homeDir:          homeDir,
		goos:             goos,
		appRoots:         appRoots,
		launchAgentsDirs: launchAgentsDirs,
		libraryDir:       libraryDir,
	}
}

func (c *leftoversCleaner) Name() string {
	return "leftovers"
}

func (c *leftoversCleaner) IsInstalled() bool {
	return c.goos == "darwin" || c.homeDir != ""
}
```

**Scan aggregation pattern** (`macos.go` lines 54-98):
```go
func (c *leftoversCleaner) Scan(ctx context.Context, rules domain.EcosystemRules) ([]domain.Target, error) {
	if !rules.Enabled {
		return nil, nil
	}

	var allTargets []domain.Target

	// 1. Scan LaunchAgents (LEFT-01, LEFT-02)
	launchTargets, err := scanLaunchAgents(ctx, c.launchAgentsDirs)
	if err == nil {
		allTargets = append(allTargets, launchTargets...)
	}

	// 2. Index installed applications & scan App Leftovers (LEFT-03)
	appIndex, err := scanInstalledApps(ctx, c.appRoots)
	if err == nil {
		leftoverTargets, err := scanAppLeftovers(ctx, c.libraryDir, appIndex)
		if err == nil {
			allTargets = append(allTargets, leftoverTargets...)
		}
	}

	return allTargets, nil
}
```

**Inviolable Clean() Refusal pattern** (`LEFT-04`, `D-13`):
```go
func (c *leftoversCleaner) Clean(ctx context.Context, targets []domain.Target) error {
	// Defensive refusal guard (LEFT-04, D-13):
	// Leftovers and LaunchAgents must NEVER be deleted automatically in v2.3.
	// This method unconditionally refuses physical deletion, ensuring zero mutations
	// even under direct test invocations or coordinator bypass.
	return nil
}
```

---

### `internal/cleaner/leftovers/plist.go` (utility / parser, file-I/O / transform)

**Analog:** `internal/cleaner/browsers/safari.go` & `internal/fs/statfs.go` (file structure decoding in pure Go)

**Imports & Structs pattern:**
```go
package leftovers

import (
	"os"

	"howett.net/plist"
)

type LaunchAgentPlist struct {
	Label            string   `plist:"Label"`
	Program          string   `plist:"Program"`
	ProgramArguments []string `plist:"ProgramArguments"`
	Disabled         bool     `plist:"Disabled"`
}

type AppBundleInfoPlist struct {
	CFBundleIdentifier string `plist:"CFBundleIdentifier"`
	CFBundleName       string `plist:"CFBundleName"`
	CFBundleExecutable string `plist:"CFBundleExecutable"`
}
```

**Universal Unmarshaling pattern** (transparently decodes XML & binary `bplist00`):
```go
func parseLaunchAgentFile(path string) (*LaunchAgentPlist, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p LaunchAgentPlist
	if _, err := plist.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func parseAppBundleInfoFile(path string) (*AppBundleInfoPlist, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var info AppBundleInfoPlist
	if _, err := plist.Unmarshal(data, &info); err != nil {
		return nil, err
	}
	return &info, nil
}
```

---

### `internal/cleaner/leftovers/launchagents.go` (service / scanner, file-I/O / batch)

**Analog:** `internal/cleaner/macos/daemons.go` (lines 31-99)

**Candidate Extraction & Stale Classification pattern** (`D-02`, `D-04`, `LEFT-02`):
```go
package leftovers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/safety"
)

func extractExecutableCandidate(p *LaunchAgentPlist) string {
	if p.Program != "" {
		return p.Program
	}
	if len(p.ProgramArguments) > 0 {
		return p.ProgramArguments[0]
	}
	return ""
}

func scanLaunchAgents(ctx context.Context, dirs []string) ([]domain.Target, error) {
	var targets []domain.Target

	for _, dir := range dirs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			if os.IsPermission(err) {
				// D-03: Graceful degradation on permission denied
				targets = append(targets, domain.Target{
					ID:           fmt.Sprintf("launchagent-dir-%s", filepath.Base(dir)),
					Name:         fmt.Sprintf("LaunchAgents: %s", dir),
					Path:         dir,
					ReviewOnly:   true,
					ReviewReason: domain.ReviewReasonSystemProtected,
					ProviderMetrics: domain.ProviderMetrics{
						Partial:    true,
						StatusNote: "permission denied (TCC / root required)",
					},
					Category: "leftovers-launchagent",
					Risk:     domain.RiskLow,
				})
			}
			continue
		}

		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".plist") {
				continue
			}

			plistPath := filepath.Join(dir, e.Name())
			info, err := os.Lstat(plistPath)
			if err != nil {
				continue
			}

			// Symlink guard & ownership check
			if info.Mode()&os.ModeSymlink != 0 {
				targets = append(targets, buildLaunchAgentTarget(plistPath, info.Size(), domain.ReviewReasonAmbiguous, "unsafe or external symlink target"))
				continue
			}
			if !isOwnedByCurrentUser(info) {
				targets = append(targets, buildLaunchAgentTarget(plistPath, info.Size(), domain.ReviewReasonAmbiguous, "insecure ownership: not owned by current user"))
				continue
			}

			p, err := parseLaunchAgentFile(plistPath)
			if err != nil {
				targets = append(targets, buildLaunchAgentTarget(plistPath, info.Size(), domain.ReviewReasonAmbiguous, fmt.Sprintf("corrupt or unreadable plist: %v", err)))
				continue
			}

			candidate := extractExecutableCandidate(p)
			if candidate == "" {
				targets = append(targets, buildLaunchAgentTarget(plistPath, info.Size(), domain.ReviewReasonAmbiguous, "empty or missing Program/ProgramArguments"))
				continue
			}

			if isInterpreter(candidate) {
				targets = append(targets, buildLaunchAgentTarget(plistPath, info.Size(), domain.ReviewReasonAmbiguous, fmt.Sprintf("interpreter or wrapper invocation: %s", candidate)))
				continue
			}

			if !filepath.IsAbs(candidate) {
				targets = append(targets, buildLaunchAgentTarget(plistPath, info.Size(), domain.ReviewReasonAmbiguous, fmt.Sprintf("relative or PATH-based executable: %s", candidate)))
				continue
			}

			// Absolute direct executable check
			if _, err := os.Stat(candidate); err != nil {
				if os.IsNotExist(err) {
					// STRONG STALE EVIDENCE (LEFT-02, D-02)
					targets = append(targets, buildLaunchAgentTarget(plistPath, info.Size(), domain.ReviewReasonStaleLaunchAgent, fmt.Sprintf("missing executable: %s", candidate)))
				} else {
					targets = append(targets, buildLaunchAgentTarget(plistPath, info.Size(), domain.ReviewReasonAmbiguous, fmt.Sprintf("executable check error: %v", err)))
				}
			}
			// If binary exists on disk -> active, do not surface
		}
	}

	return targets, nil
}
```

---

### `internal/cleaner/leftovers/apps.go` (service / scanner, file-I/O / batch)

**Analog:** `internal/cleaner/macos/appsupport.go` (lines 24-74)

**Depth-Bounded App Discovery pattern** (`D-05`, `D-06`):
```go
package leftovers

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

type AppIndex struct {
	BundleIDs map[string]bool
	AppNames  map[string]bool
}

func scanInstalledApps(ctx context.Context, appRoots []string) (*AppIndex, error) {
	index := &AppIndex{
		BundleIDs: make(map[string]bool),
		AppNames:  make(map[string]bool),
	}

	for _, root := range appRoots {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if _, err := os.Stat(root); err != nil {
			continue
		}

		cleanRoot := filepath.Clean(root)
		rootDepth := strings.Count(cleanRoot, string(filepath.Separator))

		_ = filepath.WalkDir(cleanRoot, func(path string, d os.DirEntry, err error) error {
			if err != nil || ctx.Err() != nil {
				return nil
			}

			// Bounded depth: max 3 levels deeper than root
			currentDepth := strings.Count(path, string(filepath.Separator))
			if currentDepth-rootDepth > 3 {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			if d.IsDir() && strings.HasSuffix(d.Name(), ".app") {
				appNameNoExt := strings.TrimSuffix(d.Name(), ".app")
				index.addAppName(appNameNoExt)

				// Parse Info.plist inside bundle
				infoPlistPath := filepath.Join(path, "Contents", "Info.plist")
				if info, err := parseAppBundleInfoFile(infoPlistPath); err == nil {
					if info.CFBundleIdentifier != "" {
						index.addBundleID(info.CFBundleIdentifier)
					}
					if info.CFBundleName != "" {
						index.addAppName(info.CFBundleName)
					}
					if info.CFBundleExecutable != "" {
						index.addAppName(info.CFBundleExecutable)
					}
				}

				return filepath.SkipDir // CRITICAL: do not descend into .app bundle
			}
			return nil
		})
	}

	return index, nil
}
```

---

### `internal/cleaner/leftovers/leftovers.go` (service / scanner, file-I/O / batch)

**Analog:** `internal/cleaner/macos/appsupport.go` (lines 52-95) & `internal/fs/usage.go` (lines 20-30)

**Scanning & App Leftover Consolidation pattern** (`D-08`, `D-10`, `D-11`):
```go
package leftovers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/fs"
	"github.com/gtkpad/devclean/internal/safety"
)

type AppLeftoverGroup struct {
	AppName string
	Paths   []string
}

func scanAppLeftovers(ctx context.Context, libraryDir string, index *AppIndex) ([]domain.Target, error) {
	locations := []string{
		filepath.Join(libraryDir, "Application Support"),
		filepath.Join(libraryDir, "Caches"),
		filepath.Join(libraryDir, "Containers"),
	}

	groups := make(map[string]*AppLeftoverGroup)

	for _, loc := range locations {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		entries, err := os.ReadDir(loc)
		if err != nil {
			continue
		}

		for _, e := range entries {
			if !e.IsDir() {
				continue
			}

			dirName := e.Name()
			fullPath := filepath.Join(loc, dirName)

			// 1. Exclusion Shield check (Apple, dedicated cleaners, CLI tools)
			if isExcluded(dirName, fullPath) {
				continue
			}

			// 2. Cross-check against installed AppIndex
			if index.Matches(dirName) {
				continue
			}

			// 3. Symlink guard
			linfo, err := os.Lstat(fullPath)
			if err != nil || (linfo.Mode()&os.ModeSymlink != 0 && !isSafeSymlink(loc, fullPath)) {
				continue
			}

			// Group candidates by canonical app name
			canonical := normalizeAppGroupName(dirName)
			if groups[canonical] == nil {
				groups[canonical] = &AppLeftoverGroup{
					AppName: canonical,
					Paths:   []string{},
				}
			}
			groups[canonical].Paths = append(groups[canonical].Paths, fullPath)
		}
	}

	var targets []domain.Target
	for _, g := range groups {
		var totalAlloc, totalApparent int64
		for _, p := range g.Paths {
			usage, _ := fs.CalcUsageWithContext(ctx, p)
			totalAlloc += usage.AllocatedBytes
			totalApparent += usage.ApparentBytes
		}

		size := totalAlloc
		if size == 0 {
			size = totalApparent
		}

		targets = append(targets, domain.Target{
			ID:            fmt.Sprintf("app-leftover-%s", sanitize(g.AppName)),
			Name:          fmt.Sprintf("App Leftover: %s", g.AppName),
			Path:          g.Paths[0],
			SizeBytes:     size,
			EligibleBytes: 0,
			TTLEligible:   false,
			ReviewOnly:    true,
			ReviewReason:  domain.ReviewReasonAppLeftover,
			Risk:          domain.RiskLow,
			Category:      "leftovers-app",
			ProviderMetrics: domain.ProviderMetrics{
				TotalCount:     len(g.Paths),
				SizeBytes:      size,
				ApparentBytes:  totalApparent,
				AllocatedBytes: totalAlloc,
				StatusNote:     "residual directories: " + strings.Join(g.Paths, ", "),
			},
		})
	}

	return targets, nil
}
```

---

### `internal/cleaner/leftovers/exclusions.go` (middleware / filter, transform)

**Analog:** `internal/cleaner/macos/whitelist.go` (lines 12-46, 48-94)

**Multi-Level Exclusion Filter pattern** (`D-07`):
```go
package leftovers

import (
	"strings"

	"github.com/gtkpad/devclean/internal/cleaner/macos"
	"github.com/gtkpad/devclean/internal/safety"
)

var appleSystemDirNames = map[string]bool{
	"addressbook": true, "animoji": true, "app store": true, "callhistorydb": true,
	"callhistorytransactions": true, "categories": true, "clouddocs": true,
	"controlcenter": true, "crashreporter": true, "differentialprivacy": true,
	"diskimages": true, "facetime": true, "fileprovider": true, "knowledge": true,
	"passes": true, "quicklook": true, "safari": true, "screencontinuity": true,
	"siri": true, "syncservices": true, "apple": true, "apple push service": true,
}

var dedicatedCleanerDirs = map[string]bool{
	"pnpm": true, "uv": true, "pip": true, "homebrew": true, "cocoapods": true,
	"vcpkg": true, "mise": true, "gh": true, "ms-playwright": true, "cypress": true,
	"node": true, "yarn": true, "dev.orbstack.orbstack": true, "docker": true,
	"coresimulator": true, "developer": true, "google": true, "bravesoftware": true,
	"company.thebrowser.arc": true, "microsoft edge": true, "firefox": true,
	"code": true, "cursor": true, "antigravity": true, "windsurf": true,
	"claude": true, "opencode": true, "zed": true,
}

var commonCLITools = map[string]bool{
	"git": true, "gnupg": true, "gpg": true, "ssh": true, "zsh": true,
	"oh-my-zsh": true, "pre-commit": true, "cargo": true, "rustup": true,
	"go": true, "bun": true, "brew": true, "lima": true, "colima": true,
	"podman": true, "tmux": true, "nvim": true, "vim": true, "subversion": true,
}

func isExcluded(name, fullPath string) bool {
	lower := strings.ToLower(name)
	if strings.HasPrefix(lower, "com.apple.") || strings.HasPrefix(lower, "apple") || appleSystemDirNames[lower] {
		return true
	}
	if dedicatedCleanerDirs[lower] || commonCLITools[lower] {
		return true
	}
	if safety.IsCoreBlacklisted(fullPath) || macos.IsWhitelisted(fullPath) {
		return true
	}
	return false
}
```

---

### `internal/cleaner/leftovers/ownership_darwin.go` & `ownership_other.go` (utility / guard, file-I/O)

**Analog:** `internal/cleaner/macos/ownership_darwin.go` (lines 1-22) & `internal/cleaner/macos/ownership_other.go` (lines 1-13)

**Darwin build pattern:**
```go
//go:build darwin

package leftovers

import (
	"os"
	"syscall"
)

func isOwnedByCurrentUser(info os.FileInfo) bool {
	if info == nil {
		return false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return true
	}
	return stat.Uid == uint32(os.Getuid())
}
```

**Non-Darwin build pattern:**
```go
//go:build !darwin

package leftovers

import (
	"os"
)

func isOwnedByCurrentUser(info os.FileInfo) bool {
	return true
}
```

---

### `internal/cleaner/leftovers/fixtures_test.go` (test / fixture, file-I/O)

**Analog:** `internal/cleaner/browsers/fixtures_test.go` (lines 9-99)

**Fixture Generator pattern** (`VAL-01`):
```go
package leftovers

import (
	"os"
	"path/filepath"
	"testing"

	"howett.net/plist"
)

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
		t.Fatalf("failed to marshal info.plist: %v", err)
	}
	if err := os.WriteFile(filepath.Join(contentsDir, "Info.plist"), data, 0644); err != nil {
		t.Fatalf("failed to write Info.plist: %v", err)
	}
	return appPath
}

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
```

---

### `internal/cleaner/leftovers/cleaner_test.go` (test, batch / file-I/O)

**Analog:** `internal/cleaner/macos/macos_test.go` (lines 77-125)

**Lifecycle, Stale Classification & Inviolable Clean Refusal Test pattern** (`VAL-01`):
```go
package leftovers

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
)

func TestLeftovers_LaunchAgents_XMLAndBinary(t *testing.T) {
	tempDir := t.TempDir()
	launchAgentsDir := filepath.Join(tempDir, "LaunchAgents")
	if err := os.MkdirAll(launchAgentsDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	createTestLaunchAgentXML(t, launchAgentsDir, "com.test.xml.plist", "com.test.xml", "/nonexistent/binary/xml", nil)
	createTestLaunchAgentBinary(t, launchAgentsDir, "com.test.bin.plist", "com.test.bin", "/nonexistent/binary/bin", nil)

	c := NewWithConfig(&cleaner.DefaultExecutor{}, tempDir, "darwin", nil, []string{launchAgentsDir}, "")
	targets, err := c.Scan(context.Background(), domain.EcosystemRules{Enabled: true})
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}

	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(targets))
	}
	for _, target := range targets {
		if target.ReviewReason != domain.ReviewReasonStaleLaunchAgent {
			t.Errorf("target %s reason = %q, want %q", target.ID, target.ReviewReason, domain.ReviewReasonStaleLaunchAgent)
		}
		if !target.ReviewOnly || target.EligibleBytes != 0 {
			t.Errorf("target %s must be ReviewOnly with 0 EligibleBytes", target.ID)
		}
	}
}

func TestLeftovers_CleanRefusesDeletion(t *testing.T) {
	tempDir := t.TempDir()
	launchAgentsDir := filepath.Join(tempDir, "LaunchAgents")
	_ = os.MkdirAll(launchAgentsDir, 0755)

	plistFile := createTestLaunchAgentXML(t, launchAgentsDir, "com.sentinel.plist", "com.sentinel", "/missing/bin", nil)

	c := NewWithConfig(&cleaner.DefaultExecutor{}, tempDir, "darwin", nil, []string{launchAgentsDir}, "")
	targets, _ := c.Scan(context.Background(), domain.EcosystemRules{Enabled: true})

	// Invoke Clean directly
	err := c.Clean(context.Background(), targets)
	if err != nil {
		t.Fatalf("Clean() returned error: %v", err)
	}

	// Verify 100% of sentinel files remain intact on disk (LEFT-04, D-13)
	if _, err := os.Stat(plistFile); os.IsNotExist(err) {
		t.Fatalf("CRITICAL SECURITY VIOLATION: sentinel plist was physically deleted by Clean()!")
	}
}
```

---

### `internal/domain/types.go` & `target_test.go` (model / test, config / transform)

**Analog:** `internal/domain/types.go` (lines 17-26) & `target_test.go` (lines 11-28)

**Constant additions:**
```go
// Semantic Review Reasons (POL-01, D-02, D-10, D-12)
const (
	ReviewReasonSharedStore      = "shared_store"
	ReviewReasonActiveProcess    = "active_process"
	ReviewReasonOrphanLeftover   = "orphan_leftover"
	ReviewReasonSystemProtected  = "system_protected"
	ReviewReasonSessionSensitive = "session_sensitive"
	ReviewReasonActiveVersion    = "active_version"
	ReviewReasonAmbiguous        = "ambiguous_state"
	ReviewReasonStaleLaunchAgent = "stale_launchagent"
	ReviewReasonAppLeftover      = "app_leftover"
)
```

**Unit Test assertion pattern** (`target_test.go`):
```go
	{domain.ReviewReasonStaleLaunchAgent, "stale_launchagent"},
	{domain.ReviewReasonAppLeftover, "app_leftover"},
```

---

### `internal/config/config.go` & `defaults.go` (config, transform)

**Analog:** `internal/config/config.go` (lines 50-60, 95-100) & `defaults.go` (lines 9-31, 41-63)

**Config schema additions:**
```go
type EcosystemsConfig struct {
	// ...
	MacOS     EcosystemConfig `mapstructure:"macos"`
	Browsers  EcosystemConfig `mapstructure:"browsers"`
	Leftovers EcosystemConfig `mapstructure:"leftovers"`
}

type HooksConfig struct {
	// ...
	MacOS     HookConfig `mapstructure:"macos"`
	Browsers  HookConfig `mapstructure:"browsers"`
	Leftovers HookConfig `mapstructure:"leftovers"`
}
```

**Defaults additions:**
```go
Leftovers: EcosystemConfig{Enabled: true, MaxAgeDays: 0, FailMode: "continue", ExcludePaths: []string{}},
// ...
Leftovers: HookConfig{Timeout: "30s"},
```

---

### `internal/engine/register.go`, `engine.go` & `register_test.go` (service / registry / coordinator)

**Analog:** `internal/engine/register.go` (lines 70-73), `engine.go` (lines 198-205) & `register_test.go` (lines 43-52)

**Registration wiring (`register.go`):**
```go
	eng.Register(macos.NewWithExecutor(exec))
	eng.Register(browsers.NewWithExecutor(exec))
	eng.Register(leftovers.NewWithExecutor(exec))
```

**Ecosystem config dispatch (`engine.go`):**
```go
	case "browsers":
		return e.config.Ecosystems.Browsers
	case "leftovers":
		return e.config.Ecosystems.Leftovers
```

**Engine test assertion update (`register_test.go`):**
```go
	wantEcosystems := []string{
		"ai", "browsers", "bun", "cocoapods", "cpp", "docker", "dotnet",
		"elixir", "flutter", "go", "homebrew", "java", "leftovers", "macos", "nextjs",
		"node", "pnpm", "python", "rust", "shell", "terraform", "xcode",
	} // 22 ecosystems sorted alphabetically
```

---

## Shared Patterns

### 1. Inviolable Zero-Mutation Guard (`LEFT-04`)
**Source:** `internal/cleaner/leftovers/cleaner.go` & `internal/deleter/coordinator.go`  
**Apply to:** All leftover and LaunchAgent targets  
```go
// Every target emitted sets ReviewOnly: true, EligibleBytes: 0, and TTLEligible: false.
// In cleaner.Clean(), physical deletion is unconditionally refused (noop return nil).
```

### 2. Pure Go Zero-CGO Plist Unmarshaling
**Source:** `howett.net/plist`  
**Apply to:** `internal/cleaner/leftovers/plist.go`  
```go
var p LaunchAgentPlist
_, err := plist.Unmarshal(data, &p) // Handles XML and Apple binary plists (bplist00)
```

### 3. Non-CGO Darwin/Linux Ownership Guard
**Source:** `internal/cleaner/macos/ownership_darwin.go` & `ownership_other.go`  
**Apply to:** `internal/cleaner/leftovers/ownership_darwin.go` & `ownership_other.go`  
```go
stat, ok := info.Sys().(*syscall.Stat_t)
return stat.Uid == uint32(os.Getuid())
```

### 4. Disk Usage Calculation with Inode Deduplication
**Source:** `internal/fs/usage.go` (`fs.CalcUsageWithContext`)  
**Apply to:** `internal/cleaner/leftovers/leftovers.go`  
```go
usage, _ := fs.CalcUsageWithContext(ctx, fullPath)
size := usage.AllocatedBytes
if size == 0 {
    size = usage.ApparentBytes
}
```

### 5. Multi-Level Exclusion Shielding
**Source:** `internal/cleaner/macos/whitelist.go`  
**Apply to:** `internal/cleaner/leftovers/exclusions.go`  
```go
if isExcluded(dirName, fullPath) || safety.IsCoreBlacklisted(fullPath) || macos.IsWhitelisted(fullPath) {
    continue
}
```

---

## No Analog Found

None. All 17 files have direct analogs or existing code patterns in the codebase.

---

## Metadata

**Analog search scope:** `internal/cleaner/macos`, `internal/cleaner/browsers`, `internal/fs`, `internal/domain`, `internal/config`, `internal/engine`  
**Files scanned:** 17  
**Pattern extraction date:** 2026-10-18  
