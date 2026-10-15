package cleaner_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/cleaner/cocoapods"
	"github.com/gtkpad/devclean/internal/cleaner/cpp"
	"github.com/gtkpad/devclean/internal/cleaner/elixir"
	"github.com/gtkpad/devclean/internal/cleaner/flutter"
	"github.com/gtkpad/devclean/internal/cleaner/java"
	"github.com/gtkpad/devclean/internal/cleaner/nextjs"
	"github.com/gtkpad/devclean/internal/cleaner/node"
	"github.com/gtkpad/devclean/internal/cleaner/rust"
	"github.com/gtkpad/devclean/internal/cleaner/terraform"
	"github.com/gtkpad/devclean/internal/domain"
)

type projectCleanerFixture struct {
	name         string
	makeCleaner  func(root string) domain.Cleaner
	manifestFile string
	artifactDir  string
	artifactFile string
}

func getProjectCleanerFixtures() []projectCleanerFixture {
	return []projectCleanerFixture{
		{
			name:         "nextjs",
			makeCleaner:  func(root string) domain.Cleaner { return nextjs.New([]string{root}) },
			manifestFile: "package.json",
			artifactDir:  ".next/cache",
			artifactFile: ".next/cache/build.bin",
		},
		{
			name:         "node",
			makeCleaner:  func(root string) domain.Cleaner { return node.New([]string{root}) },
			manifestFile: "package.json",
			artifactDir:  "node_modules",
			artifactFile: "node_modules/pkg/index.js",
		},
		{
			name:         "rust",
			makeCleaner:  func(root string) domain.Cleaner { return rust.New([]string{root}) },
			manifestFile: "Cargo.toml",
			artifactDir:  "target",
			artifactFile: "target/debug/app.bin",
		},
		{
			name:         "java",
			makeCleaner:  func(root string) domain.Cleaner { return java.New([]string{root}) },
			manifestFile: "pom.xml",
			artifactDir:  "target",
			artifactFile: "target/app.jar",
		},
		{
			name:         "cpp",
			makeCleaner:  func(root string) domain.Cleaner { return cpp.New([]string{root}) },
			manifestFile: "CMakeLists.txt",
			artifactDir:  "build",
			artifactFile: "build/app.bin",
		},
		{
			name:         "flutter",
			makeCleaner:  func(root string) domain.Cleaner { return flutter.New([]string{root}) },
			manifestFile: "pubspec.yaml",
			artifactDir:  "build",
			artifactFile: "build/app.apk",
		},
		{
			name:         "terraform",
			makeCleaner:  func(root string) domain.Cleaner { return terraform.New([]string{root}) },
			manifestFile: "main.tf",
			artifactDir:  ".terraform",
			artifactFile: ".terraform/provider.bin",
		},
		{
			name:         "cocoapods",
			makeCleaner:  func(root string) domain.Cleaner { return cocoapods.New([]string{root}) },
			manifestFile: "Podfile",
			artifactDir:  "Pods",
			artifactFile: "Pods/PodA/pod.bin",
		},
		{
			name:         "elixir",
			makeCleaner:  func(root string) domain.Cleaner { return elixir.New([]string{root}) },
			manifestFile: "mix.exs",
			artifactDir:  "_build",
			artifactFile: "_build/dev/app.beam",
		},
	}
}

func setupFixture(t *testing.T, fix projectCleanerFixture) (string, string) {
	t.Helper()
	root := t.TempDir()
	projDir := filepath.Join(root, "my-project")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(projDir, fix.manifestFile), []byte("manifest"), 0o644); err != nil {
		t.Fatal(err)
	}

	fullArtifactFile := filepath.Join(projDir, fix.artifactFile)
	if err := os.MkdirAll(filepath.Dir(fullArtifactFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fullArtifactFile, []byte("artifact content bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	fullArtifactDir := filepath.Join(projDir, fix.artifactDir)
	return root, fullArtifactDir
}

func TestCleaners_RootMtimeRecent_StillReportedInScan(t *testing.T) {
	fixtures := getProjectCleanerFixtures()

	for _, fix := range fixtures {
		t.Run(fix.name, func(t *testing.T) {
			root, artifactDir := setupFixture(t, fix)

			// Touch root and artifactDir to recent time (now)
			now := time.Now()
			_ = os.Chtimes(artifactDir, now, now)

			c := fix.makeCleaner(root)
			rules := domain.EcosystemRules{
				Enabled:    true,
				MaxAgeDays: 14,
			}

			targets, err := c.Scan(context.Background(), rules)
			if err != nil {
				t.Fatalf("%s Scan failed: %v", fix.name, err)
			}

			if len(targets) == 0 {
				t.Fatalf("%s expected at least 1 target reported in scan, got 0", fix.name)
			}

			tgt := targets[0]
			if tgt.SizeBytes <= 0 {
				t.Errorf("%s expected SizeBytes > 0, got %d", fix.name, tgt.SizeBytes)
			}
			if tgt.TTLEligible {
				t.Errorf("%s expected TTLEligible == false for recent target, got true", fix.name)
			}
			if tgt.EligibleBytes != 0 {
				t.Errorf("%s expected EligibleBytes == 0 for recent target, got %d", fix.name, tgt.EligibleBytes)
			}
		})
	}
}

func TestCleaners_MaxAgeZero_AlwaysTTLEligible(t *testing.T) {
	fixtures := getProjectCleanerFixtures()

	for _, fix := range fixtures {
		t.Run(fix.name, func(t *testing.T) {
			root, artifactDir := setupFixture(t, fix)

			// Set to recent time
			now := time.Now()
			_ = os.Chtimes(artifactDir, now, now)

			c := fix.makeCleaner(root)
			rules := domain.EcosystemRules{
				Enabled:    true,
				MaxAgeDays: 0,
			}

			targets, err := c.Scan(context.Background(), rules)
			if err != nil {
				t.Fatalf("%s Scan failed: %v", fix.name, err)
			}

			if len(targets) == 0 {
				t.Fatalf("%s expected at least 1 target when MaxAgeDays=0, got 0", fix.name)
			}

			tgt := targets[0]
			if tgt.SizeBytes <= 0 {
				t.Errorf("%s expected SizeBytes > 0, got %d", fix.name, tgt.SizeBytes)
			}
			if !tgt.TTLEligible {
				t.Errorf("%s expected TTLEligible == true when MaxAgeDays=0", fix.name)
			}
			if tgt.EligibleBytes != tgt.SizeBytes {
				t.Errorf("%s expected EligibleBytes == SizeBytes (%d), got %d", fix.name, tgt.SizeBytes, tgt.EligibleBytes)
			}
		})
	}
}
