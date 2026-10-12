package terraform

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/gtkpad/devclean/internal/domain"
	devfs "github.com/gtkpad/devclean/internal/fs"
	"github.com/gtkpad/devclean/internal/safety"
)

type terraformCleaner struct {
	scanPaths []string
}

// New creates a Terraform/OpenTofu ecosystem cleaner implementing domain.Cleaner.
func New(scanPaths []string) domain.Cleaner {
	return &terraformCleaner{scanPaths: scanPaths}
}

// Name returns the ecosystem identifier for Terraform/OpenTofu.
func (c *terraformCleaner) Name() string {
	return "terraform"
}

// IsInstalled performs hybrid detection (D-10): returns true if terraform or tofu
// binary is in PATH, or if any .tf files are found in scanPaths, or if global plugin cache exists.
func (c *terraformCleaner) IsInstalled() bool {
	// 1. Toolchains in PATH
	for _, bin := range []string{"terraform", "tofu"} {
		if _, err := exec.LookPath(bin); err == nil {
			return true
		}
	}

	// 2. Presence of .tf files in scanPaths
	for _, root := range c.scanPaths {
		if hasAnyTfFiles(root) {
			return true
		}
	}

	// 3. Global plugin cache
	home, _ := os.UserHomeDir()
	candidateDirs := []string{}
	if v := os.Getenv("TF_PLUGIN_CACHE_DIR"); v != "" {
		candidateDirs = append(candidateDirs, v)
	}
	if home != "" {
		candidateDirs = append(candidateDirs, filepath.Join(home, ".terraform.d", "plugin-cache"))
	}
	for _, dir := range candidateDirs {
		if _, err := os.Stat(dir); err == nil {
			return true
		}
	}

	return false
}

// Scan discovers eligible .terraform directories and global plugin caches.
func (c *terraformCleaner) Scan(ctx context.Context, rules domain.EcosystemRules) ([]domain.Target, error) {
	var targets []domain.Target

	localTargets, err := c.scanLocalTargets(ctx, rules)
	if err != nil {
		return nil, err
	}
	targets = append(targets, localTargets...)

	globalTargets := c.scanGlobalCaches()
	targets = append(targets, globalTargets...)

	return targets, nil
}

// Clean removes targets using os.RemoveAll.
func (c *terraformCleaner) Clean(_ context.Context, targets []domain.Target) error {
	for _, t := range targets {
		if err := os.RemoveAll(t.Path); err != nil {
			return err
		}
	}
	return nil
}

func (c *terraformCleaner) scanLocalTargets(ctx context.Context, rules domain.EcosystemRules) ([]domain.Target, error) {
	var (
		mu       sync.Mutex
		allPaths []string
		firstErr error
		wg       sync.WaitGroup
		sem      = make(chan struct{}, runtime.NumCPU())
	)

	for _, root := range c.scanPaths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case sem <- struct{}{}:
		}

		wg.Add(1)
		go func(r string) {
			defer wg.Done()
			defer func() { <-sem }()

			paths, err := walkForTerraformTargets(ctx, r, rules)
			mu.Lock()
			defer mu.Unlock()
			if err != nil && firstErr == nil {
				firstErr = err
			}
			allPaths = append(allPaths, paths...)
		}(root)
	}
	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}
	if len(allPaths) == 0 {
		return nil, nil
	}

	// Deduplicate by path
	uniquePaths := make([]string, 0, len(allPaths))
	seen := make(map[string]bool, len(allPaths))
	for _, p := range allPaths {
		if !seen[p] {
			seen[p] = true
			uniquePaths = append(uniquePaths, p)
		}
	}

	sizes, err := devfs.CalcDirSizes(ctx, uniquePaths)
	if err != nil {
		return nil, err
	}

	targets := make([]domain.Target, 0, len(uniquePaths))
	for _, p := range uniquePaths {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}

		parentDir := filepath.Dir(p)
		base := filepath.Base(p)

		hash := sha256.Sum256([]byte(p))
		id := fmt.Sprintf("%x", hash[:6])

		targets = append(targets, domain.Target{
			ID:           id,
			Name:         filepath.Base(parentDir) + "/" + base,
			Path:         p,
			SizeBytes:    sizes[p],
			Risk:         domain.RiskMedium,
			LastAccessed: info.ModTime(),
			Category:     "terraform-local",
		})
	}

	return targets, nil
}

func (c *terraformCleaner) scanGlobalCaches() []domain.Target {
	var targets []domain.Target
	seen := make(map[string]bool)

	home, _ := os.UserHomeDir()

	var pluginPaths []string
	if v := os.Getenv("TF_PLUGIN_CACHE_DIR"); v != "" {
		pluginPaths = append(pluginPaths, v)
	}
	if home != "" {
		pluginPaths = append(pluginPaths, filepath.Join(home, ".terraform.d", "plugin-cache"))
	}

	for _, p := range pluginPaths {
		if seen[p] {
			continue
		}
		seen[p] = true

		if info, err := os.Stat(p); err == nil {
			size, _ := devfs.CalcDirSize(p)
			hash := sha256.Sum256([]byte(p))
			id := fmt.Sprintf("%x", hash[:6])

			targets = append(targets, domain.Target{
				ID:           id,
				Name:         "Terraform Plugin Cache",
				Path:         p,
				SizeBytes:    size,
				Risk:         domain.RiskLow,
				LastAccessed: info.ModTime(),
				Category:     "terraform-plugin-cache",
			})
		}
	}

	return targets
}

func walkForTerraformTargets(ctx context.Context, root string, rules domain.EcosystemRules) ([]string, error) {
	var found []string

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if path == root {
				return err
			}
			return nil
		}

		if err := ctx.Err(); err != nil {
			return err
		}

		if !d.IsDir() {
			return nil
		}

		if d.Type()&os.ModeSymlink != 0 {
			return filepath.SkipDir
		}
		isLink, linkErr := safety.IsSymlink(path)
		if linkErr == nil && isLink {
			return filepath.SkipDir
		}

		if d.Name() == ".git" {
			return filepath.SkipDir
		}

		rel, relErr := filepath.Rel(root, path)
		if relErr == nil && rel != "." {
			for _, pattern := range rules.ExcludePaths {
				matched, _ := doublestar.Match(pattern, rel)
				if matched {
					return filepath.SkipDir
				}
			}
		}

		if d.Name() == ".terraform" {
			parentDir := filepath.Dir(path)
			// Must contain at least one .tf file in parent directory
			if !hasTerraformFiles(parentDir) {
				return filepath.SkipDir
			}

			// Critical safeguards (D-09): skip if active lock or local state is found
			if hasActiveLockOrLocalState(path) {
				return filepath.SkipDir
			}

			found = append(found, path)
			return filepath.SkipDir
		}

		return nil
	})

	return found, err
}

// hasTerraformFiles checks if a directory contains at least one .tf file.
func hasTerraformFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".tf") {
			return true
		}
	}
	return false
}

// hasActiveLockOrLocalState inspects strict safeguards (D-09):
// 1. Returns true if .terraform.tfstate.lock.info exists in parentDir or terraformDir.
// 2. Returns true if terraformDir/terraform.tfstate specifies local backend or lacks backend definition.
// 3. Returns true if parentDir/terraform.tfstate exists and contains managed resources.
func hasActiveLockOrLocalState(terraformDir string) bool {
	parentDir := filepath.Dir(terraformDir)

	// 1. Concurrent active lock (.terraform.tfstate.lock.info)
	lockFiles := []string{
		filepath.Join(parentDir, ".terraform.tfstate.lock.info"),
		filepath.Join(terraformDir, ".terraform.tfstate.lock.info"),
	}
	for _, lf := range lockFiles {
		if _, err := os.Stat(lf); err == nil {
			return true
		}
	}

	// 2. Local backend state in .terraform/terraform.tfstate
	localStatePath := filepath.Join(terraformDir, "terraform.tfstate")
	if data, err := os.ReadFile(localStatePath); err == nil {
		content := string(data)
		if strings.Contains(content, `"type": "local"`) || !strings.Contains(content, `"backend"`) {
			return true
		}
	}

	// 3. Local project state file in project root (terraform.tfstate)
	rootStatePath := filepath.Join(parentDir, "terraform.tfstate")
	if data, err := os.ReadFile(rootStatePath); err == nil {
		content := string(data)
		if strings.Contains(content, `"resources"`) || strings.Contains(content, `"type": "local"`) {
			return true
		}
	}

	return false
}

// hasAnyTfFiles checks recursively if any .tf file exists under root.
func hasAnyTfFiles(root string) bool {
	found := false
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".tf") {
			found = true
			return fs.SkipAll
		}
		return nil
	})
	return found
}
