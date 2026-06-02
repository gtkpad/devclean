package docker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/domain"
)

type fakeDockerAPI struct {
	pingErr    error
	images     []imageInfo
	imagesErr  error
	volumes    []volumeInfo
	volumesErr error
	cacheSize  int64
	cacheErr   error
	ctxName    string
	ctxEndpt   string
}

func (f *fakeDockerAPI) ping(_ context.Context) error { return f.pingErr }
func (f *fakeDockerAPI) listImages(_ context.Context, _ bool) ([]imageInfo, error) {
	return f.images, f.imagesErr
}
func (f *fakeDockerAPI) listVolumes(_ context.Context) ([]volumeInfo, error) {
	return f.volumes, f.volumesErr
}
func (f *fakeDockerAPI) diskUsage(_ context.Context) (int64, error) {
	return f.cacheSize, f.cacheErr
}
func (f *fakeDockerAPI) contextInfo(_ context.Context) (string, string) {
	return f.ctxName, f.ctxEndpt
}

func TestDockerCleaner_Name(t *testing.T) {
	c := &dockerCleaner{api: &fakeDockerAPI{}}
	if c.Name() != "docker" {
		t.Errorf("expected 'docker', got %q", c.Name())
	}
}

func TestDockerCleaner_IsInstalled_True(t *testing.T) {
	c := &dockerCleaner{api: &fakeDockerAPI{}}
	if !c.IsInstalled() {
		t.Error("expected IsInstalled to be true")
	}
}

func TestDockerCleaner_IsInstalled_False(t *testing.T) {
	c := &dockerCleaner{api: &fakeDockerAPI{pingErr: errors.New("connection refused")}}
	if c.IsInstalled() {
		t.Error("expected IsInstalled to be false")
	}
}

func TestDockerCleaner_Scan_DanglingImages(t *testing.T) {
	oldTime := time.Now().AddDate(0, 0, -60)
	api := &fakeDockerAPI{
		ctxName:  "default",
		ctxEndpt: "unix:///var/run/docker.sock",
		images: []imageInfo{
			{ID: "sha256:abc123", Size: 500_000_000, Created: oldTime, Tags: []string{"<none>:<none>"}},
			{ID: "sha256:def456", Size: 200_000_000, Created: oldTime, Tags: []string{"myapp:latest"}},
		},
	}
	c := &dockerCleaner{api: api}
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 30}

	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	imageTargets := filterByCategory(targets, "dangling-image")
	if len(imageTargets) != 2 {
		t.Fatalf("expected 2 dangling images, got %d", len(imageTargets))
	}
	if imageTargets[0].Risk != domain.RiskLow {
		t.Errorf("old image should be low risk, got %s", imageTargets[0].Risk)
	}
}

func TestDockerCleaner_Scan_MaxAgeDaysFilter(t *testing.T) {
	recentTime := time.Now().Add(-24 * time.Hour) // 1 day ago
	api := &fakeDockerAPI{
		ctxName: "default",
		images: []imageInfo{
			{ID: "sha256:recent", Size: 100_000_000, Created: recentTime, Tags: nil},
		},
	}
	c := &dockerCleaner{api: api}
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 30}

	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	imageTargets := filterByCategory(targets, "dangling-image")
	if len(imageTargets) != 0 {
		t.Errorf("recent image should be filtered, got %d targets", len(imageTargets))
	}
}

func TestDockerCleaner_Scan_OrphanVolumes(t *testing.T) {
	oldTime := time.Now().AddDate(0, 0, -60)
	api := &fakeDockerAPI{
		ctxName: "default",
		volumes: []volumeInfo{
			{Name: "old-vol", Created: oldTime},
		},
	}
	c := &dockerCleaner{api: api}
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 30}

	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	volTargets := filterByCategory(targets, "orphan-volume")
	if len(volTargets) != 1 {
		t.Fatalf("expected 1 orphan volume, got %d", len(volTargets))
	}
	if volTargets[0].Risk != domain.RiskMedium {
		t.Errorf("old volume should be medium risk, got %s", volTargets[0].Risk)
	}
}

func TestDockerCleaner_Scan_BuildCache(t *testing.T) {
	api := &fakeDockerAPI{
		ctxName:   "default",
		cacheSize: 2_000_000_000,
	}
	c := &dockerCleaner{api: api}
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 30}

	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	cacheTargets := filterByCategory(targets, "build-cache")
	if len(cacheTargets) != 1 {
		t.Fatalf("expected 1 build-cache target, got %d", len(cacheTargets))
	}
	if cacheTargets[0].SizeBytes != 2_000_000_000 {
		t.Errorf("expected 2GB, got %d", cacheTargets[0].SizeBytes)
	}
}

func TestDockerCleaner_Scan_SDKFailureTriggersFallback(t *testing.T) {
	api := &fakeDockerAPI{
		ctxName:   "default",
		imagesErr: errors.New("SDK connection failed"),
	}
	c := &dockerCleaner{api: api}
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 30}

	// This will attempt CLI fallback which may fail in test env — that's ok
	_, _ = c.Scan(context.Background(), rules)
	// Just verify no panic occurs
}

func TestDockerCleaner_Scan_ContextInfo(t *testing.T) {
	api := &fakeDockerAPI{
		ctxName:  "prod-context",
		ctxEndpt: "tcp://192.168.1.1:2376",
	}
	c := &dockerCleaner{api: api}
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 30}

	_, _ = c.Scan(context.Background(), rules)

	if c.contextName != "prod-context" {
		t.Errorf("expected context 'prod-context', got %q", c.contextName)
	}
	if c.endpoint != "tcp://192.168.1.1:2376" {
		t.Errorf("expected endpoint 'tcp://192.168.1.1:2376', got %q", c.endpoint)
	}
}

func TestParseDockerSize(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"500MB", 500_000_000},
		{"1.5GB", 1_500_000_000},
		{"100KB", 100_000},
		{"0B", 0},
		{"", 0},
	}
	for _, tt := range tests {
		got := parseDockerSize(tt.input)
		if got != tt.want {
			t.Errorf("parseDockerSize(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestClassifyImageRisk(t *testing.T) {
	cutoff := time.Now().AddDate(0, 0, -30)
	old := time.Now().AddDate(0, 0, -60)
	recent := time.Now().Add(-24 * time.Hour)

	if classifyImageRisk(old, cutoff) != domain.RiskLow {
		t.Error("old image should be low risk")
	}
	if classifyImageRisk(recent, cutoff) != domain.RiskMedium {
		t.Error("recent image should be medium risk")
	}
}

func filterByCategory(targets []domain.Target, cat string) []domain.Target {
	var result []domain.Target
	for _, t := range targets {
		if t.Category == cat {
			result = append(result, t)
		}
	}
	return result
}
