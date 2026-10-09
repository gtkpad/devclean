package ai

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gtkpad/devclean/internal/domain"
	devfs "github.com/gtkpad/devclean/internal/fs"
)

type ollamaManifest struct {
	Config struct {
		Digest string `json:"digest"`
	} `json:"config"`
	Layers []struct {
		Digest string `json:"digest"`
	} `json:"layers"`
}

func scanOllama(modelsDir string, rules domain.EcosystemRules) ([]domain.Target, error) {
	cleanModelsDir := filepath.Clean(modelsDir)
	if _, err := os.Stat(cleanModelsDir); os.IsNotExist(err) {
		return nil, nil
	}

	manifestsDir := filepath.Join(cleanModelsDir, "manifests")
	blobsDir := filepath.Join(cleanModelsDir, "blobs")

	validDigests := make(map[string]bool)
	var targets []domain.Target
	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)

	// 1. Ler manifests válidos
	_ = filepath.Walk(manifestsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil || len(data) == 0 {
			// Manifesto corrompido
			targets = append(targets, domain.Target{
				ID:           hashPath(path),
				Name:         "Ollama: corrupted manifest " + filepath.Base(path),
				Path:         path,
				SizeBytes:    info.Size(),
				Risk:         domain.RiskMedium,
				LastAccessed: info.ModTime(),
				Category:     "ai-ollama-corrupted",
			})
			return nil
		}

		var mf ollamaManifest
		if err := json.Unmarshal(data, &mf); err != nil {
			targets = append(targets, domain.Target{
				ID:           hashPath(path),
				Name:         "Ollama: invalid manifest JSON " + filepath.Base(path),
				Path:         path,
				SizeBytes:    info.Size(),
				Risk:         domain.RiskMedium,
				LastAccessed: info.ModTime(),
				Category:     "ai-ollama-corrupted",
			})
			return nil
		}

		if mf.Config.Digest != "" {
			validDigests[digestToFilename(mf.Config.Digest)] = true
		}
		for _, l := range mf.Layers {
			if l.Digest != "" {
				validDigests[digestToFilename(l.Digest)] = true
			}
		}

		// Modelos íntegros elegíveis APENAS se ultrapassarem MaxAgeDays (D-01)
		if rules.MaxAgeDays > 0 && info.ModTime().Before(cutoff) {
			modelSize, _ := devfs.CalcDirSize(filepath.Dir(path))
			if modelSize == 0 {
				modelSize = info.Size()
			}
			targets = append(targets, domain.Target{
				ID:           hashPath(path),
				Name:         "Ollama model (stale): " + filepath.Base(path),
				Path:         path,
				SizeBytes:    modelSize,
				Risk:         domain.RiskHigh, // D-01: Modelos intactos exigem confirmação explícita
				LastAccessed: info.ModTime(),
				Category:     "ai-ollama-model-stale",
			})
		}

		return nil
	})

	// 2. Identificar blobs órfãos em blobs/
	entries, err := os.ReadDir(blobsDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			fname := entry.Name()
			if !validDigests[fname] {
				blobPath := filepath.Join(blobsDir, fname)
				info, err := entry.Info()
				if err != nil {
					continue
				}

				targets = append(targets, domain.Target{
					ID:           hashPath(blobPath),
					Name:         "Ollama orphan blob: " + fname[:min(len(fname), 16)],
					Path:         blobPath,
					SizeBytes:    info.Size(),
					Risk:         domain.RiskLow,
					LastAccessed: info.ModTime(),
					Category:     "ai-ollama-orphan",
				})
			}
		}
	}

	return targets, nil
}

func digestToFilename(digest string) string {
	return strings.ReplaceAll(digest, ":", "-")
}

func hashPath(path string) string {
	sum := sha256.Sum256([]byte(path))
	return fmt.Sprintf("%x", sum[:6])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
