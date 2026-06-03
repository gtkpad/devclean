package cleaner

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/domain"
)

func TestClassifyLocalArtifactRisk_NoManifests(t *testing.T) {
	tmp := t.TempDir()
	artifactDir := filepath.Join(tmp, "target")
	os.MkdirAll(artifactDir, 0o755)

	risk := ClassifyLocalArtifactRisk(artifactDir, []string{
		filepath.Join(tmp, "Cargo.toml"),
		filepath.Join(tmp, "Cargo.lock"),
	}, 30)

	if risk != domain.RiskLow {
		t.Errorf("expected RiskLow for no manifests, got %s", risk)
	}
}

func TestClassifyLocalArtifactRisk_RecentArtifactAndManifest(t *testing.T) {
	tmp := t.TempDir()
	artifactDir := filepath.Join(tmp, "target")
	os.MkdirAll(artifactDir, 0o755)

	manifest := filepath.Join(tmp, "Cargo.toml")
	os.WriteFile(manifest, []byte("[package]"), 0o644)

	risk := ClassifyLocalArtifactRisk(artifactDir, []string{manifest}, 30)

	if risk != domain.RiskHigh {
		t.Errorf("expected RiskHigh for recent artifact+manifest, got %s", risk)
	}
}

func TestClassifyLocalArtifactRisk_OldArtifactAndManifest(t *testing.T) {
	tmp := t.TempDir()
	artifactDir := filepath.Join(tmp, "target")
	os.MkdirAll(artifactDir, 0o755)

	manifest := filepath.Join(tmp, "Cargo.toml")
	os.WriteFile(manifest, []byte("[package]"), 0o644)

	oldTime := time.Now().AddDate(0, 0, -60)
	os.Chtimes(artifactDir, oldTime, oldTime)
	os.Chtimes(manifest, oldTime, oldTime)

	risk := ClassifyLocalArtifactRisk(artifactDir, []string{manifest}, 30)

	if risk != domain.RiskLow {
		t.Errorf("expected RiskLow for old artifact+manifest, got %s", risk)
	}
}

func TestClassifyLocalArtifactRisk_OldArtifactRecentManifest(t *testing.T) {
	tmp := t.TempDir()
	artifactDir := filepath.Join(tmp, "target")
	os.MkdirAll(artifactDir, 0o755)

	manifest := filepath.Join(tmp, "Cargo.toml")
	os.WriteFile(manifest, []byte("[package]"), 0o644)

	oldTime := time.Now().AddDate(0, 0, -60)
	os.Chtimes(artifactDir, oldTime, oldTime)
	// manifest stays recent (just created)

	risk := ClassifyLocalArtifactRisk(artifactDir, []string{manifest}, 30)

	if risk != domain.RiskHigh {
		t.Errorf("expected RiskHigh for old artifact but recent manifest, got %s", risk)
	}
}

func TestClassifyLocalArtifactRisk_ArtifactNotExist(t *testing.T) {
	tmp := t.TempDir()
	risk := ClassifyLocalArtifactRisk(filepath.Join(tmp, "nonexistent"), []string{}, 30)

	if risk != domain.RiskLow {
		t.Errorf("expected RiskLow for nonexistent artifact, got %s", risk)
	}
}
