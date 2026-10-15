package java

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
	devfs "github.com/gtkpad/devclean/internal/fs"
	"github.com/gtkpad/devclean/internal/safety"
)

type javaCleaner struct {
	scanPaths []string
}

// New creates a Java/Kotlin ecosystem cleaner that discovers local build/target directories and global caches.
func New(scanPaths []string) domain.Cleaner {
	return &javaCleaner{scanPaths: scanPaths}
}

func (c *javaCleaner) Name() string { return "java" }

func (c *javaCleaner) IsInstalled() bool {
	// Check for java, gradle or mvn in PATH
	path := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(path) {
		for _, bin := range []string{"gradle", "mvn", "java"} {
			if _, err := os.Stat(filepath.Join(dir, bin)); err == nil {
				return true
			}
		}
	}
	// Fallback: check if gradle/maven cache directories exist on disk
	if devfs.DirExists(gradleHome()) || devfs.DirExists(mavenHome()) {
		return true
	}
	return false
}


func (c *javaCleaner) Scan(ctx context.Context, rules domain.EcosystemRules) ([]domain.Target, error) {
	var targets []domain.Target

	localTargets, err := c.scanLocalTargets(rules)
	if err != nil {
		return nil, err
	}
	targets = append(targets, localTargets...)

	globalTargets := c.scanGlobalCaches(rules)
	targets = append(targets, globalTargets...)

	return targets, nil
}

func (c *javaCleaner) Clean(_ context.Context, targets []domain.Target) error {
	for _, t := range targets {
		if err := os.RemoveAll(t.Path); err != nil {
			return err
		}
	}
	return nil
}

func (c *javaCleaner) scanLocalTargets(rules domain.EcosystemRules) ([]domain.Target, error) {
	var (
		mu       sync.Mutex
		allFound []localTarget
		firstErr error
		wg       sync.WaitGroup
		sem      = make(chan struct{}, runtime.NumCPU())
	)

	for _, root := range c.scanPaths {
		sem <- struct{}{}
		wg.Add(1)
		go func(r string) {
			defer wg.Done()
			defer func() { <-sem }()

			found, err := walkForJavaTargets(r, rules)
			mu.Lock()
			defer mu.Unlock()
			if err != nil && firstErr == nil {
				firstErr = err
			}
			allFound = append(allFound, found...)
		}(root)
	}
	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}
	if len(allFound) == 0 {
		return nil, nil
	}

	paths := make([]string, len(allFound))
	for i, f := range allFound {
		paths[i] = f.path
	}

	sizes, err := devfs.CalcDirSizes(context.Background(), paths)
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)
	targets := make([]domain.Target, 0, len(allFound))
	for _, f := range allFound {
		info, err := os.Stat(f.path)
		if err != nil {
			continue
		}

		hash := sha256.Sum256([]byte(f.path))
		id := fmt.Sprintf("%x", hash[:6])

		size := sizes[f.path]
		ttlEligible := false
		eligibleBytes := int64(0)
		if rules.MaxAgeDays == 0 || info.ModTime().Before(cutoff) {
			ttlEligible = true
			eligibleBytes = size
		}

		targets = append(targets, domain.Target{
			ID:            id,
			Name:          filepath.Base(filepath.Dir(f.path)) + "/" + filepath.Base(f.path),
			Path:          f.path,
			SizeBytes:     size,
			EligibleBytes: eligibleBytes,
			TTLEligible:   ttlEligible,
			Risk:          cleaner.ClassifyLocalArtifactRisk(f.path, f.manifests, rules.MaxAgeDays),
			LastAccessed:  info.ModTime(),
			Category:      f.category,
		})
	}

	return targets, nil
}

type localTarget struct {
	path      string
	manifests []string
	category  string
}

func (c *javaCleaner) scanGlobalCaches(rules domain.EcosystemRules) []domain.Target {
	var targets []domain.Target
	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)

	caches := []struct {
		name     string
		path     string
		category string
	}{
		{"gradle caches", filepath.Join(gradleHome(), "caches"), "gradle-cache"},
		{"gradle wrapper", filepath.Join(gradleHome(), "wrapper", "dists"), "gradle-wrapper"},
		{"maven repository", filepath.Join(mavenHome(), "repository"), "maven-repo"},
	}

	for _, cache := range caches {
		info, err := os.Stat(cache.path)
		if err != nil {
			continue
		}

		size, _ := devfs.CalcDirSize(cache.path)
		hash := sha256.Sum256([]byte(cache.path))
		id := fmt.Sprintf("%x", hash[:6])

		ttlEligible := false
		eligibleBytes := int64(0)
		if rules.MaxAgeDays == 0 || info.ModTime().Before(cutoff) {
			ttlEligible = true
			eligibleBytes = size
		}

		targets = append(targets, domain.Target{
			ID:            id,
			Name:          cache.name,
			Path:          cache.path,
			SizeBytes:     size,
			EligibleBytes: eligibleBytes,
			TTLEligible:   ttlEligible,
			Risk:          domain.RiskLow,
			LastAccessed:  info.ModTime(),
			Category:      cache.category,
		})
	}

	return targets
}

func walkForJavaTargets(root string, rules domain.EcosystemRules) ([]localTarget, error) {
	var found []localTarget

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if path == root {
				return err
			}
			return nil
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

		rel, relErr := filepath.Rel(root, path)
		if relErr == nil {
			for _, pattern := range rules.ExcludePaths {
				matched, _ := doublestar.Match(pattern, rel)
				if matched {
					return filepath.SkipDir
				}
			}
		}

		parentDir := filepath.Dir(path)

		if d.Name() == "build" {
			// Gradle: build/ with sibling build.gradle or build.gradle.kts
			gradleFile := filepath.Join(parentDir, "build.gradle")
			gradleKtsFile := filepath.Join(parentDir, "build.gradle.kts")
			if _, err := os.Stat(gradleFile); err == nil {
				found = append(found, localTarget{
					path:      path,
					manifests: []string{gradleFile},
					category:  "gradle-build",
				})
			} else if _, err := os.Stat(gradleKtsFile); err == nil {
				found = append(found, localTarget{
					path:      path,
					manifests: []string{gradleKtsFile},
					category:  "gradle-build",
				})
			}
			return filepath.SkipDir
		}

		if d.Name() == "target" {
			pomFile := filepath.Join(parentDir, "pom.xml")
			cargoFile := filepath.Join(parentDir, "Cargo.toml")
			// Maven: target/ with pom.xml sibling AND no Cargo.toml (disambiguation)
			if _, err := os.Stat(pomFile); err == nil {
				if _, err := os.Stat(cargoFile); err != nil {
					// No Cargo.toml → Java owns this target/
					found = append(found, localTarget{
						path:      path,
						manifests: []string{pomFile},
						category:  "maven-target",
					})
				}
			}
			return filepath.SkipDir
		}

		return nil
	})

	return found, err
}

func gradleHome() string {
	if v := os.Getenv("GRADLE_USER_HOME"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".gradle")
}

func mavenHome() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".m2")
}
