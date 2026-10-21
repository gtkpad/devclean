package ai

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
	devfs "github.com/gtkpad/devclean/internal/fs"
	"github.com/gtkpad/devclean/internal/safety"
)

var protectedClaudeFiles = map[string]bool{
	"config.json":      true,
	"history.jsonl":    true,
	"settings.json":    true,
	"auth.json":        true,
	"credentials.json": true,
}

type claudeVersionsResult struct {
	activeVersionPath string
	ambiguous         bool
	missingSymlink    bool
	brokenSymlink     bool
	candidateVersions []string
}

func resolveActiveClaudeVersion(home string) claudeVersionsResult {
	versionsDir := filepath.Join(home, ".local", "share", "claude", "versions")
	localDir := filepath.Join(home, ".claude", "local")

	var candidateVersions []string
	if entries, err := os.ReadDir(versionsDir); err == nil {
		for _, e := range entries {
			p := filepath.Join(versionsDir, e.Name())
			if isLink, err := safety.IsSymlink(p); err == nil && isLink {
				continue
			}
			if _, err := os.Stat(p); err == nil {
				candidateVersions = append(candidateVersions, p)
			}
		}
	}

	if _, err := os.Stat(localDir); err == nil {
		if isLink, err := safety.IsSymlink(localDir); err == nil && !isLink {
			candidateVersions = append(candidateVersions, localDir)
		}
	}

	if len(candidateVersions) == 0 {
		return claudeVersionsResult{}
	}

	symlinkCandidates := []string{
		filepath.Join(home, ".local", "bin", "claude"),
		filepath.Join(home, ".local", "share", "claude", "current"),
		filepath.Join(home, ".local", "share", "claude", "versions", "current"),
		filepath.Join(home, ".local", "share", "claude", "claude"),
	}

	foundSymlinks := 0
	resolvedTargets := make(map[string]bool)
	var activePath string

	for _, link := range symlinkCandidates {
		isLink, err := safety.IsSymlink(link)
		if err != nil || !isLink {
			continue
		}
		foundSymlinks++

		dest, err := filepath.EvalSymlinks(link)
		if err != nil {
			return claudeVersionsResult{
				brokenSymlink:     true,
				candidateVersions: candidateVersions,
			}
		}

		cleanDest := filepath.Clean(dest)
		resolvedTargets[cleanDest] = true
		activePath = cleanDest
	}

	if foundSymlinks == 0 {
		return claudeVersionsResult{
			missingSymlink:    true,
			candidateVersions: candidateVersions,
		}
	}

	if len(resolvedTargets) > 1 {
		return claudeVersionsResult{
			ambiguous:         true,
			candidateVersions: candidateVersions,
		}
	}

	matched := false
	for _, v := range candidateVersions {
		cleanV := filepath.Clean(v)
		if evalV, err := filepath.EvalSymlinks(v); err == nil {
			cleanV = evalV
		}
		if cleanV == activePath || strings.HasPrefix(activePath, cleanV+string(filepath.Separator)) {
			matched = true
			break
		}
	}
	if !matched {
		return claudeVersionsResult{
			ambiguous:         true,
			candidateVersions: candidateVersions,
		}
	}

	return claudeVersionsResult{
		activeVersionPath: activePath,
		candidateVersions: candidateVersions,
	}
}

func scanClaudeVersions(home string, rules domain.EcosystemRules) []domain.Target {
	res := resolveActiveClaudeVersion(home)
	if len(res.candidateVersions) == 0 {
		return nil
	}

	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)
	var targets []domain.Target

	for _, verPath := range res.candidateVersions {
		info, err := os.Stat(verPath)
		if err != nil {
			continue
		}

		sz, _ := devfs.CalcDirSize(verPath)
		if sz == 0 {
			sz = info.Size()
		}

		cleanVer := filepath.Clean(verPath)
		if evalVer, err := filepath.EvalSymlinks(verPath); err == nil {
			cleanVer = evalVer
		}

		// If a valid active version was resolved, protect it from deletion
		if !res.ambiguous && !res.missingSymlink && !res.brokenSymlink && res.activeVersionPath != "" {
			if cleanVer == res.activeVersionPath || strings.HasPrefix(res.activeVersionPath, cleanVer+string(filepath.Separator)) {
				// PROTECTED: Active version is strictly preserved and never targeted
				continue
			}
		}

		target := domain.Target{
			ID:           hashPath(verPath),
			Name:         "Claude CLI version: " + filepath.Base(verPath),
			Path:         verPath,
			SizeBytes:    sz,
			Risk:         domain.RiskLow,
			LastAccessed: info.ModTime(),
			Category:     "ai-claude-versions",
		}

		switch {
		case res.missingSymlink:
			target.ReviewOnly = true
			target.ReviewReason = domain.ReviewReasonActiveVersion
			target.EligibleBytes = 0
			target.TTLEligible = false
		case res.brokenSymlink || res.ambiguous:
			target.ReviewOnly = true
			target.ReviewReason = domain.ReviewReasonAmbiguous
			target.EligibleBytes = 0
			target.TTLEligible = false
		default:
			// Confirmed inactive version
			if rules.MaxAgeDays == 0 || info.ModTime().Before(cutoff) {
				target.TTLEligible = true
				target.EligibleBytes = sz
			}
		}

		targets = append(targets, target)
	}

	return targets
}

func scanClaude(ctx context.Context, exec cleaner.CommandExecutor, home string, rules domain.EcosystemRules) ([]domain.Target, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	running, uncertain := checkProcessStatus(ctx, exec, "claude")
	isBlocked := running || uncertain

	var targets []domain.Target
	cleanClaudeDir := filepath.Clean(filepath.Join(home, ".claude"))
	if info, err := os.Stat(cleanClaudeDir); err == nil && info.IsDir() {
		candidateDirs := []string{"debug", "tmp", "cache", "telemetry"}
		cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)

		for _, sub := range candidateDirs {
			p := filepath.Join(cleanClaudeDir, sub)
			info, err := os.Stat(p)
			if err != nil || !info.IsDir() {
				continue
			}

			size, _ := devfs.CalcDirSize(p)
			if size > 0 {
				eligibleBytes := int64(0)
				ttlEligible := false
				if rules.MaxAgeDays == 0 || info.ModTime().Before(cutoff) {
					eligibleBytes = size
					ttlEligible = true
				}
				trg := domain.Target{
					ID:            hashPath(p),
					Name:          "Claude Code: " + sub + " cache",
					Path:          p,
					SizeBytes:     size,
					EligibleBytes: eligibleBytes,
					TTLEligible:   ttlEligible,
					Risk:          domain.RiskLow,
					LastAccessed:  info.ModTime(),
					Category:      "ai-claude-cache",
				}
				if isBlocked {
					trg.ReviewOnly = true
					trg.ReviewReason = domain.ReviewReasonActiveProcess
					trg.EligibleBytes = 0
					trg.TTLEligible = false
				}
				targets = append(targets, trg)
			}
		}

		// Logs soltos em ~/.claude/*.log
		entries, err := os.ReadDir(cleanClaudeDir)
		if err == nil {
			for _, e := range entries {
				if e.IsDir() || !strings.HasSuffix(e.Name(), ".log") {
					continue
				}
				if protectedClaudeFiles[e.Name()] {
					continue
				}

				p := filepath.Join(cleanClaudeDir, e.Name())
				info, err := e.Info()
				if err != nil {
					continue
				}

				eligibleBytes := int64(0)
				ttlEligible := false
				if rules.MaxAgeDays == 0 || info.ModTime().Before(cutoff) {
					eligibleBytes = info.Size()
					ttlEligible = true
				}

				trg := domain.Target{
					ID:            hashPath(p),
					Name:          "Claude Code log: " + e.Name(),
					Path:          p,
					SizeBytes:     info.Size(),
					EligibleBytes: eligibleBytes,
					TTLEligible:   ttlEligible,
					Risk:          domain.RiskLow,
					LastAccessed:  info.ModTime(),
					Category:      "ai-claude-logs",
				}
				if isBlocked {
					trg.ReviewOnly = true
					trg.ReviewReason = domain.ReviewReasonActiveProcess
					trg.EligibleBytes = 0
					trg.TTLEligible = false
				}
				targets = append(targets, trg)
			}
		}
	}

	verTargets := scanClaudeVersions(home, rules)
	if isBlocked {
		for i := range verTargets {
			verTargets[i].ReviewOnly = true
			verTargets[i].ReviewReason = domain.ReviewReasonActiveProcess
			verTargets[i].EligibleBytes = 0
			verTargets[i].TTLEligible = false
		}
	}
	targets = append(targets, verTargets...)

	return targets, nil
}

// scanClaudeDesktopLogs scans ~/Library/Logs/Claude/*.log into a consolidated target ai://claude-desktop/logs.
func scanClaudeDesktopLogs(ctx context.Context, exec cleaner.CommandExecutor, home string, rules domain.EcosystemRules) ([]domain.Target, error) {
	if home == "" {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	logsDir := filepath.Join(home, "Library", "Logs", "Claude")
	entries, err := os.ReadDir(logsDir)
	if err != nil || len(entries) == 0 {
		return nil, nil
	}

	var subPaths []string
	var totalSize int64
	var eligibleBytes int64
	var hasEligible bool
	var latestModTime time.Time

	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".log") {
			continue
		}
		p := filepath.Join(logsDir, e.Name())
		info, err := e.Info()
		if err != nil || info.Size() == 0 {
			continue
		}

		sz := info.Size()
		totalSize += sz
		subPaths = append(subPaths, p)
		if info.ModTime().After(latestModTime) {
			latestModTime = info.ModTime()
		}

		if rules.MaxAgeDays == 0 || info.ModTime().Before(cutoff) {
			eligibleBytes += sz
			hasEligible = true
		}
	}

	if totalSize == 0 {
		return nil, nil
	}

	if latestModTime.IsZero() {
		latestModTime = time.Now()
	}

	running, uncertain := checkProcessStatus(ctx, exec, "Claude", "claude-desktop")
	isBlocked := running || uncertain

	trg := domain.Target{
		ID:             hashPath("ai://claude-desktop/logs"),
		Name:           "Claude Desktop: Application Logs",
		Path:           "ai://claude-desktop/logs",
		SizeBytes:      totalSize,
		EligibleBytes:  eligibleBytes,
		TTLEligible:    hasEligible,
		Risk:           domain.RiskLow,
		LastAccessed:   latestModTime,
		Category:       "ai-claude-desktop-logs",
		IsConsolidated: true,
		SubPaths:       subPaths,
	}

	if isBlocked {
		trg.ReviewOnly = true
		trg.ReviewReason = domain.ReviewReasonActiveProcess
		trg.EligibleBytes = 0
		trg.TTLEligible = false
	}

	return []domain.Target{trg}, nil
}
