package macos

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
)

var (
	defaultSysDiagDir = "/Library/Logs/DiagnosticReports"
	defaultVarLogDir  = "/private/var/log"
)

// scanSystemLogsAndReports scans user diagnostic reports, system diagnostic reports,
// system logs with TCC graceful degradation, and code sign clones (D-09, D-10, D-11, D-12).
// DevClean operates strictly with normal user process privileges and zero sudo.
func scanSystemLogsAndReports(ctx context.Context, exec cleaner.CommandExecutor, homeDir string) ([]domain.Target, error) {
	return scanSystemLogsAndReportsWithPaths(ctx, exec, homeDir, defaultSysDiagDir, defaultVarLogDir)
}

// scanSystemLogsAndReportsWithPaths allows hermetic path injection for testing.
func scanSystemLogsAndReportsWithPaths(ctx context.Context, exec cleaner.CommandExecutor, homeDir, sysDiagDir, varLogDir string) ([]domain.Target, error) {
	var targets []domain.Target

	// 1. User Diagnostic Reports (Eligible) (D-12)
	userDiagDir := filepath.Join(homeDir, "Library", "Logs", "DiagnosticReports")
	if info, err := os.Stat(userDiagDir); err == nil && info.IsDir() {
		usage, partial, note := scanDirectoryWithTCCFallback(ctx, userDiagDir)
		targets = append(targets, domain.Target{
			ID:            "macos-user-diagnostic-reports",
			Name:          "macOS User Diagnostic Reports",
			Path:          userDiagDir,
			SizeBytes:     usage,
			EligibleBytes: usage,
			TTLEligible:   true,
			ReviewOnly:    false,
			Risk:          domain.RiskLow,
			Category:      "macos-diagnostic-reports",
			ProviderMetrics: &domain.ProviderMetrics{
				SizeBytes:  usage,
				Partial:    partial,
				StatusNote: note,
			},
		})
	}

	// 2. System Diagnostic Reports (ReviewOnly) (D-12, D-09)
	if info, err := os.Stat(sysDiagDir); err == nil && info.IsDir() {
		usage, partial, note := scanDirectoryWithTCCFallback(ctx, sysDiagDir)
		targets = append(targets, domain.Target{
			ID:            "macos-system-diagnostic-reports",
			Name:          "macOS System Diagnostic Reports",
			Path:          sysDiagDir,
			SizeBytes:     usage,
			EligibleBytes: 0,
			TTLEligible:   false,
			ReviewOnly:    true,
			ReviewReason:  domain.ReviewReasonSystemProtected,
			Risk:          domain.RiskLow,
			Category:      "macos-diagnostic-reports",
			ProviderMetrics: &domain.ProviderMetrics{
				SizeBytes:  usage,
				Partial:    partial,
				StatusNote: note,
			},
		})
	}

	// 3. System Logs (/private/var/log) (ReviewOnly with TCC graceful degradation) (D-09, D-10)
	if info, err := os.Stat(varLogDir); err == nil && info.IsDir() {
		usage, partial, note := scanDirectoryWithTCCFallback(ctx, varLogDir)
		targets = append(targets, domain.Target{
			ID:            "macos-system-logs",
			Name:          "macOS System Logs (/private/var/log)",
			Path:          varLogDir,
			SizeBytes:     usage,
			EligibleBytes: 0,
			TTLEligible:   false,
			ReviewOnly:    true,
			ReviewReason:  domain.ReviewReasonSystemProtected,
			Risk:          domain.RiskLow,
			Category:      "macos-system-logs",
			ProviderMetrics: &domain.ProviderMetrics{
				SizeBytes:  usage,
				Partial:    partial,
				StatusNote: note,
			},
		})
	}

	// 4. Code Sign Clones in Darwin User Cache Dir (D-11)
	if cloneTarget, ok := scanCodeSignClones(ctx, exec); ok {
		targets = append(targets, cloneTarget)
	}

	return targets, nil
}

// scanDirectoryWithTCCFallback walks a directory trapping permission errors (D-09).
func scanDirectoryWithTCCFallback(ctx context.Context, root string) (totalSize int64, partial bool, note string) {
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if walkErr != nil {
			if errors.Is(walkErr, os.ErrPermission) || os.IsPermission(walkErr) {
				partial = true
				if note == "" {
					note = "permission denied (TCC / root required)"
				}
				if d != nil && d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			return nil
		}
		if d.IsDir() || d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			if errors.Is(err, os.ErrPermission) || os.IsPermission(err) {
				partial = true
				if note == "" {
					note = "permission denied (TCC / root required)"
				}
			}
			return nil
		}
		totalSize += info.Size()
		return nil
	})

	if err != nil && (errors.Is(err, os.ErrPermission) || os.IsPermission(err)) {
		partial = true
		note = "permission denied (TCC / root required)"
	}
	return totalSize, partial, note
}

// scanCodeSignClones isolates search to current user's Darwin cache directory (D-11).
func scanCodeSignClones(ctx context.Context, exec cleaner.CommandExecutor) (domain.Target, bool) {
	userTmp := os.Getenv("TMPDIR")
	if userTmp == "" {
		return domain.Target{}, false
	}

	cleanTmp := filepath.Clean(userTmp)
	parent := filepath.Dir(cleanTmp)
	cDir := filepath.Join(parent, "C")

	info, err := os.Stat(cDir)
	if err != nil || !info.IsDir() {
		return domain.Target{}, false
	}

	var cloneSize int64
	var cloneCount int

	_ = filepath.WalkDir(cDir, func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.HasSuffix(d.Name(), ".code_sign_clone") {
			if fInfo, err := d.Info(); err == nil {
				cloneSize += fInfo.Size()
				cloneCount++
			}
		}
		return nil
	})

	if cloneCount == 0 {
		return domain.Target{}, false
	}

	// Process guard: Chrome or Chromium running? (D-14)
	running, uncertain := CheckProcessStatus(ctx, exec, "Google Chrome", "Chromium")
	isActive := running || uncertain

	reviewOnly := isActive
	reason := ""
	eligible := cloneSize
	if isActive {
		reviewOnly = true
		reason = domain.ReviewReasonActiveProcess
		eligible = 0
	}

	return domain.Target{
		ID:            "macos-code-sign-clones",
		Name:          "macOS Code-Signature Clones",
		Path:          cDir,
		SizeBytes:     cloneSize,
		EligibleBytes: eligible,
		TTLEligible:   !reviewOnly,
		ReviewOnly:    reviewOnly,
		ReviewReason:  reason,
		Risk:          domain.RiskLow,
		Category:      "macos-system-clones",
	}, true
}
