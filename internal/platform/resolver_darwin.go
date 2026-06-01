//go:build darwin

package platform

import (
	"os"
	"path/filepath"
)

type darwinResolver struct{}

func New() Resolver { return &darwinResolver{} }

func (r *darwinResolver) Resolve() Paths {
	home, _ := os.UserHomeDir()
	return Paths{
		DockerSocket:   resolveDockerSocket(),
		DockerDataRoot: filepath.Join(home, "Library/Containers/com.docker.docker/Data"),
		GoModCache:     "",
		GoBuildCache:   "",
		DotNetNuGet:    filepath.Join(home, ".nuget/packages"),
		DotNetHTTP:     filepath.Join(home, ".local/share/NuGet/http-cache"),
		DotNetTemp:     filepath.Join(home, ".local/share/NuGet/temp"),
		PythonUVCache:  filepath.Join(home, "Library/Caches/uv"),
		ConfigDir:      filepath.Join(home, ".config/devclean"),
	}
}

func resolveDockerSocket() string {
	if host := os.Getenv("DOCKER_HOST"); host != "" {
		return host
	}
	candidates := []string{
		filepath.Join(os.Getenv("HOME"), ".docker/run/docker.sock"),
		"/var/run/docker.sock",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return "/var/run/docker.sock"
}

func (r *darwinResolver) IsNetworkMount(path string) bool {
	return isNetworkPath(path)
}

func isNetworkPath(path string) bool {
	// /net is automount for NFS on macOS
	if len(path) > 5 && path[:5] == "/net/" {
		return true
	}
	return false
}
