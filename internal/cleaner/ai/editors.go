package ai

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gtkpad/devclean/internal/domain"
	devfs "github.com/gtkpad/devclean/internal/fs"
)

func scanEditors(home string, rules domain.EcosystemRules) ([]domain.Target, error) {
	if home == "" {
		return nil, nil
	}

	var targets []domain.Target
	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)

	// Diretórios de dados de usuários por plataforma
	var appSupportDirs []string
	if runtime.GOOS == "darwin" {
		appSupportDirs = []string{
			filepath.Join(home, "Library", "Application Support", "Code"),
			filepath.Join(home, "Library", "Application Support", "Cursor"),
		}
	} else {
		appSupportDirs = []string{
			filepath.Join(home, ".config", "Code"),
			filepath.Join(home, ".config", "Cursor"),
		}
	}

	// 1. CachedData (bytecode V8) e Logs
	for _, appDir := range appSupportDirs {
		editorName := filepath.Base(appDir)
		cachedDataDir := filepath.Join(appDir, "CachedData")
		if info, err := os.Stat(cachedDataDir); err == nil && info.IsDir() {
			size, _ := devfs.CalcDirSize(cachedDataDir)
			if size > 0 {
				eligibleBytes := int64(0)
				ttlEligible := false
				if rules.MaxAgeDays == 0 || info.ModTime().Before(cutoff) {
					eligibleBytes = size
					ttlEligible = true
				}
				targets = append(targets, domain.Target{
					ID:            hashPath(cachedDataDir),
					Name:          editorName + ": CachedData (V8 bytecode)",
					Path:          cachedDataDir,
					SizeBytes:     size,
					EligibleBytes: eligibleBytes,
					TTLEligible:   ttlEligible,
					Risk:          domain.RiskLow,
					LastAccessed:  info.ModTime(),
					Category:      "editor-cached-data",
				})
			}
		}

		logsDir := filepath.Join(appDir, "logs")
		if info, err := os.Stat(logsDir); err == nil && info.IsDir() {
			size, _ := devfs.CalcDirSize(logsDir)
			if size > 0 {
				eligibleBytes := int64(0)
				ttlEligible := false
				if rules.MaxAgeDays == 0 || info.ModTime().Before(cutoff) {
					eligibleBytes = size
					ttlEligible = true
				}
				targets = append(targets, domain.Target{
					ID:            hashPath(logsDir),
					Name:          editorName + ": logs",
					Path:          logsDir,
					SizeBytes:     size,
					EligibleBytes: eligibleBytes,
					TTLEligible:   ttlEligible,
					Risk:          domain.RiskLow,
					LastAccessed:  info.ModTime(),
					Category:      "editor-logs",
				})
			}
		}
	}

	// 2. Extensões obsoletas via .obsolete em ~/.vscode/extensions e ~/.cursor/extensions
	extRoots := []string{
		filepath.Join(home, ".vscode", "extensions"),
		filepath.Join(home, ".cursor", "extensions"),
	}

	for _, extRoot := range extRoots {
		obsoletePath := filepath.Join(extRoot, ".obsolete")
		data, err := os.ReadFile(obsoletePath)
		if err != nil {
			continue
		}

		var obsoleteMap map[string]bool
		if err := json.Unmarshal(data, &obsoleteMap); err != nil {
			continue
		}

		for extFolder, isObsolete := range obsoleteMap {
			if !isObsolete {
				continue
			}
			cleanFolder := filepath.Clean(extFolder)
			if strings.Contains(cleanFolder, "..") || filepath.IsAbs(cleanFolder) {
				continue
			}
			fullExtDir := filepath.Join(extRoot, cleanFolder)
			rel, err := filepath.Rel(extRoot, fullExtDir)
			if err != nil || strings.HasPrefix(rel, "..") || rel == "." {
				continue
			}
			if info, err := os.Stat(fullExtDir); err == nil && info.IsDir() {
				size, _ := devfs.CalcDirSize(fullExtDir)
				eligibleBytes := int64(0)
				ttlEligible := false
				if rules.MaxAgeDays == 0 || info.ModTime().Before(cutoff) {
					eligibleBytes = size
					ttlEligible = true
				}
				targets = append(targets, domain.Target{
					ID:            hashPath(fullExtDir),
					Name:          filepath.Base(extRoot) + ": obsolete extension " + extFolder,
					Path:          fullExtDir,
					SizeBytes:     size,
					EligibleBytes: eligibleBytes,
					TTLEligible:   ttlEligible,
					Risk:          domain.RiskLow,
					LastAccessed:  info.ModTime(),
					Category:      "editor-obsolete-extension",
				})
			}
		}
	}

	return targets, nil
}
