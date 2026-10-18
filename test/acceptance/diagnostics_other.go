//go:build acceptance && !darwin

package acceptance

import "runtime"

func probeOSVersion() string {
	return runtime.GOOS
}

func probeAPFS(path string) (bool, string) {
	return false, "non-darwin"
}
