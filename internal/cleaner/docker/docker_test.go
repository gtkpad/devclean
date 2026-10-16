package docker_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/cleaner/docker"
	"github.com/gtkpad/devclean/internal/domain"
)

func TestDockerCleaner_SystemDF_CategorizedReviewOnly(t *testing.T) {
	mockExec := cleaner.NewMockExecutor()

	dfOutput := `{"Type":"Images","TotalCount":5,"Active":2,"Size":"2.4GB","Reclaimable":"1.1GB (45%)"}
{"Type":"Containers","TotalCount":"3","Active":"1","Size":"120MB","Reclaimable":"40MB (33%)"}
{"Type":"Local Volumes","TotalCount":4,"Active":"2","Size":"500MB","Reclaimable":"200MB (40%)"}
{"Type":"Build Cache","TotalCount":"18","Active":0,"Size":"1.5GB","Reclaimable":"1.5GB (100%)"}
`
	mockExec.Outputs["docker system df --format {{json .}}"] = []byte(dfOutput)

	c := docker.NewWithExecutor(mockExec)

	targets, err := c.Scan(context.Background(), domain.EcosystemRules{})
	if err != nil {
		t.Fatalf("unexpected Scan error: %v", err)
	}

	if len(targets) != 4 {
		t.Fatalf("expected 4 targets, got %d", len(targets))
	}

	expected := map[string]struct {
		id          string
		path        string
		category    string
		total       int
		active      int
		size        int64
		reclaimable int64
	}{
		"docker-images": {
			id:          "docker-images",
			path:        "docker://system-df/images",
			category:    "docker-images",
			total:       5,
			active:      2,
			size:        2_400_000_000,
			reclaimable: 1_100_000_000,
		},
		"docker-containers": {
			id:          "docker-containers",
			path:        "docker://system-df/containers",
			category:    "docker-containers",
			total:       3,
			active:      1,
			size:        120_000_000,
			reclaimable: 40_000_000,
		},
		"docker-volumes": {
			id:          "docker-volumes",
			path:        "docker://system-df/volumes",
			category:    "docker-volumes",
			total:       4,
			active:      2,
			size:        500_000_000,
			reclaimable: 200_000_000,
		},
		"docker-build-cache": {
			id:          "docker-build-cache",
			path:        "docker://system-df/build-cache",
			category:    "docker-build-cache",
			total:       18,
			active:      0,
			size:        1_500_000_000,
			reclaimable: 1_500_000_000,
		},
	}

	for _, tgt := range targets {
		exp, ok := expected[tgt.ID]
		if !ok {
			t.Errorf("unexpected target ID: %q", tgt.ID)
			continue
		}

		if tgt.Path != exp.path {
			t.Errorf("target %s: expected Path %q, got %q", tgt.ID, exp.path, tgt.Path)
		}
		if tgt.Category != exp.category {
			t.Errorf("target %s: expected Category %q, got %q", tgt.ID, exp.category, tgt.Category)
		}
		if !tgt.ReviewOnly {
			t.Errorf("target %s: expected ReviewOnly=true", tgt.ID)
		}
		if tgt.ReviewReason != domain.ReviewReasonSharedStore {
			t.Errorf("target %s: expected ReviewReason=%q, got %q", tgt.ID, domain.ReviewReasonSharedStore, tgt.ReviewReason)
		}
		if tgt.EligibleBytes != 0 {
			t.Errorf("target %s: expected EligibleBytes=0, got %d", tgt.ID, tgt.EligibleBytes)
		}
		if tgt.TTLEligible {
			t.Errorf("target %s: expected TTLEligible=false", tgt.ID)
		}
		if tgt.SizeBytes != exp.size {
			t.Errorf("target %s: expected SizeBytes=%d, got %d", tgt.ID, exp.size, tgt.SizeBytes)
		}
		if tgt.ProviderMetrics == nil {
			t.Fatalf("target %s: ProviderMetrics is nil", tgt.ID)
		}
		if tgt.ProviderMetrics.TotalCount != exp.total {
			t.Errorf("target %s: expected TotalCount=%d, got %d", tgt.ID, exp.total, tgt.ProviderMetrics.TotalCount)
		}
		if tgt.ProviderMetrics.ActiveCount != exp.active {
			t.Errorf("target %s: expected ActiveCount=%d, got %d", tgt.ID, exp.active, tgt.ProviderMetrics.ActiveCount)
		}
		if tgt.ProviderMetrics.SizeBytes != exp.size {
			t.Errorf("target %s: expected ProviderMetrics.SizeBytes=%d, got %d", tgt.ID, exp.size, tgt.ProviderMetrics.SizeBytes)
		}
		if tgt.ProviderMetrics.ReclaimableBytes != exp.reclaimable {
			t.Errorf("target %s: expected ReclaimableBytes=%d, got %d", tgt.ID, exp.reclaimable, tgt.ProviderMetrics.ReclaimableBytes)
		}
		if !strings.HasPrefix(tgt.Name, "Docker ") {
			t.Errorf("target %s: expected Name to start with 'Docker ', got %q", tgt.ID, tgt.Name)
		}
	}

	// Verify command was called without shell interpolation
	if len(mockExec.Calls) != 1 {
		t.Fatalf("expected 1 exec call, got %d", len(mockExec.Calls))
	}
	expectedCall := []string{"docker", "system", "df", "--format", "{{json .}}"}
	call := mockExec.Calls[0]
	if len(call) != len(expectedCall) {
		t.Fatalf("expected call %v, got %v", expectedCall, call)
	}
	for i := range expectedCall {
		if call[i] != expectedCall[i] {
			t.Errorf("call arg %d: expected %q, got %q", i, expectedCall[i], call[i])
		}
	}
}

func TestDockerCleaner_Name(t *testing.T) {
	c := docker.New()
	if c.Name() != "docker" {
		t.Errorf("expected 'docker', got %q", c.Name())
	}
}

func TestDockerCleaner_DefaultConstructor(t *testing.T) {
	c := docker.New()
	if c == nil {
		t.Fatal("expected non-nil Cleaner from New()")
	}
}

func TestDockerCleaner_DaemonOffline_StatusTarget(t *testing.T) {
	mockExec := cleaner.NewMockExecutor()
	mockExec.Errors["docker system df --format {{json .}}"] = errors.New("Cannot connect to the Docker daemon at unix:///var/run/docker.sock")
	mockExec.Outputs["lookpath:docker"] = []byte("/usr/local/bin/docker")

	c := docker.NewWithExecutor(mockExec)
	targets, err := c.Scan(context.Background(), domain.EcosystemRules{})
	if err != nil {
		t.Fatalf("Scan returned unexpected error: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("expected 1 target on daemon offline, got %d", len(targets))
	}
	tgt := targets[0]
	if tgt.ID != "docker-daemon-status" {
		t.Errorf("expected ID 'docker-daemon-status', got %q", tgt.ID)
	}
	if tgt.Path != "docker://daemon/status" {
		t.Errorf("expected Path 'docker://daemon/status', got %q", tgt.Path)
	}
	if !tgt.ReviewOnly {
		t.Error("expected ReviewOnly == true")
	}
	if tgt.ReviewReason != domain.ReviewReasonAmbiguous {
		t.Errorf("expected ReviewReason %q, got %q", domain.ReviewReasonAmbiguous, tgt.ReviewReason)
	}
	if tgt.EligibleBytes != 0 {
		t.Errorf("expected EligibleBytes == 0, got %d", tgt.EligibleBytes)
	}
	if tgt.ProviderMetrics == nil || !tgt.ProviderMetrics.Partial {
		t.Error("expected ProviderMetrics.Partial == true")
	}
}

func TestDockerCleaner_MalformedJSON_PartialTolerance(t *testing.T) {
	mockExec := cleaner.NewMockExecutor()
	dfOutput := `{"Type":"Images","TotalCount":5,"Active":2,"Size":"2.4GB","Reclaimable":"1.1GB (45%)"}
{"Type": "broken json line...
`
	mockExec.Outputs["docker system df --format {{json .}}"] = []byte(dfOutput)

	c := docker.NewWithExecutor(mockExec)
	targets, err := c.Scan(context.Background(), domain.EcosystemRules{})
	if err != nil {
		t.Fatalf("unexpected Scan error: %v", err)
	}

	if len(targets) != 2 {
		t.Fatalf("expected 2 targets (1 valid + 1 parse error), got %d", len(targets))
	}

	var hasImage, hasParseError bool
	for _, tgt := range targets {
		if tgt.ID == "docker-images" {
			hasImage = true
		}
		if tgt.ID == "docker-daemon-parse-error" {
			hasParseError = true
			if !tgt.ReviewOnly {
				t.Error("expected ReviewOnly == true for parse error")
			}
			if tgt.ReviewReason != domain.ReviewReasonAmbiguous {
				t.Errorf("expected ReviewReason %q, got %q", domain.ReviewReasonAmbiguous, tgt.ReviewReason)
			}
			if tgt.ProviderMetrics == nil || !tgt.ProviderMetrics.Partial {
				t.Error("expected ProviderMetrics.Partial == true")
			}
		}
	}
	if !hasImage {
		t.Error("expected docker-images target to be parsed successfully")
	}
	if !hasParseError {
		t.Error("expected docker-daemon-parse-error target")
	}
}

func TestDockerCleaner_IsInstalled_HybridDiscovery(t *testing.T) {
	t.Run("CLI in PATH", func(t *testing.T) {
		mockExec := cleaner.NewMockExecutor()
		mockExec.Outputs["lookpath:docker"] = []byte("/usr/local/bin/docker")
		c := docker.NewWithExecutor(mockExec)
		if !c.IsInstalled() {
			t.Error("expected IsInstalled == true when docker is in PATH")
		}
	})

	t.Run("Disk directory present without CLI in PATH", func(t *testing.T) {
		fakeHome := t.TempDir()
		t.Setenv("HOME", fakeHome)
		dotDocker := filepath.Join(fakeHome, ".docker")
		if err := os.MkdirAll(dotDocker, 0755); err != nil {
			t.Fatal(err)
		}

		mockExec := cleaner.NewMockExecutor()
		mockExec.Errors["lookpath:docker"] = errors.New("not found")
		mockExec.Errors["lookpath:orb"] = errors.New("not found")

		c := docker.NewWithExecutor(mockExec)
		if !c.IsInstalled() {
			t.Error("expected IsInstalled == true when ~/.docker exists on disk")
		}
	})

	t.Run("Neither CLI nor disk candidate", func(t *testing.T) {
		fakeHome := t.TempDir()
		t.Setenv("HOME", fakeHome)

		mockExec := cleaner.NewMockExecutor()
		mockExec.Errors["lookpath:docker"] = errors.New("not found")
		mockExec.Errors["lookpath:orb"] = errors.New("not found")

		c := docker.NewWithExecutor(mockExec)
		if c.IsInstalled() {
			t.Error("expected IsInstalled == false when neither CLI nor candidate dirs exist")
		}
	})
}


