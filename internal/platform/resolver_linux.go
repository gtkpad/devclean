//go:build linux

package platform

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

type linuxResolver struct{}

func New() Resolver { return &linuxResolver{} }

func (r *linuxResolver) Resolve() Paths {
	home, _ := os.UserHomeDir()
	cacheDir := os.Getenv("XDG_CACHE_HOME")
	if cacheDir == "" {
		cacheDir = filepath.Join(home, ".cache")
	}
	return Paths{
		DockerSocket:   resolveDockerSocket(),
		DockerDataRoot: "/var/lib/docker",
		GoModCache:     "",
		GoBuildCache:   "",
		DotNetNuGet:    filepath.Join(home, ".nuget", "packages"),
		DotNetHTTP:     filepath.Join(home, ".local", "share", "NuGet", "http-cache"),
		DotNetTemp:     filepath.Join(home, ".local", "share", "NuGet", "temp"),
		PythonUVCache:  filepath.Join(cacheDir, "uv"),
		ConfigDir:      filepath.Join(home, ".config", "devclean"),
	}
}

func resolveDockerSocket() string {
	if host := os.Getenv("DOCKER_HOST"); host != "" {
		return host
	}
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		sock := filepath.Join(xdg, "docker.sock")
		if _, err := os.Stat(sock); err == nil {
			return sock
		}
	}
	return "/var/run/docker.sock"
}

func (r *linuxResolver) IsNetworkMount(path string) bool {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return false
	}
	defer f.Close()

	networkTypes := map[string]bool{
		"nfs": true, "nfs4": true, "cifs": true,
		"smbfs": true, "fuse.sshfs": true,
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 3 {
			mountPoint := fields[1]
			fsType := fields[2]
			if networkTypes[fsType] && strings.HasPrefix(path, mountPoint) {
				return true
			}
		}
	}
	return false
}
