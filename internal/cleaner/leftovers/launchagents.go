package leftovers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
)

var knownInterpreters = map[string]bool{
	"sh": true, "bash": true, "zsh": true, "csh": true, "tcsh": true, "ksh": true, "fish": true,
	"python": true, "python2": true, "python3": true, "pypy": true, "pypy3": true,
	"perl": true, "ruby": true, "osascript": true, "tclsh": true, "wish": true,
	"env": true, "node": true, "deno": true, "bun": true,
}

// isInterpreter returns true if the executable matches a known shell, runtime, or wrapper script interpreter.
func isInterpreter(execPath string) bool {
	base := strings.ToLower(filepath.Base(execPath))
	if knownInterpreters[base] {
		return true
	}
	for prefix := range knownInterpreters {
		if strings.HasPrefix(base, prefix) {
			return true
		}
	}
	return false
}

// extractExecutableCandidate extracts the primary executable from a LaunchAgent plist.
// Per launchd.plist(5) semantics: Program takes precedence over ProgramArguments[0].
func extractExecutableCandidate(p *LaunchAgentPlist) string {
	if p.Program != "" {
		return p.Program
	}
	if len(p.ProgramArguments) > 0 {
		return p.ProgramArguments[0]
	}
	return ""
}

// extractInterpreterScriptTarget inspects subsequent arguments to extract the first script/bundle path (D-10).
func extractInterpreterScriptTarget(args []string) string {
	for i := 1; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		if arg == "" || strings.HasPrefix(arg, "-") {
			continue // Skip flags like -c, --wait-apps, -e
		}
		// Return first non-flag argument that looks like an absolute path or bundle
		if strings.HasPrefix(arg, "/") {
			return arg
		}
	}
	return ""
}

// sanitizeID sanitizes a string for use in target IDs.
func sanitizeID(s string) string {
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, "\\", "-")
	s = strings.ReplaceAll(s, " ", "-")
	return s
}

// buildLaunchAgentTarget constructs an individual target for a LaunchAgent plist (D-09).
// In accordance with LEFT-04, EligibleBytes is always 0 and ReviewOnly is always true.
func buildLaunchAgentTarget(plistPath string, size int64, reason, note string) domain.Target {
	baseName := filepath.Base(plistPath)
	name := fmt.Sprintf("Stale LaunchAgent: %s", baseName)
	if reason == domain.ReviewReasonAmbiguous {
		name = fmt.Sprintf("LaunchAgent (ambiguous): %s", baseName)
	}

	return domain.Target{
		ID:            fmt.Sprintf("launchagent-%s", sanitizeID(baseName)),
		Name:          name,
		Path:          plistPath,
		SizeBytes:     size,
		EligibleBytes: 0,
		TTLEligible:   false,
		ReviewOnly:    true,
		ReviewReason:  reason,
		Category:      "leftovers-launchagent",
		Risk:          domain.RiskLow,
		ProviderMetrics: &domain.ProviderMetrics{
			SizeBytes:  size,
			StatusNote: note,
		},
	}
}

// scanLaunchAgents scans the specified LaunchAgents directories for stale or ambiguous launchd plists.
// Strictly excludes /System/Library (D-03). Degrades gracefully on permission denial.
func scanLaunchAgents(ctx context.Context, exec cleaner.CommandExecutor, dirs []string) ([]domain.Target, error) {
	if exec == nil {
		exec = &cleaner.DefaultExecutor{}
	}
	var targets []domain.Target

	for _, dir := range dirs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		cleanDir := filepath.Clean(dir)

		// D-03: Incondicionalmente excluir /System/Library (protegido por SIP/Apple)
		if strings.Contains(cleanDir, filepath.Join("System", "Library")) ||
			strings.Contains(cleanDir, "/System/Library") {
			continue
		}

		isSystemDir := strings.HasPrefix(cleanDir, "/Library/LaunchAgents")
		entries, err := os.ReadDir(cleanDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			if os.IsPermission(err) {
				// D-03: Graceful degradation on permission denied without root/elevation
				targets = append(targets, domain.Target{
					ID:           fmt.Sprintf("launchagent-dir-%s", sanitizeID(filepath.Base(cleanDir))),
					Name:         fmt.Sprintf("LaunchAgents: %s", cleanDir),
					Path:         cleanDir,
					ReviewOnly:   true,
					ReviewReason: domain.ReviewReasonSystemProtected,
					Category:     "leftovers-launchagent",
					Risk:         domain.RiskLow,
					ProviderMetrics: &domain.ProviderMetrics{
						Partial:    true,
						StatusNote: "permission denied (TCC / root required)",
					},
				})
				continue
			}
			continue
		}

		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".plist") {
				continue
			}

			plistPath := filepath.Join(cleanDir, e.Name())
			info, err := os.Lstat(plistPath)
			if err != nil {
				continue
			}

			// Symlink Guard & Ownership check (D-09, D-16)
			if info.Mode()&os.ModeSymlink != 0 {
				targets = append(targets, buildLaunchAgentTarget(plistPath, info.Size(), domain.ReviewReasonAmbiguous, "unsafe or external symlink target"))
				continue
			}
			if !isAllowedOwnership(info, isSystemDir) {
				targets = append(targets, buildLaunchAgentTarget(plistPath, info.Size(), domain.ReviewReasonAmbiguous, "insecure ownership: not owned by current user"))
				continue
			}

			// Parse plist in pure Go
			p, err := parseLaunchAgentFile(plistPath)
			if err != nil {
				// D-04: Fail-closed on corrupt or unreadable plists with size 0
				targets = append(targets, buildLaunchAgentTarget(plistPath, 0, domain.ReviewReasonAmbiguous, fmt.Sprintf("corrupt or unreadable plist: %v", err)))
				continue
			}

			candidate := extractExecutableCandidate(p)
			if candidate == "" {
				// Empty plist (e.g. com.google.keystone.agent.plist <dict/>): benign migrated stub, skip
				if len(p.ProgramArguments) == 0 && p.Program == "" {
					continue
				}
				targets = append(targets, buildLaunchAgentTarget(plistPath, info.Size(), domain.ReviewReasonAmbiguous, "empty or missing Program/ProgramArguments"))
				continue
			}

			// Non-absolute candidate resolution via LookPath (D-11)
			resolvedExe := candidate
			if !filepath.IsAbs(candidate) {
				if path, err := exec.LookPath(candidate); err == nil {
					resolvedExe = path
				} else {
					targets = append(targets, buildLaunchAgentTarget(plistPath, info.Size(), domain.ReviewReasonAmbiguous, fmt.Sprintf("relative or PATH-based executable not found: %s", candidate)))
					continue
				}
			}

			// Interpreter / launcher wrapper check (D-10)
			if isInterpreter(resolvedExe) || filepath.Base(resolvedExe) == "open" {
				scriptTarget := extractInterpreterScriptTarget(p.ProgramArguments)
				if scriptTarget != "" {
					if _, err := os.Stat(scriptTarget); err == nil {
						// Script or bundle exists on disk -> LaunchAgent is active and valid!
						continue
					} else if os.IsNotExist(err) {
						targets = append(targets, buildLaunchAgentTarget(plistPath, info.Size(), domain.ReviewReasonStaleLaunchAgent, fmt.Sprintf("missing script target: %s", scriptTarget)))
						continue
					} else {
						targets = append(targets, buildLaunchAgentTarget(plistPath, info.Size(), domain.ReviewReasonAmbiguous, fmt.Sprintf("script target check error: %v", err)))
						continue
					}
				}
				// If no script target is discernible, treat as ambiguous rather than stale
				targets = append(targets, buildLaunchAgentTarget(plistPath, info.Size(), domain.ReviewReasonAmbiguous, fmt.Sprintf("interpreter or wrapper invocation without verifiable target: %s", candidate)))
				continue
			}

			// Direct executable check
			if _, err := os.Stat(resolvedExe); err != nil {
				if os.IsNotExist(err) {
					targets = append(targets, buildLaunchAgentTarget(plistPath, info.Size(), domain.ReviewReasonStaleLaunchAgent, fmt.Sprintf("missing executable: %s", resolvedExe)))
				} else {
					targets = append(targets, buildLaunchAgentTarget(plistPath, info.Size(), domain.ReviewReasonAmbiguous, fmt.Sprintf("executable check error: %v", err)))
				}
			}
			// If binary exists on disk (err == nil), item is active and NOT surfaced
		}
	}

	return targets, nil
}
