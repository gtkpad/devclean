package platform

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
}

// Resolver provides platform-specific path resolution.
type Resolver interface {
	Resolve() Paths
	IsNetworkMount(path string) bool
}
