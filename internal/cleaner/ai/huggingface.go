package ai

import (
	"os"
	"path/filepath"
	"time"

	"github.com/gtkpad/devclean/internal/domain"
	devfs "github.com/gtkpad/devclean/internal/fs"
)

func scanHuggingFace(hfRoot string, rules domain.EcosystemRules) ([]domain.Target, error) {
	cleanHFRoot := filepath.Clean(hfRoot)
	if _, err := os.Stat(cleanHFRoot); os.IsNotExist(err) {
		return nil, nil
	}

	var targets []domain.Target
	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)

	subdirs := []struct {
		folder   string
		category string
		label    string
	}{
		{"hub", "ai-huggingface-hub", "HuggingFace Hub snapshot"},
		{"datasets", "ai-huggingface-datasets", "HuggingFace dataset cache"},
	}

	for _, s := range subdirs {
		targetDir := filepath.Join(cleanHFRoot, s.folder)
		entries, err := os.ReadDir(targetDir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			itemPath := filepath.Join(targetDir, entry.Name())
			info, err := entry.Info()
			if err != nil {
				continue
			}

			if rules.MaxAgeDays > 0 && info.ModTime().After(cutoff) {
				continue
			}

			size, _ := devfs.CalcDirSize(itemPath)
			if size > 0 {
				targets = append(targets, domain.Target{
					ID:           hashPath(itemPath),
					Name:         s.label + ": " + entry.Name(),
					Path:         itemPath,
					SizeBytes:    size,
					Risk:         domain.RiskMedium, // D-03: Re-download possível sob demanda
					LastAccessed: info.ModTime(),
					Category:     s.category,
				})
			}
		}
	}

	return targets, nil
}
