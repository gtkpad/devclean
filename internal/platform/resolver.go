package platform

import (
	"path/filepath"
)

// Paths holds resolved filesystem paths for the current platform.
type Paths struct {
	DockerSocket   string
	DockerDataRoot string
	GoModCache     string
	GoBuildCache   string
	DotNetNuGet    string
	DotNetHTTP     string
	DotNetTemp     string
	PythonUVCache  string
	ConfigDir      string
	LogDir         string
}

// Resolver provides platform-specific path resolution.
type Resolver interface {
	Resolve() Paths
	IsNetworkMount(path string) bool
}

// ResolveDarwinLogDir returns canonical log dir for Darwin (~/Library/Logs/devclean).
func ResolveDarwinLogDir(home string) string {
	return filepath.Clean(filepath.Join(home, "Library", "Logs", "devclean"))
}

// ResolveLinuxLogDir returns canonical log dir for Linux (~/.local/state/devclean/logs or $XDG_STATE_HOME/devclean/logs).
func ResolveLinuxLogDir(home string, xdgStateHome string) string {
	stateDir := xdgStateHome
	if stateDir == "" {
		stateDir = filepath.Join(home, ".local", "state")
	}
	return filepath.Clean(filepath.Join(stateDir, "devclean", "logs"))
}
