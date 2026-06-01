package platform

import (
	"os"
	"testing"
)

func TestNew_ReturnsNonNil(t *testing.T) {
	r := New()
	if r == nil {
		t.Fatal("New() returned nil")
	}
}

func TestResolve_ConfigDirNotEmpty(t *testing.T) {
	r := New()
	paths := r.Resolve()

	if paths.ConfigDir == "" {
		t.Error("ConfigDir should not be empty")
	}
}

func TestResolve_PathsContainHomeDir(t *testing.T) {
	r := New()
	paths := r.Resolve()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot determine home dir: %v", err)
	}

	if len(paths.ConfigDir) < len(home) {
		t.Errorf("ConfigDir %q should be under home %q", paths.ConfigDir, home)
	}
}

func TestIsNetworkMount_LocalPath(t *testing.T) {
	r := New()
	if r.IsNetworkMount("/tmp") {
		t.Error("IsNetworkMount(/tmp) should be false")
	}
}

func TestResolve_DockerSocket_RespectsEnv(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix:///custom/docker.sock")
	r := New()
	paths := r.Resolve()

	if paths.DockerSocket != "unix:///custom/docker.sock" {
		t.Errorf("DockerSocket = %q, want unix:///custom/docker.sock", paths.DockerSocket)
	}
}
