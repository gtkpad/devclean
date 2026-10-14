package platform

import (
	"os"
	"path/filepath"
	"runtime"
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

func TestResolver_DarwinLogDir(t *testing.T) {
	home := "/Users/testuser"
	got := ResolveDarwinLogDir(home)
	expected := filepath.Join(home, "Library", "Logs", "devclean")
	if got != expected {
		t.Errorf("ResolveDarwinLogDir(%q) = %q, want %q", home, got, expected)
	}

	if runtime.GOOS == "darwin" {
		r := New()
		paths := r.Resolve()
		userHome, _ := os.UserHomeDir()
		want := filepath.Join(userHome, "Library", "Logs", "devclean")
		if paths.LogDir != want {
			t.Errorf("Resolve().LogDir = %q, want %q", paths.LogDir, want)
		}
	}
}

func TestResolver_LinuxLogDirDefault(t *testing.T) {
	home := "/home/testuser"
	got := ResolveLinuxLogDir(home, "")
	expected := filepath.Join(home, ".local", "state", "devclean", "logs")
	if got != expected {
		t.Errorf("ResolveLinuxLogDir(%q, \"\") = %q, want %q", home, got, expected)
	}
}

func TestResolver_LinuxLogDirXDGStateHome(t *testing.T) {
	home := "/home/testuser"
	custom := "/var/custom/state"
	got := ResolveLinuxLogDir(home, custom)
	expected := filepath.Join(custom, "devclean", "logs")
	if got != expected {
		t.Errorf("ResolveLinuxLogDir(%q, %q) = %q, want %q", home, custom, got, expected)
	}
}
