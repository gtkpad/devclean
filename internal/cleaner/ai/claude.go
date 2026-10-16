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
)

var protectedClaudeFiles = map[string]bool{
	"config.json":      true,
	"history.jsonl":    true,
	"settings.json":    true,
	"auth.json":        true,
	"credentials.json": true,
}

func scanClaude(ctx context.Context, exec cleaner.CommandExecutor, home string, rules domain.EcosystemRules) ([]domain.Target, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cleanClaudeDir := filepath.Clean(filepath.Join(home, ".claude"))
	if _, err := os.Stat(cleanClaudeDir); os.IsNotExist(err) {
		return nil, nil
	}

	var targets []domain.Target
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
			targets = append(targets, domain.Target{
				ID:            hashPath(p),
				Name:          "Claude Code: " + sub + " cache",
				Path:          p,
				SizeBytes:     size,
				EligibleBytes: eligibleBytes,
				TTLEligible:   ttlEligible,
				Risk:          domain.RiskLow,
				LastAccessed:  info.ModTime(),
				Category:      "ai-claude-cache",
			})
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

			targets = append(targets, domain.Target{
				ID:            hashPath(p),
				Name:          "Claude Code log: " + e.Name(),
				Path:          p,
				SizeBytes:     info.Size(),
				EligibleBytes: eligibleBytes,
				TTLEligible:   ttlEligible,
				Risk:          domain.RiskLow,
				LastAccessed:  info.ModTime(),
				Category:      "ai-claude-logs",
			})
		}
	}

	return targets, nil
}
